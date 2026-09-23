package marquee

import (
	"testing"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func open(t *testing.T, pack string, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, pack)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	p, err := New(a, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Window.Close() })
	a.PumpOnce()
	return a, p
}

func TestTheSkinIsThere(t *testing.T) {
	sk, ok := style.LoadSkin(Skin)
	if !ok {
		t.Fatalf("%q did not load", Skin)
	}
	if sk.Window == nil || len(sk.Window.Shape) == 0 {
		t.Fatal("the skin declares no window shape")
	}
	if sk.Design.Pixelated {
		t.Error("the cabinet's skin is drawn from paths; it should not be pixelated")
	}
	for _, sh := range sk.Sheets {
		if !sh.Covers(2) {
			t.Errorf("sheet %s has no asset at 2x", sh.Name)
		}
	}
}

func TestTheCabinetIsAccessible(t *testing.T) {
	for _, pack := range []string{Skin, "breeze-night"} {
		t.Run(pack, func(t *testing.T) {
			a, p := open(t, pack, 1)
			tree := playertest.Audit(t, "cabinet", p.Window)
			for _, want := range []string{"Play", "Stop", "Seek", "Volume", "Queue", "Compact mode"} {
				if !named(tree, want) {
					t.Errorf("no node named %q", want)
				}
			}
			if playertest.Count(tree, a11y.RoleList) == 0 {
				t.Error("the queue is not a list in the tree")
			}
			// Folded, and audited again: the controls that went away must
			// be gone from the tree rather than still in it and invisible.
			p.SetCompact(true)
			a.PumpOnce()
			folded := playertest.Audit(t, "folded", p.Window)
			if named(folded, "Stop") {
				t.Error("the stop button is still in the tree when the player is folded")
			}
			if !named(folded, "Full mode") {
				t.Error("the folded player has no way back")
			}
		})
	}
}

func TestEveryControlIsOnTheKeyboardInBothModes(t *testing.T) {
	a, p := open(t, Skin, 1)
	full := playertest.FocusRing(t, a, p.Window)
	for _, c := range []widget.Component{
		p.body.prev, p.body.play, p.body.stop, p.body.next,
		p.body.mute, p.body.seek, p.body.volume,
		p.body.shuffle, p.body.repeat, p.body.fold, p.body.queue,
	} {
		if !playertest.Reaches(full, c) {
			t.Errorf("Tab never reaches %T in the cabinet", c)
		}
	}
	p.SetCompact(true)
	a.PumpOnce()
	folded := playertest.FocusRing(t, a, p.Window)
	if len(folded) >= len(full) {
		t.Errorf("folding did not take anything out of the tab order (%d then %d)", len(full), len(folded))
	}
	for _, c := range []widget.Component{p.body.play, p.body.seek, p.body.volume, p.body.fold} {
		if !playertest.Reaches(folded, c) {
			t.Errorf("Tab never reaches %T when the player is folded", c)
		}
	}
	if playertest.Reaches(folded, p.body.stop) {
		t.Error("Tab reaches the stop button, which the folded player does not show")
	}
}

// Folding changes three things at once, and this is the one test that says
// so: the size, the layout, and whose outline the window has.
func TestFoldingChangesTheSizeTheLayoutAndTheShape(t *testing.T) {
	a, p := open(t, Skin, 1)

	fullW, fullH := p.Window.Size()
	fullProfile := playertest.Profile(p.Window, 4)
	if !p.Window.ShapeActive() {
		t.Fatal("the cabinet is not shaped")
	}
	// The cabinet's own outline is the skin's: the top rows are narrower
	// than the middle (the brow) and so are the bottom ones (the dome).
	mid := fullProfile[len(fullProfile)/2]
	if fullProfile[0] >= mid {
		t.Errorf("the brow does not step the top corners in (%d then %d)", fullProfile[0], mid)
	}
	if last := fullProfile[len(fullProfile)-1]; last >= mid {
		t.Errorf("the cabinet does not stand on a dome (%d at the bottom, %d in the middle)", last, mid)
	}
	seekFull := p.body.seek.Bounds()

	p.SetCompact(true)
	a.PumpOnce()
	w, h := p.Window.Size()
	if w >= fullW || h >= fullH {
		t.Errorf("folded to %dx%d, which is not smaller than %dx%d", w, h, fullW, fullH)
	}
	if !p.Window.ShapeActive() {
		t.Fatal("the folded player lost its outline")
	}
	// A stadium: widest across its waist, and its first and last rows are
	// much narrower than that.
	prof := playertest.Profile(p.Window, 1)
	widest := playertest.Widest(prof)
	if widest < w-2 {
		t.Errorf("the folded player's widest row is %d of %d", widest, w)
	}
	// A stadium's first row is short by twice its radius, less the sliver
	// the curve gives back: half the window's height is the radius here,
	// so the top row is the best part of a hundred pixels narrower.
	if widest-prof[0] < h/2 {
		t.Errorf("the folded player's top row covers %d of %d — that is not a stadium", prof[0], widest)
	}
	if seekFull == p.body.seek.Bounds() {
		t.Error("the seek bar did not move: the layout did not change")
	}

	p.SetCompact(false)
	a.PumpOnce()
	if w, h := p.Window.Size(); w != fullW || h != fullH {
		t.Errorf("unfolded to %dx%d, want %dx%d", w, h, fullW, fullH)
	}
	back := playertest.Profile(p.Window, 4)
	if len(back) != len(fullProfile) || back[0] != fullProfile[0] {
		t.Error("the cabinet did not get its own outline back")
	}
}

// The look's outline is the same shape at every scale, and it is the shape
// it is meant to be: a brow at the top and a dome at the bottom.
//
// The comparison is of *fractions* of the window rather than of pixels,
// which is what "stated in design pixels and resolved against the window"
// buys — a bitmap stretched to fit would not hold this.
func TestTheCabinetKeepsItsShapeAtEveryScale(t *testing.T) {
	var base []float64
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		a, p := open(t, Skin, scale)
		_ = a
		if !p.Window.ShapeActive() {
			t.Fatalf("%.2fx: not shaped", scale)
		}
		const n = 64
		out := playertest.Outline(p.Window, n)
		if len(out) != n {
			t.Fatalf("%.2fx: no outline", scale)
		}
		if base == nil {
			base = out
			// What the outline is, stated as relations rather than as
			// numbers: the brow holds the top in, the middle is the whole
			// window, and the dome takes the bottom back off.
			mid := out[n/2]
			if mid < 0.99 {
				t.Errorf("the middle of the cabinet covers %.3f of the width", mid)
			}
			if out[0] > mid-0.05 {
				t.Errorf("the brow covers %.3f against the middle's %.3f", out[0], mid)
			}
			if last := out[n-1]; last > mid-0.03 {
				t.Errorf("the cabinet's last row covers %.3f against the middle's %.3f — that is not a dome", last, mid)
			}
			continue
		}
		if d := playertest.OutlineDiff(base, out); d > 0.02 {
			t.Errorf("%.2fx: the outline differs from 1x by %.3f of the window's width\n  1x:     %.3v\n  %.2fx: %.3v",
				scale, d, base, scale, out)
		}
	}
}

func TestTheThemedFallbackIsAWholeApp(t *testing.T) {
	a, p := open(t, "breeze-night", 1)
	if p.Window.ShapeActive() {
		t.Error("breeze-night framed a shaped window")
	}
	img := p.Window.Capture()
	if img == nil || flat(img) {
		t.Fatal("the themed cabinet painted nothing, or one flat colour")
	}
	if len(playertest.FocusRing(t, a, p.Window)) < 8 {
		t.Error("the themed cabinet has too few keyboard stops")
	}
	// And folding still works with no skin at all: the outline in that
	// mode is the app's own, so it does not depend on the pack.
	p.SetCompact(true)
	a.PumpOnce()
	if !p.Window.ShapeActive() {
		t.Error("the folded player has no outline in an ordinary theme")
	}
	playertest.Audit(t, "themed folded", p.Window)
}

func TestCtrlMFolds(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Window.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyM, Mods: platform.ModCtrl})
	a.PumpOnce()
	if !p.Compact() {
		t.Fatal("Ctrl+M did not fold the player")
	}
	p.Window.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyM, Mods: platform.ModCtrl})
	a.PumpOnce()
	if p.Compact() {
		t.Error("Ctrl+M did not unfold it again")
	}
}

func TestTheDisplayFollowsTheTransport(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Pose(97 * time.Second)
	a.PumpOnce()
	if got := p.body.seek.Value; got < 0.4 || got > 0.5 {
		t.Errorf("the seek bar reads %.3f at 1:37 of 3:47", got)
	}
	if p.Transport.Track().Title == "" {
		t.Fatal("no track")
	}
	tree := p.Window.AccessibleTree()
	if !namedContaining(tree, p.Transport.Track().Title) {
		t.Error("the display does not name the track to a screen reader")
	}
	if !namedContaining(tree, players.Disclaimer) {
		t.Error("nothing in the tree says this is a demo")
	}
}

// ---- helpers ------------------------------------------------------------------

func named(tree *a11y.Node, name string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if n.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func namedContaining(tree *a11y.Node, part string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if contains(n.Name, part) || contains(n.Description, part) {
			found = true
		}
		return !found
	})
	return found
}

func contains(hay, needle string) bool {
	return len(needle) > 0 && len(hay) >= len(needle) && indexOf(hay, needle) >= 0
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func flat(img *paintengine2d.Image) bool {
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
