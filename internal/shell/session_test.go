package shell

import (
	"os"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/panel"
	"github.com/codemodify/media-player-music/internal/players/standard"
	"github.com/codemodify/media-player-music/internal/service"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
)

func rememberedShell(t *testing.T, skin string) (*ui.Application, *Shell, *fakeEngine) {
	t.Helper()
	return rememberedShellOn(t, skin, service.Saved{})
}

// rememberedShellOn opens a shell that remembers, on a daemon of its own
// made as saved: a daemon started again between two runs of the shell, the
// first daemon's Saved() given.
func rememberedShellOn(t *testing.T, skin string, saved service.Saved) (*ui.Application, *Shell, *fakeEngine) {
	t.Helper()
	opts := Options{Skin: skin, Headless: true, Remember: true}
	t.Setenv(style.ThemeEnv, LookFor(SkinFor(opts)))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	b := newFakeEngine()
	s, _, err := startDaemon(t, a, b, saved, func(o Options) (*Shell, error) { return New(a, o) }, opts)
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
			// the second track, as its decoder told it, paused in it
			first.Host.SelectTrack(1)
			loaded(first)
			update(first, playback.Update{Kind: playback.UpdateMetadata, Info: playback.TrackInfo{Title: "Remembered title"}})
			update(first, playback.Update{Kind: playback.UpdateDuration, Duration: time.Minute})
			pauseAt(t, first, 9250*time.Millisecond)
			desktopMove(t, firstApp, first, first.MainWindow(), 420, 210)
			first.presentation.Advance(0)
			first.presentation.ShowEqualiser(false)
			if skin == players.DefaultSkin {
				first.MainWindow().SetSize(1000, 620)
				firstApp.PumpOnce()
			}
			saved := serviceOf(first).service.Saved()
			first.Close()

			// the daemon started again too: the session is what it kept
			nextApp, next, backend := rememberedShellOn(t, "", saved)
			// The saved place holds through the settle, past the desktop's own.
			awaitSettled(t, nextApp, next)
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
			if model.Transport.State != players.Stopped || countCommand(backend, playback.OpLoad) != 0 {
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
			if load, ok := backend.last(playback.OpLoad); countCommand(backend, playback.OpLoad) != 1 || !ok || load.Position != 9250*time.Millisecond {
				t.Fatal("Play did not load the restored selection at its saved offset")
			}
		})
	}
}

func TestSessionRetainsSeparateDefaultAndSkinArrangements(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, _ := rememberedShell(t, panel.Skin)
	desktopMove(t, a, s, s.MainWindow(), 380, 230)
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
	if err := s.SetSkin(players.SlateSkin); err != nil {
		t.Fatal(err)
	}
	settle()
	desktopMove(t, a, s, s.MainWindow(), 380, 230)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.WingspanSkin); err != nil {
		t.Fatal(err)
	}
	settle()
	s.MainWindow().Move(510, 340)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.PodSkin); err != nil {
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
		{players.SilverSkin, 380, 230},
		{players.DefaultSkin, 720, 80},
		{players.WingspanSkin, 510, 340},
		{players.PodSkin, 850, 400},
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
	if len(s.Windows()) != 2 || s.settings.Windows[players.WingspanSkin]["main"].X != 510 || s.settings.Windows[players.PodSkin]["main"].X != 850 || s.settings.Windows[players.SlateSkin]["main"].X != 380 || s.settings.Windows[players.DefaultSkin]["main"].X != 720 {
		t.Fatalf("presentation families overwrote each other: %v", s.settings.Windows)
	}
	if countCommand(backend, playback.OpLoad) != 0 {
		t.Fatal("changing presentation started the restored empty queue")
	}
	s.Close()
	_, reopened, nextBackend := rememberedShell(t, "")
	if reopened.Skin() != players.PodSkin || len(reopened.Windows()) != 2 {
		t.Fatal("saved Xbox selection did not reopen its own presentation")
	}
	if x, y, ok := reopened.MainWindow().Position(); !ok || x != 850 || y != 400 || countCommand(nextBackend, playback.OpLoad) != 0 {
		t.Fatalf("Xbox reopened at (%d,%d,%t) or started playback", x, y, ok)
	}
	for _, named := range reopened.Windows() {
		if named.Name == "playlist" {
			x, y, ok := named.Window.Position()
			width, height := named.Window.Size()
			if !ok || x != 900 || y != 670 || width != 610 || height != 430 || named.Window.Visible() || reopened.Host.Model.PlaylistVisible {
				t.Fatalf("Pod playlist lost saved placement, size or visibility: %d,%d,%t %dx%d shown=%t", x, y, ok, width, height, named.Window.Visible())
			}
			return
		}
	}
	t.Fatal("Xbox did not retain its independently placed playlist")
}

func TestMissingFilesDoNotBlockSessionRestoreOrReuseAnotherTracksOffset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	paths := mediaPaths(t, 3)
	saved := service.Saved{Queue: []musicapi.Track{{Path: paths[0]}, {Path: paths[1]}, {Path: paths[2]}}, Index: 1, PositionMS: 9000}
	if err := os.Remove(paths[1]); err != nil {
		t.Fatal(err)
	}
	_, s, _ := rememberedShellOn(t, "", saved)
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
	saved := serviceOf(first).service.Saved()
	first.Close()
	_, next, _ := rememberedShellOn(t, "", saved)
	if next.Host.Model.Transport.List.Len() != 0 || len(saved.Queue) != 0 {
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
	t.Setenv(style.ThemeEnv, LookFor(panel.Skin))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	shot, err := New(a, Options{Headless: true, Skin: panel.Skin})
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
