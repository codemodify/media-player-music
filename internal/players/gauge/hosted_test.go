package gauge

import (
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/style"
)

// Sharing windows (Wayland, or a group hosted by choice), the three panels
// are one shaped window in the source skin's arrangement, named once; the
// keys know which panel has the focus; the shutter recuts the window; a
// panel closed by the desktop's word hides, and the model says so.
func TestSharedWindowHoldsTheSourceArrangement(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, players.GaugeSkin)
	a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.SkinnedIDs
	p, err := New(h, Options{Headless: true, Scale: 1.75, Group: rack.GroupHosted})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	if p.Eq() != p.Main() || p.List() != p.Main() || len(p.Windows()) != 1 {
		t.Fatalf("the panels are not in one window: %+v", p.Windows())
	}
	// The equaliser 80 up the player's right edge, the playlist under it.
	if w, h := p.Main().Size(); w != MainW+EqW || h != EqH+ListH {
		t.Fatalf("the shared window is %dx%d", w, h)
	}
	if !p.Main().ShapeActive() {
		t.Fatal("the shared window is not cut to the panels' artwork")
	}
	p.Main().RequestFocus(p.listPane.list)
	if !p.inPlaylist(p.Main()) {
		t.Fatal("the focus in the playlist's pane was not taken for the playlist")
	}
	p.Main().RequestFocus(p.eqPane.bands[0])
	if p.inPlaylist(p.Main()) {
		t.Fatal("the focus on the equaliser was taken for the playlist")
	}
	before := p.g.Rack.Pane(p.iMain).Box
	p.body.cycleShutter()
	a.PumpOnce()
	if p.g.Rack.Pane(p.iMain).Box != before || !p.Main().ShapeActive() {
		t.Fatal("the shutter moved the player or lost the window's cut")
	}
	p.ShowEqualiser(false)
	a.PumpOnce()
	if p.Host.Model.EqualizerVisible || p.Eq() != nil || p.List() != p.Main() {
		t.Fatal("hiding the equaliser did not keep the playlist in the player's window")
	}
	p.ShowEqualiser(true)
	a.PumpOnce()
	if p.Eq() != p.Main() || !p.Host.Model.EqualizerVisible {
		t.Fatal("the equaliser did not come back into the player's window")
	}
	saved := p.CapturePlacement()
	if eq := saved["equaliser"]; eq.Positioned || eq.Dock == nil || eq.Dock.Parent != "main" || eq.Dock.Along != -80 {
		t.Fatalf("the shared equaliser was saved as %+v", eq)
	}
}
