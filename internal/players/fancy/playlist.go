package fancy

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/players"
	uiskins "goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type playlistPane struct {
	widget.Base
	p       *Player
	slots   *uiskins.Slots
	list    *players.QueueView
	buttons map[string]*players.GlyphButton
}

func newPlaylistPane(p *Player) *playlistPane {
	b := &playlistPane{p: p, slots: uiskins.NewSlots(LayoutPlaylist), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Fancy playlist")
	b.list = players.NewQueueView(p.Host)
	b.list.RowGeo = b.rowGeometry
	b.list.RowPaint = b.paintRow
	b.list.ScrollPaint = b.paintScroll
	b.list.OnContext = func(_ int, at paint2d.Point) { b.contextMenu(b.list, at) }
	b.slots.Bind("list", b.list)
	b.Add(b.list)
	add := func(slot, label, text string, action func()) *players.GlyphButton {
		c := button(LayoutPlaylist, slot, label, text, players.GlyphNone, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	add("close", "Close playlist", "", func() { p.ShowPlaylist(false) })
	add("open", "Load music files", "LOAD", func() { p.Host.OpenFiles(b.buttons["open"], false) })
	add("save", "Save playlist", "SAVE", func() { p.savePlaylist(b.buttons["save"]) })
	add("clear", "Clear playlist", "CLEAR", p.clearPlaylist)
	add("add", "Add music files", "+", func() { p.Host.OpenFiles(b.buttons["add"], true) })
	add("remove", "Remove current track", "REMOVE", p.Host.RemoveTrack)
	add("shuffle", "Shuffle", "RANDOM", func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	add("repeat", "Repeat", "REPEAT", func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	grip := &resizeGrip{p: p}
	grip.Init(grip)
	grip.SetTransparent(true)
	grip.SetAccessibleName("Resize playlist")
	b.slots.Bind("resize", grip)
	b.Add(grip)
	return b
}
func (b *playlistPane) rowGeometry(lk style.LookAndFeel) *widgets.RowGeometry {
	box := b.LocalBounds()
	rows, ok := uiskins.SkinSlotRect(lk, LayoutPlaylist, "rows", box)
	if !ok {
		return nil
	}
	bar, _ := uiskins.SkinSlotRect(lk, LayoutPlaylist, "scroll", box)
	o := b.list.Bounds().Min
	return &widgets.RowGeometry{Rows: rows.Translate(paint2d.Pt(-o.X, -o.Y)), Bar: bar.Translate(paint2d.Pt(-o.X, -o.Y)), RowHeight: style.Dip(lk, 17)}
}
func (b *playlistPane) paintRow(ctx *paint2d.Context, row paint2d.Rect, i int, st style.ControlState) bool {
	col := paint2d.RGB(.67, .74, .78)
	if b.list.QueueIndex(i) == b.p.Transport.List.Index() {
		col = cyan()
	}
	if i == b.list.Selected {
		ctx.DrawRect(row, paint2d.Fill(paint2d.RGB(.071, .18, .208)))
		col = paint2d.RGB(.6, .92, .95)
	}
	lk := b.list.Look()
	pad := style.Dip(lk, 3)
	length := style.Dip(lk, 35)
	track := b.p.Transport.List.At(b.list.QueueIndex(i))
	label := row.Inset(pad)
	label.Max.X -= length
	players.DrawTextIn(ctx, face(lk, 9), label, fmt.Sprintf("%02d %s", b.list.QueueIndex(i)+1, track.Label()), col, style.AlignStart)
	clock := row.Inset(pad)
	clock.Min.X = clock.Max.X - length
	time := players.Clock(track.Length)
	if track.Length <= 0 {
		time = "--:--"
	}
	players.DrawTextIn(ctx, face(lk, 8), clock, time, ink(), style.AlignEnd)
	if st.Focused() && i == b.list.Selected {
		lk.DrawFocusRing(ctx, row)
	}
	return true
}
func (b *playlistPane) paintScroll(ctx *paint2d.Context, track, thumb paint2d.Rect, _ style.ControlState) bool {
	ctx.DrawRoundRect(track, 2, 2, paint2d.Fill(paint2d.RGB(.03, .04, .06)))
	ctx.DrawRoundRect(thumb, 2, 2, paint2d.Fill(ink()))
	return true
}
func (b *playlistPane) sync() {
	b.list.Sync()
	b.buttons["shuffle"].SetChecked(b.p.Transport.Random)
	b.buttons["repeat"].SetChecked(b.p.Transport.Repeat != players.RepeatOff)
	b.buttons["repeat"].Tip = "Repeat: " + b.p.Transport.Repeat.String()
	b.buttons["remove"].SetEnabled(b.p.Transport.List.Len() > 0)
	b.buttons["clear"].SetEnabled(b.p.Transport.List.Len() > 0)
	b.Invalidate()
}
func (b *playlistPane) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *playlistPane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *playlistPane) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	paintLayout(ctx, lk, box, LayoutPlaylist, b.p.List())
	if r, ok := uiskins.SkinSlotRect(lk, LayoutPlaylist, "logo", box); ok {
		uiskins.DrawSkinSlot(lk, ctx, r, LayoutPlaylist, "logo", 0)
	}
	textSlot(ctx, lk, box, LayoutPlaylist, "header", "Playlist", 9, ink(), style.AlignStart)
	textSlot(ctx, lk, box, LayoutPlaylist, "status", fmt.Sprintf("%d tracks · %s", b.p.Transport.List.Len(), players.Clock(b.p.Transport.List.Total())), 8, ink(), style.AlignStart)
}
func (b *playlistPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.contextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		return b.p.g.StartMove(b.p.iList)
	}
	return false
}
func (b *playlistPane) CaptionAt(paint2d.Point) bool { return true }
func (b *playlistPane) contextMenu(from widget.Component, at paint2d.Point) {
	h := b.p.Host
	items := []*widgets.MenuItem{widgets.Item("Open music files…", func() { h.OpenFiles(from, false) }), widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }), widgets.Item("Save playlist…", func() { b.p.savePlaylist(from) }), widgets.Item("Remove current track", h.RemoveTrack), widgets.Item("Clear playlist", b.p.clearPlaylist), widgets.Sep(), widgets.CheckItem("Shuffle", b.p.Transport.Random, func() { h.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+b.p.Transport.Repeat.String(), func() { h.Command(players.CmdRepeat) })}
	if skins := h.SkinItems(); len(skins) > 0 {
		items = append(items, widgets.Sep(), widgets.Submenu("Appearance", skins...))
	}
	if h.ShowSettings != nil {
		items = append(items, widgets.ItemAccel("Settings…", "Ctrl+,", h.OpenSettings))
	}
	widgets.ShowContextMenu(from, at, items...)
}
func (b *playlistPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.List(), e) }
func (b *playlistPane) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *playlistPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *playlistPane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
func (b *playlistPane) Describe(n *a11y.Node)        { n.Role, n.Name = a11y.RoleGroup, "Fancy playlist" }

type resizeGrip struct {
	widget.Base
	p *Player
}

func (g *resizeGrip) Paint(ctx *paint2d.Context) {
	d := style.Dip(g.Look(), 1)
	r := g.LocalBounds().Inset(d)
	col := ink()
	if g.Hovered() {
		col = cyan()
	}
	for _, span := range []float32{4, 8, 12} {
		ctx.DrawLine(paint2d.Pt(r.Max.X-min(span*d, r.Dx()), r.Max.Y), paint2d.Pt(r.Max.X, r.Max.Y-min(span*d, r.Dy())), paint2d.StrokePaint(col, d))
	}
}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.listSize.Release()
	return g.p.g.StartResize(g.p.iList, platform.EdgeBottom|platform.EdgeRight)
}
func (p *Player) clearPlaylist() {
	queue := p.Transport.List
	for queue.Len() > 0 {
		queue.Remove(queue.Len()-1, p.Transport.Random)
	}
	p.Host.Command(players.CmdStop)
	p.Refresh()
}
