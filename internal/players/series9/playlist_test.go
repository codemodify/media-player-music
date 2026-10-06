package series9

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func TestPlaylistBrowsingActivationAndMetadataStayWithTheSharedQueue(t *testing.T) {
	a, p := open(t, 1.75)
	list, plays := p.body.playlist.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	at := list.AccessibleItems()[2].Bounds.Center()
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	p.Refresh()
	a.PumpOnce()
	if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || plays != 0 {
		t.Fatal("single-click browsing changed playback or refresh reset selection")
	}
	key(a, p.Main, platform.KeyDown, 0)
	if list.Selected != 3 || plays != 0 {
		t.Fatal("arrow navigation started playback")
	}
	key(a, p.Main, platform.KeyReturn, 0)
	if p.Transport.List.Index() != 3 || p.Transport.State != players.Playing || plays != 1 {
		t.Fatal("Enter did not activate the browsed track exactly once")
	}
	p.Transport.State = players.Paused
	if !p.Main.AccessibleAction(widget.ItemID(list, 3), a11y.ActionDefault) || p.Transport.State != players.Playing || plays != 2 {
		t.Fatal("repeated accessible activation did not resume the selected track once")
	}
	click(p.Main, platform.ButtonLeft, at)
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if p.Transport.List.Index() != 2 || plays != 3 {
		t.Fatal("double-click did not activate its track exactly once")
	}
	p.Transport.List.Tracks[2].Title = "Updated title"
	p.Transport.List.Tracks[2].Artist = "Updated artist"
	p.Transport.List.Tracks[2].Length = 143 * time.Second
	p.Refresh()
	a.PumpOnce()
	if list.ItemText(2) != "Updated artist - Updated title" || list.ItemDetail(2) != "2:23" || list.Selected != 2 || plays != 3 {
		t.Fatal("decoder metadata did not refresh the same selected row without playback")
	}
}

func TestPlaylistContextScrollbarAndPointerExitInterruptActivation(t *testing.T) {
	a, p := open(t, 1.75)
	list, plays := p.body.playlist.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	at := list.AccessibleItems()[2].Bounds.Center()
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	click(p.Main, platform.ButtonRight, list.AccessibleItems()[1].Bounds.Center())
	a.PumpOnce()
	if p.Main.Popup() == nil || list.Selected != 2 || p.Transport.List.Index() != 0 || plays != 0 || p.Transport.Pos != 17*time.Second {
		t.Fatal("context press changed browsed selection or paused playback")
	}
	key(a, p.Main, platform.KeyEscape, 0)
	if p.Main.Popup() != nil || a.Quitting() {
		t.Fatal("dismissing the playlist menu exited the player")
	}
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 0 {
		t.Fatal("a context interruption combined separate clicks into playback")
	}
	g := list.geometry(list.Look())
	if g == nil || g.Bar.Empty() {
		t.Fatal("playlist fixture has no authored scrollbar")
	}
	click(p.Main, platform.ButtonLeft, g.Bar.Center().Add(widget.DeviceOrigin(list)))
	a.PumpOnce()
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if list.Selected != 2 || plays != 0 {
		t.Fatal("scrollbar press activated a row or preserved double-click history")
	}
	// The guard also handles a public mouse event outside the horizontal
	// row range; toolkit v0.23.4 RowAt considers only the vertical range.
	list.MousePress(widget.MouseEvent{Button: platform.ButtonLeft, Pos: paintengine2d.Pt(g.Rows.Min.X-1, g.Rows.Min.Y+g.RowHeight/2)})
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if list.Selected != 2 || plays != 0 {
		t.Fatal("horizontal gutter selected a row or preserved double-click history")
	}
	p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.body.main.buttons["open"]).Center()})
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 0 {
		t.Fatal("leaving the row combined interrupted clicks into playback")
	}
	list.SetEnabled(false)
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		click(p.Main, button, at)
		a.PumpOnce()
	}
	key(a, p.Main, platform.KeyDown, 0)
	key(a, p.Main, platform.KeyReturn, 0)
	if p.Main.AccessibleAction(widget.ItemID(list, 2), a11y.ActionDefault) || list.Selected != 2 || plays != 0 || p.Main.Popup() != nil {
		t.Fatal("disabled playlist selected, activated or opened a context menu")
	}
}

func TestPlaylistDrawerResizeKeepsRowsAndControlsInsideOneMain(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			main, native := p.Main, p.Main.Surface()
			var initialRows paintengine2d.Rect
			for i, size := range [][2]int{{596, 468}, {740, 574}} {
				main.SetSize(size[0], size[1])
				settle(a, p)
				if w, h := main.Size(); w != size[0] || h != size[1] {
					t.Fatal("full drawer resize was not acknowledged")
				}
				if p.Main != main || p.Main.Surface() != native || len(a.Windows()) != 1 {
					t.Fatal("resizing the embedded playlist replaced the main or created a satellite")
				}
				pane := p.body.playlist
				for _, c := range pane.Children() {
					r, parent := c.Bounds(), pane.LocalBounds()
					if c.Visible() && (r.Empty() || r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > parent.Max.X || r.Max.Y > parent.Max.Y) {
						t.Fatalf("%dx%d playlist clipped %T: %v in %v", size[0], size[1], c, r, parent)
					}
				}
				g := pane.list.geometry(pane.list.Look())
				if g == nil || g.Rows.Empty() || g.Bar.Empty() || g.Rows.Max.X > g.Bar.Min.X {
					t.Fatal("playlist rows overlap their scrollbar or lack authored geometry")
				}
				if i == 0 {
					initialRows = g.Rows
				} else if g.Rows.Dy() <= initialRows.Dy() || g.Rows.Dx() != initialRows.Dx() {
					t.Fatal("full resize did not grow the rows vertically while preserving original drawer width")
				}
			}
		})
	}
}

func TestPlaylistRowsHeaderAndEmptyBackgroundAcceptOnlyLocalCopyDrops(t *testing.T) {
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
	data := []byte(uri(file) + "\r\n" + uri(folder) + "\r\n")
	want := []string{file, folder}
	calls := 0
	var got []string
	p.Host.DropFiles = func(paths []string) { calls++; got = paths }
	off := p.Main.Surface().(*platform.Offscreen)
	pane := p.body.playlist
	header, ok := style.SkinSlotRect(pane.Look(), LayoutPlaylist, "header", pane.LocalBounds())
	if !ok || header.Empty() {
		t.Fatal("playlist has no authored header")
	}
	for _, at := range []paintengine2d.Point{
		pane.list.AccessibleItems()[0].Bounds.Center(),
		header.Center().Add(widget.DeviceOrigin(pane)),
	} {
		before := calls
		off.SimulateDropAction(at, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 || !reflect.DeepEqual(got, want) {
			t.Fatal("playlist row/header did not deliver local paths once as Copy")
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
	off.SimulateDropAction(widget.DeviceBounds(pane.list).Center(), map[string][]byte{"text/uri-list": data}, platform.DragCopy, platform.DragCopy)
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
