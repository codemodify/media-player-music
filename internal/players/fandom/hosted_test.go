package fandom

import (
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/style"
)

// Where the desktop places no windows (Wayland), the group shares one among
// the panels stuck together; a group hosted by choice is the same on the
// offscreen backend. The player and its playlist are one shaped window,
// named once, with the playlist down the player's right edge; the keys know
// which panel has the focus; hiding the playlist shrinks the window, and a
// saved layout comes back unpositioned with its bond.
func TestSharedWindowHoldsThePlayerAndItsPlaylist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, players.FandomSkin)
	a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.SkinnedIDs
	p, err := New(h, Options{Headless: true, Scale: 1.75, Group: rack.GroupHosted})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	if p.g.Mode() != rack.GroupHosted || p.List() != p.Main() {
		t.Fatalf("mode %v: the playlist is not in the player's window", p.g.Mode())
	}
	if ws := p.Windows(); len(ws) != 1 || ws[0].Name != "main" || ws[0].Window != p.MainWindow() {
		t.Fatalf("windows %+v", ws)
	}
	if w, h := p.Main().Size(); w != MainW+ListW || h != MainH {
		t.Fatalf("the shared window is %dx%d, want %dx%d", w, h, MainW+ListW, MainH)
	}
	if !p.Main().ShapeActive() {
		t.Fatal("the shared window is not cut to the panels' artwork")
	}
	p.Main().RequestFocus(p.listPane.list)
	if !p.inPlaylist(p.Main()) {
		t.Fatal("the focus in the playlist's pane was not taken for the playlist")
	}
	p.Main().RequestFocus(p.body.play)
	if p.inPlaylist(p.Main()) {
		t.Fatal("the focus on the player was taken for the playlist")
	}
	p.ShowPlaylist(false)
	a.PumpOnce()
	if w, h := p.Main().Size(); w != MainW || h != MainH || p.Host.Model.PlaylistVisible || p.List() != nil {
		t.Fatalf("hiding the playlist left the window %dx%d", w, h)
	}
	p.ShowPlaylist(true)
	a.PumpOnce()
	if p.List() != p.Main() || !p.Host.Model.PlaylistVisible {
		t.Fatal("the playlist did not come back into the player's window")
	}
	saved := p.CapturePlacement()
	if list := saved["playlist"]; list.Positioned || list.Dock == nil || list.Dock.Parent != "main" || list.Dock.Along != 46 {
		t.Fatalf("the shared playlist was saved as %+v", list)
	}
	// Saved loose, the playlist comes back in a window of its own.
	p.RestorePlacement(map[string]players.WindowPlacement{"playlist": {Width: 300, Height: 300}})
	a.PumpOnce()
	if p.List() == nil || p.List() == p.Main() || len(p.Windows()) != 2 {
		t.Fatal("the playlist saved loose did not come back in a window of its own")
	}
	if w, h := p.List().Size(); w != 300 || h != 300 {
		t.Fatalf("the loose playlist's window is %dx%d", w, h)
	}
}
