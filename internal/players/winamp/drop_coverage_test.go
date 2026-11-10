package winamp

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestPlaylistFolderDropCoverageAtFractionalScale(t *testing.T) {
	for _, skin := range Skins {
		for _, size := range [][2]int{{StripW, ListH}, {510, 410}} {
			t.Run(fmt.Sprintf("%s/%dx%d", skin, size[0], size[1]), func(t *testing.T) {
				const scale = 1.75
				a, p := open(t, skin, scale)
				p.Transport.List = players.NewPlaylist(players.Track{Title: "Silent drop fixture"})
				p.Refresh()
				p.List.SetSize(size[0], size[1])
				p.List.RequestLayout()
				a.PumpOnce()
				var paths []string
				calls := 0
				p.Host.DropFiles = func(got []string) { paths = got; calls++ }
				l := p.listPane
				origin := widget.DeviceOrigin(l.list)
				rows, scroll := l.list.geo.rows, l.list.geo.bar
				body := widget.DeviceBounds(l)
				targets := []struct {
					name string
					at   paint2d.Point
				}{
					{"first row", origin.Add(rows.Min).Add(paint2d.Pt(20*scale, 3*scale))},
					{"empty row area", origin.Add(rows.Center())},
					{"scrollbar", origin.Add(scroll.Center())},
					{"scroll arrow", origin.Add(paint2d.Pt(scroll.Center().X, scroll.Max.Y+4*scale))},
					{"body chrome", body.Min.Add(paint2d.Pt(0.5*scale, 0.5*scale))},
				}
				for _, control := range []struct {
					name string
					c    widget.Component
				}{{"Add", l.add}, {"Remove", l.rem}, {"Select", l.sel}, {"Skins", l.misc},
					{"footer info", l.foot}, {"list options", l.opts}} {
					targets = append(targets, struct {
						name string
						at   paint2d.Point
					}{control.name, widget.DeviceBounds(control.c).Center()})
				}
				for i, control := range l.mini {
					targets = append(targets, struct {
						name string
						at   paint2d.Point
					}{fmt.Sprintf("mini transport %d", i), widget.DeviceBounds(control).Center()})
				}
				off := p.List.Surface().(*platform.Offscreen)
				for _, target := range targets {
					before := calls
					off.SimulateDropAction(target.at, map[string][]byte{
						"text/uri-list": []byte("file:///home/user/Music/net\r\n"),
						"text/plain":    []byte("/home/user/Music/net"),
					}, platform.DragCopy|platform.DragMove, platform.DragMove)
					a.PumpOnce()
					if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 ||
						!reflect.DeepEqual(paths, []string{"/home/user/Music/net"}) {
						t.Fatalf("folder drop at %s %v failed: paths=%q calls=%d taken=%v", target.name, target.at, paths, calls, taken)
					}
					t.Logf("accepted folder at %s device=(%.1f,%.1f)", target.name, target.at.X, target.at.Y)
				}
				at := widget.DeviceBounds(p.List.Caption()).Center()
				before := calls
				off.SimulateDrop(at, map[string][]byte{"text/uri-list": []byte("file:///home/user/Music/net\r\n")})
				a.PumpOnce()
				if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 {
					t.Fatalf("title band did not accept the same folder drop: taken=%v calls=%d", taken, calls)
				}
			})
		}
	}
}
