package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/tags/tagstest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
	all "goframework.io/system-audio-all"
	"goframework.io/system-audio-spec/audiotest"
)

// The shell with the real built-in engine, playing into audiotest's sound
// system in memory: no device, nothing heard on the host.
func TestBuiltinEnginePlaysTheQueueThroughTheShell(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(players.DefaultSkin))
	audio, err := os.ReadFile(filepath.Join("..", "playback", "testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var paths []string
	for _, title := range []string{"First Light", "Second Wind"} {
		path := filepath.Join(dir, title+".mp3")
		tag := tagstest.V2(4, tagstest.Frame{ID: "TIT2", Data: tagstest.Text(4, title)},
			tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(4, "REPLAYGAIN_TRACK_GAIN", "-6 dB")})
		if err := os.WriteFile(path, bytes.Join([][]byte{tag, audio}, nil), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	system := audiotest.New()
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	s, err := New(a, Options{Headless: true, Engine: playback.NewBuiltin(playback.BuiltinOptions{System: all.Wrap(system)})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.SetReplayGain(players.ReplayGain{Mode: players.ReplayGainTrack})
	if err := s.LoadFiles(paths, false); err != nil {
		t.Fatal(err)
	}
	transport := s.Host.Model.Transport
	if transport.Track().Title != "First Light" {
		t.Fatalf("the library did not read the first file's tag: %+v", transport.Track())
	}
	moved := false
	deadline := time.Now().Add(20 * time.Second)
	for transport.State == players.Playing {
		a.PumpOnce()
		if s.Host.Model.Error != "" {
			t.Fatalf("an error: %s", s.Host.Model.Error)
		}
		if transport.Pos > 300*time.Millisecond {
			moved = true
		}
		if time.Now().After(deadline) {
			t.Fatalf("still playing track %d at %v", transport.List.Index(), transport.Pos)
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Repeat off: the queue played through, and stopped at its end.
	if !moved || transport.State != players.Stopped {
		t.Fatalf("the clock did not move (%v) or the queue did not stop (%s)", moved, transport.State)
	}
	for i, track := range transport.List.Tracks {
		if track.Length < 900*time.Millisecond || track.Length > 1200*time.Millisecond || track.Rate != 22 || track.Channels != 1 {
			t.Fatalf("track %d as the engine reported it: %+v", i, track)
		}
	}
	played, err := system.Played(audiotest.Speaker)
	if err != nil {
		t.Fatal(err)
	}
	// Two one-second tracks at 48 kHz or so; at least most of both.
	if seconds := float64(played.Samples) / float64(played.Format.Rate); seconds < 1.5 {
		t.Fatalf("%.2f s heard", seconds)
	}
}
