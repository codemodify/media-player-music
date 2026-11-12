package aluminium

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// playlistList keeps the stock list's navigation and scrolling and reserves
// the player's transport shortcuts and context menu for the application.
type playlistList struct {
	*widgets.ListView
	p            *Player
	queue        *players.Playlist
	playing      int
	indices      []int
	query        string
	sortColumn   int
	descending   bool
	tracks       []players.Track
	pointerPress bool
	pointerRow   int
	lastClickRow int
	lastClickAt  time.Time
}

func newPlaylistList(p *Player) *playlistList {
	l := &playlistList{p: p, sortColumn: -1}
	l.ListView = widgets.NewListView(p.Transport.List.Len(), func(i int) string {
		return l.track(i).Label()
	}, l.choose)
	l.Init(l)
	l.OnActivate = func(i int) { l.activate(i) }
	l.ContextKeepsSelection = true
	l.OnContext = func(_ int, at paint2d.Point) { p.body.contextMenu(l, at) }
	l.SetFocusVisibleOnly(true)
	l.SetAccessibleName("Music library")
	l.DisableTypeAhead = true
	l.ItemDetail = func(i int) string { return players.Clock(l.track(i).Length) }
	l.RowGeo = l.geometry
	l.RowPaint = l.paintRow
	l.ScrollPaint = l.paintScroll
	l.DropMimes = []string{"text/uri-list"}
	l.DropActions = platform.DragCopy
	l.OnDropAt = func(_ int, e widget.DropEvent) bool { return p.Host.AcceptFileDrop(e) }
	l.syncQueue()
	return l
}

func (l *playlistList) geometry(lk style.LookAndFeel) *widgets.RowGeometry {
	parent := l.Parent()
	if parent == nil {
		return nil
	}
	box := parent.LocalBounds()
	rows, ok := skins.SkinSlotRect(lk, LayoutMain, "rows", box)
	if !ok {
		return nil
	}
	bar, _ := skins.SkinSlotRect(lk, LayoutMain, "scroll", box)
	o := l.Bounds().Min
	return &widgets.RowGeometry{
		Rows:      rows.Translate(paint2d.Pt(-o.X, -o.Y)),
		Bar:       bar.Translate(paint2d.Pt(-o.X, -o.Y)),
		RowHeight: style.Dip(lk, 17),
	}
}

func (l *playlistList) paintRow(ctx *paint2d.Context, row paint2d.Rect, i int, st style.ControlState) bool {
	lk := l.Look()
	col := paint2d.RGB(.08, .09, .1)
	bg := paint2d.White
	if i%2 == 1 {
		bg = paint2d.RGB(.92, .95, .98)
	}
	if i == l.Selected {
		bg = paint2d.RGB(.29, .5, .76)
		col = paint2d.White
	}
	ctx.DrawRect(row, paint2d.Fill(bg))
	if i == l.Selected && st.Focused() {
		lk.DrawFocusRing(ctx, row)
	}
	values := columnValues(l.track(i))
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 11))
	pad := style.Dip(lk, 4)
	for column, cell := range columnRects(row, lk) {
		align := style.AlignStart
		if column == 1 {
			align = style.AlignEnd
		}
		if column == 0 && l.queueIndex(i) == l.p.Transport.List.Index() {
			players.DrawGlyph(ctx, paint2d.XYWH(cell.Min.X, cell.Min.Y, style.Dip(lk, 15), cell.Dy()), players.GlyphPlay, col, style.Dip(lk, 1))
			cell.Min.X += style.Dip(lk, 15)
		}
		players.DrawTextIn(ctx, f, cell.Inset(pad), values[column], col, align)
	}
	return true
}
func (l *playlistList) paintScroll(ctx *paint2d.Context, track, thumb paint2d.Rect, st style.ControlState) bool {
	ctx.DrawRect(track, paint2d.Fill(paint2d.RGB(.87, .9, .94)))
	ctx.DrawRoundRect(thumb, style.Dip(l.Look(), 6), style.Dip(l.Look(), 6), paint2d.Fill(paint2d.RGB(.38, .63, .85)))
	ctx.DrawRoundRect(thumb, style.Dip(l.Look(), 6), style.Dip(l.Look(), 6), paint2d.StrokePaint(paint2d.RGB(.2, .4, .65), style.Dip(l.Look(), 1)))
	return true
}
func columnRects(row paint2d.Rect, lk style.LookAndFeel) [4]paint2d.Rect {
	timeW := style.Dip(lk, 55)
	remaining := max(row.Dx()-timeW, 0)
	widths := [4]float32{remaining * .4, timeW, remaining * .3, remaining * .3}
	var out [4]paint2d.Rect
	x := row.Min.X
	for i, w := range widths {
		out[i] = paint2d.XYWH(x, row.Min.Y, w, row.Dy())
		x += w
	}
	return out
}
func columnValues(t players.Track) [4]string {
	clock := "--:--"
	if t.Length > 0 {
		clock = players.Clock(t.Length)
	}
	return [4]string{t.Title, clock, t.Artist, t.Album}
}
func (l *playlistList) queueIndex(i int) int {
	if i < 0 || i >= len(l.indices) {
		return -1
	}
	return l.indices[i]
}
func (l *playlistList) track(i int) players.Track { return l.p.Transport.List.At(l.queueIndex(i)) }
func (l *playlistList) viewIndex(queueIndex int) int {
	for i, v := range l.indices {
		if v == queueIndex {
			return i
		}
	}
	return -1
}
func (l *playlistList) applyFilter(query string) {
	l.query = query
	l.rebuild()
	l.Selected = l.viewIndex(l.p.Transport.List.Index())
	l.OffsetY = 0
	l.resetClick()
	l.Invalidate()
}
func (l *playlistList) sortBy(column int) {
	if l.sortColumn == column {
		l.descending = !l.descending
	} else {
		l.sortColumn = column
		l.descending = false
	}
	l.rebuild()
	l.resetClick()
	l.Invalidate()
}
func (l *playlistList) rebuild() {
	queue := l.p.Transport.List
	selected := l.queueIndex(l.Selected)
	ids := queue.Find(l.query)
	if ids == nil {
		ids = make([]int, queue.Len())
		for i := range ids {
			ids[i] = i
		}
	}
	if l.sortColumn >= 0 {
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := queue.At(ids[i]), queue.At(ids[j])
			if l.sortColumn == 1 {
				if l.descending {
					return a.Length > b.Length
				}
				return a.Length < b.Length
			}
			av, bv := columnValues(a), columnValues(b)
			x, y := strings.ToLower(av[l.sortColumn]), strings.ToLower(bv[l.sortColumn])
			if l.descending {
				return x > y
			}
			return x < y
		})
	}
	l.indices = ids
	l.Count = len(ids)
	l.Selected = l.viewIndex(selected)
	l.clampSelected()
	l.tracks = append([]players.Track(nil), queue.Tracks...)
}
func (l *playlistList) clampSelected() {
	if l.Selected >= l.Count {
		l.Selected = l.Count - 1
	}
}

func (l *playlistList) choose(i int) {
	if i < 0 || i >= l.Count {
		return
	}
	l.Selected = i
	l.EnsureVisible(i)
	l.Invalidate()
	if l.pointerPress {
		l.pointerRow = i
	}
}

func (l *playlistList) activate(i int) bool {
	if !l.Enabled() || i < 0 || i >= l.Count {
		return false
	}
	l.choose(i)
	l.p.Host.SelectTrack(l.queueIndex(i))
	return true
}

func (l *playlistList) resetClick() { l.lastClickAt = time.Time{} }

// syncQueue preserves a browsed selection across playback ticks. When the
// selection follows playback, transport changes can keep it following.
func (l *playlistList) syncQueue() {
	queue := l.p.Transport.List
	playing := queue.Index()
	follow := l.queueIndex(l.Selected) == l.playing
	changed := l.queue != queue || !reflect.DeepEqual(l.tracks, queue.Tracks)
	if changed {
		l.rebuild()
		l.resetClick()
	}
	if l.queue != queue || l.Selected < 0 || (follow && playing != l.playing) {
		l.Selected = l.viewIndex(playing)
		if l.Selected >= 0 {
			l.EnsureVisible(l.Selected)
		}
	}
	l.queue, l.playing = queue, playing
}

func (l *playlistList) MousePress(e widget.MouseEvent) bool {
	if !l.Enabled() {
		l.resetClick()
		return true
	}
	// UITK v0.23.4 RowAt checks the vertical range only. iTunes's
	// original playlist leaves space beside its rows, which is not a row.
	if g := l.geometry(l.Look()); g != nil && !g.Rows.Contains(e.Pos) && !g.Bar.Contains(e.Pos) {
		l.resetClick()
		if e.Button == platform.ButtonLeft {
			return true
		}
	}
	if e.Button == platform.ButtonRight {
		l.resetClick()
	}
	l.MarkPointerFocus()
	l.pointerRow, l.pointerPress = -1, true
	// v0.23.4 keeps its double-click history after an interrupted gesture.
	// Let it select and open context menus, then commit only the uninterrupted
	// gesture below through its public activation command.
	activate := l.OnActivate
	l.OnActivate = func(int) {}
	handled := l.ListView.MousePress(e)
	l.OnActivate = activate
	l.pointerPress = false
	if !handled || e.Button != platform.ButtonLeft || e.Mods.Ctrl() || e.Mods.Shift() || l.pointerRow < 0 {
		l.resetClick()
		return handled
	}
	// Match the toolkit TableView's double-click interval. OnSelect identifies
	// a real row hit, so scrollbar presses cannot activate the selected row.
	if l.pointerRow == l.lastClickRow && !l.lastClickAt.IsZero() && time.Since(l.lastClickAt) < 400*time.Millisecond {
		l.resetClick()
		l.Activate(l.pointerRow)
	} else {
		l.lastClickRow, l.lastClickAt = l.pointerRow, time.Now()
	}
	return handled
}

func (l *playlistList) MouseExit() {
	l.resetClick()
	l.ListView.MouseExit()
}

func (l *playlistList) FocusLost() {
	l.resetClick()
	l.ListView.FocusLost()
}

func (l *playlistList) MouseWheel(e widget.MouseEvent) bool {
	l.resetClick()
	return l.ListView.MouseWheel(e)
}

func (l *playlistList) KeyPress(e widget.KeyEvent) bool {
	l.resetClick()
	if e.Key == platform.KeySpace || e.Mods.Ctrl() || e.Mods.Alt() {
		return false
	}
	if handled := l.ListView.KeyPress(e); handled {
		l.MarkKeyboardFocus()
		return true
	}
	return false
}

func (l *playlistList) AccessibleAction(i int, action a11y.Action) bool {
	l.resetClick()
	return l.ListView.AccessibleAction(i, action)
}
