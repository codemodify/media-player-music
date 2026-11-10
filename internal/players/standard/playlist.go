package standard

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
	p                                       *Player
	list                                    *playlistList
	add, remove, selectTrack, misc, options *widgets.Button
	mini                                    [6]*players.GlyphButton
}

func newQueuePane(p *Player) *queuePane {
	q := &queuePane{p: p}
	q.Init(q)
	q.SetManagesChildren(true)
	q.list = newPlaylistList(p)
	q.add = widgets.NewButton("Add", func() { p.Host.OpenFiles(q.add, true) })
	q.remove = widgets.NewButton("Remove", p.Host.RemoveTrack)
	q.selectTrack = widgets.NewButton("Select", func() {
		q.showMenu(q.selectTrack, widgets.Item("Show the playing track", func() { q.list.EnsureVisible(p.Transport.List.Index()); p.Main.RequestFocus(q.list) }))
	})
	q.misc = widgets.NewButton("Misc", func() {
		o := widget.DeviceOrigin(q.misc)
		p.Host.ContextMenu(q.misc, paint2d.Pt(o.X, o.Y+q.misc.Bounds().Dy()))
	})
	q.options = widgets.NewButton("List options", func() {
		q.showMenu(q.options,
			widgets.Item("Play the first track", func() { p.Host.SelectTrack(0) }),
			widgets.Item("Close the playlist", func() { p.ShowPlaylist(false) }))
	})
	q.mini = p.transportButtons()
	q.Add(q.list)
	for _, c := range []widget.Component{q.add, q.remove, q.selectTrack, q.misc, q.options} {
		q.Add(c)
	}
	for _, c := range q.mini {
		q.Add(c)
	}
	return q
}
func (q *queuePane) showMenu(from widget.Component, items ...*widgets.MenuItem) {
	o := widget.DeviceOrigin(from)
	widgets.ShowContextMenu(from, paint2d.Pt(o.X, o.Y+from.Bounds().Dy()), items...)
}
func (q *queuePane) sync() {
	q.list.syncQueue()
	q.list.Invalidate()
	q.Invalidate()
}
func (q *queuePane) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(q.Look(), 320), style.Dip(q.Look(), 400)))
}
func (q *queuePane) Arrange(r paint2d.Rect) {
	q.SetBounds(r)
	b, lk := q.LocalBounds(), q.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	row := max(lk.Metrics().ControlH, d(28))
	foot := d(20)
	bottom := foot + 3*row + d(12)
	top := d(26)
	q.list.Arrange(paint2d.XYWH(0, top, b.Dx(), max(b.Dy()-top-bottom, d(40))))
	y := b.Dy() - bottom + d(4)
	for _, buttons := range [][]*widgets.Button{{q.add, q.remove, q.selectTrack}, {q.misc, q.options}} {
		x := float32(0)
		w := (b.Dx() - d(4)*float32(len(buttons)-1)) / float32(len(buttons))
		for _, c := range buttons {
			c.Arrange(paint2d.XYWH(x, y, w, row))
			x += w + d(4)
		}
		y += row + d(4)
	}
	w := (b.Dx() - d(4)*float32(len(q.mini)-1)) / float32(len(q.mini))
	for i, c := range q.mini {
		c.Arrange(paint2d.XYWH(float32(i)*(w+d(4)), y, w, row))
	}
}
func (q *queuePane) Paint(ctx *paint2d.Context) {
	lk, b := q.Look(), q.LocalBounds()
	pal := lk.Palette()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	lk.DrawLabel(ctx, paint2d.XYWH(0, 0, b.Dx(), d(22)), "Playlist", pal.Text, style.AlignStart)
	list := q.p.Transport.List
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), d(10)), paint2d.XYWH(0, b.Dy()-d(18), b.Dx(), d(18)), fmt.Sprintf("%d tracks · %s", list.Len(), players.Clock(list.Total())), pal.TextMuted, style.AlignStart)
}
func (q *queuePane) KeyPress(e widget.KeyEvent) bool {
	if e.Key == platform.KeyReturn && q.list.Focused() {
		return q.list.activate(q.list.Selected)
	}
	return false
}

// The whole playlist accepts files, including its heading and controls.
// The list's closer target handles rows and empty space with its own hover UI.
func (q *queuePane) DropTypes() []string { return []string{"text/uri-list"} }
func (q *queuePane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (q *queuePane) Drop(e widget.DropEvent) bool { return q.p.Host.AcceptFileDrop(e) }
func (q *queuePane) Describe(n *a11y.Node)        { n.Role, n.Name = a11y.RoleGroup, "Playlist" }
