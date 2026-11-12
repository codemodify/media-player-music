package shell

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/playback"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
)

func TestImportedSkinsPreserveThePlayingQueueAndWindowGroup(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, s := openAtScale(t, players.WinampSkin, scale)
			model := s.Host.Model
			tr := model.Transport
			tr.List.Select(2)
			tr.SetVolume(.37)
			tr.SetPan(-.4)
			s.Pose(97 * time.Second)
			s.Pulse().Stop()
			s.MainWindow().Move(420, 210)
			s.presentation.Advance(0)
			for _, named := range s.Windows() {
				if named.Name == "playlist" {
					named.Window.Move(850, 540)
				}
			}
			s.presentation.Advance(0)
			before := s.presentation.CapturePlacement()
			if before["equaliser"].Dock == nil || before["playlist"].Dock != nil {
				t.Fatalf("fixture needs docked EQ/detached playlist: %v", before)
			}
			queue := append([]players.Track(nil), tr.List.Tracks...)
			presentation := s.presentation
			for _, id := range players.PanelSkins()[1:] {
				if err := s.SetSkin(id); err != nil {
					t.Fatal(err)
				}
				// Painting a new pack may take longer than a playback tick.
				// Freeze the fixture clock so elapsed time is not a reset.
				s.Pulse().Stop()
				a.PumpOnce()
				s.presentation.Advance(0)
				if s.Host.Model != model || s.presentation != presentation || tr.State != players.Playing || tr.Pos != 97*time.Second || tr.List.Index() != 2 || tr.Volume != .37 || tr.Pan != -.4 || !reflect.DeepEqual(tr.List.Tracks, queue) {
					t.Fatalf("%s changed the playing queue or controls", id)
				}
				if got := s.presentation.CapturePlacement(); !reflect.DeepEqual(got, before) {
					t.Fatalf("%s changed the window group: got %v want %v", id, got, before)
				}
			}
		})
	}
}

func TestWMPSwitchReplacesPresentationFamiliesAroundLivePlayback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, backend := rememberedShell(t, players.WinampSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	model, transport, queue := s.Host.Model, s.Host.Model.Transport, s.Host.Model.Transport.List
	transport.Pos = 17 * time.Second
	transport.SetVolume(.37)
	transport.SetPan(-.4)
	model.Equaliser.SetAuto(true, 0)
	model.Equaliser.Set(4, 6)
	model.EqualizerVisible, model.PlaylistVisible = false, true
	s.refresh()
	loads := countCommand(backend, playback.OpLoad)
	for _, id := range []string{players.HeadspaceSkin, players.XboxSkin, players.HalfLife2Skin, players.FandomSkin, players.ITunesSkin, players.Series9Skin, players.VLCSkin, players.RealPlayerSkin, players.AmarokSkin, players.WinampSilverSkin, players.DefaultSkin, players.Series9Skin, players.ITunesSkin, players.HeadspaceSkin, players.FandomSkin, players.HalfLife2Skin, players.XboxSkin} {
		old := s.Windows()
		before := s.presentation
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		if s.presentation == before {
			t.Fatalf("%s reused a different presentation family's controls", id)
		}
		for _, named := range old {
			if !named.Window.Closed() {
				t.Fatalf("%s left its previous %s window open", id, named.Name)
			}
		}
		if s.Host.Model != model || model.Transport != transport || transport.List != queue || transport.State != players.Playing || transport.Pos != 17*time.Second || transport.Volume != .37 || transport.Pan != -.4 || model.Equaliser.Gains[4] != 6 || !model.Equaliser.Auto || model.EqualizerVisible || !model.PlaylistVisible {
			t.Fatalf("%s changed shared playback or section preferences", id)
		}
		if countCommand(backend, playback.OpLoad) != loads || backend.closed || a.Quitting() {
			t.Fatalf("%s interrupted the active backend/application", id)
		}
		count := 1
		if players.IsPanelSkin(id) || id == players.HalfLife2Skin {
			count = 3
		} else if id == players.XboxSkin || id == players.FandomSkin || id == players.ITunesSkin {
			count = 2
		}
		if len(s.Windows()) != count || s.MainWindow().Closed() || !s.MainWindow().Visible() {
			t.Fatalf("%s did not open its complete window arrangement", id)
		}
	}
}
