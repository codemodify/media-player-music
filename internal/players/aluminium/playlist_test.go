package aluminium

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func libraryFixture(p *Player) []players.Track {
	tracks := []players.Track{
		{Title: "Echo", Artist: "Eli", Album: "Blue", Length: 50 * time.Second},
		{Title: "Bravo", Artist: "Ada", Album: "Copper", Length: 20 * time.Second},
		{Title: "Delta", Artist: "Dara", Album: "Gold", Length: 40 * time.Second},
		{Title: "Charlie", Artist: "Cora", Album: "COPPER", Length: 30 * time.Second},
		{Title: "Alpha", Artist: "Bo", Album: "Blue"},
	}
	p.Transport.List = players.NewPlaylist(tracks...)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	p.Refresh()
	return append([]players.Track(nil), tracks...)
}

func TestPlaylistSearchMapsVisibleRowsToTheQueueAndActivatesExactlyOnce(t *testing.T) {
	a, p := open(t, 1.75)
	before := libraryFixture(p)
	a.PumpOnce()
	list, plays := p.body.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	search := func(text string) {
		p.Main.RequestFocus(p.body.search)
		key(a, p.Main, platform.KeyA, platform.ModCtrl)
		key(a, p.Main, platform.KeyBackspace, 0)
		for _, r := range text {
			p.Main.Inject(platform.Event{Kind: platform.EventText, Rune: r})
		}
		a.PumpOnce()
	}
	search("cOpPeR")
	if list.Count != 2 || !reflect.DeepEqual(list.indices, []int{1, 3}) || list.Selected != -1 || plays != 0 || p.Transport.List.Index() != 0 || p.Transport.Pos != 17*time.Second || !reflect.DeepEqual(p.Transport.List.Tracks, before) {
		t.Fatal("typing an album filter changed playback/queue or lost visible index mapping")
	}
	at := list.AccessibleItems()[1].Bounds.Center()
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	p.Refresh()
	a.PumpOnce()
	if list.Selected != 1 || p.Transport.List.Index() != 0 || plays != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
		t.Fatal("filtered single-click browsing played a track or refresh reset the row")
	}
	key(a, p.Main, platform.KeyReturn, 0)
	if p.Transport.List.Index() != 3 || p.Transport.State != players.Playing || plays != 1 {
		t.Fatal("Enter activated the visible index rather than its queue track, or played twice")
	}
	p.Transport.State = players.Paused
	if !p.Main.AccessibleAction(widget.ItemID(list, 1), a11y.ActionDefault) || p.Transport.List.Index() != 3 || p.Transport.State != players.Playing || plays != 2 {
		t.Fatal("accessible repeated activation did not resume the mapped paused track once")
	}
	click(p.Main, platform.ButtonLeft, list.AccessibleItems()[0].Bounds.Center())
	click(p.Main, platform.ButtonLeft, list.AccessibleItems()[0].Bounds.Center())
	a.PumpOnce()
	if p.Transport.List.Index() != 1 || plays != 3 {
		t.Fatal("filtered double-click did not activate its queue track exactly once")
	}
	search("no matching song")
	if list.Count != 0 || len(list.indices) != 0 || list.Selected != -1 || p.Main.AccessibleAction(widget.ItemID(list, 0), a11y.ActionDefault) {
		t.Fatal("an unmatched query showed all tracks or advertised an activatable row")
	}
	list.Activate(0)
	if plays != 3 || p.Transport.List.Index() != 1 {
		t.Fatal("empty filtered view activated a hidden queue track")
	}
	search("")
	if list.Count != len(before) || !reflect.DeepEqual(list.indices, []int{0, 1, 2, 3, 4}) || !reflect.DeepEqual(p.Transport.List.Tracks, before) {
		t.Fatal("clearing search lost the complete queue or changed its order")
	}
}

func TestPlaylistSortAndMetadataRefreshKeepTrackIdentityAndBrowseSelection(t *testing.T) {
	a, p := open(t, 1.75)
	before := libraryFixture(p)
	a.PumpOnce()
	list := p.body.list
	list.choose(3)
	click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.headers[0]).Center())
	a.PumpOnce()
	if !reflect.DeepEqual(list.indices, []int{4, 1, 3, 2, 0}) || list.Selected != 2 || !reflect.DeepEqual(p.Transport.List.Tracks, before) || p.Transport.List.Index() != 0 || p.Transport.Pos != 17*time.Second {
		t.Fatal("sorting names changed queue order, playback or selected track identity")
	}
	click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.headers[1]).Center())
	a.PumpOnce()
	if !reflect.DeepEqual(list.indices, []int{4, 1, 3, 2, 0}) || list.Selected != 2 {
		t.Fatal("time sorting did not use duration or retain the selected track")
	}
	click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.headers[1]).Center())
	a.PumpOnce()
	if !reflect.DeepEqual(list.indices, []int{0, 2, 3, 1, 4}) || list.Selected != 2 {
		t.Fatal("descending time order lost its queue mapping")
	}
	p.Transport.List.Tracks[3].Length = 75 * time.Second
	p.Refresh()
	a.PumpOnce()
	if !reflect.DeepEqual(list.indices, []int{3, 0, 2, 1, 4}) || list.Selected != 0 || p.Transport.List.Index() != 0 || p.Transport.Pos != 17*time.Second {
		t.Fatal("live metadata resort reset a browsed selection or moved playback")
	}
	list.applyFilter("Copper")
	p.Transport.List.Tracks[1].Album = "Other"
	p.Refresh()
	a.PumpOnce()
	if list.Count != 1 || !reflect.DeepEqual(list.indices, []int{3}) || list.track(0).Title != "Charlie" {
		t.Fatal("decoder metadata did not rebuild the filtered table")
	}
}

func TestPlaylistContextGutterAndDisabledInputDoNotActivateTracks(t *testing.T) {
	a, p := open(t, 1.75)
	libraryFixture(p)
	a.PumpOnce()
	list, plays := p.body.list, 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	at := list.AccessibleItems()[2].Bounds.Center()
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	click(p.Main, platform.ButtonRight, list.AccessibleItems()[1].Bounds.Center())
	a.PumpOnce()
	if p.Main.Popup() == nil || list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.Pos != 17*time.Second || plays != 0 {
		t.Fatal("context press selected or played its row rather than preserving selection")
	}
	key(a, p.Main, platform.KeyEscape, 0)
	if p.Main.Popup() != nil || a.Quitting() {
		t.Fatal("dismissing the library menu exited the player")
	}
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 0 {
		t.Fatal("context interruption combined separate clicks into playback")
	}
	g := list.geometry(list.Look())
	if g == nil || g.Rows.Min.X <= 0 {
		t.Fatal("library fixture lacks the authored row padding needed for a gutter hit")
	}
	local := at.Sub(widget.DeviceOrigin(list))
	local.X = g.Rows.Min.X / 2
	click(p.Main, platform.ButtonLeft, local.Add(widget.DeviceOrigin(list)))
	a.PumpOnce()
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if list.Selected != 2 || plays != 0 {
		t.Fatal("horizontal gutter selected a row or preserved an interrupted double-click")
	}
	p.Main.Inject(platform.Event{Kind: platform.EventPointerLeave})
	click(p.Main, platform.ButtonLeft, at)
	a.PumpOnce()
	if plays != 0 {
		t.Fatal("pointer exit combined interrupted clicks into playback")
	}
	list.SetEnabled(false)
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		click(p.Main, button, at)
		a.PumpOnce()
	}
	key(a, p.Main, platform.KeyDown, 0)
	key(a, p.Main, platform.KeyReturn, 0)
	if p.Main.AccessibleAction(widget.ItemID(list, 2), a11y.ActionDefault) || list.Selected != 2 || plays != 0 || p.Main.Popup() != nil {
		t.Fatal("disabled library changed selection, opened a menu or activated playback")
	}
}

func TestPlaylistResizeAndLocalCopyDropsRemainUsableWithAnEmptyFilter(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			libraryFixture(p)
			var small paint2d.Rect
			for i, size := range [][2]int{{640, 400}, {940, 650}} {
				p.Main.SetSize(size[0], size[1])
				a.PumpOnce()
				g := p.body.list.geometry(p.body.list.Look())
				if g == nil || g.Rows.Empty() || g.Bar.Empty() || g.Rows.Max.X > g.Bar.Min.X || !p.body.list.LocalBounds().Contains(g.Rows.Center()) {
					t.Fatal("resized library lost rows or overlapped its scrollbar")
				}
				if i == 0 {
					small = g.Rows
				} else if g.Rows.Dx() <= small.Dx() || g.Rows.Dy() <= small.Dy() {
					t.Fatal("enlarging the full player did not enlarge its row viewport")
				}
			}
			path := filepath.Join(t.TempDir(), "track one.mp3")
			if err := os.WriteFile(path, []byte("unchanged source"), 0600); err != nil {
				t.Fatal(err)
			}
			payload := []byte((&url.URL{Scheme: "file", Path: path}).String() + "\r\n")
			calls := 0
			p.Host.DropFiles = func(paths []string) {
				calls++
				if !reflect.DeepEqual(paths, []string{path}) {
					t.Errorf("drop decoded paths=%v", paths)
				}
			}
			off := p.Main.Surface().(*platform.Offscreen)
			drop := func(at paint2d.Point, data []byte, actions platform.DragAction) bool {
				off.SimulateDropAction(at, map[string][]byte{"text/uri-list": data}, actions, actions)
				a.PumpOnce()
				taken := off.DropTaken()
				return taken != nil && *taken
			}
			for _, at := range []paint2d.Point{p.body.list.AccessibleItems()[0].Bounds.Center(), widget.DeviceBounds(p.body.search).Center()} {
				before := calls
				if !drop(at, payload, platform.DragCopy) || calls != before+1 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || p.Transport.List.Index() != 0 {
					t.Fatal("library row/header did not append a local Copy drop without changing playback")
				}
				if drop(at, []byte("file://remote-host/tmp/music.mp3\r\n"), platform.DragCopy) || drop(at, payload, platform.DragMove) || calls != before+1 {
					t.Fatal("remote or Move-only drop reached the loader")
				}
			}
			p.body.list.applyFilter("no matching song")
			a.PumpOnce()
			if p.body.list.Count != 0 || !drop(widget.DeviceBounds(p.body.list).Center(), payload, platform.DragCopy) || calls != 3 {
				t.Fatal("empty filtered library rejected a local file drop")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "unchanged source" {
				t.Fatal("Copy drop changed the source file")
			}
		})
	}
}
