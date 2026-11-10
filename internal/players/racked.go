package players

import (
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
)

// ---- the racked faces' panels ---------------------------------------------------

// The faces with panels (winamp, fancy, fandom, halflife2) declare them as
// a rack.Group's members: the group gives each a window where the desktop
// lets a program place windows and keeps them stuck together, and shares one
// window among the panels stuck together where it does not (Wayland). What
// is left to a face is the same for all four: naming its windows, and
// saving and restoring where its panels were.

// GroupWindows is each window of g once, named by the first member in it
// (names[i] is member i's name; a member past names takes its rack name).
// Where the panels share a window, the equaliser and the playlist stuck to
// the player are in the player's window, and are not listed apart.
func GroupWindows(g *rack.Group, names ...string) []NamedWindow {
	var out []NamedWindow
	seen := map[*ui.Window]bool{}
	for i := range g.Len() {
		w := g.Window(i)
		if w == nil || seen[w] {
			continue
		}
		seen[w] = true
		name := g.Rack.Pane(i).Name
		if i < len(names) {
			name = names[i]
		}
		out = append(out, NamedWindow{Name: name, Window: w})
	}
	return out
}

// CaptureGroup records each panel's box and bond, the hidden ones' too. A
// box is a position on the desktop only where each panel has a window of its
// own; sharing windows, the boxes are in the group's own planes, and the
// placement says so (Positioned false).
func CaptureGroup(g *rack.Group) map[string]WindowPlacement {
	out := make(map[string]WindowPlacement)
	for i := range g.Len() {
		pane := g.Rack.Pane(i)
		if pane == nil {
			continue
		}
		positioned := false
		if w := g.Window(i); g.Mode() == rack.GroupWindows && w != nil && !w.Closed() {
			_, _, positioned = w.Position()
		}
		placement := WindowPlacement{
			X: pane.Box.X, Y: pane.Box.Y, Width: pane.Box.W, Height: pane.Box.H,
			Positioned: positioned,
		}
		if parent := g.Rack.Pane(pane.To); parent != nil && pane.Bond.Stuck() {
			placement.Dock = &WindowDock{Parent: parent.Name, Side: pane.Bond.Side.String(), Along: pane.Bond.Along}
		}
		out[pane.Name] = placement
	}
	return out
}

// RestoreGroup puts a saved layout back: the resizable member's size (held
// to minW by minH), each docked panel's bond along its edge, and the place
// of each panel that hangs from nothing. The first member never hangs from
// another, and a bond the group refuses (one that would make a cycle, a
// settings file edited by hand) leaves the panel loose where it was saved.
// The fixed panels keep the face's sizes. Restored just after the group was
// realised, the layout is kept past the window manager's own placement.
//
// It returns the size it gave the resizable member, for the face to keep
// while the group settles (HeldSize.Keep).
func RestoreGroup(g *rack.Group, saved map[string]WindowPlacement, resizable, minW, minH int) HeldSize {
	var held HeldSize
	if placement, ok := saved[paneName(g, resizable)]; ok && (placement.Width > 0 || placement.Height > 0) {
		box := g.Rack.Pane(resizable).Box
		w, h := box.W, box.H
		if placement.Width > 0 {
			w = max(placement.Width, minW)
		}
		if placement.Height > 0 {
			h = max(placement.Height, minH)
		}
		g.SetSize(resizable, w, h)
		held = HeldSize{i: resizable, w: w, h: h, until: time.Now().Add(heldFor)}
	}
	// The saved bonds go back from the first member out, so a layout the
	// user turned round (the playlist under the player, the equaliser under
	// the playlist) is not refused for a cycle with the bonds it replaces.
	parent := make([]int, g.Len())
	for i := range parent {
		parent[i] = -1
		placement, ok := saved[paneName(g, i)]
		if !ok || i == 0 || placement.Dock == nil {
			continue
		}
		if _, to := g.Rack.Find(placement.Dock.Parent); to != i && SavedDockSide(placement.Dock.Side) != rack.SideNone {
			parent[i] = to
		}
	}
	var order []int
	done := make([]bool, g.Len())
	for grew := true; grew; {
		grew = false
		for i, to := range parent {
			if !done[i] && (to < 0 || done[to]) {
				done[i], grew = true, true
				order = append(order, i)
			}
		}
	}
	for i := range done {
		if !done[i] {
			// A cycle in the saved bonds: tried last, and refused.
			order = append(order, i)
		}
	}
	for _, i := range order {
		pane := g.Rack.Pane(i)
		placement, ok := saved[pane.Name]
		if !ok {
			continue
		}
		if to := parent[i]; to >= 0 {
			bond := rack.Bond{Side: SavedDockSide(placement.Dock.Side), Along: placement.Dock.Along}
			g.Stick(i, to, bond)
			if pane.To == to && pane.Bond == bond {
				continue
			}
		}
		if pane.To >= 0 {
			g.Detach(i)
		}
		if placement.Positioned {
			g.Place(i, placement.X, placement.Y)
		}
	}
	return held
}

// HeldSize is the size RestoreGroup gave a panel, kept while the group
// settles.
//
// R21: rack.Group keeps the bonds and places declared before Realise, or
// just after it, past the window manager's own placement, but not a size
// declared with SetSize: as it settles it takes each window's size from the
// desktop, so the first configure arriving late (the window mapped at the
// size it opened at, after the program asked for the saved one) puts the
// restored playlist back to its opening size. Group lacks a declared size
// kept like a place (SetSize before Realise or while it settles, put back
// as it settles); until it has one, the face puts the size back on its
// clock for as long as the group may settle.
type HeldSize struct {
	i, w, h int
	until   time.Time
}

// heldFor is how long a group may settle: rack's two seconds.
const heldFor = 2 * time.Second

// Keep puts the held size back on member i of g if its window lost it, for
// as long as the group may still be settling.
func (s *HeldSize) Keep(g *rack.Group) {
	if s.until.IsZero() {
		return
	}
	if time.Now().After(s.until) {
		s.until = time.Time{}
		return
	}
	w := g.Window(s.i)
	if g.Mode() != rack.GroupWindows || w == nil || w.Closed() {
		return
	}
	if width, height := w.Size(); width != s.w || height != s.h {
		g.SetSize(s.i, s.w, s.h)
	}
}

// Release lets the size go: the user is resizing the panel.
func (s *HeldSize) Release() { s.until = time.Time{} }

// paneName is member i's name in g's rack, or "" for none.
func paneName(g *rack.Group, i int) string {
	if p := g.Rack.Pane(i); p != nil {
		return p.Name
	}
	return ""
}

// SavedDockSide is the side a saved dock names, or rack.SideNone.
func SavedDockSide(side string) rack.Side {
	switch side {
	case "top":
		return rack.SideTop
	case "bottom":
		return rack.SideBottom
	case "left":
		return rack.SideLeft
	case "right":
		return rack.SideRight
	}
	return rack.SideNone
}
