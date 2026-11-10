package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

func fancyModes(t *testing.T, s *Shell) compactPresentation {
	t.Helper()
	p, ok := s.Presentation().(compactPresentation)
	if !ok {
		t.Fatalf("Fancy presentation %T does not expose full/compact state", s.Presentation())
	}
	return p
}

func TestFancyRequiresEveryLayoutBeforeReplacingPresentation(t *testing.T) {
	names := []string{"fancy.main", "fancy.compact", "fancy.equalizer", "fancy.playlist"}
	for _, missing := range names {
		t.Run(missing, func(t *testing.T) {
			_, s := open(t, players.DefaultSkin)
			main, model := s.MainWindow(), s.Host.Model
			layouts := map[string]any{}
			for _, name := range names {
				if name != missing {
					layouts[name] = map[string]any{"size": []int{420, 220}}
				}
			}
			dir := skins.UserSkinDir(players.FancySkin)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"skin": 1, "label": "WinAmp - Wrong", "base": "dark", "layouts": layouts})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), data, 0600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if err := s.SetSkin(players.FancySkin); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("missing layout %q", missing)) {
				t.Fatalf("incomplete Fancy override: %v", err)
			}
			if s.MainWindow() != main || main.Closed() || s.Skin() != players.DefaultSkin || s.Host.Model != model {
				t.Fatal("invalid Fancy override replaced the working presentation")
			}
		})
	}
}

func TestFancySkinKeysAndCompactMenuUseExactLabel(t *testing.T) {
	a, s := open(t, players.AmarokSkin)
	model := s.Host.Model
	for _, tc := range []struct {
		mods platform.Modifiers
		want string
	}{
		{platform.ModCtrl, players.FancySkin},
		{platform.ModCtrl, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.FancySkin},
	} {
		s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Mods: tc.mods})
		a.PumpOnce()
		a.PumpOnce()
		if s.Skin() != tc.want || s.Host.Model != model {
			t.Fatalf("skin shortcut chose %s, want %s with the shared model", s.Skin(), tc.want)
		}
	}
	if len(s.Windows()) != 3 || presentationFamily(s.Skin()) != players.FancySkin {
		t.Fatal("Fancy did not open its independent three-panel family")
	}
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("menu@%g", scale), func(t *testing.T) {
			t.Setenv(platform.EnvPopups, "layer")
			a, s := openAtScale(t, players.WinampSkin, scale)
			w := s.MainWindow()
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyMenu})
			a.PumpOnce()
			pop, ok := w.Popup().(*widgets.PopupMenu)
			if !ok {
				t.Fatalf("context menu is %T", w.Popup())
			}
			choice := len(Appearances) - 1
			if Appearances[choice] != players.FancySkin || pop.Items[choice].Text != "Fancy" || pop.MaxOffset() <= 0 {
				t.Fatal("compact menu lost Fancy's exact label, order or scrolling")
			}
			menuChoice(t, a, w, pop, choice)
			model := s.Host.Model
			if !w.AccessibleAction(widget.ItemID(pop, choice), a11y.ActionDefault) {
				t.Fatal("Fancy context choice rejected accessible activation")
			}
			a.PumpOnce()
			a.PumpOnce()
			if s.Skin() != players.FancySkin || s.Host.Model != model || !w.Closed() || len(s.Windows()) != 3 {
				t.Fatal("Fancy menu choice did not replace the old window family safely")
			}
		})
	}
}

func TestFancyCompactAndTrayKeepPlaybackAndCurrentWindows(t *testing.T) {
	a, s, tray, backend := openFakeTray(t, players.FancySkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	identify(s, 91)
	model, transport, queue := s.Host.Model, s.Host.Model.Transport, s.Host.Model.Transport.List
	transport.Pos = 17 * time.Second
	s.Host.SetVolume(.37)
	s.Host.SetPan(-.4)
	s.presentation.ShowEqualiser(true)
	s.presentation.ShowPlaylist(true)
	s.Start()
	main, native, pulse := s.MainWindow(), s.MainWindow().Surface(), s.Pulse()
	loads, stops := countCommand(backend, "loadfile"), countCommand(backend, "stop")
	settle := func() { a.PumpOnce(); s.presentation.Advance(0) }
	main.Move(420, 210)
	p := fancyModes(t, s)
	for _, compact := range []bool{true, false, true} {
		p.SetCompact(compact)
		settle()
		wantH := 220
		if compact {
			wantH = 40
		}
		w, h := main.Size()
		if p.Compact() != compact || w != 420 || h != wantH || s.MainWindow() != main || main.Surface() != native {
			t.Fatalf("compact=%t changed main identity or geometry: %dx%d", compact, w, h)
		}
		if x, y, known := main.Position(); !known || x != 420 || y != 210 {
			t.Fatalf("compact switch moved main: %d,%d,%t", x, y, known)
		}
		if s.Host.Model != model || model.Transport != transport || transport.List != queue || transport.State != players.Playing || transport.Pos != 17*time.Second || transport.Volume != .37 || transport.Pan != -.4 || !model.EqualizerVisible || !model.PlaylistVisible {
			t.Fatal("compact switch changed playback or requested panel visibility")
		}
		if s.Pulse() != pulse || !pulse.Running() || s.tray != tray || len(a.StatusItems()) != 1 || backend.closed || countCommand(backend, "loadfile") != loads || countCommand(backend, "stop") != stops {
			t.Fatal("compact switch interrupted backend, tray or update timer")
		}
	}
	visible := map[string]bool{}
	for _, named := range s.Windows() {
		visible[named.Name] = named.Window.Visible()
	}
	main.Inject(platform.Event{Kind: platform.EventClose})
	a.PumpOnce()
	for _, named := range s.Windows() {
		if named.Window.Visible() || named.Window.Closed() {
			t.Fatal("close-to-tray failed to hide intact Fancy panels")
		}
	}
	if s.trayHidden == nil || !pulse.Running() || backend.closed {
		t.Fatal("closing Fancy did not keep tray playback alive")
	}
	tray.Click()
	a.DrainPosted()
	for _, named := range s.Windows() {
		if named.Window.Visible() != visible[named.Name] {
			t.Fatalf("tray did not restore %s visibility", named.Name)
		}
	}
	if !p.Compact() || s.trayHidden != nil || main.Closed() || backend.closed {
		t.Fatal("tray restoration changed Fancy's compact mode or backend")
	}
	if err := s.SetSkin(players.DefaultSkin); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSkin(players.FancySkin); err != nil {
		t.Fatal(err)
	}
	settle()
	if !main.Closed() || s.MainWindow() == main || !fancyModes(t, s).Compact() || s.tray != tray || s.Host.Model != model || backend.closed || countCommand(backend, "loadfile") != loads {
		t.Fatal("family replacement lost remembered mode or captured a stale main/backend")
	}
}

func TestFancySessionRestoresCompactAndAllPanelPlacementIndependently(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, backend := rememberedShell(t, players.DefaultSkin)
	settle := func() { s.Pulse().Stop(); a.PumpOnce(); s.presentation.Advance(0) }
	s.MainWindow().Move(720, 80)
	settle()
	if err := s.SetSkin(players.FancySkin); err != nil {
		t.Fatal(err)
	}
	settle()
	s.MainWindow().Move(420, 210)
	for _, named := range s.Windows() {
		switch named.Name {
		case "equaliser":
			named.Window.Move(930, 500)
		case "playlist":
			named.Window.Move(1140, 700)
			named.Window.SetSize(610, 230)
		}
	}
	settle()
	s.presentation.ShowEqualiser(false)
	s.presentation.ShowPlaylist(true)
	fancyModes(t, s).SetCompact(true)
	settle()
	state, placement := fancyModes(t, s).CaptureState(), s.presentation.CapturePlacement()
	for _, skin := range []string{players.DefaultSkin, players.FancySkin} {
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		settle()
		if skin == players.DefaultSkin {
			if x, y, known := s.MainWindow().Position(); !known || x != 720 || y != 80 {
				t.Fatal("Fancy changed Default's independent position")
			}
		}
	}
	if !reflect.DeepEqual(fancyModes(t, s).CaptureState(), state) || !reflect.DeepEqual(s.presentation.CapturePlacement(), placement) || countCommand(backend, "loadfile") != 0 {
		t.Fatal("family roundtrip changed Fancy mode/placement or started playback")
	}
	s.Close()
	_, reopened, nextBackend := rememberedShell(t, "")
	if reopened.Skin() != players.FancySkin || !fancyModes(t, reopened).Compact() || !reflect.DeepEqual(fancyModes(t, reopened).CaptureState(), state) || !reflect.DeepEqual(reopened.presentation.CapturePlacement(), placement) || len(reopened.Windows()) != 3 {
		t.Fatal("saved session lost Fancy compact state or named window placement")
	}
	if presentationFamily(reopened.Skin()) != players.FancySkin || reopened.settings.Windows[players.DefaultSkin]["main"].X != 720 || countCommand(nextBackend, "loadfile") != 0 || reopened.Host.Model.EqualizerVisible || !reopened.Host.Model.PlaylistVisible {
		t.Fatal("saved Fancy session merged families, autoplayed or forgot panel preferences")
	}
	for _, named := range reopened.Windows() {
		off := named.Window.Surface().(*platform.Offscreen)
		if named.Name != "main" && (off.WindowRole() != platform.RoleUtility || off.Owner() != reopened.MainWindow().Surface() || !off.SkipsTaskbar()) {
			t.Fatalf("restored %s lost utility ownership/taskbar policy", named.Name)
		}
	}
}
