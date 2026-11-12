package panel

import (
	"fmt"
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

// tracks supplies the playlist's labels, actions and artwork to a stock
// ListView. Selection, scrolling, thumb dragging, keyboard navigation and
// accessible items belong to the toolkit; the player supplies its look and
// the geometry from its skin's layout.
type tracks struct {
	*widgets.ListView
	p            *Player
	geo          *listGeo
	queue        *players.Playlist
	playing      int
	pointerPress bool
	pointerRow   int
	lastClickRow int
	lastClickAt  time.Time
}

// listGeo is a panel face's playlist, in the list's own coordinates.
type listGeo struct {
	rows, bar paint2d.Rect
	rowH      float32
}

func newTracks(p *Player) *tracks {
	t := &tracks{p: p}
	t.ListView = widgets.NewListView(p.Transport.List.Len(), t.label, t.choose)
	// Bind the outer component so focus and hit testing reach the small
	// adapters that reserve player shortcuts and its skin context menu.
	t.Init(t)
	t.OnActivate = func(i int) { t.activate(i) }
	t.ContextKeepsSelection = true
	t.SetFocusVisibleOnly(true)
	t.SetAccessibleName("Playlist")
	// Bare letters are transport commands, so they must bubble to the pane.
	t.DisableTypeAhead = true
	t.ItemDetail = func(i int) string { return durationText(p.Transport.List.At(i).Length) }
	t.RowGeo = t.rowGeometry
	t.RowPaint = t.paintRow
	t.ScrollPaint = t.paintScroll
	t.OnContext = func(_ int, at paint2d.Point) { p.skinMenu(t, at) }
	t.DropMimes = []string{"text/uri-list"}
	t.DropActions = platform.DragCopy
	t.OnDropAt = func(_ int, e widget.DropEvent) bool { return p.Host.AcceptFileDrop(e) }
	return t
}

func (t *tracks) label(i int) string {
	return fmt.Sprintf("%d. %s", i+1, t.p.Transport.List.At(i).Label())
}

func (t *tracks) rowGeometry(lk style.LookAndFeel) *widgets.RowGeometry {
	if t.geo == nil || !faceOf(lk).panelled() {
		return nil
	}
	g := &widgets.RowGeometry{Rows: t.geo.rows, Bar: t.geo.bar, RowHeight: t.geo.rowH}
	if _, h, ok := skins.SkinSpriteSize(lk, "list.thumb"); ok {
		g.ThumbLength = style.Dip(lk, h)
	}
	return g
}

// choose changes the highlighted row without changing playback.
func (t *tracks) choose(i int) {
	if i < 0 || i >= t.Count {
		return
	}
	t.Selected = i
	t.EnsureVisible(i)
	t.Invalidate()
	if t.pointerPress {
		t.pointerRow = i
	}
}

func (t *tracks) activate(i int) bool {
	if !t.Enabled() || i < 0 || i >= t.Count {
		return false
	}
	t.choose(i)
	t.p.Host.SelectTrack(i)
	return true
}

func (t *tracks) resetClick() { t.lastClickAt = time.Time{} }

// syncQueue preserves a browsed selection across playback ticks. When the
// selection follows playback, transport changes can keep it following.
func (t *tracks) syncQueue() {
	queue := t.p.Transport.List
	count, playing := queue.Len(), queue.Index()
	if t.queue != queue || t.Count != count {
		t.resetClick()
	}
	t.Count = count
	if t.queue != queue || t.Selected < 0 || t.Selected >= count || (t.Selected == t.playing && playing != t.playing) {
		t.Selected = playing
		if playing >= 0 {
			t.EnsureVisible(playing)
		}
	}
	t.queue, t.playing = queue, playing
}

func (t *tracks) paintRow(ctx *paint2d.Context, row paint2d.Rect, i int, st style.ControlState) bool {
	lk := t.Look()
	f := faceOf(lk)
	playing := i == t.p.Transport.List.Index()
	// Focus, hover and inactivity come from ListView. The player marks the
	// playing track independently of its current keyboard selection.
	st &^= style.StateChecked
	if playing {
		st |= style.StateChecked
	}
	if t.geo == nil || !f.panelled() {
		tr := t.p.Transport.List.At(i)
		text := fmt.Sprintf("%2d.  %s   %s", i+1, durationText(tr.Length), tr.Label())
		lk.DrawListRow(ctx, row, st, text)
		return true
	}
	in := f.ink(lk)
	base := lk.Font()
	if in.rowBold {
		base = lk.BoldFont()
	}
	face := players.ScaledFace(base, style.Dip(lk, in.rowSize))
	col := in.row
	if playing {
		col = in.current
	}
	if i == t.Selected {
		ctx.DrawRect(row, paint2d.Fill(in.selected))
		if !playing {
			col = in.selectedText
		}
	}
	if i == t.Selected && t.State().Focused() {
		lk.DrawFocusRing(ctx, row)
	}
	clock := t.ItemDetail(i)
	cw, pad := face.Advance(clock), style.Dip(lk, 3)
	players.DrawTextIn(ctx, face, paint2d.XYWH(row.Max.X-cw-pad, row.Min.Y, cw, row.Dy()), clock, col, style.AlignEnd)
	players.DrawTextIn(ctx, face, paint2d.XYWH(row.Min.X+pad, row.Min.Y, max(row.Dx()-cw-3*pad, 0), row.Dy()), t.label(i), col, style.AlignStart)
	return true
}

func (t *tracks) paintScroll(ctx *paint2d.Context, track, thumb paint2d.Rect, st style.ControlState) bool {
	lk := t.Look()
	if t.geo == nil || !faceOf(lk).panelled() {
		return false
	}
	// The artwork sits on the panel's design grid; ListView owns the
	// scroll offset and all hit testing and dragging of the thumb.
	thumb = paint2d.XYWH(thumb.Min.X, track.Min.Y+snapDesign(thumb.Min.Y-track.Min.Y, style.Dip(lk, 1)), thumb.Dx(), thumb.Dy())
	name := "list.thumb"
	if st.Pressed() {
		name = "list.thumb.down"
	}
	return art(lk, ctx, thumb, name)
}

func (t *tracks) MousePress(e widget.MouseEvent) bool {
	if !t.Enabled() {
		t.resetClick()
		return t.ListView.MousePress(e)
	}
	if e.Button == platform.ButtonRight {
		t.resetClick()
	}
	t.MarkPointerFocus()
	t.pointerRow, t.pointerPress = -1, true
	// v0.23.4 keeps its double-click history after an interrupted gesture.
	// Let it select and open context menus, then commit only the uninterrupted
	// gesture below through its public activation command.
	activate := t.OnActivate
	t.OnActivate = func(int) {}
	handled := t.ListView.MousePress(e)
	t.OnActivate = activate
	t.pointerPress = false
	if !handled || e.Button != platform.ButtonLeft || e.Mods.Ctrl() || e.Mods.Shift() || t.pointerRow < 0 {
		t.resetClick()
		return handled
	}
	// Match the toolkit TableView's double-click interval. OnSelect identifies
	// a real row hit, so scrollbar presses cannot activate the selected row.
	if t.pointerRow == t.lastClickRow && !t.lastClickAt.IsZero() && time.Since(t.lastClickAt) < 400*time.Millisecond {
		t.resetClick()
		t.Activate(t.pointerRow)
	} else {
		t.lastClickRow, t.lastClickAt = t.pointerRow, time.Now()
	}
	return handled
}

func (t *tracks) MouseExit() {
	t.resetClick()
	t.ListView.MouseExit()
}

func (t *tracks) FocusLost() {
	t.resetClick()
	t.ListView.FocusLost()
}

func (t *tracks) MouseWheel(e widget.MouseEvent) bool {
	t.resetClick()
	return t.ListView.MouseWheel(e)
}

func (t *tracks) KeyPress(e widget.KeyEvent) bool {
	t.resetClick()
	if e.Key == platform.KeySpace || e.Mods.Ctrl() || e.Mods.Alt() {
		return false
	}
	if handled := t.ListView.KeyPress(e); handled {
		t.MarkKeyboardFocus()
		return true
	}
	return false
}

func (t *tracks) AccessibleAction(i int, a a11y.Action) bool {
	t.resetClick()
	return t.ListView.AccessibleAction(i, a)
}
