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

func TestFandomRequiresItsPlaylistLayoutBeforeReplacingPresentation(t *testing.T) {
	a, s := open(t, players.GaugeSkin)
	w, model := s.MainWindow(), s.Host.Model
	dir := skins.UserSkinDir(players.FandomSkin)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"skin":1,"label":"Incomplete Fandom","base":"dark","layouts":{"fandom.main":{"size":[490,330]}}}`
	if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	skins.InvalidateSkinCache()
	t.Cleanup(skins.InvalidateSkinCache)
	if err := s.SetSkin(players.FandomSkin); err == nil || !strings.Contains(err.Error(), `missing layout "fandom.playlist"`) {
		t.Fatalf("incomplete Fandom override: %v", err)
	}
	if s.MainWindow() != w || w.Closed() || s.Skin() != players.GaugeSkin || s.Host.Model != model {
		t.Fatal("invalid Fandom choice damaged the active presentation")
	}
	if err := s.SetSkin(players.DefaultSkin); err != nil {
		t.Fatal(err)
	}
	s.Host.DropSkin()
	a.PumpOnce()
	if s.Skin() != players.GaugeSkin {
		t.Fatal("invalid Fandom choice replaced the last working skin")
	}
}

func TestFandomSkinKeysCycleFromGaugeAndRememberChoice(t *testing.T) {
	a, s := open(t, players.GaugeSkin)
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
		{platform.ModCtrl, players.FandomSkin},
		{platform.ModCtrl | platform.ModShift, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.FandomSkin},
	} {
		key(s.MainWindow(), tc.mods)
		if s.Skin() != tc.want || s.Host.Model != model {
			t.Fatalf("skin keys chose %s, want %s around the same model", s.Skin(), tc.want)
		}
	}
	if len(s.Windows()) != 2 {
		t.Fatal("Fandom should embed Settings/EQ and use one playlist satellite")
	}
	list := s.Windows()[1].Window
	key(list, platform.ModCtrl|platform.ModShift)
	if s.Skin() != players.DefaultSkin || !list.Closed() {
		t.Fatal("Fandom playlist key did not replace its family with Default")
	}
	key(s.MainWindow(), platform.ModCtrl|platform.ModShift)
	if s.Skin() != players.FandomSkin {
		t.Fatal("Default forgot Fandom as the last optional skin")
	}
}

func TestFandomSessionKeepsItsMainAndPlaylistSeparateFromEveryFamily(t *testing.T) {
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
		{players.FandomSkin, 600, 200},
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
		if named.Name == "playlist" {
			desktopMove(t, a, s, named.Window, 1140, 320)
			named.Window.SetSize(310, 356)
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
	if reopened.Skin() != players.FandomSkin || len(reopened.Windows()) != 2 || countCommand(nextBackend, playback.OpLoad) != 0 {
		t.Fatal("Fandom session did not reopen both windows without autoplay")
	}
	seen := map[string]bool{}
	for _, named := range reopened.Windows() {
		seen[named.Name] = true
		x, y, known := named.Window.Position()
		width, height := named.Window.Size()
		switch named.Name {
		case "main":
			if !known || x != 600 || y != 200 || width != 490 || height != 330 || !named.Window.Visible() {
				t.Fatal("Fandom main lost its own fixed geometry or placement")
			}
		case "playlist":
			if !known || x != 1140 || y != 320 || width != 310 || height != 356 || named.Window.Visible() || reopened.Host.Model.PlaylistVisible {
				t.Fatalf("Fandom playlist lost placement, size or visibility: %d,%d,%t %dx%d", x, y, known, width, height)
			}
			off := named.Window.Surface().(*platform.Offscreen)
			if off.WindowRole() != platform.RoleUtility || off.Owner() != reopened.MainWindow().Surface() || !off.SkipsTaskbar() {
				t.Fatal("restored Fandom playlist lost utility ownership/taskbar policy")
			}
		default:
			t.Fatalf("unexpected Fandom window %q", named.Name)
		}
	}
	if !seen["main"] || !seen["playlist"] || reopened.Host.Model.EqualizerVisible {
		t.Fatal("Fandom did not retain both named windows or the embedded EQ preference")
	}
}
