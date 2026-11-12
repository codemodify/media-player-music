package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/link"
	"github.com/codemodify/media-player-music/internal/tui"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/cliflags"
	"goframework.io/terminal-ui/uitest"
)

// TestFlags: --version says the version; --daemon and --token are read.
func TestFlags(t *testing.T) {
	invocation, err := cliflags.Parse(command, []string{"--version"})
	if err != nil || !invocation.Version {
		t.Fatalf("--version: %+v %v", invocation, err)
	}
	var out bytes.Buffer
	if err := invocation.WriteVersion(&out); err != nil || out.String() != "media-player-music-tui "+Version+"\n" {
		t.Fatalf("--version wrote %q: %v", out.String(), err)
	}
	invocation, err = cliflags.Parse(command, []string{"--daemon", "ws://127.0.0.1:50130/clients", "--token", "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if given := invocation.Flags.(*flags); given.Daemon != "ws://127.0.0.1:50130/clients" || given.Token != "secret" {
		t.Fatalf("flags: %+v", given)
	}
}

// screen runs the view on a terminal in memory.
func screen(t *testing.T) (*uitest.Driver, *tui.View) {
	driver := uitest.NewDriver(t, uitest.Size(60, 14))
	view := tui.New(driver.App)
	driver.Start(view.Build())
	return driver, view
}

// TestTheConnectionTellsTheView: what link hears is posted to the view:
// the states, how the connection stands, why it failed.
func TestTheConnectionTellsTheView(t *testing.T) {
	driver, view := screen(t)
	given := options(driver.App, view, "unix:///nowhere", "")
	if given.Name != "terminal" || given.Version != Version || given.Ensure != nil {
		t.Fatalf("options: %+v", given)
	}
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "connecting to the music daemon…") })
	given.Status("", false)
	given.State(musicapi.State{Seq: 7, Status: musicapi.StatusPlaying, Index: 0, Volume: 0.5, Repeat: musicapi.RepeatOne,
		Track: musicapi.Track{Title: "Airbag", Artist: "Radiohead", LengthMS: 284000}, Queue: musicapi.QueueInfo{Version: 1, Length: 1}})
	given.Queue(musicapi.Queue{Version: 1, Tracks: []musicapi.Track{{Title: "Airbag", Artist: "Radiohead", LengthMS: 284000}}})
	given.Now(musicapi.Now{Seq: 7, Index: 0, Status: musicapi.StatusPlaying, PositionMS: 61000, LengthMS: 284000})
	given.Library(musicapi.Library{Version: 1})
	driver.Wait(func() bool {
		screen := driver.Screen()
		return strings.Contains(screen, "▶ Radiohead - Airbag") && strings.Contains(screen, "1:01/4:44") && strings.Contains(screen, "vol 50% rep one")
	})
	// another daemon, met after the connection ended: its states count
	// from 1 again
	given.Connected(false)
	given.Connected(true)
	given.State(musicapi.State{Seq: 1, Status: musicapi.StatusPaused, Index: 0, Volume: 0.5,
		Track: musicapi.Track{Title: "Lucky", Artist: "Radiohead"}, Queue: musicapi.QueueInfo{Version: 1, Length: 1}})
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "‖ Radiohead - Lucky") })
	given.Failed(errors.New("refused: a wrong token"))
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "refused: a wrong token") })
}

// TestNoDaemon: with no daemon at the address, the screen says it cannot
// reach it, and keeps trying; keys do nothing wrong meanwhile.
func TestNoDaemon(t *testing.T) {
	driver, view := screen(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	address := "unix://" + filepath.ToSlash(filepath.Join(t.TempDir(), "none.sock"))
	view.SetPlayer(link.Start(ctx, options(driver.App, view, address, "")))
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "can't reach the music daemon") })
	driver.Press("c", "2", "enter", "4", "enter", "o")
	driver.Type("/nowhere")
	driver.Press("enter")
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "can't reach the music daemon") })
}
