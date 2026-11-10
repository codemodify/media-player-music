package fandom

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestPlaylistResizeKeepsSlotsRowsAndMainGeometryUsable(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			var initialRows paint2d.Rect
			for i, size := range [][2]int{{190, 246}, {310, 356}} {
				p.List().SetSize(size[0], size[1])
				a.PumpOnce()
				if w, h := p.List().Size(); w != size[0] || h != size[1] {
					t.Fatal("playlist resize was not acknowledged")
				}
				if w, h := p.Main().Size(); w != 490 || h != 330 {
					t.Fatal("resizing playlist changed the fixed combined Main/Settings window")
				}
				pane := p.listPane
				for _, c := range pane.Children() {
					r, parent := c.Bounds(), pane.LocalBounds()
					if c.Visible() && (r.Empty() || r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > parent.Max.X || r.Max.Y > parent.Max.Y) {
						t.Fatalf("%dx%d playlist clipped %T: %v", size[0], size[1], c, r)
					}
				}
				g := pane.list.geometry(pane.list.Look())
				if g == nil || g.Rows.Empty() || g.Bar.Empty() || g.Rows.Max.X > g.Bar.Min.X {
					t.Fatal("playlist rows overlap their scrollbar or lack authored geometry")
				}
				if i == 0 {
					initialRows = g.Rows
				} else if g.Rows.Dx() <= initialRows.Dx() || g.Rows.Dy() <= initialRows.Dy() {
					t.Fatal("enlarging the playlist did not enlarge its row viewport")
				}
				if !p.List().ShapeActive() || p.List().CaptureSurface().NRGBAAt(0, 0).A != 0 {
					t.Fatal("resizing discarded the playlist's transparent outline")
				}
				playertest.Audit(t, "Fandom playlist", p.List())
			}
		})
	}
}

func TestPlaylistBrowsesThenActivatesExactlyOnceAndResumes(t *testing.T) {
	a, p := open(t, 1.75)
	list, plays := p.listPane.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	at := list.AccessibleItems()[2].Bounds.Center()
	click(p.List(), platform.ButtonLeft, at)
	a.PumpOnce()
	p.Refresh()
	a.PumpOnce()
	if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || plays != 0 {
		t.Fatal("single-click browsing changed playback or refresh reset selection")
	}
	key(a, p.List(), platform.KeyDown, 0)
	if list.Selected != 3 || plays != 0 {
		t.Fatal("arrow navigation started playback")
	}
	key(a, p.List(), platform.KeyReturn, 0)
	if p.Transport.List.Index() != 3 || p.Transport.State != players.Playing || plays != 1 {
		t.Fatal("Enter did not activate the browsed track exactly once")
	}
	p.Transport.State = players.Paused
	if !p.List().AccessibleAction(widget.ItemID(list, 3), a11y.ActionDefault) || p.Transport.State != players.Playing || plays != 2 {
		t.Fatal("accessible repeated activation did not resume the selected track once")
	}
	click(p.List(), platform.ButtonLeft, at)
	click(p.List(), platform.ButtonLeft, at)
	a.PumpOnce()
	if p.Transport.List.Index() != 2 || plays != 3 {
		t.Fatal("double-click did not activate its track exactly once")
	}
}

func TestPlaylistRejectsGutterInterruptedClicksAndDisabledInput(t *testing.T) {
	a, p := open(t, 1.75)
	list, plays := p.listPane.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	at := list.AccessibleItems()[2].Bounds.Center()
	click(p.List(), platform.ButtonLeft, at)
	a.PumpOnce()
	click(p.List(), platform.ButtonRight, list.AccessibleItems()[1].Bounds.Center())
	a.PumpOnce()
	if p.List().Popup() == nil || list.Selected != 2 || p.Transport.List.Index() != 0 || plays != 0 || p.Transport.Pos != 17*time.Second {
		t.Fatal("context press selected or played its row instead of keeping the browsed selection")
	}
	key(a, p.List(), platform.KeyEscape, 0)
	if p.List().Popup() != nil || a.Quitting() {
		t.Fatal("dismissing a playlist menu exited the player")
	}
	click(p.List(), platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 0 {
		t.Fatal("a context interruption combined separate clicks into playback")
	}
	// Hit the authored padding inside the list, at a valid row Y: stock
	// RowAt's vertical-only check must not select this horizontal gutter.
	g := list.geometry(list.Look())
	local := at.Sub(widget.DeviceOrigin(list))
	local.X = g.Rows.Min.X / 2
	click(p.List(), platform.ButtonLeft, local.Add(widget.DeviceOrigin(list)))
	a.PumpOnce()
	click(p.List(), platform.ButtonLeft, at)
	a.PumpOnce()
	if list.Selected != 2 || plays != 0 {
		t.Fatal("horizontal gutter selected a row or preserved double-click history")
	}
	p.List().Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.listPane.buttons["open"]).Center()})
	click(p.List(), platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 0 {
		t.Fatal("leaving the row combined interrupted clicks into playback")
	}
	list.SetEnabled(false)
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		click(p.List(), button, at)
		a.PumpOnce()
	}
	key(a, p.List(), platform.KeyDown, 0)
	key(a, p.List(), platform.KeyReturn, 0)
	if p.List().AccessibleAction(widget.ItemID(list, 2), a11y.ActionDefault) || list.Selected != 2 || plays != 0 || p.List().Popup() != nil {
		t.Fatal("disabled playlist selected, activated or opened a context menu")
	}
}

func TestPlaylistAndItsControlsAcceptOnlyLocalCopyDrops(t *testing.T) {
	a, p := open(t, 1.75)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	dir := t.TempDir()
	file, folder := filepath.Join(dir, "track one.mp3"), filepath.Join(dir, "music folder")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	uri := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	data := []byte("# local file and folder\r\n" + uri(file) + "\r\n" + uri(folder) + "\r\n")
	want := []string{file, folder}
	calls := 0
	var got []string
	p.Host.DropFiles = func(paths []string) { calls++; got = paths }
	off := p.List().Surface().(*platform.Offscreen)
	for _, at := range []paint2d.Point{
		p.listPane.list.AccessibleItems()[0].Bounds.Center(),
		widget.DeviceBounds(p.listPane.buttons["open"]).Center(),
	} {
		before := calls
		off.SimulateDropAction(at, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 || !reflect.DeepEqual(got, want) {
			t.Fatal("playlist row/control did not deliver decoded local paths once as Copy")
		}
		if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || p.Transport.List.Index() != 0 {
			t.Fatal("drop changed playback before the shared loader committed")
		}
		for _, payload := range []struct {
			data   []byte
			action platform.DragAction
		}{
			{[]byte("file://remote-host/tmp/music.mp3\r\n"), platform.DragCopy},
			{append(append([]byte{}, data...), []byte("https://example.com/music.mp3\r\n")...), platform.DragCopy},
			{data, platform.DragMove},
		} {
			before := calls
			off.SimulateDropAction(at, map[string][]byte{"text/uri-list": payload.data}, payload.action, payload.action)
			a.PumpOnce()
			if taken := off.DropTaken(); taken == nil || *taken || calls != before {
				t.Fatal("remote/mixed or Move-only drop reached the shared loader")
			}
		}
	}
	p.Transport.List = players.NewPlaylist()
	p.Refresh()
	a.PumpOnce()
	before := calls
	off.SimulateDropAction(widget.DeviceBounds(p.listPane.list).Center(), map[string][]byte{"text/uri-list": data}, platform.DragCopy, platform.DragCopy)
	a.PumpOnce()
	if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 || !reflect.DeepEqual(got, want) {
		t.Fatal("empty playlist refused a local file/folder drop")
	}
	if content, err := os.ReadFile(file); err != nil || string(content) != "fixture" {
		t.Fatal("Copy drop changed its source file")
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		t.Fatal("Copy drop moved its source folder")
	}
}
