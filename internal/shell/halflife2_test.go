package shell

import (
	"github.com/codemodify/media-player-music/internal/playback"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
)

func TestHalfLife2RequiresBothSatelliteLayoutsBeforeReplacingPresentation(t *testing.T) {
	for _, missing := range []string{"halflife2.equalizer", "halflife2.playlist"} {
		t.Run(missing, func(t *testing.T) {
			a, s := open(t, players.XboxSkin)
			w, model := s.MainWindow(), s.Host.Model
			dir := skins.UserSkinDir(players.HalfLife2Skin)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			layouts := `"halflife2.main":{"size":[339,300]}`
			if missing == "halflife2.playlist" {
				layouts += `,"halflife2.equalizer":{"size":[430,239]}`
			}
			manifest := `{"skin":1,"label":"Incomplete Half Life 2","base":"dark","layouts":{` + layouts + `}}`
			if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if err := s.SetSkin(players.HalfLife2Skin); err == nil || !strings.Contains(err.Error(), `missing layout "`+missing+`"`) {
				t.Fatalf("incomplete Half Life 2 override: %v", err)
			}
			if s.MainWindow() != w || w.Closed() || s.Skin() != players.XboxSkin || s.Host.Model != model {
				t.Fatal("invalid Half Life 2 choice damaged the current presentation")
			}
			if err := s.SetSkin(players.DefaultSkin); err != nil {
				t.Fatal(err)
			}
			s.Host.DropSkin()
			a.PumpOnce()
			if s.Skin() != players.XboxSkin {
				t.Fatal("invalid Half Life 2 choice replaced the last working skin")
			}
		})
	}
}

func TestHalfLife2SkinKeysCycleFromXboxAndRememberChoice(t *testing.T) {
	a, s := open(t, players.XboxSkin)
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
		{platform.ModCtrl, players.HalfLife2Skin},
		{platform.ModCtrl | platform.ModShift, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.HalfLife2Skin},
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
		if s.Skin() != players.HalfLife2Skin {
			t.Fatal("Default forgot Half Life 2 as the last optional skin")
		}
	}
}

func TestHalfLife2SessionKeepsAllWindowsSeparateFromOtherFamilies(t *testing.T) {
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
		{players.WinampSkin, 380, 230},
		{players.HeadspaceSkin, 510, 340},
		{players.XboxSkin, 850, 400},
		{players.HalfLife2Skin, 620, 230},
	}
	for _, pos := range positions {
		if err := s.SetSkin(pos.id); err != nil {
			t.Fatal(err)
		}
		settle()
		s.MainWindow().Move(pos.x, pos.y)
		s.presentation.Advance(0)
	}
	for _, named := range s.Windows() {
		switch named.Name {
		case "equaliser":
			named.Window.Move(700, 530)
		case "playlist":
			named.Window.Move(1080, 290)
			named.Window.SetSize(610, 430)
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
	_, reopened, nextBackend := rememberedShell(t, "")
	if reopened.Skin() != players.HalfLife2Skin || len(reopened.Windows()) != 3 || countCommand(nextBackend, playback.OpLoad) != 0 {
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
				t.Fatalf("Half Life 2 playlist lost placement, size or visibility: %d,%d,%t %dx%d", x, y, known, width, height)
			}
		default:
			t.Fatalf("unexpected Half Life 2 window %q", named.Name)
		}
	}
	if !seen["main"] || !seen["equaliser"] || !seen["playlist"] {
		t.Fatal("Half Life 2 did not retain all named player windows")
	}
}
