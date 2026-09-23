// Package playertest is what the three player demos' tests do to a window.
//
// It exists because all three ask a window the same four questions — is
// every control reachable from the keyboard, does the accessibility tree
// hold up, what does the silhouette actually look like at this scale, and
// does any of it change when the skin is dropped — and a helper copied into
// three test files is a helper that is right in one of them.
package playertest

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
)

// Audit runs a11y.Check over a window's tree and fails the test with
// everything a screen reader would stumble over. It returns the tree so a
// caller can ask its own questions of it.
func Audit(t *testing.T, name string, w *app.Window) *a11y.Node {
	t.Helper()
	tree := w.AccessibleTree()
	if tree == nil || len(tree.Children) == 0 {
		t.Fatalf("%s: empty accessibility tree", name)
	}
	var lines []string
	for _, p := range a11y.Check(tree) {
		lines = append(lines, p.String())
	}
	if len(lines) > 0 {
		t.Errorf("%s: %d accessibility problems:\n%s", name, len(lines), strings.Join(lines, "\n"))
	}
	return tree
}

// Count is how many nodes of a role the tree holds.
func Count(tree *a11y.Node, role a11y.Role) int {
	n := 0
	tree.Walk(func(x *a11y.Node) bool {
		if x.Role == role {
			n++
		}
		return true
	})
	return n
}

// Names are the accessible names of every node of a role.
func Names(tree *a11y.Node, role a11y.Role) []string {
	var out []string
	tree.Walk(func(x *a11y.Node) bool {
		if x.Role == role && x.Name != "" {
			out = append(out, x.Name)
		}
		return true
	})
	sort.Strings(out)
	return out
}

// FocusRing is every stop Tab visits in a window, in order, until it comes
// back to where it started. A window nothing can focus gives nil.
func FocusRing(t *testing.T, a *app.Application, w *app.Window) []widget.Component {
	t.Helper()
	tab := func() {
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyTab})
		a.PumpOnce()
	}
	tab()
	first := w.Focus()
	if first == nil {
		return nil
	}
	ring := []widget.Component{first}
	for i := 0; i < 2000; i++ {
		tab()
		c := w.Focus()
		if c == nil || c == first {
			return ring
		}
		ring = append(ring, c)
	}
	t.Fatalf("the focus ring never came back round (%d stops)", len(ring))
	return nil
}

// RingNames is what a screen reader would call each stop of a focus ring,
// in order.
func RingNames(w *app.Window, ring []widget.Component) []string {
	tree := w.AccessibleTree()
	out := make([]string, 0, len(ring))
	for _, c := range ring {
		name := ""
		if n := tree.Find(c.ID()); n != nil {
			name = n.Name
		}
		if name == "" {
			if a, ok := c.(interface{ AccessibleName() string }); ok {
				name = a.AccessibleName()
			}
		}
		if name == "" {
			name = fmt.Sprintf("<unnamed %T>", c)
		}
		out = append(out, name)
	}
	return out
}

// Reaches reports whether a focus ring contains a component.
func Reaches(ring []widget.Component, c widget.Component) bool {
	for _, x := range ring {
		if x == c {
			return true
		}
	}
	return false
}

// Profile is a silhouette read off a window as the width it covers on each
// row, in device pixels, sampled every step rows from the top.
//
// It is how a shape is checked without a golden image: the numbers say what
// the outline *is* — a strip that narrows at the bottom, a stadium that is
// widest in the middle — and they can be compared between scales after
// dividing out, which a picture cannot.
func Profile(w *app.Window, step int) []int {
	s := w.Shape()
	if s == nil {
		return nil
	}
	// A silhouette is rasterised in device pixels, which is what the
	// window's box is; Size is the logical one.
	ww, hh := w.PixelSize()
	r := s.Raster(ww, hh)
	if r == nil {
		return nil
	}
	if step < 1 {
		step = 1
	}
	out := make([]int, 0, hh/step+1)
	for y := 0; y < hh; y += step {
		out = append(out, coverage(r, y))
	}
	return out
}

// coverage is how many pixels of a row the silhouette covers.
func coverage(r *platform.ShapeRaster, y int) int {
	n := 0
	for _, rect := range r.Rects {
		if y >= rect.Y && y < rect.Y+rect.H {
			n += rect.W
		}
	}
	return n
}

// Outline is a silhouette as the *fraction* of the window's width it
// covers, sampled at n rows evenly down the window.
//
// It is the scale-free form of Profile, and it is what "the same outline at
// every scale" actually means: a shape stated in design pixels and resolved
// against the window gives the same fractions at 1 and at 1.75, while a
// bitmap stretched to fit does not. A test compares two of these rather
// than two pictures, so a failure says which row is wrong.
func Outline(w *app.Window, n int) []float64 {
	s := w.Shape()
	if s == nil || n < 2 {
		return nil
	}
	// A silhouette is rasterised in device pixels, which is what the
	// window's box is; Size is the logical one.
	ww, hh := w.PixelSize()
	r := s.Raster(ww, hh)
	if r == nil || ww < 1 {
		return nil
	}
	out := make([]float64, n)
	for i := range out {
		// The first and last samples sit half a step in, so neither lands
		// on the very edge row, where a rounding of one pixel is a large
		// fraction of a small coverage.
		y := int((float64(i) + 0.5) * float64(hh) / float64(n))
		out[i] = float64(coverage(r, y)) / float64(ww)
	}
	return out
}

// OutlineDiff is the largest disagreement between two outlines, and -1 when
// they are not the same length.
func OutlineDiff(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return -1
	}
	worst := 0.0
	for i := range a {
		if d := a[i] - b[i]; d > worst {
			worst = d
		} else if -d > worst {
			worst = -d
		}
	}
	return worst
}

// Insets are how far in the silhouette starts from each side on the row a
// fraction of the way down the window, in device pixels.
//
// It is what an *asymmetric* outline needs: a skirt swept round on one
// corner and square on the other has the same coverage as one swept half as
// much on both, and only the two insets tell them apart.
func Insets(w *app.Window, at float64) (left, right int) {
	s := w.Shape()
	if s == nil {
		return 0, 0
	}
	// A silhouette is rasterised in device pixels, which is what the
	// window's box is; Size is the logical one.
	ww, hh := w.PixelSize()
	r := s.Raster(ww, hh)
	if r == nil {
		return 0, 0
	}
	y := int(at * float64(hh))
	if y < 0 {
		y = 0
	}
	if y >= hh {
		y = hh - 1
	}
	lo, hi := ww, 0
	for _, rect := range r.Rects {
		if y < rect.Y || y >= rect.Y+rect.H {
			continue
		}
		if rect.X < lo {
			lo = rect.X
		}
		if rect.X+rect.W > hi {
			hi = rect.X + rect.W
		}
	}
	if hi <= lo {
		return ww, ww
	}
	return lo, ww - hi
}

// Widest and Narrowest are the extremes of a profile, which is usually all
// a test needs to say what shape something is.
func Widest(p []int) int {
	m := 0
	for _, v := range p {
		if v > m {
			m = v
		}
	}
	return m
}

// Narrowest is the smallest covered row, ignoring rows the silhouette does
// not reach at all (the space above a tab, below a dome).
func Narrowest(p []int) int {
	m := -1
	for _, v := range p {
		if v > 0 && (m < 0 || v < m) {
			m = v
		}
	}
	if m < 0 {
		return 0
	}
	return m
}
