package vlc

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type queuePane struct {
	widget.Base
	p           *Player
	list        *players.QueueView
	add, remove *widgets.Button
}

func newQueuePane(p *Player) *queuePane {
	q := &queuePane{p: p}
	q.Init(q)
	q.SetManagesChildren(true)
	q.SetAccessibleName("Playlist")
	q.list = players.NewQueueView(p.Host)
	q.list.OnContext = func(_ int, at paint2d.Point) { p.body.contextMenu(q.list, at) }
	q.list.RowGeo = func(lk style.LookAndFeel) *widgets.RowGeometry {
		b := q.list.LocalBounds()
		d := func(v float32) float32 { return style.Dip(lk, v) }
		return &widgets.RowGeometry{Rows: paint2d.XYWH(d(2), d(2), max(0, b.Dx()-d(18)), max(0, b.Dy()-d(4))), Bar: paint2d.XYWH(b.Dx()-d(15), d(2), d(13), max(0, b.Dy()-d(4))), RowHeight: d(20)}
	}
	q.list.RowPaint = q.paintRow
	q.add = widgets.NewButton("Add…", func() { p.Host.OpenFiles(q.add, true) })
	q.remove = widgets.NewButton("Remove", p.Host.RemoveTrack)
	q.remove.Tip = "Remove the current playing track"
	for _, c := range []widget.Component{q.list, q.add, q.remove} {
		q.Add(c)
	}
	return q
}
func (q *queuePane) sync() {
	q.list.Sync()
	q.list.Invalidate()
	q.remove.SetEnabled(q.p.Transport.List.Len() > 0)
	q.Invalidate()
}
func (q *queuePane) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(q.Look(), 428), style.Dip(q.Look(), 180)))
}
func (q *queuePane) Arrange(r paint2d.Rect) {
	q.SetBounds(r)
	b, lk := q.LocalBounds(), q.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	q.add.Arrange(paint2d.XYWH(b.Dx()-d(150), 0, d(70), d(23)))
	q.remove.Arrange(paint2d.XYWH(b.Dx()-d(74), 0, d(74), d(23)))
	q.list.Arrange(paint2d.XYWH(0, d(26), b.Dx(), max(0, b.Dy()-d(26))))
}
func (q *queuePane) Paint(ctx *paint2d.Context) {
	lk, b := q.Look(), q.LocalBounds()
	text := fmt.Sprintf("Playlist · %d items", q.p.Transport.List.Len())
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 11)), paint2d.XYWH(2, 0, max(0, b.Dx()-style.Dip(lk, 160)), style.Dip(lk, 23)), text, paint2d.Black, style.AlignStart)
}
func (q *queuePane) paintRow(ctx *paint2d.Context, r paint2d.Rect, i int, st style.ControlState) bool {
	lk := q.list.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	bg, fg := paint2d.White, paint2d.Black
	if st.Checked() {
		bg, fg = paint2d.RGB(.04, .14, .42), paint2d.White
	}
	ctx.DrawRect(r, paint2d.Fill(bg))
	track := q.p.Transport.List.At(q.list.QueueIndex(i))
	text := r
	text.Min.X += d(5)
	text.Max.X -= d(70)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), d(11)), text, track.Label(), fg, style.AlignStart)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), d(10)), paint2d.XYWH(r.Max.X-d(62), r.Min.Y, d(56), r.Dy()), players.Clock(track.Length), fg, style.AlignEnd)
	return true
}
func (q *queuePane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Playlist" }
func (q *queuePane) DropTypes() []string   { return []string{"text/uri-list"} }
func (q *queuePane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (q *queuePane) Drop(e widget.DropEvent) bool { return q.p.Host.AcceptFileDrop(e) }
