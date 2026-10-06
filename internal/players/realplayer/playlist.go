package realplayer

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

type queuePane struct {
	widget.Base
	p       *Player
	slots   *widget.Slots
	list    *players.QueueView
	buttons map[string]*players.GlyphButton
}

func newQueuePane(p *Player) *queuePane {
	b := &queuePane{p: p, slots: widget.NewSlots(LayoutPlaylist), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Playlist")
	b.list = players.NewQueueView(p.Host)
	b.list.OnContext = func(_ int, at paintengine2d.Point) { b.context(b.list, at) }
	b.list.RowGeo = func(lk style.LookAndFeel) *widgets.RowGeometry {
		rows, _ := style.SkinSlotRect(lk, LayoutPlaylist, "rows", b.LocalBounds())
		bar, _ := style.SkinSlotRect(lk, LayoutPlaylist, "scroll", b.LocalBounds())
		o := b.list.Bounds().Min
		return &widgets.RowGeometry{Rows: rows.Translate(paintengine2d.Pt(-o.X, -o.Y)), Bar: bar.Translate(paintengine2d.Pt(-o.X, -o.Y)), RowHeight: style.Dip(lk, 18)}
	}
	b.list.RowPaint = b.paintRow
	b.list.ScrollPaint = func(ctx *paintengine2d.Context, track, thumb paintengine2d.Rect, st style.ControlState) bool {
		ctx.DrawRect(track, paintengine2d.Fill(paintengine2d.RGB(.68, .68, .68)))
		ctx.DrawRect(thumb, paintengine2d.Fill(paintengine2d.RGB(.75, .75, .75)))
		drawBevel(ctx, thumb, true)
		return true
	}
	b.slots.Bind("list", b.list)
	b.Add(b.list)
	for _, item := range []struct {
		slot, text string
		run        func()
	}{{"header", "Playlist", func() {
		b.context(b.buttons["header"], widget.DeviceOrigin(b.buttons["header"]).Add(paintengine2d.Pt(0, b.buttons["header"].Bounds().Dy())))
	}}, {"add", "Add…", func() { p.Host.OpenFiles(b.buttons["add"], true) }}, {"remove", "Remove", p.Host.RemoveTrack}, {"hide", "Hide", func() { p.ShowPlaylist(false) }}} {
		b.buttons[item.slot] = textButton(b, b.slots, LayoutPlaylist, item.slot, item.text, item.text, item.run)
	}
	b.buttons["remove"].Tip = "Remove the current playing track"
	return b
}
func (b *queuePane) sync() {
	b.list.Sync()
	b.buttons["remove"].SetEnabled(b.p.Transport.List.Len() > 0)
	b.buttons["header"].Tip = fmt.Sprintf("%d tracks · playlist options", b.list.Count)
	b.Invalidate()
}
func (b *queuePane) Measure(c layout.Constraints) paintengine2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *queuePane) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *queuePane) Paint(ctx *paintengine2d.Context) {
	style.DrawSkinLayout(b.Look(), ctx, b.LocalBounds(), LayoutPlaylist)
}
func (b *queuePane) paintRow(ctx *paintengine2d.Context, r paintengine2d.Rect, i int, st style.ControlState) bool {
	lk := b.Look()
	bg, fg := paintengine2d.Black, ink(lk, "row", paintengine2d.White)
	if b.list.QueueIndex(i) == b.p.Transport.List.Index() {
		fg = ink(lk, "current", paintengine2d.RGB(.45, 1, .45))
	}
	if st.Checked() {
		bg, fg = ink(lk, "selected", paintengine2d.RGB(0, 0, .5)), ink(lk, "selectedText", paintengine2d.White)
	}
	ctx.DrawRect(r, paintengine2d.Fill(bg))
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 11))
	pad := style.Dip(lk, 3)
	clock := b.list.ItemDetail(i)
	width := f.Advance(clock)
	players.DrawTextIn(ctx, f, paintengine2d.XYWH(r.Min.X+pad, r.Min.Y, max(0, r.Dx()-width-3*pad), r.Dy()), b.list.ItemText(i), fg, style.AlignStart)
	players.DrawTextIn(ctx, f, paintengine2d.XYWH(r.Max.X-width-pad, r.Min.Y, width, r.Dy()), clock, fg, style.AlignEnd)
	if st.Focused() {
		lk.DrawFocusRing(ctx, r)
	}
	return true
}
func (b *queuePane) context(from widget.Component, at paintengine2d.Point) {
	h, t := b.p.Host, b.p.Transport
	remove := widgets.Item("Remove current track", h.RemoveTrack)
	remove.Disabled = t.List.Len() == 0
	widgets.ShowContextMenu(from, at, widgets.Item("Open music files…", func() { h.OpenFiles(from, false) }), widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }), remove, widgets.Sep(), widgets.CheckItem("Shuffle", t.Random, func() { h.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+t.Repeat.String(), func() { h.Command(players.CmdRepeat) }))
}
func (b *queuePane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Playlist" }
func (b *queuePane) DropTypes() []string   { return []string{"text/uri-list"} }
func (b *queuePane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *queuePane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
