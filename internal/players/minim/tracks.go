package minim

import (
	"fmt"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// tracks is the playlist's list: a row per track, the one playing marked,
// and a scroll bar.
//
// It is a component of the player's own rather than a widgets.ListView
// because a panel sets its rows in a size and an ink of its own — ten design
// pixels, green on black, the playing track in white — and a list view sets
// them in the look's body face, sixteen pixels in the look's colours, which
// is a playlist of a different era. Everything a list view is to a keyboard
// and a screen reader it still is: arrows, Page Up and Down, Home and End,
// Return, a wheel, a scroll thumb to drag, and a list of named items in the
// accessibility tree. Under a pack that is not a panel it paints the look's
// own rows and scroll bar, so the widget face looks as it always did.
type tracks struct {
	widget.Base
	p *Player

	// Selected is the row the keyboard is on. Choosing a row plays it, as
	// it always has in this player.
	Selected int
	offset   float32
	hovered  int
	// grab is where on the thumb a drag took it, -1 when no drag is on.
	grab float32
	// reveal is a row to bring into view at the next layout, plus one: a
	// row asked for before the list has its size cannot be scrolled to.
	reveal int
	// geo is where the panel face's rows, groove and first row are in the
	// list's own box, from the skin's layout; nil in the widget face.
	geo *listGeo
}

// listGeo is a panel face's playlist, in the list's own coordinates.
type listGeo struct {
	rows, bar paintengine2d.Rect
	rowH      float32
}

func newTracks(p *Player) *tracks {
	t := &tracks{p: p, Selected: -1, hovered: -1, grab: -1}
	t.Init(t)
	t.SetWantsFocus(true)
	t.SetFocusVisibleOnly(true)
	t.SetAccessibleName("Playlist")
	return t
}

func (t *tracks) count() int { return t.p.Transport.List.Len() }

// rowH is one row: the panel's own height, or the look's row fitted to its
// font as a list view would fit it.
func (t *tracks) rowH() float32 {
	lk := t.Look()
	if t.geo != nil && t.geo.rowH > 0 {
		return t.geo.rowH
	}
	return style.FittedRowHeight(lk, 28)
}

// frame is the look's view frame round the rows in the widget face; a panel
// prints its own well.
func (t *tracks) frame() style.Insets {
	if t.geo != nil {
		return style.Insets{}
	}
	return style.ViewFrameInsetsOf(t.Look())
}

// rows is the rows' viewport, and bar the scroll bar's groove beside it.
func (t *tracks) rows() paintengine2d.Rect {
	lk, b := t.Look(), t.LocalBounds()
	if t.geo != nil {
		return t.geo.rows
	}
	in := t.frame().Apply(b)
	if t.overflows(in.Dy()) {
		in.Max.X -= lk.Metrics().Scroll
	}
	return in
}

func (t *tracks) bar() paintengine2d.Rect {
	lk, b := t.Look(), t.LocalBounds()
	if t.geo != nil {
		return t.geo.bar
	}
	in := t.frame().Apply(b)
	if !t.overflows(in.Dy()) {
		return paintengine2d.Rect{}
	}
	return paintengine2d.Rect{Min: paintengine2d.Pt(in.Max.X-lk.Metrics().Scroll, in.Min.Y), Max: in.Max}
}

func (t *tracks) overflows(h float32) bool { return float32(t.count())*t.rowH() > h+0.5 }

func (t *tracks) maxOffset() float32 {
	return max(float32(t.count())*t.rowH()-t.rows().Dy(), 0)
}

func (t *tracks) clamp() { t.offset = min(max(t.offset, 0), t.maxOffset()) }

// thumb is the scroll thumb inside the groove.
func (t *tracks) thumb() paintengine2d.Rect {
	g := t.bar()
	if g.Empty() {
		return g
	}
	lk := t.Look()
	total := float32(t.count()) * t.rowH()
	h := g.Dy() * min(t.rows().Dy()/max(total, 1), 1)
	if t.geo != nil {
		// A panel's thumb is a picture of a fixed size, as the era's was.
		if _, th, ok := style.SkinSpriteSize(lk, "list.thumb"); ok {
			h = style.Dip(lk, th)
		}
	}
	h = max(h, style.Dip(lk, 8))
	span := g.Dy() - h
	y := float32(0)
	if m := t.maxOffset(); m > 0 {
		y = span * t.offset / m
	}
	if t.geo != nil {
		// On the panel's grid, like every other piece of it.
		return paintengine2d.XYWH(g.Min.X, g.Min.Y+snapDesign(y, style.Dip(lk, 1)), g.Dx(), h)
	}
	return paintengine2d.XYWH(g.Min.X, snapDev(g.Min.Y+y), g.Dx(), h)
}

// EnsureVisible scrolls the least needed to show row i. Asked before the
// list is laid out at its size, it waits for the layout.
func (t *tracks) EnsureVisible(i int) {
	rh, view := t.rowH(), t.rows().Dy()
	t.reveal = i + 1
	if view < rh {
		return
	}
	t.reveal = 0
	top := float32(i) * rh
	if top < t.offset {
		t.offset = top
	} else if top+rh > t.offset+view {
		t.offset = top + rh - view
	}
	t.clamp()
	t.Invalidate()
}

// choose moves the keyboard to row i and plays it.
func (t *tracks) choose(i int) {
	if i < 0 || i >= t.count() {
		return
	}
	t.Selected = i
	t.EnsureVisible(i)
	t.p.Transport.SelectTrack(i)
	if t.p.Transport.State != players.Playing {
		t.p.Command(players.CmdPlayPause)
	}
}

func (t *tracks) label(i int) string {
	tr := t.p.Transport.List.At(i)
	return fmt.Sprintf("%d. %s", i+1, tr.Label())
}

func (t *tracks) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(t.Look(), 200), style.Dip(t.Look(), 120)))
}

func (t *tracks) Arrange(r paintengine2d.Rect) {
	t.SetBounds(r)
	if t.reveal > 0 {
		t.EnsureVisible(t.reveal - 1)
	}
	t.clamp()
}

func (t *tracks) Paint(ctx *paintengine2d.Context) {
	t.clamp()
	lk := t.Look()
	f := faceOf(lk)
	if t.geo != nil && f.panelled() {
		t.paintPanel(ctx, lk, f.ink())
		return
	}
	b := t.LocalBounds()
	st := t.State()
	in := t.frame()
	if !in.Zero() {
		style.DrawViewFrameOf(lk, ctx, b, st)
	}
	rows := t.rows()
	ctx.DrawRect(rows, paintengine2d.Fill(style.ViewBackgroundOf(lk, st)))
	rh := t.rowH()
	playing := t.p.Transport.List.Index()
	ctx.Save()
	ctx.ClipRect(rows)
	for i := 0; i < t.count(); i++ {
		y := rows.Min.Y + float32(i)*rh - t.offset
		if y+rh < rows.Min.Y || y > rows.Max.Y {
			continue
		}
		tr := t.p.Transport.List.At(i)
		// The time before the title, as this face always had it: a row
		// that runs out of room loses the end of the title, not the one
		// number a playlist is there to show.
		text := fmt.Sprintf("%2d.  %s   %s", i+1, players.Clock(tr.Length), tr.Label())
		rs := widget.RowItemState(t, i, i == playing, i == t.hovered, i == t.Selected)
		lk.DrawListRow(ctx, paintengine2d.XYWH(rows.Min.X, y, rows.Dx(), rh), rs, text)
	}
	ctx.Restore()
	if g := t.bar(); !g.Empty() {
		lk.DrawScrollBar(ctx, g, t.thumb(), st)
	}
	if t.Focused() && t.Selected < 0 {
		lk.DrawFocusRing(ctx, rows)
	}
}

// paintPanel sets the rows the way a panel's playlist did: small type in the
// face's ink on the black the art printed, the playing track in white (or on
// a bar, in the silver face), the length pushed to the right, and the
// thumb a picture riding the groove.
func (t *tracks) paintPanel(ctx *paintengine2d.Context, lk style.LookAndFeel, in *ink) {
	rows := t.rows()
	rh := t.rowH()
	base := lk.Font()
	if in.rowBold {
		base = lk.BoldFont()
	}
	face := players.ScaledFace(base, style.Dip(lk, in.rowSize))
	playing := t.p.Transport.List.Index()
	pad := style.Dip(lk, 3)
	ctx.Save()
	ctx.ClipRect(rows)
	for i := 0; i < t.count(); i++ {
		y := rows.Min.Y + float32(i)*rh - t.offset
		if y+rh < rows.Min.Y || y > rows.Max.Y {
			continue
		}
		row := paintengine2d.XYWH(rows.Min.X, y, rows.Dx(), rh)
		col := in.row
		if i == playing {
			col = in.current
			if faceOf(lk) == faceSilver {
				ctx.DrawRect(row, paintengine2d.Fill(in.selected))
			}
		}
		if i == t.Selected && t.State().Focused() {
			lk.DrawFocusRing(ctx, row)
		}
		tr := t.p.Transport.List.At(i)
		clock := players.Clock(tr.Length)
		cw := face.Advance(clock)
		players.DrawTextIn(ctx, face, paintengine2d.XYWH(row.Max.X-cw-pad, y, cw, rh), clock, col, style.AlignEnd)
		players.DrawTextIn(ctx, face, paintengine2d.XYWH(row.Min.X+pad, y, max(row.Dx()-cw-3*pad, 0), rh), t.label(i), col, style.AlignStart)
	}
	ctx.Restore()
	if th := t.thumb(); !th.Empty() {
		name := "list.thumb"
		if t.grab >= 0 {
			name = "list.thumb.down"
		}
		art(lk, ctx, th, name)
	}
}

func (t *tracks) indexAt(y float32) int {
	rows := t.rows()
	if y < rows.Min.Y || y >= rows.Max.Y {
		return -1
	}
	i := int((y - rows.Min.Y + t.offset) / t.rowH())
	if i < 0 || i >= t.count() {
		return -1
	}
	return i
}

func (t *tracks) MousePress(e widget.MouseEvent) bool {
	if t.p.rightClick(t, e) {
		return true
	}
	t.MarkPointerFocus()
	t.RequestFocus()
	if th := t.thumb(); !th.Empty() && th.Contains(e.Pos) {
		t.grab = e.Pos.Y - th.Min.Y
		t.Invalidate()
		return true
	}
	if g := t.bar(); !g.Empty() && g.Contains(e.Pos) {
		// A press in the groove pages towards it.
		if e.Pos.Y < t.thumb().Min.Y {
			t.offset -= t.rows().Dy()
		} else {
			t.offset += t.rows().Dy()
		}
		t.clamp()
		t.Invalidate()
		return true
	}
	if i := t.indexAt(e.Pos.Y); i >= 0 {
		t.choose(i)
	}
	return true
}

func (t *tracks) MouseMove(e widget.MouseEvent) bool {
	if t.grab >= 0 {
		g, th := t.bar(), t.thumb()
		span := g.Dy() - th.Dy()
		if span > 0 {
			t.offset = (e.Pos.Y - t.grab - g.Min.Y) / span * t.maxOffset()
			t.clamp()
			t.Invalidate()
		}
		return true
	}
	if h := t.indexAt(e.Pos.Y); h != t.hovered {
		t.hovered = h
		t.Invalidate()
	}
	return true
}

func (t *tracks) MouseRelease(widget.MouseEvent) bool {
	if t.grab >= 0 {
		t.grab = -1
		t.Invalidate()
		return true
	}
	return false
}

func (t *tracks) MouseExit() {
	t.hovered = -1
	t.Invalidate()
	t.Base.MouseExit()
}

// MouseWheel scrolls by rows, and lets the wheel bubble at either end.
func (t *tracks) MouseWheel(e widget.MouseEvent) bool {
	if t.maxOffset() <= 0 {
		return false
	}
	before := t.offset
	step := e.Scroll.Y * t.rowH()
	if !e.Precise {
		step *= 3
	}
	t.offset -= step
	t.clamp()
	if t.offset == before {
		return false
	}
	t.Invalidate()
	return true
}

func (t *tracks) KeyPress(e widget.KeyEvent) bool {
	n := t.count()
	if n == 0 || e.Mods.Ctrl() || e.Mods.Alt() {
		return false
	}
	next := t.Selected
	page := max(int(t.rows().Dy()/t.rowH())-1, 1)
	switch e.Key {
	case platform.KeyDown:
		next++
	case platform.KeyUp:
		next--
	case platform.KeyPageDown:
		next += page
	case platform.KeyPageUp:
		next -= page
	case platform.KeyHome:
		next = 0
	case platform.KeyEnd:
		next = n - 1
	case platform.KeyReturn:
		t.MarkKeyboardFocus()
		t.choose(max(t.Selected, 0))
		return true
	default:
		return false
	}
	t.MarkKeyboardFocus()
	t.choose(min(max(next, 0), n-1))
	return true
}

// Describe is the list as a screen reader meets it, and AccessibleItems its
// rows, as a list view gives them.
func (t *tracks) Describe(n *a11y.Node) {
	n.Role = a11y.RoleList
	if n.Name == "" {
		n.Name = "Playlist"
	}
}

func (t *tracks) AccessibleItems() []*a11y.Node {
	rows, rh := t.rows(), t.rowH()
	out := make([]*a11y.Node, 0, t.count())
	for i := 0; i < t.count(); i++ {
		tr := t.p.Transport.List.At(i)
		r := paintengine2d.XYWH(rows.Min.X, rows.Min.Y+float32(i)*rh-t.offset, rows.Dx(), rh)
		nd := &a11y.Node{
			ID:     widget.ItemID(t, i),
			Role:   a11y.RoleListItem,
			Name:   fmt.Sprintf("%s, %s", t.label(i), players.Clock(tr.Length)),
			Bounds: widget.LocalToWindow(t, r),
		}
		if !r.Overlaps(rows) {
			nd.State |= a11y.StateOffscreen
		}
		nd.State |= a11y.StateSelectable
		if i == t.Selected {
			nd.State |= a11y.StateSelected
		}
		nd.Index, nd.Count = i+1, t.count()
		nd.Actions = nd.Actions.With(a11y.ActionDefault).With(a11y.ActionScrollIntoView)
		out = append(out, nd)
	}
	return out
}

func (t *tracks) AccessibleAction(i int, a a11y.Action) bool {
	if i < 0 || i >= t.count() {
		return false
	}
	switch a {
	case a11y.ActionDefault:
		t.choose(i)
		return true
	case a11y.ActionScrollIntoView:
		t.EnsureVisible(i)
		return true
	}
	return false
}

func (t *tracks) AccessibleFocusItem() int {
	if t.Selected >= 0 && t.Selected < t.count() {
		return t.Selected
	}
	return -1
}

func (t *tracks) AccessibleItemCount() int { return t.count() }
