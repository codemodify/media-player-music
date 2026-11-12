// Package e2e runs media-player-musicd as a program, in folders of its
// own, with a sound system in memory (nothing is heard), and drives it as
// its renderers do: two at once, through internal/link.
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/link"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/app-plugin/apptest"
)

// daemon starts media-player-musicd (built by apptest.Build) in the test's
// folders, its sound in memory, its WebSocket on the port given, and
// returns once it listens (READY=1); it is stopped at the test's end. It
// returns its Unix socket's and WebSocket's addresses.
func daemon(t *testing.T, binary string, places *apptest.Places, port string) (socket, websocket string, process *apptest.Process) {
	t.Helper()
	socket = "unix://" + filepath.ToSlash(filepath.Join(places.Runtime, "music.sock"))
	websocket = "ws://" + port + "/clients"
	places.Set("MEDIA_PLAYER_MUSICD_LISTEN", socket+","+websocket)
	places.Set("MEDIA_PLAYER_MUSICD_SOUND", "memory")
	process = apptest.Start(t, binary, apptest.Options{Env: places.Environ(), Addresses: []string{socket, port}})
	return socket, websocket, process
}

// readToken reads media-player-musicd.token, which the daemon made in the
// test's runtime folder: the token its WebSocket's clients present.
func readToken(t *testing.T, places *apptest.Places) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(places.Runtime, musicapi.Name, musicapi.TokenFile))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

// renderer is a renderer's side: the player's API and the last states the
// daemon told it.
type renderer struct {
	player musicapi.Player
	lock   sync.Mutex
	state  musicapi.State
	queue  musicapi.Queue
	now    musicapi.Now
}

func connect(t *testing.T, ctx context.Context, address, token, name string) *renderer {
	made := &renderer{}
	made.player = link.Start(ctx, link.Options{
		Address: address, Name: name, Version: "test", Token: token,
		State:  func(state musicapi.State) { made.lock.Lock(); made.state = state; made.lock.Unlock() },
		Queue:  func(queue musicapi.Queue) { made.lock.Lock(); made.queue = queue; made.lock.Unlock() },
		Now:    func(now musicapi.Now) { made.lock.Lock(); made.now = now; made.lock.Unlock() },
		Failed: func(err error) { t.Errorf("%s: %v", name, err) },
	})
	return made
}

// wait waits for what a renderer was told to come true.
func (thisRef *renderer) wait(t *testing.T, what string, condition func(musicapi.State, musicapi.Queue, musicapi.Now) bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		thisRef.lock.Lock()
		state, queue, now := thisRef.state, thisRef.queue, thisRef.now
		thisRef.lock.Unlock()
		if condition(state, queue, now) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not come true: %+v %+v %+v", what, state, queue, now)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// music copies the engine's test MP3 (one second, 22 kHz, mono) into a
// folder as the files named.
func music(t *testing.T, names ...string) []string {
	t.Helper()
	audio, err := os.ReadFile(filepath.Join("..", "internal", "playback", "testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var paths []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, audio, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestTwoRenderersOnePlayer(t *testing.T) {
	binary, places, port := apptest.Build(t, "../cmd/media-player-musicd"), apptest.NewPlaces(t), apptest.Address(t)
	socket, websocket, first := daemon(t, binary, places, port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	desktop := connect(t, ctx, socket, "", "desktop")              // as the GUI and the TUI dial it: no token
	web := connect(t, ctx, websocket, readToken(t, places), "web") // as the web UI's server passes the page on
	desktop.wait(t, "two renderers", func(s musicapi.State, _ musicapi.Queue, _ musicapi.Now) bool { return s.Clients == 2 })

	// a queue given in one plays, and shows in the other
	paths := music(t, "01 one.mp3", "02 two.mp3")
	added, err := web.player.Add(ctx, musicapi.Add{Paths: paths, Replace: true})
	if err != nil || added.Tracks != 2 {
		t.Fatalf("Add: %+v %v", added, err)
	}
	desktop.wait(t, "the desktop sees the queue play", func(s musicapi.State, q musicapi.Queue, n musicapi.Now) bool {
		return s.Status == musicapi.StatusPlaying && len(q.Tracks) == 2 && q.Tracks[0].Title == "01 one" && n.PositionMS > 200
	})
	// the levels and the equaliser set in one show in the other
	if _, err := desktop.player.SetLevels(ctx, musicapi.Levels{Volume: .5, Balance: .25}); err != nil {
		t.Fatal(err)
	}
	if _, err := desktop.player.SetEqualizer(ctx, musicapi.Equalizer{On: true, Preset: "Vocal"}); err != nil {
		t.Fatal(err)
	}
	web.wait(t, "the web sees the levels", func(s musicapi.State, _ musicapi.Queue, _ musicapi.Now) bool {
		return s.Volume == .5 && s.Balance == .25 && s.Equalizer.On && s.Equalizer.Preset == "Vocal"
	})
	// the queue plays to its end, a second a track, and stops at its start
	// (repeat off)
	web.wait(t, "the queue's end", func(s musicapi.State, _ musicapi.Queue, _ musicapi.Now) bool {
		return s.Status == musicapi.StatusStopped && s.Index == 0 && s.Track.LengthMS > 900 && s.Track.Rate == 22050
	})
	// what is kept, kept across a restart; the renderers connect again by themselves
	if _, err := web.player.Play(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := web.player.Command(ctx, musicapi.CommandStop); err != nil {
		t.Fatal(err)
	}
	if err := first.Stop(); err != nil {
		t.Fatalf("stopping the daemon: %v", err)
	}
	desktop.wait(t, "the desktop's connection ends", func(musicapi.State, musicapi.Queue, musicapi.Now) bool {
		_, err := desktop.player.State(ctx)
		return err != nil
	})
	daemon(t, binary, places, port) // the same folders, the same port
	deadline := time.Now().Add(30 * time.Second)
	for {
		state, err := web.player.State(ctx) // through the new daemon, once the web connected again
		if err == nil {
			if state.Queue.Length != 2 || state.Index != 1 || state.Status != musicapi.StatusStopped || state.Volume != .5 || state.Equalizer.Preset != "Vocal" {
				t.Fatalf("after the restart: %+v", state)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the web did not connect again: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// a WebSocket without the token is refused
	stranger := link.Start(ctx, link.Options{Address: websocket, Name: "stranger", Version: "test", Token: "not the token"})
	time.Sleep(500 * time.Millisecond)
	if _, err := stranger.State(ctx); err == nil {
		t.Fatal("a WebSocket client without the token came in")
	}
}
