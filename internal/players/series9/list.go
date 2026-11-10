package series9

import (
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
	pointerPress bool
	pointerRow   int
	lastClickRow int
	lastClickAt  time.Time
}

func newPlaylistList(p *Player) *playlistList {
	l := &playlistList{p: p}
	l.ListView = widgets.NewListView(p.Transport.List.Len(), func(i int) string {
		return p.Transport.List.At(i).Label()
	}, l.choose)
	l.Init(l)
	l.OnActivate = func(i int) { l.activate(i) }
	l.ContextKeepsSelection = true
	l.OnContext = func(_ int, at paint2d.Point) { p.body.playlist.contextMenu(l, at) }
	l.SetFocusVisibleOnly(true)
	l.SetAccessibleName("Playlist")
	l.DisableTypeAhead = true
	l.ItemDetail = func(i int) string { return players.Clock(p.Transport.List.At(i).Length) }
	l.RowGeo = l.geometry
	l.RowPaint = l.paintRow
	l.ScrollPaint = l.paintScroll
	l.DropMimes = []string{"text/uri-list"}
	l.DropActions = platform.DragCopy
	l.OnDropAt = func(_ int, e widget.DropEvent) bool { return p.Host.AcceptFileDrop(e) }
	return l
}

func (l *playlistList) geometry(lk style.LookAndFeel) *widgets.RowGeometry {
	parent := l.Parent()
	if parent == nil {
		return nil
	}
	box := parent.LocalBounds()
	rows, ok := skins.SkinSlotRect(lk, LayoutPlaylist, "rows", box)
	if !ok {
		return nil
	}
	bar, _ := skins.SkinSlotRect(lk, LayoutPlaylist, "scroll", box)
	o := l.Bounds().Min
	return &widgets.RowGeometry{
		Rows:      rows.Translate(paint2d.Pt(-o.X, -o.Y)),
		Bar:       bar.Translate(paint2d.Pt(-o.X, -o.Y)),
		RowHeight: style.Dip(lk, 17),
	}
}

func (l *playlistList) paintRow(ctx *paint2d.Context, row paint2d.Rect, i int, st style.ControlState) bool {
	lk := l.Look()
	col := paint2d.White
	if i == l.p.Transport.List.Index() {
		col = paint2d.White
	}
	if i == l.Selected {
		ctx.DrawRect(row, paint2d.Fill(paint2d.RGB(.17, .28, .51)))
		if st.Focused() {
			lk.DrawFocusRing(ctx, row)
		}
	}
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 11))
	clock := l.ItemDetail(i)
	pad, width := style.Dip(lk, 2), f.Advance(clock)
	players.DrawTextIn(ctx, f, paint2d.XYWH(row.Max.X-width-pad, row.Min.Y, width, row.Dy()), clock, col, style.AlignEnd)
	players.DrawTextIn(ctx, f, paint2d.XYWH(row.Min.X+pad, row.Min.Y, max(row.Dx()-width-3*pad, 0), row.Dy()), l.ItemText(i), col, style.AlignStart)
	return true
}

func (l *playlistList) paintScroll(ctx *paint2d.Context, track, thumb paint2d.Rect, st style.ControlState) bool {
	ctx.DrawRect(track, paint2d.Fill(paint2d.RGB(.07, .11, .2)))
	ctx.DrawRect(thumb, paint2d.Fill(paint2d.RGB(.57, .65, .81)))
	ctx.DrawRect(thumb, paint2d.StrokePaint(paint2d.RGB(.27, .36, .54), 1))
	return true
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
	l.p.Host.SelectTrack(i)
	return true
}

func (l *playlistList) resetClick() { l.lastClickAt = time.Time{} }

// syncQueue preserves a browsed selection across playback ticks. When the
// selection follows playback, transport changes can keep it following.
func (l *playlistList) syncQueue() {
	queue := l.p.Transport.List
	count, playing := queue.Len(), queue.Index()
	if l.queue != queue || l.Count != count {
		l.resetClick()
	}
	l.Count = count
	if l.queue != queue || l.Selected < 0 || l.Selected >= count || (l.Selected == l.playing && playing != l.playing) {
		l.Selected = playing
		if playing >= 0 {
			l.EnsureVisible(playing)
		}
	}
	l.queue, l.playing = queue, playing
}

func (l *playlistList) MousePress(e widget.MouseEvent) bool {
	if !l.Enabled() {
		l.resetClick()
		return true
	}
	// UITK v0.23.4 RowAt checks the vertical range only. the original skin's
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
