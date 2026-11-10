package winamp

import (
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/style"
)

// Sharing windows (Wayland, or a group hosted by choice), the strip, the
// equaliser and the playlist are one window named once, as big as the stack
// with each panel's frame; the window itself has no frame, each panel
// wearing the skin's frame for its role; the toggles hide and show a panel
// in place; and the stack is saved with its bonds, unpositioned.
func TestSharedWindowHoldsTheFramedStack(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, skin)
			a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
			h := players.NewHost(a)
			h.Packs = func() []string { return Skins }
			p, err := New(h, Options{Headless: true, Scale: 1.75, Group: rack.GroupHosted})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			a.PumpOnce()
			if p.Eq() != p.Main() || p.List() != p.Main() {
				t.Fatal("the panels are not in one window")
			}
			if ws := p.Windows(); len(ws) != 1 || ws[0].Name != "strip" {
				t.Fatalf("windows %+v", ws)
			}
			if w, h := p.Main().Size(); w != StripW || h != StripH+EqH+ListH {
				t.Fatalf("the shared window is %dx%d", w, h)
			}
			if p.Main().Caption() != nil {
				t.Fatal("the shared window wears a frame of its own over the panels' frames")
			}
			if !strings.Contains(p.Status(), "stacked") {
				t.Fatalf("status %q", p.Status())
			}
			p.toggle(p.iList)
			a.PumpOnce()
			if p.Host.Model.PlaylistVisible || p.strip.listBtn.Checked {
				t.Fatal("the playlist toggle did not hide the playlist")
			}
			if w, h := p.Main().Size(); w != StripW || h != StripH+EqH {
				t.Fatalf("without the playlist the window is %dx%d", w, h)
			}
			p.toggle(p.iList)
			a.PumpOnce()
			if !p.Host.Model.PlaylistVisible || p.List() != p.Main() {
				t.Fatal("the playlist toggle did not show the playlist in place")
			}
			saved := p.CapturePlacement()
			if eq := saved["equaliser"]; eq.Positioned || eq.Dock == nil || eq.Dock.Parent != "main" || eq.Dock.Side != "bottom" {
				t.Fatalf("the equaliser was saved as %+v", eq)
			}
		})
	}
}
