package vlc

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
	q.list.OnContext = func(_ int, at paintengine2d.Point) { p.body.contextMenu(q.list, at) }
	q.list.RowGeo = func(lk style.LookAndFeel) *widgets.RowGeometry {
		b := q.list.LocalBounds()
		d := func(v float32) float32 { return style.Dip(lk, v) }
		return &widgets.RowGeometry{Rows: paintengine2d.XYWH(d(2), d(2), max(0, b.Dx()-d(18)), max(0, b.Dy()-d(4))), Bar: paintengine2d.XYWH(b.Dx()-d(15), d(2), d(13), max(0, b.Dy()-d(4))), RowHeight: d(20)}
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
func (q *queuePane) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(q.Look(), 428), style.Dip(q.Look(), 180)))
}
func (q *queuePane) Arrange(r paintengine2d.Rect) {
	q.SetBounds(r)
	b, lk := q.LocalBounds(), q.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	q.add.Arrange(paintengine2d.XYWH(b.Dx()-d(150), 0, d(70), d(23)))
	q.remove.Arrange(paintengine2d.XYWH(b.Dx()-d(74), 0, d(74), d(23)))
	q.list.Arrange(paintengine2d.XYWH(0, d(26), b.Dx(), max(0, b.Dy()-d(26))))
}
func (q *queuePane) Paint(ctx *paintengine2d.Context) {
	lk, b := q.Look(), q.LocalBounds()
	text := fmt.Sprintf("Playlist · %d items", q.p.Transport.List.Len())
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 11)), paintengine2d.XYWH(2, 0, max(0, b.Dx()-style.Dip(lk, 160)), style.Dip(lk, 23)), text, paintengine2d.Black, style.AlignStart)
}
func (q *queuePane) paintRow(ctx *paintengine2d.Context, r paintengine2d.Rect, i int, st style.ControlState) bool {
	lk := q.list.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	bg, fg := paintengine2d.White, paintengine2d.Black
	if st.Checked() {
		bg, fg = paintengine2d.RGB(.04, .14, .42), paintengine2d.White
	}
	ctx.DrawRect(r, paintengine2d.Fill(bg))
	track := q.p.Transport.List.At(q.list.QueueIndex(i))
	text := r
	text.Min.X += d(5)
	text.Max.X -= d(70)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), d(11)), text, track.Label(), fg, style.AlignStart)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), d(10)), paintengine2d.XYWH(r.Max.X-d(62), r.Min.Y, d(56), r.Dy()), players.Clock(track.Length), fg, style.AlignEnd)
	return true
}
func (q *queuePane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Playlist" }
func (q *queuePane) DropTypes() []string   { return []string{"text/uri-list"} }
func (q *queuePane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (q *queuePane) Drop(e widget.DropEvent) bool { return q.p.Host.AcceptFileDrop(e) }
