package winamp

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

func TestBothTransportRowsDispatchPlaybackCommands(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			var commands []players.Command
			p.Host.Run = func(c players.Command) {
				commands = append(commands, c)
				p.Transport.Do(c)
			}
			for _, row := range [][5]*players.GlyphButton{
				{p.strip.prev, p.strip.play, p.strip.pause, p.strip.stop, p.strip.next},
				{p.listPane.mini[0], p.listPane.mini[1], p.listPane.mini[2], p.listPane.mini[3], p.listPane.mini[4]},
			} {
				commands = nil
				p.Transport.Stop()
				playbackPress(t, row[2])
				if len(commands) != 0 || p.Transport.State != players.Stopped {
					t.Fatal("Pause dispatched playback while stopped")
				}
				for _, button := range []*players.GlyphButton{row[1], row[1], row[2], row[1], row[0], row[4], row[3]} {
					playbackPress(t, button)
				}
				want := []players.Command{
					players.CmdPlay, players.CmdPlay, players.CmdPlayPause, players.CmdPlay,
					players.CmdPrev, players.CmdNext, players.CmdStop,
				}
				if !reflect.DeepEqual(commands, want) {
					t.Fatalf("transport sent %v, want %v", commands, want)
				}
			}
		})
	}
}

func TestFileControlsUseHostCallbacksAndPreservePlaybackWhileChoosing(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			p.Transport.SelectTrack(2)
			p.Transport.Play()
			p.Transport.Pos = 53 * time.Second
			index, count := p.Transport.List.Index(), p.Transport.List.Len()
			var from widget.Component
			var appendToList bool
			opens, removes := 0, 0
			p.Host.LoadFiles = func(origin widget.Component, appendFiles bool) {
				from, appendToList = origin, appendFiles
				opens++
			}
			p.Host.DeleteTrack = func() { removes++ }
			for _, choice := range []struct {
				button *players.GlyphButton
				append bool
			}{
				{p.strip.eject, false},
				{p.listPane.mini[5], false},
				{p.listPane.add, true},
			} {
				before := opens
				playbackPress(t, choice.button)
				if opens != before+1 || from != choice.button || appendToList != choice.append {
					t.Fatal("file button did not pass its origin and append policy to the host")
				}
				if p.Transport.State != players.Playing || p.Transport.Pos != 53*time.Second || p.Transport.List.Index() != index || p.Transport.List.Len() != count {
					t.Fatal("opening the file chooser changed current playback before files were chosen")
				}
			}
			playbackPress(t, p.listPane.rem)
			if removes != 1 {
				t.Fatal("Remove did not call the host's removal action")
			}
		})
	}
}

func TestPlaybackSlidersSynchronizeThroughTheHost(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			syncs := 0
			p.Host.SyncPlayback = func() { syncs++ }
			for _, slider := range []*players.Fader{p.strip.seek, p.strip.volume, p.strip.balance} {
				before := syncs
				if !slider.AccessibleAction(0, a11y.ActionIncrement) || syncs != before+1 {
					t.Fatalf("%s did not synchronize one user edit through the host", slider.Label)
				}
			}
			if p.Transport.Pos <= 0 || p.Transport.Volume != p.strip.volume.Value/100 || p.Transport.Pan != p.strip.balance.Value {
				t.Fatal("the host did not receive the slider values")
			}
			p.Refresh()
			if syncs != 3 {
				t.Fatal("refreshing the controls sent playback edits back to the host")
			}
		})
	}
}

func TestPlaylistActivationAndKeyboardReachTheSharedPlaybackHost(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1)
			var commands []players.Command
			p.Host.Run = func(c players.Command) {
				commands = append(commands, c)
				p.Transport.Do(c)
			}
			list := p.listPane.list
			if !list.AccessibleAction(2, a11y.ActionDefault) || p.Transport.List.Index() != 2 {
				t.Fatal("activating a row did not select its track")
			}
			p.Transport.State = players.Paused
			if !list.AccessibleAction(2, a11y.ActionDefault) || p.Transport.State != players.Playing {
				t.Fatal("activating the current row did not resume through the host")
			}
			p.List.RequestFocus(list)
			p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyDown})
			a.PumpOnce()
			if list.Selected != 3 || p.Transport.List.Index() != 2 || len(commands) != 2 {
				t.Fatal("Down did not select the next row without changing playback")
			}
			p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
			a.PumpOnce()
			if p.Transport.List.Index() != 3 {
				t.Fatal("Return did not activate the selected playlist track")
			}
			p.Main.RequestFocus(p.strip.seek)
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
			a.PumpOnce()
			want := []players.Command{players.CmdPlay, players.CmdPlay, players.CmdPlay, players.CmdNext}
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("row/key actions sent %v, want %v", commands, want)
			}
		})
	}
}

func TestEqualiserEditsNotifyTheSharedModel(t *testing.T) {
	_, p := open(t, Skin, 1)
	changed := 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() {
		changed++
		refresh()
	}
	on := p.Equaliser.On
	playbackPress(t, p.eqPane.on)
	if changed != 1 || p.Equaliser.On == on {
		t.Fatal("the EQ On button did not notify the shared equaliser")
	}
	for _, fader := range []*players.Fader{p.eqPane.preamp, p.eqPane.bands[0]} {
		before := changed
		if !fader.AccessibleAction(0, a11y.ActionIncrement) || changed != before+1 {
			t.Fatal("an EQ fader did not notify the shared equaliser exactly once")
		}
	}
}

func TestLiveEmptyPlaylistAndUnknownMetadataAreDescribedHonestly(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1)
			p.Host.Model.Live = true
			p.Transport.List = players.NewPlaylist()
			p.Host.Model.Error = "mpv was not found on PATH"
			p.Refresh()
			a.PumpOnce()
			if p.listPane.list.Count != 0 || p.listPane.list.Selected != -1 {
				t.Fatal("emptying the live playlist kept a stale row selection")
			}
			var node a11y.Node
			p.strip.readout.Describe(&node)
			if !strings.Contains(node.Name, "Open audio files") || node.Description != p.Host.Model.Error {
				t.Fatalf("empty player description = %+v", node)
			}
			if !strings.Contains(p.listPane.foot.text(), p.Host.Model.Error) || strings.Contains(p.listPane.foot.text(), "visual demo") {
				t.Fatal("the playlist status hid the playback error or advertised simulated playback")
			}

			p.Transport.List = players.NewPlaylist(players.Track{Title: "A real file", Path: "/music/real.flac"})
			p.Refresh()
			p.strip.readout.Describe(&node)
			if !strings.Contains(node.Name, "of --:--") || p.listPane.list.ItemDetail(0) != "--:--" {
				t.Fatal("unknown duration was presented as a known zero duration")
			}
			if got := currentTitle(p.Transport); strings.Contains(got, "0:00") || !strings.Contains(got, "A real file") {
				t.Fatalf("unknown-duration title = %q", got)
			}
		})
	}
}

func playbackPress(t *testing.T, b *players.GlyphButton) {
	t.Helper()
	if !b.AccessibleAction(0, a11y.ActionDefault) {
		t.Fatalf("button %q did not accept activation", b.Label)
	}
}
