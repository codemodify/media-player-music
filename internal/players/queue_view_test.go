package players

import (
	"fmt"
	"testing"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func openQueueView(t *testing.T) (*ui.Application, *ui.Window, *Host, *QueueView, *int) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, "dark")
	a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
	w, err := a.NewWindow(platform.WindowOptions{Width: 240, Height: 200, Headless: true, Decorations: platform.DecorationsNone})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	h := NewHost(a)
	h.Model.Transport.List = NewPlaylist(Track{Title: "One", Length: time.Minute}, Track{Title: "Two"}, Track{Title: "Three"})
	plays := new(int)
	h.Run = func(c Command) {
		if c == CmdPlay {
			*plays++
		}
		h.Model.Transport.Do(c)
	}
	q := NewQueueView(h)
	q.RowGeo = func(lk style.LookAndFeel) *widgets.RowGeometry {
		b, d := q.LocalBounds(), style.Dip(lk, 1)
		return &widgets.RowGeometry{Rows: paint2d.XYWH(8*d, 8*d, b.Dx()-36*d, b.Dy()-16*d), Bar: paint2d.XYWH(b.Dx()-22*d, 8*d, 14*d, b.Dy()-16*d), RowHeight: 18 * d}
	}
	w.SetContent(q)
	a.PumpOnce()
	return a, w, h, q, plays
}

func queueClick(w *ui.Window, at paint2d.Point, button platform.MouseButton) {
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Pos: at, Button: button})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Pos: at, Button: button})
}

func TestQueueViewFilteredOrderBrowsesAndActivatesSharedTrackIdentity(t *testing.T) {
	a, w, h, q, plays := openQueueView(t)
	q.SyncRows([]int{2, -1, 0, 2, 99})
	a.PumpOnce()
	if q.Count != 2 || q.QueueIndex(0) != 2 || q.QueueIndex(1) != 0 || q.QueueIndex(2) != -1 {
		t.Fatal("filtered rows lost valid unique queue identities")
	}
	queueClick(w, q.AccessibleItems()[0].Bounds.Center(), platform.ButtonLeft)
	a.PumpOnce()
	q.SyncRows([]int{0, 2})
	a.PumpOnce()
	if q.Selected != 1 || h.Model.Transport.List.Index() != 0 || *plays != 0 {
		t.Fatal("browse/reorder changed playback or the browsed track")
	}
	w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
	a.PumpOnce()
	if h.Model.Transport.List.Index() != 2 || *plays != 1 {
		t.Fatal("Enter activated a view index instead of its mapped track")
	}
	h.Model.Transport.State = Paused
	if !w.AccessibleAction(widget.ItemID(q, 1), a11y.ActionDefault) || *plays != 2 || h.Model.Transport.State != Playing {
		t.Fatal("accessible mapped activation failed to resume once")
	}
	h.Model.Transport.List.Tracks[2].Title = "Updated"
	q.SyncRows([]int{0, 2})
	if q.ItemText(1) != "Updated" || q.Selected != 1 || *plays != 2 {
		t.Fatal("metadata refresh changed selection or started playback")
	}
}

func TestQueueViewDoubleClickAndInterruptedGestures(t *testing.T) {
	a, w, h, q, plays := openQueueView(t)
	at := q.AccessibleItems()[1].Bounds.Center()
	queueClick(w, at, platform.ButtonLeft)
	a.PumpOnce()
	w.Inject(platform.Event{Kind: platform.EventScroll, Pos: at, Scroll: paint2d.Pt(0, 1)})
	a.PumpOnce()
	queueClick(w, at, platform.ButtonLeft)
	a.PumpOnce()
	if *plays != 0 {
		t.Fatal("an interrupted click activated playback")
	}
	queueClick(w, at, platform.ButtonLeft)
	a.PumpOnce()
	if *plays != 1 || h.Model.Transport.List.Index() != 1 {
		t.Fatal("uninterrupted double-click did not play once")
	}
	q.SetEnabled(false)
	queueClick(w, at, platform.ButtonLeft)
	queueClick(w, at, platform.ButtonLeft)
	a.PumpOnce()
	if w.AccessibleAction(widget.ItemID(q, 1), a11y.ActionDefault) || *plays != 1 {
		t.Fatal("disabled playlist input started playback")
	}
}

func TestQueueViewGutterAndFilteredRefreshInterruptPointerActivation(t *testing.T) {
	a, w, _, q, plays := openQueueView(t)
	at := q.AccessibleItems()[1].Bounds.Center()
	queueClick(w, at, platform.ButtonLeft)
	a.PumpOnce()
	g := q.RowGeo(q.Look())
	gutter := widget.DeviceOrigin(q).Add(paint2d.Pt(g.Rows.Min.X-2, g.Rows.Min.Y+g.RowHeight/2))
	queueClick(w, gutter, platform.ButtonLeft)
	a.PumpOnce()
	queueClick(w, at, platform.ButtonLeft)
	a.PumpOnce()
	if *plays != 0 || q.Selected != 1 {
		t.Fatal("horizontal gutter counted as a playlist row")
	}
	q.SyncRows([]int{2, 1, 0})
	a.PumpOnce()
	queueClick(w, q.AccessibleItems()[1].Bounds.Center(), platform.ButtonLeft)
	a.PumpOnce()
	if *plays != 0 {
		t.Fatal("row reordering retained a stale double-click")
	}
}

func TestQueueViewScrollbarCannotExportRowsAndDropsUseTheAuthoredViewport(t *testing.T) {
	a, w, h, q, plays := openQueueView(t)
	var tracks []Track
	for i := 0; i < 20; i++ {
		tracks = append(tracks, Track{Title: fmt.Sprintf("Track %02d", i)})
	}
	h.Model.Transport.List = NewPlaylist(tracks...)
	q.Sync()
	q.RowGeo = func(lk style.LookAndFeel) *widgets.RowGeometry {
		b, d := q.LocalBounds(), style.Dip(lk, 1)
		return &widgets.RowGeometry{Rows: paint2d.XYWH(8*d, 44*d, b.Dx()-36*d, b.Dy()-52*d), Bar: paint2d.XYWH(b.Dx()-22*d, 44*d, 14*d, b.Dy()-52*d), RowHeight: 18 * d}
	}
	q.OffsetY = 18 * 1.75
	q.Selected = 1
	a.PumpOnce()
	g := q.RowGeo(q.Look())
	drags := 0
	q.OnDrag = func(rows []int) *widget.Drag {
		drags++
		if len(rows) != 1 || rows[0] != 1 {
			t.Fatal("row export lost selection identity")
		}
		return &widget.Drag{Types: []string{"text/plain"}, Payload: rows}
	}
	// This uses the inherited stock DragSource entry point, which is called
	// separately from our application MousePress gesture guard.
	for x := g.Rows.Max.X; x < q.LocalBounds().Max.X; x++ {
		if q.DragAt(paint2d.Pt(x, g.Rows.Min.Y+g.RowHeight/3)) != nil {
			t.Fatal("scrollbar/gutter exported the row at the same height")
		}
	}
	if drags != 0 || *plays != 0 || q.Selected != 1 {
		t.Fatal("non-row drag changed selection/playback")
	}
	if q.DragAt(g.Rows.Min.Add(paint2d.Pt(5, g.RowHeight/3))) == nil || drags != 1 {
		t.Fatal("valid selected-row drag no longer reaches the stock source")
	}
	loads, index := 0, -1
	h.DropFiles = func(paths []string) {
		loads++
		if len(paths) != 1 || paths[0] != "/tmp/queue viewport.mp3" {
			t.Fatal("file-drop decoding changed")
		}
	}
	accept := q.OnDropAt
	q.OnDropAt = func(i int, e widget.DropEvent) bool { index = i; return accept(i, e) }
	off := w.Surface().(*platform.Offscreen)
	pos := widget.DeviceOrigin(q).Add(g.Rows.Min).Add(paint2d.Pt(5, g.RowHeight*.1))
	off.SimulateDropAction(pos, map[string][]byte{"text/uri-list": []byte("file:///tmp/queue%20viewport.mp3\r\n")}, platform.DragCopy, platform.DragCopy)
	a.PumpOnce()
	if taken := off.DropTaken(); taken == nil || !*taken || loads != 1 || index != 1 || *plays != 0 {
		t.Fatalf("drop missed authored origin/offset: accepted%v loads%d index%d plays%d", taken, loads, index, *plays)
	}
}
