package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
)

// A user override with only the main artwork must fail before changing the
// existing windows or the last working appearance.
func TestPodRequiresPlaylistLayoutBeforeReplacingPresentation(t *testing.T) {
	a, s := open(t, players.WingspanSkin)
	w, model := s.MainWindow(), s.Host.Model
	dir := skins.UserSkinDir(players.PodSkin)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"skin":1,"label":"Xbox main only","base":"dark","layouts":{"pod.main":{"size":[350,216]}}}`
	if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	skins.InvalidateSkinCache()
	t.Cleanup(skins.InvalidateSkinCache)
	if err := s.SetSkin(players.PodSkin); err == nil || !strings.Contains(err.Error(), `missing layout "pod.playlist"`) {
		t.Fatalf("Xbox main-only override: %v", err)
	}
	if s.MainWindow() != w || w.Closed() || s.Skin() != players.WingspanSkin || s.Host.Model != model {
		t.Fatal("invalid Xbox choice damaged the current presentation")
	}
	if err := s.SetSkin(players.DefaultSkin); err != nil {
		t.Fatal(err)
	}
	s.Host.DropSkin()
	a.PumpOnce()
	if s.Skin() != players.WingspanSkin {
		t.Fatal("invalid Xbox choice replaced the last working skin")
	}
}

func TestPodSkinKeysCycleFromWingspanAndRememberChoice(t *testing.T) {
	a, s := open(t, players.WingspanSkin)
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
		{platform.ModCtrl, players.PodSkin},
		{platform.ModCtrl | platform.ModShift, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.PodSkin},
	} {
		key(s.MainWindow(), tc.mods)
		if s.Skin() != tc.want || s.Host.Model != model {
			t.Fatalf("skin keys chose %s, want %s around the same model", s.Skin(), tc.want)
		}
	}
	for _, named := range s.Windows() {
		if named.Name != "playlist" {
			continue
		}
		key(named.Window, platform.ModCtrl|platform.ModShift)
		if s.Skin() != players.DefaultSkin || !named.Window.Closed() {
			t.Fatal("Pod playlist key did not replace its family with Default")
		}
		key(s.MainWindow(), platform.ModCtrl|platform.ModShift)
		if s.Skin() != players.PodSkin {
			t.Fatal("Default forgot Xbox as the last optional skin")
		}
		return
	}
	t.Fatal("Pod playlist did not receive the shared skin shortcuts")
}
