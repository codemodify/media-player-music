package shell

import (
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func TestHeadlessScreenshotShellDoesNotCreateTray(t *testing.T) {
	t.Setenv(platform.EnvTray, "fake")
	for _, skin := range Appearances {
		t.Run(skin, func(t *testing.T) {
			a, s := open(t, skin)
			if s.tray != nil || len(a.StatusItems()) != 0 {
				t.Fatal("headless screenshot mode created a status item")
			}
		})
	}
}

func openFakeTray(t *testing.T, skin string) (*ui.Application, *Shell, *platform.FakeStatusItem, *fakeEngine) {
	t.Helper()
	t.Setenv(platform.EnvTray, "fake")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(skin))
	// The shell follows the live path, while the toolkit's public fake tray
	// and offscreen windows keep this test away from the real desktop/audio.
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	a.SetIcon(paint2d.NewImage(16, 16), paint2d.NewImage(128, 128), paint2d.NewImage(32, 32))
	b := newFakeEngine()
	s, err := New(a, Options{Skin: skin, Engine: b})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	fake, ok := s.tray.(*platform.FakeStatusItem)
	if !ok {
		t.Skip("this platform wraps its fake status item to provide toolkit menu chrome")
	}
	return a, s, fake, b
}

func TestTrayUsesApplicationLogoAndShowsCurrentWindowAcrossSkins(t *testing.T) {
	a, s, tray, _ := openFakeTray(t, players.DefaultSkin)
	if tray.Icon().Image != a.Icon()[1] || tray.Icon().Name != "" {
		t.Fatal("tray did not request the largest application-logo raster without a competing icon name")
	}
	menu := tray.Menu()
	if len(menu) != 2 || menu[0].Text != "Show player" || menu[1].Text != "Quit" {
		t.Fatalf("unexpected tray menu: %+v", menu)
	}
	for _, skin := range append(players.AppearanceIDs(), players.DefaultSkin) {
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		window := s.MainWindow()
		offscreen := window.Surface().(*platform.Offscreen)
		before := offscreen.Activations()
		window.Hide()
		tray.Click()
		if window.Visible() {
			t.Fatal("tray callback ran before UI dispatch")
		}
		a.DrainPosted()
		if !window.Visible() || offscreen.Activations() != before+1 {
			t.Fatalf("%s tray activation did not show and activate the current window", skin)
		}
		window.Hide()
		tray.ClickMenu(0)
		a.DrainPosted()
		if !window.Visible() || offscreen.Activations() != before+2 {
			t.Fatalf("%s tray Show command used a stale window", skin)
		}
		if len(a.StatusItems()) != 1 || !tray.Alive() || s.tray != tray {
			t.Fatal("changing skin replaced or duplicated the system tray item")
		}
	}
}

func TestTrayQuitAndMainCloseWithoutHostExitAndReleaseTray(t *testing.T) {
	for _, skin := range Appearances {
		for _, throughTray := range []bool{false, true} {
			name := skin + "/main-close"
			if throughTray {
				name = skin + "/tray-quit"
			}
			t.Run(name, func(t *testing.T) {
				a, s, tray, backend := openFakeTray(t, skin)
				s.Start()
				windows := s.Windows()
				if throughTray {
					s.MainWindow().Inject(platform.Event{Kind: platform.EventClose})
					a.PumpOnce()
					tray.ClickMenu(1)
					a.DrainPosted()
				} else {
					tray.SetShown(false)
					s.MainWindow().Inject(platform.Event{Kind: platform.EventClose})
					a.PumpOnce()
				}
				if !a.Quitting() || s.Pulse().Running() {
					t.Fatal("exit action did not quit the application and stop its clock")
				}
				s.Close()
				s.Close()
				if !tray.Closed() || !backend.closed || s.tray != nil {
					t.Fatal("shutdown left the system tray or audio backend alive")
				}
				for _, named := range windows {
					if !named.Window.Closed() {
						t.Fatalf("shutdown left %s open", named.Name)
					}
				}
			})
		}
	}
}

func TestMainCloseToTrayKeepsPlaybackAndRestoresVisiblePanels(t *testing.T) {
	for _, skin := range Appearances {
		t.Run(skin, func(t *testing.T) {
			a, s, tray, backend := openFakeTray(t, skin)
			s.Presentation().ShowEqualiser(false)
			s.Presentation().ShowPlaylist(true)
			if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
				t.Fatal(err)
			}
			loaded(s)
			s.Pulse().Stop()
			ticks := 0
			s.Pulse().SetClock(func() time.Time { ticks++; return time.Now() })
			s.Start()
			windows := s.Windows()
			visible := make(map[string]bool)
			for _, named := range windows {
				visible[named.Name] = named.Window.Visible()
			}
			stops := countCommand(backend, playback.OpStop)
			// Use the real desktop close-request path, including a repeat
			// request while hidden, rather than calling the tray helper.
			for i := 0; i < 2; i++ {
				s.MainWindow().Inject(platform.Event{Kind: platform.EventClose})
				a.PumpOnce()
			}
			if a.Quitting() || s.trayHidden == nil || !s.Pulse().Running() || backend.closed {
				t.Fatal("main close shut down playback instead of hiding to the tray")
			}
			for _, named := range windows {
				if named.Window.Visible() || named.Window.Closed() {
					t.Fatalf("%s was not hidden intact", named.Name)
				}
			}
			// A real scheduled UI pulse and a broker-delivered decoder event
			// must keep running while every player window is hidden.
			backend.events <- playback.Update{Kind: playback.UpdatePosition, Generation: s.playback.generation, Position: 7500 * time.Millisecond}
			deadline := time.Now().Add(2 * time.Second)
			for ticks < 2 || s.Host.Model.Transport.Pos != 7500*time.Millisecond {
				a.DrainPosted()
				if time.Now().After(deadline) {
					t.Fatal("hidden player lost its timer or decoder event updates")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if s.Host.Model.Transport.State != players.Playing || countCommand(backend, playback.OpStop) != stops {
				t.Fatal("closing to the tray interrupted playback")
			}
			a.PumpOnce()
			for _, named := range windows {
				if named.Window.Visible() {
					t.Fatalf("background maintenance reopened %s", named.Name)
				}
			}
			s.remember()
			if s.Host.Model.EqualizerVisible || !s.Host.Model.PlaylistVisible || s.settings.Panels.Equaliser || !s.settings.Panels.Playlist {
				t.Fatal("temporary tray hiding changed the requested session panel visibility")
			}
			tray.Click()
			a.DrainPosted()
			for _, named := range windows {
				if named.Window.Visible() != visible[named.Name] {
					t.Fatalf("%s did not recover its previous visibility", named.Name)
				}
			}
			if s.trayHidden != nil || s.MainWindow().Surface().(*platform.Offscreen).Activations() == 0 {
				t.Fatal("tray activation did not restore and activate the main window")
			}
		})
	}
}

func TestTrayHiddenPlayerSwitchesSkinAndRecoversAfterHostLoss(t *testing.T) {
	a, s, tray, backend := openFakeTray(t, players.DefaultSkin)
	s.Presentation().ShowEqualiser(false)
	s.Presentation().ShowPlaylist(true)
	s.Start()
	s.Host.OpenFiles(s.MainWindow().Content(), false)
	a.DrainPosted()
	chooser := s.fileWindow
	if chooser == nil || !chooser.Visible() {
		t.Fatal("file chooser fixture did not open")
	}
	s.MainWindow().Inject(platform.Event{Kind: platform.EventClose})
	a.PumpOnce()
	if chooser.Visible() || chooser.Closed() {
		t.Fatal("closing to the tray left an isolated file chooser or destroyed it")
	}
	for _, skin := range []string{winamp.Skin, winamp.SkinSilver, players.DefaultSkin, winamp.Skin} {
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		for _, named := range s.Windows() {
			if named.Window.Visible() || named.Window.Closed() {
				t.Fatalf("%s %s escaped tray hiding after a skin change", skin, named.Name)
			}
		}
		if !s.Pulse().Running() || backend.closed || len(a.StatusItems()) != 1 || s.tray != tray {
			t.Fatal("hidden skin change stopped playback or duplicated the tray")
		}
	}
	if err := s.SetSkin("unavailable-skin"); err == nil || s.trayHidden == nil {
		t.Fatal("failed skin change did not retain the hidden presentation")
	}
	// Alive is still true here: only Shown identifies a recoverable host.
	tray.SetShown(false)
	a.DrainPosted()
	if a.Quitting() || !s.MainWindow().Visible() || s.trayHidden != nil || !tray.Alive() {
		t.Fatal("loss of the tray host stranded the hidden player")
	}
	if !chooser.Visible() || chooser != s.fileWindow {
		t.Fatal("host recovery did not restore the file chooser")
	}
	for _, named := range s.Windows() {
		if named.Name == "equaliser" && named.Window.Visible() {
			t.Fatal("host recovery reopened a disabled equaliser panel")
		}
		if named.Name == "playlist" && !named.Window.Visible() {
			t.Fatal("host recovery did not reopen the playlist")
		}
	}
	if !s.Pulse().Running() || backend.closed {
		t.Fatal("host recovery interrupted the live player")
	}
}

func TestExplicitKeyboardQuitWhileHiddenExits(t *testing.T) {
	a, s, tray, backend := openFakeTray(t, winamp.Skin)
	s.Start()
	s.MainWindow().Inject(platform.Event{Kind: platform.EventClose})
	a.PumpOnce()
	s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyQ, Mods: platform.ModCtrl})
	a.PumpOnce()
	if !a.Quitting() || s.Pulse().Running() {
		t.Fatal("explicit keyboard Quit hid the player instead of exiting")
	}
	s.Close()
	if !tray.Closed() || !backend.closed {
		t.Fatal("explicit Quit did not clean up the tray and backend")
	}
}

func TestQueuedTrayActivationAfterShutdownDoesNotReopenPlayer(t *testing.T) {
	a, s, tray, _ := openFakeTray(t, winamp.Skin)
	window := s.MainWindow()
	offscreen := window.Surface().(*platform.Offscreen)
	before := offscreen.Activations()
	tray.Click()
	s.Close()
	a.DrainPosted()
	if !window.Closed() || offscreen.Activations() != before {
		t.Fatal("a queued tray callback activated a closed presentation")
	}
}
