package players

import (
	"testing"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func openQueueView(t *testing.T) (*uitoolkit.Application, *app.Window, *Host, *QueueView, *int) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, "dark")
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
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
		return &widgets.RowGeometry{Rows: paintengine2d.XYWH(8*d, 8*d, b.Dx()-36*d, b.Dy()-16*d), Bar: paintengine2d.XYWH(b.Dx()-22*d, 8*d, 14*d, b.Dy()-16*d), RowHeight: 18 * d}
	}
	w.SetContent(q)
	a.PumpOnce()
	return a, w, h, q, plays
}

func queueClick(w *app.Window, at paintengine2d.Point, button platform.MouseButton) {
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
	w.Inject(platform.Event{Kind: platform.EventScroll, Pos: at, Scroll: paintengine2d.Pt(0, 1)})
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
	gutter := widget.DeviceOrigin(q).Add(paintengine2d.Pt(g.Rows.Min.X-2, g.Rows.Min.Y+g.RowHeight/2))
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
