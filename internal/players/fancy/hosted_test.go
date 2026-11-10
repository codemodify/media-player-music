package fancy

import (
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/style"
)

// Sharing windows (Wayland, or a group hosted by choice), the stack is one
// shaped window named once; the compact player swaps the player's content
// and its window shrinks with it, the stack moving up; the keys know which
// panel has the focus.
func TestSharedWindowHoldsTheStackAndTheCompactPlayer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, players.FancySkin)
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
	if w, h := p.Main().Size(); w != MainW || h != MainH+EqH+ListH {
		t.Fatalf("the shared window is %dx%d", w, h)
	}
	p.SetCompact(true)
	a.PumpOnce()
	if w, h := p.Main().Size(); w != MainW || h != CompactH+EqH+ListH {
		t.Fatalf("compact, the shared window is %dx%d", w, h)
	}
	if eq := p.g.Rack.Pane(p.iEq).Box; eq.Y != p.g.Rack.Pane(p.iMain).Box.Y+CompactH {
		t.Fatal("the equaliser did not move up under the compact player")
	}
	if p.compact.Parent() == nil || p.body.Parent() != nil {
		t.Fatal("the compact player is not the one shown")
	}
	p.SetCompact(false)
	a.PumpOnce()
	if w, h := p.Main().Size(); w != MainW || h != MainH+EqH+ListH || p.body.Parent() == nil {
		t.Fatalf("back to full, the shared window is %dx%d", w, h)
	}
	p.Main().RequestFocus(p.listPane.list)
	if !p.inPlaylist(p.Main()) {
		t.Fatal("the focus in the playlist's pane was not taken for the playlist")
	}
	p.Main().RequestFocus(p.body.play)
	if p.inPlaylist(p.Main()) {
		t.Fatal("the focus on the player was taken for the playlist")
	}
}
