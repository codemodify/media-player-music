package shell

import (
	"os"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/standard"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
)

func rememberedShell(t *testing.T, skin string) (*ui.Application, *Shell, *recordingBackend) {
	t.Helper()
	opts := Options{Skin: skin, Headless: true, Remember: true}
	t.Setenv(style.ThemeEnv, LookFor(SkinFor(opts)))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	b := &recordingBackend{events: make(chan playback.Event, 100)}
	opts.Backend = b
	s, err := New(a, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	s.presentation.Advance(0)
	return a, s, b
}

func TestSessionRestoresQueueSelectionSkinAndPositionWithoutAutoplay(t *testing.T) {
	for _, skin := range Appearances {
		t.Run(skin, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			paths := mediaPaths(t, 2)
			firstApp, first, _ := rememberedShell(t, skin)
			if err := first.LoadFiles(paths, false); err != nil {
				t.Fatal(err)
			}
			transport := first.Host.Model.Transport
			transport.List.Tracks[1].Title = "Remembered title"
			transport.List.Tracks[1].Length = time.Minute
			transport.List.Select(1)
			transport.Pos = 9250 * time.Millisecond
			first.MainWindow().Move(420, 210)
			first.presentation.Advance(0)
			first.presentation.ShowEqualiser(false)
			if skin == players.DefaultSkin {
				first.MainWindow().SetSize(1000, 620)
				firstApp.PumpOnce()
			}
			first.Close()

			_, next, backend := rememberedShell(t, "")
			model := next.Host.Model
			if next.Skin() != skin || model.Transport.List.Len() != 2 || model.Transport.List.Index() != 1 {
				t.Fatalf("restored skin=%s queue=%d selection=%d", next.Skin(), model.Transport.List.Len(), model.Transport.List.Index())
			}
			if model.Transport.Track().Path != paths[1] || model.Transport.Track().Title != "Remembered title" || model.Transport.Pos != 9250*time.Millisecond {
				t.Fatalf("lost queue metadata/offset: %#v, %s", model.Transport.Track(), model.Transport.Pos)
			}
			if model.EqualizerVisible || !model.PlaylistVisible {
				t.Fatal("lost panel visibility")
			}
			if model.Transport.State != players.Stopped || countCommand(backend, "loadfile") != 0 {
				t.Fatal("restoring a session automatically started media")
			}
			if x, y, ok := next.MainWindow().Position(); !ok || x != 420 || y != 210 {
				t.Fatalf("restored main position=(%d,%d,%v)", x, y, ok)
			}
			if skin == players.DefaultSkin {
				if width, height := next.MainWindow().Size(); width != 1000 || height != 620 {
					t.Fatalf("lost resizable default size: %dx%d", width, height)
				}
			}
			next.Command(players.CmdPlay)
			if countCommand(backend, "loadfile") != 1 || next.playback.seekOnLoad != 9250*time.Millisecond {
				t.Fatal("Play did not load the restored selection at its saved offset")
			}
		})
	}
}

func TestSessionRetainsSeparateDefaultAndSkinArrangements(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, _ := rememberedShell(t, winamp.Skin)
	s.MainWindow().Move(380, 230)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.DefaultSkin); err != nil {
		t.Fatal(err)
	}
	s.Pulse().Stop()
	a.PumpOnce()
	s.presentation.Advance(0)
	s.MainWindow().Move(720, 80)
	s.MainWindow().SetSize(1000, 650)
	a.PumpOnce()
	for _, id := range players.PanelSkins()[1:] {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		s.presentation.Advance(0)
		if x, y, _ := s.MainWindow().Position(); x != 380 || y != 230 {
			t.Fatalf("%s lost the Winamp arrangement: %d,%d", id, x, y)
		}
	}
	if err := s.SetSkin(players.DefaultSkin); err != nil {
		t.Fatal(err)
	}
	s.Pulse().Stop()
	a.PumpOnce()
	s.presentation.Advance(0)
	if x, y, _ := s.MainWindow().Position(); x != 720 || y != 80 {
		t.Fatalf("Default lost its independent position: %d,%d", x, y)
	}
	if width, height := s.MainWindow().Size(); width != 1000 || height != 650 {
		t.Fatal("Default lost its size across appearance changes")
	}
}

func TestWMPSessionKeepsEachPlacementSeparateFromWinampAndDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, backend := rememberedShell(t, players.DefaultSkin)
	settle := func() {
		s.Pulse().Stop()
		a.PumpOnce()
		s.presentation.Advance(0)
	}
	s.MainWindow().Move(720, 80)
	s.MainWindow().SetSize(1000, 650)
	a.PumpOnce()
	if err := s.SetSkin(players.WinampSkin); err != nil {
		t.Fatal(err)
	}
	settle()
	s.MainWindow().Move(380, 230)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.HeadspaceSkin); err != nil {
		t.Fatal(err)
	}
	settle()
	s.MainWindow().Move(510, 340)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.XboxSkin); err != nil {
		t.Fatal(err)
	}
	settle()
	s.MainWindow().Move(850, 400)
	for _, named := range s.Windows() {
		if named.Name == "playlist" {
			named.Window.Move(900, 670)
			named.Window.SetSize(610, 430)
		}
	}
	settle()
	s.presentation.ShowPlaylist(false)
	for _, tc := range []struct {
		skin string
		x, y int
	}{
		{players.WinampSilverSkin, 380, 230},
		{players.DefaultSkin, 720, 80},
		{players.HeadspaceSkin, 510, 340},
		{players.XboxSkin, 850, 400},
	} {
		if err := s.SetSkin(tc.skin); err != nil {
			t.Fatal(err)
		}
		settle()
		if x, y, ok := s.MainWindow().Position(); !ok || x != tc.x || y != tc.y {
			t.Fatalf("%s restored (%d,%d,%t), want (%d,%d)", tc.skin, x, y, ok, tc.x, tc.y)
		}
		if tc.skin == players.DefaultSkin {
			if width, height := s.MainWindow().Size(); width != 1000 || height != 650 {
				t.Fatalf("WMP skin replaced Default's saved size: %dx%d", width, height)
			}
		}
	}
	if len(s.Windows()) != 2 || s.settings.Windows[players.HeadspaceSkin]["main"].X != 510 || s.settings.Windows[players.XboxSkin]["main"].X != 850 || s.settings.Windows[players.WinampSkin]["main"].X != 380 || s.settings.Windows[players.DefaultSkin]["main"].X != 720 {
		t.Fatalf("presentation families overwrote each other: %v", s.settings.Windows)
	}
	if countCommand(backend, "loadfile") != 0 {
		t.Fatal("changing presentation started the restored empty queue")
	}
	s.Close()
	_, reopened, nextBackend := rememberedShell(t, "")
	if reopened.Skin() != players.XboxSkin || len(reopened.Windows()) != 2 {
		t.Fatal("saved Xbox selection did not reopen its own presentation")
	}
	if x, y, ok := reopened.MainWindow().Position(); !ok || x != 850 || y != 400 || countCommand(nextBackend, "loadfile") != 0 {
		t.Fatalf("Xbox reopened at (%d,%d,%t) or started playback", x, y, ok)
	}
	for _, named := range reopened.Windows() {
		if named.Name == "playlist" {
			x, y, ok := named.Window.Position()
			width, height := named.Window.Size()
			if !ok || x != 900 || y != 670 || width != 610 || height != 430 || named.Window.Visible() || reopened.Host.Model.PlaylistVisible {
				t.Fatalf("Xbox playlist lost saved placement, size or visibility: %d,%d,%t %dx%d shown=%t", x, y, ok, width, height, named.Window.Visible())
			}
			return
		}
	}
	t.Fatal("Xbox did not retain its independently placed playlist")
}

func TestMissingFilesDoNotBlockSessionRestoreOrReuseAnotherTracksOffset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	paths := mediaPaths(t, 3)
	saved := players.Settings{Skin: players.DefaultSkin, Playlist: &players.SavedPlaylist{
		Tracks: []players.Track{{Path: paths[0]}, {Path: paths[1]}, {Path: paths[2]}}, Index: 1, Position: 9 * time.Second,
	}}
	if err := saved.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths[1]); err != nil {
		t.Fatal(err)
	}
	_, s, _ := rememberedShell(t, "")
	transport := s.Host.Model.Transport
	if transport.List.Len() != 2 || transport.Track().Path != paths[0] || transport.Pos != 0 {
		t.Fatalf("bad missing-file recovery: %#v at %s", transport.List.Tracks, transport.Pos)
	}
}

func TestRemovingTheLastTrackPersistsAnEmptyQueue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, first, _ := rememberedShell(t, players.DefaultSkin)
	if err := first.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	first.removeTrack()
	first.Close()
	_, next, _ := rememberedShell(t, "")
	if next.Host.Model.Transport.List.Len() != 0 || players.LoadSettings().Playlist == nil {
		t.Fatal("a deliberately cleared queue was resurrected")
	}
}

func TestSessionAutosaveAndScreenshotIsolation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, first, _ := rememberedShell(t, players.DefaultSkin)
	first.MainWindow().Move(630, 310)
	first.tick(2 * time.Second)
	saved := players.LoadSettings()
	if position := saved.Windows[players.DefaultSkin]["main"]; !position.Positioned || position.X != 630 || position.Y != 310 {
		t.Fatal("idle window movement was not saved")
	}
	first.Close()
	before, err := os.ReadFile(players.SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(style.ThemeEnv, LookFor(winamp.Skin))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	shot, err := New(a, Options{Headless: true, Skin: winamp.Skin})
	if err != nil {
		t.Fatal(err)
	}
	shot.Pulse().Stop()
	shot.Pose(time.Minute)
	shot.Close()
	after, err := os.ReadFile(players.SettingsPath())
	if err != nil || string(after) != string(before) {
		t.Fatal("screenshot fixtures replaced the live session")
	}
}

func TestRestoreDefaultSizeHonorsItsMinimum(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	saved := players.Settings{Skin: players.DefaultSkin, Windows: map[string]map[string]players.WindowPlacement{
		players.DefaultSkin: {"main": {Width: 1, Height: 1, Positioned: true}},
	}}
	if err := saved.Save(); err != nil {
		t.Fatal(err)
	}
	_, s, _ := rememberedShell(t, "")
	if width, height := s.MainWindow().Size(); width < standard.MinW || height < standard.MinH {
		t.Fatalf("restored a window below its usable minimum: %dx%d", width, height)
	}
}
