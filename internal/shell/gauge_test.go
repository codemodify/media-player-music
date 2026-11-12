package shell

import (
	"github.com/codemodify/media-player-music/internal/playback"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
)

func TestGaugeRequiresBothSatelliteLayoutsBeforeReplacingPresentation(t *testing.T) {
	for _, missing := range []string{"gauge.equalizer", "gauge.playlist"} {
		t.Run(missing, func(t *testing.T) {
			a, s := open(t, players.PodSkin)
			w, model := s.MainWindow(), s.Host.Model
			dir := skins.UserSkinDir(players.GaugeSkin)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			layouts := `"gauge.main":{"size":[339,300]}`
			if missing == "gauge.playlist" {
				layouts += `,"gauge.equalizer":{"size":[430,239]}`
			}
			manifest := `{"skin":1,"label":"Incomplete Half Life 2","base":"dark","layouts":{` + layouts + `}}`
			if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if err := s.SetSkin(players.GaugeSkin); err == nil || !strings.Contains(err.Error(), `missing layout "`+missing+`"`) {
				t.Fatalf("incomplete Half Life 2 override: %v", err)
			}
			if s.MainWindow() != w || w.Closed() || s.Skin() != players.PodSkin || s.Host.Model != model {
				t.Fatal("invalid Half Life 2 choice damaged the current presentation")
			}
			if err := s.SetSkin(players.DefaultSkin); err != nil {
				t.Fatal(err)
			}
			s.Host.DropSkin()
			a.PumpOnce()
			if s.Skin() != players.PodSkin {
				t.Fatal("invalid Half Life 2 choice replaced the last working skin")
			}
		})
	}
}

func TestGaugeSkinKeysCycleFromPodAndRememberChoice(t *testing.T) {
	a, s := open(t, players.PodSkin)
	model := s.Host.Model
	key := func(w *ui.Window, mods platform.Modifiers) {
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Mods: mods})
		a.PumpOnce()
		a.PumpOnce()
	}
	for _, tc := range []struct {
		mods platform.Modifiers
		want string
	}{
		{platform.ModCtrl, players.GaugeSkin},
		{platform.ModCtrl | platform.ModShift, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.GaugeSkin},
	} {
		key(s.MainWindow(), tc.mods)
		if s.Skin() != tc.want || s.Host.Model != model {
			t.Fatalf("skin keys chose %s, want %s around the same model", s.Skin(), tc.want)
		}
	}
	for i := 1; i < len(s.Windows()); i++ {
		w := s.Windows()[i].Window
		key(w, platform.ModCtrl|platform.ModShift)
		if s.Skin() != players.DefaultSkin || !w.Closed() {
			t.Fatal("Half Life 2 satellite key did not replace its family with Default")
		}
		key(s.MainWindow(), platform.ModCtrl|platform.ModShift)
		if s.Skin() != players.GaugeSkin {
			t.Fatal("Default forgot Half Life 2 as the last optional skin")
		}
	}
}

func TestGaugeSessionKeepsAllWindowsSeparateFromOtherFamilies(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, backend := rememberedShell(t, players.DefaultSkin)
	settle := func() {
		s.Pulse().Stop()
		a.PumpOnce()
		s.presentation.Advance(0)
	}
	positions := []struct {
		id   string
		x, y int
	}{
		{players.DefaultSkin, 720, 80},
		{players.SlateSkin, 380, 230},
		{players.WingspanSkin, 510, 340},
		{players.PodSkin, 850, 400},
		{players.GaugeSkin, 620, 230},
	}
	for _, pos := range positions {
		if err := s.SetSkin(pos.id); err != nil {
			t.Fatal(err)
		}
		settle()
		desktopMove(t, a, s, s.MainWindow(), pos.x, pos.y)
		s.presentation.Advance(0)
	}
	for _, named := range s.Windows() {
		switch named.Name {
		case "equaliser":
			desktopMove(t, a, s, named.Window, 700, 530)
		case "playlist":
			desktopMove(t, a, s, named.Window, 1080, 290)
			named.Window.SetSize(610, 430)
			// The group takes the new size from the window's resize, two turns on.
			playertest.Follow(a)
		}
	}
	settle()
	s.presentation.ShowEqualiser(false)
	s.presentation.ShowPlaylist(false)
	for _, pos := range positions {
		if err := s.SetSkin(pos.id); err != nil {
			t.Fatal(err)
		}
		settle()
		if x, y, known := s.MainWindow().Position(); !known || x != pos.x || y != pos.y {
			t.Fatalf("%s placement crossed families: %d,%d,%t", pos.id, x, y, known)
		}
	}
	if len(s.settings.Windows) != len(positions) || countCommand(backend, playback.OpLoad) != 0 {
		t.Fatal("window family state was merged or started playback")
	}
	s.Close()
	reopenedApp, reopened, nextBackend := rememberedShell(t, "")
	awaitSettled(t, reopenedApp, reopened)
	if reopened.Skin() != players.GaugeSkin || len(reopened.Windows()) != 3 || countCommand(nextBackend, playback.OpLoad) != 0 {
		t.Fatal("Half Life 2 session did not reopen all windows without autoplay")
	}
	seen := map[string]bool{}
	for _, named := range reopened.Windows() {
		seen[named.Name] = true
		x, y, known := named.Window.Position()
		width, height := named.Window.Size()
		switch named.Name {
		case "main":
			if !known || x != 620 || y != 230 || !named.Window.Visible() {
				t.Fatal("Half Life 2 main lost its own placement")
			}
		case "equaliser":
			if !known || x != 700 || y != 530 || named.Window.Visible() || reopened.Host.Model.EqualizerVisible {
				t.Fatal("Half Life 2 equaliser lost placement or its hidden preference")
			}
		case "playlist":
			if !known || x != 1080 || y != 290 || width != 610 || height != 430 || named.Window.Visible() || reopened.Host.Model.PlaylistVisible {
				t.Fatalf("Gauge playlist lost placement, size or visibility: %d,%d,%t %dx%d", x, y, known, width, height)
			}
		default:
			t.Fatalf("unexpected Half Life 2 window %q", named.Name)
		}
	}
	if !seen["main"] || !seen["equaliser"] || !seen["playlist"] {
		t.Fatal("Half Life 2 did not retain all named player windows")
	}
}
