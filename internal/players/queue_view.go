package players

import (
	"slices"
	"time"

	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// QueueView gives skins the stock list's navigation, scrolling and accessibility
// with the music player's browsing, activation and file-drop policy. A skin
// supplies RowGeo and painting; filtered/sorted views supply queue indices.
type QueueView struct {
	*widgets.ListView
	host         *Host
	queue        *Playlist
	rows         []int
	playing      int
	pointerPress bool
	pointerRow   int
	lastClickRow int
	lastClickAt  time.Time
}

func NewQueueView(h *Host) *QueueView {
	l := &QueueView{host: h, playing: -1}
	l.ListView = widgets.NewListView(0, func(i int) string {
		return h.Model.Transport.List.At(l.QueueIndex(i)).Label()
	}, l.choose)
	l.Init(l)
	l.OnActivate = l.activate
	l.ContextKeepsSelection = true
	l.SetFocusVisibleOnly(true)
	l.SetAccessibleName("Playlist")
	l.DisableTypeAhead = true
	l.ItemDetail = func(i int) string { return Clock(h.Model.Transport.List.At(l.QueueIndex(i)).Length) }
	l.DropMimes, l.DropActions = []string{"text/uri-list"}, platform.DragCopy
	l.OnDropAt = func(_ int, e widget.DropEvent) bool { return h.AcceptFileDrop(e) }
	l.Sync()
	return l
}

// QueueIndex resolves a visible row to the shared queue, or -1 when absent.
func (l *QueueView) QueueIndex(row int) int {
	if row < 0 || row >= len(l.rows) {
		return -1
	}
	return l.rows[row]
}

// Sync shows the shared queue in its natural order, retaining a browsed row.
func (l *QueueView) Sync() {
	rows := make([]int, l.host.Model.Transport.List.Len())
	for i := range rows {
		rows[i] = i
	}
	l.SyncRows(rows)
}

// SyncRows shows valid, unique queue indices in the supplied order. Filtering
// and sorting never change the underlying queue or activation's track identity.
func (l *QueueView) SyncRows(rows []int) {
	queue := l.host.Model.Transport.List
	oldSelected := l.Selected
	selected := l.QueueIndex(l.Selected)
	valid := make([]int, 0, len(rows))
	seen := make(map[int]bool, len(rows))
	for _, row := range rows {
		if row >= 0 && row < queue.Len() && !seen[row] {
			valid = append(valid, row)
			seen[row] = true
		}
	}
	changed := l.queue != queue || !slices.Equal(l.rows, valid)
	if changed {
		l.InterruptActivation()
	}
	playing := queue.Index()
	if l.queue != queue || selected < 0 || selected == l.playing && playing != l.playing {
		selected = playing
	}
	l.rows, l.Count, l.Selected = valid, len(valid), slices.Index(valid, selected)
	l.queue, l.playing = queue, playing
	if l.Selected >= 0 && (changed || oldSelected != l.Selected) {
		l.EnsureVisible(l.Selected)
	}
	l.Invalidate()
}

func (l *QueueView) choose(i int) {
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
func (l *QueueView) activate(row int) {
	i := l.QueueIndex(row)
	if !l.Enabled() || i < 0 || i >= l.host.Model.Transport.List.Len() {
		return
	}
	l.choose(row)
	l.host.SelectTrack(i)
}

// InterruptActivation clears a pending pointer click when the view changes.
func (l *QueueView) InterruptActivation() { l.lastClickAt = time.Time{} }

func (l *QueueView) MousePress(e widget.MouseEvent) bool {
	if !l.Enabled() {
		l.InterruptActivation()
		return true
	}
	// UITK item 28: RowAt checks Y alone. Honor the actual placed row viewport.
	if l.RowGeo != nil {
		if g := l.RowGeo(l.Look()); g != nil && !g.Rows.Contains(e.Pos) && !g.Bar.Contains(e.Pos) {
			l.InterruptActivation()
			if e.Button == platform.ButtonLeft {
				return true
			}
		}
	}
	if e.Button == platform.ButtonRight {
		l.InterruptActivation()
	}
	l.MarkPointerFocus()
	l.pointerRow, l.pointerPress = -1, true
	// UITK item 29: retain stock selection/context policy, but commit only
	// this application's uninterrupted double-click through stock Activate.
	activate := l.OnActivate
	l.OnActivate = func(int) {}
	handled := l.ListView.MousePress(e)
	l.OnActivate, l.pointerPress = activate, false
	if !handled || e.Button != platform.ButtonLeft || e.Mods.Ctrl() || e.Mods.Shift() || l.pointerRow < 0 {
		l.InterruptActivation()
		return handled
	}
	if l.pointerRow == l.lastClickRow && !l.lastClickAt.IsZero() && time.Since(l.lastClickAt) < 400*time.Millisecond {
		l.InterruptActivation()
		l.Activate(l.pointerRow)
	} else {
		l.lastClickRow, l.lastClickAt = l.pointerRow, time.Now()
	}
	return handled
}
func (l *QueueView) MouseExit() { l.InterruptActivation(); l.ListView.MouseExit() }
func (l *QueueView) FocusLost() { l.InterruptActivation(); l.ListView.FocusLost() }
func (l *QueueView) MouseWheel(e widget.MouseEvent) bool {
	l.InterruptActivation()
	return l.ListView.MouseWheel(e)
}
func (l *QueueView) KeyPress(e widget.KeyEvent) bool {
	l.InterruptActivation()
	if e.Key == platform.KeySpace || e.Mods.Ctrl() || e.Mods.Alt() {
		return false
	}
	if l.ListView.KeyPress(e) {
		l.MarkKeyboardFocus()
		return true
	}
	return false
}
func (l *QueueView) AccessibleAction(i int, action a11y.Action) bool {
	l.InterruptActivation()
	return l.ListView.AccessibleAction(i, action)
}
