package lantern

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// The playlist window.
//
// It is a window of its own rather than a panel that slides out of the main
// one, because a playlist that anchors to the edge of the player and travels
// with it is one of the two things this demo is for. rack.Rack is the
// arithmetic; the window half is rack.Desk, and what it needs of a
// desktop — being told where a window is and being able to put one
// somewhere — only X11 offers. The status line at the bottom of the main
// window says which of the two it got.

// queuePane is the playlist: a sortable table of the invented library, and
// a footer that counts it.
type queuePane struct {
	widget.Base
	p *Player

	table  *widgets.TableView
	search *widgets.TextField
	foot   *foot
	anchor *widgets.ComboBox

	// rows maps a table row onto a track index while a filter is on, and
	// is nil when the table is showing the whole list.
	rows []int
}

// track is the track a table row is showing.
func (q *queuePane) track(row int) int {
	if q.rows == nil {
		return row
	}
	if row < 0 || row >= len(q.rows) {
		return -1
	}
	return q.rows[row]
}

func newQueuePane(p *Player) *queuePane {
	q := &queuePane{p: p}
	q.Init(q)
	q.SetManagesChildren(true)

	pl := p.Transport.List
	q.table = widgets.NewTableView([]widgets.TableColumn{
		{Title: "#", Width: 34, Align: style.AlignEnd},
		{Title: "Track"},
		{Title: "Time", Width: 64, Align: style.AlignEnd},
	}, pl.Len(), func(row, col int) string {
		i := q.track(row)
		t := pl.At(i)
		switch col {
		case 0:
			return fmt.Sprintf("%d", i+1)
		case 2:
			return players.Clock(t.Length)
		default:
			return t.Label()
		}
	}, func(row int) { p.Transport.SelectTrack(q.track(row)) })
	q.table.OnActivate = func(row int) {
		p.Transport.SelectTrack(q.track(row))
		if p.Transport.State != players.Playing {
			p.Command(players.CmdPlayPause)
		}
	}
	q.table.SetAccessibleName("Playlist")

	// A filter that filters nothing: the field is here because a playlist
	// window of this shape had one, and because it is the one control in
	// all three players that takes text — which is what proves the bare
	// letters the transport uses are not stealing keys from a text field.
	q.search = widgets.NewTextField("", "Filter the queue", func(s string) { q.filter(s) })
	q.search.SetAccessibleName("Filter")

	q.anchor = widgets.NewComboBox([]string{"Right of the player", "Below the player", "Loose"}, 0, func(i int) {
		switch i {
		case 0:
			p.Desk.Attach(p.iList, p.iMain, rack.SideRight)
		case 1:
			p.Desk.Attach(p.iList, p.iMain, rack.SideBottom)
		default:
			p.Desk.Rack.Detach(p.iList)
		}
		p.refresh()
	})
	q.anchor.SetAccessibleName("Where the playlist sits")

	q.foot = &foot{p: p}
	q.foot.Init(q.foot)

	q.Add(q.search)
	q.Add(q.table)
	q.Add(q.anchor)
	q.Add(q.foot)
	return q
}

// filter narrows the visible rows. It is the one piece of behaviour in the
// playlist that is real, which makes it the honest place to show that
// typing in a field does not reach the transport's bare-letter keys.
func (q *queuePane) filter(text string) {
	q.rows = q.p.Transport.List.Find(text)
	if q.rows == nil {
		q.table.RowCount = q.p.Transport.List.Len()
	} else {
		q.table.RowCount = len(q.rows)
	}
	q.table.Selected = -1
	q.table.Invalidate()
	q.foot.Invalidate()
}

func (q *queuePane) sync() {
	pl := q.p.Transport.List
	if i := pl.Index(); i >= 0 {
		if row := q.rowOf(i); row >= 0 && q.table.Selected != row {
			q.table.Selected = row
		}
	}
	q.table.Invalidate()
	q.foot.Invalidate()
}

// rowOf is where a track index sits in the filtered rows, or -1.
func (q *queuePane) rowOf(track int) int {
	if q.rows == nil {
		return track
	}
	for row, i := range q.rows {
		if i == track {
			return row
		}
	}
	return -1
}

func (q *queuePane) Measure(c layout.Constraints) paintengine2d.Point {
	lk := q.Look()
	return c.Constrain(paintengine2d.Pt(style.Dip(lk, ListW), style.Dip(lk, 460)))
}

func (q *queuePane) Arrange(r paintengine2d.Rect) {
	q.SetBounds(r)
	lk := q.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	b := q.LocalBounds()
	h := lk.Metrics().ControlH
	q.search.Arrange(paintengine2d.XYWH(0, 0, b.Dx(), h))
	footH := dip(22)
	comboH := lk.Metrics().ComboH
	tableTop := h + dip(6)
	tableH := max(b.Dy()-tableTop-comboH-footH-dip(10), dip(60))
	q.table.Arrange(paintengine2d.XYWH(0, tableTop, b.Dx(), tableH))
	y := tableTop + tableH + dip(6)
	q.anchor.Arrange(paintengine2d.XYWH(0, y, b.Dx(), comboH))
	q.foot.Arrange(paintengine2d.XYWH(0, y+comboH+dip(4), b.Dx(), footH))
}

func (q *queuePane) KeyPress(e widget.KeyEvent) bool { return q.p.Keys(q.p.List, e) }

func (q *queuePane) Describe(n *a11y.Node) {
	n.Role = a11y.RoleGroup
	if n.Name == "" {
		n.Name = "Playlist"
	}
}

// foot counts what the table is showing.
type foot struct {
	widget.Base
	p *Player
}

func (f *foot) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(f.Look(), 120), style.Dip(f.Look(), 22)))
}

func (f *foot) Arrange(b paintengine2d.Rect) { f.SetBounds(b) }

func (f *foot) Paint(ctx *paintengine2d.Context) {
	lk, b := f.Look(), f.LocalBounds()
	lk.DrawLabel(ctx, b, f.text(), lk.Palette().TextMuted, style.AlignStart)
}

func (f *foot) text() string {
	pl := f.p.Transport.List
	return fmt.Sprintf("%d tracks · %s", pl.Len(), players.Clock(pl.Total()))
}

func (f *foot) Describe(n *a11y.Node) {
	n.Role = a11y.RoleStatusBar
	n.Name = f.text()
}
