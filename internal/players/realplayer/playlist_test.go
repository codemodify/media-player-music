package realplayer

import (
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
	"reflect"
	"testing"
	"time"
)

func pointer(w *app.Window, button platform.MouseButton, at paintengine2d.Point) {
	w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: at})
}
func TestPlaylistBrowseActivateContextAndInterrupt(t *testing.T) {
	a, p := open(t, 1.75)
	list, plays := p.body.queue.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	at := list.AccessibleItems()[2].Bounds.Center()
	pointer(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	p.Refresh()
	a.PumpOnce()
	if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || plays != 0 {
		t.Fatal("single click changed playback or lost browsing")
	}
	key(a, p.Main, platform.KeyDown, 0)
	if list.Selected != 3 || plays != 0 {
		t.Fatal("Down started playback")
	}
	key(a, p.Main, platform.KeyReturn, 0)
	if plays != 1 || p.Transport.List.Index() != 3 || p.Transport.State != players.Playing {
		t.Fatal("Enter did not play once")
	}
	p.Transport.State = players.Paused
	if !p.Main.AccessibleAction(widget.ItemID(list, 3), a11y.ActionDefault) || plays != 2 || p.Transport.State != players.Playing {
		t.Fatal("accessible activation did not resume")
	}
	pointer(p.Main, platform.ButtonLeft, at)
	pointer(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 3 || p.Transport.List.Index() != 2 {
		t.Fatal("double click did not activate once")
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 11*time.Second
	pointer(p.Main, platform.ButtonRight, list.AccessibleItems()[1].Bounds.Center())
	a.PumpOnce()
	if p.Main.Popup() == nil || plays != 3 || list.Selected != 2 || p.Transport.Pos != 11*time.Second {
		t.Fatal("context menu changed selection/playback")
	}
	key(a, p.Main, platform.KeyEscape, 0)
	pointer(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 3 {
		t.Fatal("context preserved stale double click")
	}
	g := list.RowGeo(list.Look())
	pointer(p.Main, platform.ButtonLeft, widget.DeviceOrigin(list).Add(g.Bar.Center()))
	a.PumpOnce()
	pointer(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 3 {
		t.Fatal("scrollbar gesture activated row")
	}
	list.MousePress(widget.MouseEvent{Button: platform.ButtonLeft, Pos: paintengine2d.Pt(g.Rows.Min.X-1, g.Rows.Min.Y+g.RowHeight/2)})
	pointer(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 3 {
		t.Fatal("gutter gesture activated row")
	}
	list.MouseExit()
	pointer(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 3 {
		t.Fatal("pointer exit preserved double click")
	}
	list.SetEnabled(false)
	pointer(p.Main, platform.ButtonLeft, at)
	pointer(p.Main, platform.ButtonRight, at)
	a.PumpOnce()
	key(a, p.Main, platform.KeyReturn, 0)
	if plays != 3 || p.Main.Popup() != nil || p.Main.AccessibleAction(widget.ItemID(list, 2), a11y.ActionDefault) {
		t.Fatal("disabled queue accepted activation/context")
	}
}
func TestPlaylistScrollDropAndSliderSecondaryPolicy(t *testing.T) {
	a, p := open(t, 1.75)
	list := p.body.queue.list
	for i := 0; i < 70; i++ {
		p.Transport.List.Tracks = append(p.Transport.List.Tracks, players.Track{Title: "Long queue item", Length: 2 * time.Minute})
	}
	p.Refresh()
	a.PumpOnce()
	p.Main.RequestFocus(list)
	key(a, p.Main, platform.KeyEnd, 0)
	items := list.AccessibleItems()
	last := items[len(items)-1]
	if list.Selected != list.Count-1 || last.Bounds.Empty() || last.Bounds.Max.Y > widget.DeviceBounds(list).Max.Y+1 {
		t.Fatal("End did not scroll last item into viewport")
	}
	var got [][]string
	p.Host.DropFiles = func(paths []string) { got = append(got, paths) }
	e := widget.DropEvent{Mime: "text/uri-list", Action: platform.DragCopy, Paths: []string{"/tmp/music-folder", "/tmp/one.mp3"}}
	for _, target := range []widget.DropTarget{list, p.body.queue, p.body} {
		if !target.Drop(e) {
			t.Fatalf("%T refused local Copy drop", target)
		}
	}
	if len(got) != 3 || !reflect.DeepEqual(got[0], e.Paths) {
		t.Fatal("local paths not delivered")
	}
	e.Paths[0] = "mutated"
	if got[0][0] == e.Paths[0] {
		t.Fatal("drop paths were aliased")
	}
	e.Remote = []string{"file://remote/tmp/music"}
	if p.body.Drop(e) {
		t.Fatal("remote URI accepted")
	}
	e.Remote = nil
	e.Action = platform.DragMove
	if p.body.queue.Drop(e) {
		t.Fatal("Move accepted")
	}
	if len(got) != 3 {
		t.Fatal("rejected drop delivered")
	}
	for _, f := range []*classicFader{p.body.seek, p.body.volume, p.body.eq.preamp, p.body.eq.bands[0], p.body.eq.balance} {
		before := f.Value
		at := widget.DeviceOrigin(f).Add(paintengine2d.Pt(2, 2))
		pointer(p.Main, platform.ButtonRight, at)
		a.PumpOnce()
		if f.Value != before || f.Dragging() || p.Main.Popup() == nil {
			t.Fatalf("secondary slider press edited value/failed context: %s", f.Label)
		}
		key(a, p.Main, platform.KeyEscape, 0)
	}
}
