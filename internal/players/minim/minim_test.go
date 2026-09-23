package minim

import (
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// open is a headless player in a named pack at a scale.
func open(t *testing.T, pack string, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, pack)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	// The face on its own, with a model of its own: a host with no
	// application behind it is what a face's own tests open it with.
	h := players.NewHost(a)
	h.Packs = func() []string { return Skins }
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, w := range []*app.Window{p.Main, p.Eq, p.List} {
			if w != nil {
				w.Close()
			}
		}
	})
	a.PumpOnce()
	return a, p
}

// The skin is a pack and the app asks for it by id, like any other.
func TestTheSkinIsThere(t *testing.T) {
	if !style.IsSkin(Skin) {
		t.Fatalf("%q is not a skin", Skin)
	}
	sk, ok := style.LoadSkin(Skin)
	if !ok {
		t.Fatalf("%q did not load", Skin)
	}
	if sk.Window == nil || len(sk.Window.Shape) == 0 {
		t.Fatal("the skin declares no window shape")
	}
	if !sk.Design.Pixelated {
		t.Error("the compact player's skin is meant to be a pixel sheet")
	}
	for _, sh := range sk.Sheets {
		if !sh.Covers(2) {
			t.Errorf("sheet %s has no asset at 2x", sh.Name)
		}
	}
}

// Every one of the three windows passes the audit, in the skin and with the
// skin dropped.
func TestEveryWindowIsAccessible(t *testing.T) {
	for _, pack := range []string{Skin, "breeze-night"} {
		t.Run(pack, func(t *testing.T) {
			a, p := open(t, pack, 1)
			for name, w := range map[string]*app.Window{
				"strip": p.Main, "equaliser": p.Eq, "playlist": p.List,
			} {
				tree := playertest.Audit(t, name, w)
				if playertest.Count(tree, a11y.RoleWindow) == 0 {
					t.Errorf("%s: no window node", name)
				}
			}
			// The strip's controls are all named: a transport row is
			// glyphs, so an unnamed one is a button a screen reader calls
			// "button" and nothing else.
			tree := p.Main.AccessibleTree()
			for _, want := range []string{"Play", "Stop", "Next track", "Previous track", "Seek", "Volume"} {
				if !hasNamed(tree, want) {
					t.Errorf("the strip has no node named %q", want)
				}
			}
			if playertest.Count(p.Eq.AccessibleTree(), a11y.RoleSlider) < len(players.EqBands)+1 {
				t.Errorf("the equaliser has %d sliders, want %d",
					playertest.Count(p.Eq.AccessibleTree(), a11y.RoleSlider), len(players.EqBands)+1)
			}
			_ = a
		})
	}
}

// Everything the pointer can work, the keyboard can reach.
func TestEveryControlIsOnTheKeyboard(t *testing.T) {
	a, p := open(t, Skin, 1)
	ring := playertest.FocusRing(t, a, p.Main)
	if len(ring) == 0 {
		t.Fatal("nothing in the strip takes the focus")
	}
	for _, c := range []widget.Component{
		p.strip.prev, p.strip.play, p.strip.stop, p.strip.next,
		p.strip.mute, p.strip.seek, p.strip.volume,
		p.strip.shuffle, p.strip.repeat, p.strip.eqBtn, p.strip.listBtn, p.strip.skinBtn,
	} {
		if !playertest.Reaches(ring, c) {
			t.Errorf("Tab never reaches %v (%q)", c.Name(), nameOf(p.Main, c))
		}
	}
	// And the equaliser's eleven faders.
	eqRing := playertest.FocusRing(t, a, p.Eq)
	if len(eqRing) < len(players.EqBands)+1 {
		t.Errorf("the equaliser's focus ring has %d stops, want at least %d",
			len(eqRing), len(players.EqBands)+1)
	}
}

// The shared transport keys work wherever the focus is, in every window.
func TestTheTransportKeysWorkInEveryWindow(t *testing.T) {
	a, p := open(t, Skin, 1)
	for _, w := range []*app.Window{p.Main, p.Eq, p.List} {
		p.Transport.State = players.Stopped
		a.PumpOnce()
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
		a.PumpOnce()
		if p.Transport.State != players.Playing {
			t.Errorf("%s: space did not start the transport (%v)", w.Title(), p.Transport.State)
		}
	}
	// And the keys that are letters.
	before := p.Transport.List.Index()
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
	a.PumpOnce()
	if p.Transport.List.Index() == before {
		t.Error("B did not move to the next track")
	}
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyM})
	a.PumpOnce()
	if !p.Transport.Muted() {
		t.Error("M did not mute")
	}
}

// The window is cut to the skin's outline, and the outline is the same
// shape at every scale rather than a picture of one.
//
// The comparison is of *fractions* of the window rather than of pixels,
// which is the thing to prove: a silhouette stated in design pixels and
// resolved against the window is the same outline at 1.75 as at 1, and a
// bitmap stretched to fit would not be.
func TestTheStripStandsOnItsChinAtEveryScale(t *testing.T) {
	const n = 64
	var base []float64
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		a, p := open(t, Skin, scale)
		_ = a
		if !p.Main.ShapeActive() {
			t.Fatalf("%.2fx: the strip is not shaped", scale)
		}
		out := playertest.Outline(p.Main, n)
		if len(out) != n {
			t.Fatalf("%.2fx: no silhouette", scale)
		}
		if base == nil {
			base = out
			if out[0] < 0.999 || out[n/2] < 0.999 {
				t.Errorf("the strip is not full width at the top (%.3f) or the middle (%.3f)", out[0], out[n/2])
			}
			// The chin steps in twice: the last row is narrower than the
			// body by twice the deepest step, and the row before the chin
			// begins is narrower than the body but wider than the last.
			last := out[n-1]
			if last > 0.90 {
				t.Errorf("the strip's last row covers %.3f of the width — that is not a chin", last)
			}
			step := out[n-5]
			if step <= last || step >= 1 {
				t.Errorf("the chin does not step in twice: %.3f then %.3f then 1.000", last, step)
			}
			continue
		}
		if d := playertest.OutlineDiff(base, out); d > 0.02 {
			t.Errorf("%.2fx: the outline differs from 1x by %.3f of the window's width\n  1x:    %.3v\n  %.2fx: %.3v",
				scale, d, base, scale, out)
		}
	}
}

// With the skin dropped the app is an ordinary themed app: no silhouette,
// every control still a control, and a window that paints.
func TestTheThemedFallbackIsAWholeApp(t *testing.T) {
	a, p := open(t, "breeze-night", 1)
	if p.Main.ShapeActive() {
		t.Error("breeze-night framed a shaped window")
	}
	img := p.Main.Capture()
	if img == nil || img.Width < 10 || img.Height < 10 {
		t.Fatal("the themed strip painted nothing")
	}
	if blank(img) {
		t.Error("the themed strip is one flat colour")
	}
	ring := playertest.FocusRing(t, a, p.Main)
	if len(ring) < 8 {
		t.Errorf("the themed strip has %d keyboard stops, want at least 8", len(ring))
	}
	playertest.Audit(t, "themed strip", p.Main)
}

// The three windows travel as one: the equaliser and the playlist are
// bonded under the strip, and moving the strip moves all three.
func TestTheStackTravelsWithTheStrip(t *testing.T) {
	a, p := open(t, Skin, 1)
	if !p.Desk.Places() {
		t.Skip("this backend does not place windows")
	}
	eq := p.Desk.Rack.Pane(p.iEq)
	list := p.Desk.Rack.Pane(p.iList)
	if eq.To != p.iMain || eq.Bond.Side != rack.SideBottom {
		t.Fatalf("the equaliser is bonded to %d on the %v", eq.To, eq.Bond.Side)
	}
	if list.To != p.iEq {
		t.Fatalf("the playlist is bonded to %d, want the equaliser (%d)", list.To, p.iEq)
	}

	// Move the strip the way a window manager would, and let the desk
	// notice on its next look.
	p.Main.Move(400, 260)
	a.PumpOnce()
	p.Desk.Follow()
	a.PumpOnce()

	main := p.Desk.Rack.Pane(p.iMain).Box
	if main.X != 400 || main.Y != 260 {
		t.Fatalf("the strip is at %+v", main)
	}
	if eq.Box.X != main.X || eq.Box.Y != main.Bottom() {
		t.Errorf("the equaliser is at %+v, want flush under %+v", eq.Box, main)
	}
	if list.Box.X != eq.Box.X || list.Box.Y != eq.Box.Bottom() {
		t.Errorf("the playlist is at %+v, want flush under %+v", list.Box, eq.Box)
	}
	// And the desktop was actually told.
	if x, y, ok := p.Eq.Position(); !ok || x != eq.Box.X || y != eq.Box.Y {
		t.Errorf("the equaliser's window is at %d,%d, the rack says %+v", x, y, eq.Box)
	}
}

// A window dragged away comes loose, and snaps back when it is let go near
// an edge again.
func TestAWindowComesLooseAndSnapsBack(t *testing.T) {
	a, p := open(t, Skin, 1)
	if !p.Desk.Places() {
		t.Skip("this backend does not place windows")
	}
	p.List.Move(900, 700)
	a.PumpOnce()
	p.Desk.Follow()
	if pane := p.Desk.Rack.Pane(p.iList); pane.To != -1 {
		t.Fatalf("the playlist stayed bonded to %d after being dragged away", pane.To)
	}
	// Let it go four pixels off flush under the equaliser.
	eq := p.Desk.Rack.Pane(p.iEq).Box
	p.List.Move(eq.X+4, eq.Bottom()+3)
	a.PumpOnce()
	p.Desk.Follow()
	pane := p.Desk.Rack.Pane(p.iList)
	if pane.To != p.iEq || pane.Bond.Side != rack.SideBottom {
		t.Fatalf("the playlist did not snap back: bonded to %d on the %v", pane.To, pane.Bond.Side)
	}
	if pane.Box.X != eq.X || pane.Box.Y != eq.Bottom() {
		t.Errorf("snapped to %+v, want flush at %d,%d", pane.Box, eq.X, eq.Bottom())
	}
}

// The clock, the seek bar and the analyser agree with one another.
func TestTheDisplayFollowsTheTransport(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Pose(97 * time.Second)
	a.PumpOnce()
	if got := p.strip.seek.Value; got < 0.4 || got > 0.5 {
		t.Errorf("the seek bar reads %.3f at 1:37 of 3:47", got)
	}
	loud := false
	for _, v := range p.Spectrum.Bars {
		if v > 0.05 {
			loud = true
		}
	}
	if !loud {
		t.Error("the analyser is flat while the player is posed as playing")
	}
	tree := p.Main.AccessibleTree()
	if !hasNamedContaining(tree, "1:37") {
		t.Error("the display does not read the time out to a screen reader")
	}
}

// ---- helpers ------------------------------------------------------------------

func hasNamed(tree *a11y.Node, name string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if n.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func hasNamedContaining(tree *a11y.Node, part string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if strings.Contains(n.Name, part) {
			found = true
		}
		return !found
	})
	return found
}

func nameOf(w *app.Window, c widget.Component) string {
	if n := w.AccessibleTree().Find(c.ID()); n != nil {
		return n.Name
	}
	return ""
}

// blank reports whether an image is one flat colour, which is what a window
// that painted nothing looks like.
func blank(img *paintengine2d.Image) bool {
	r0, g0, b0, _ := img.PremulAt(0, 0)
	for y := 0; y < img.Height; y += 3 {
		for x := 0; x < img.Width; x += 3 {
			r, g, b, _ := img.PremulAt(x, y)
			if r != r0 || g != g0 || b != b0 {
				return false
			}
		}
	}
	return true
}
