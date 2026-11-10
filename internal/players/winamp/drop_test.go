package winamp

import (
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestPlaylistFileDropsReachTheHostWithoutChangingPlayback(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		for _, empty := range []bool{false, true} {
			t.Run(skin+map[bool]string{true: "/empty", false: "/playing"}[empty], func(t *testing.T) {
				a, p := open(t, skin, 1.5)
				if empty {
					p.Transport.List = players.NewPlaylist()
				} else {
					p.Transport.List.Select(2)
					p.Transport.State = players.Paused
					p.Transport.Pos = 5 * time.Second
				}
				p.Refresh()
				a.PumpOnce()
				var got []string
				calls := 0
				p.Host.DropFiles = func(paths []string) { calls++; got = paths }
				list, state, position, index := p.Transport.List, p.Transport.State, p.Transport.Pos, p.Transport.List.Index()
				off := p.List.Surface().(*platform.Offscreen)
				for _, at := range []paint2d.Point{
					widget.DeviceBounds(p.listPane.list).Center(),
					widget.DeviceBounds(p.listPane.add).Center(),
					widget.DeviceBounds(p.listPane.foot).Center(),
				} {
					before := calls
					off.SimulateDropAction(at, map[string][]byte{
						"text/uri-list": []byte("# audio and folder\r\nfile:///tmp/track%20one.wav\r\nfile:///tmp/music%20folder\r\n"),
					}, platform.DragCopy|platform.DragMove, platform.DragCopy)
					a.PumpOnce()
					if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 ||
						!reflect.DeepEqual(got, []string{"/tmp/track one.wav", "/tmp/music folder"}) {
						t.Fatalf("drop at %v did not reach the append hook exactly once: paths=%q calls=%d taken=%v", at, got, calls, taken)
					}
					if p.Transport.List != list || p.Transport.State != state || p.Transport.Pos != position || p.Transport.List.Index() != index {
						t.Fatal("accepting a file drop changed playback before the host loaded files")
					}
				}
				for _, data := range []map[string][]byte{
					{"text/plain": []byte("/tmp/music.wav")},
					{"text/uri-list": []byte("file://remote-server/tmp/music.wav\r\n")},
					{"text/uri-list": []byte("https://example.com/music.wav\r\n")},
				} {
					before := calls
					off.SimulateDrop(widget.DeviceBounds(p.listPane.list).Center(), data)
					a.PumpOnce()
					if taken := off.DropTaken(); taken == nil || *taken || calls != before {
						t.Fatal("a non-local audio drop was accepted")
					}
				}
				if action := p.listPane.list.DropActionFor(platform.DragMove); action&platform.DragMove != 0 {
					t.Fatal("the playlist offered to move source files")
				}
				p.Host.DropFiles = nil
				off.SimulateDrop(widget.DeviceBounds(p.listPane.add).Center(), map[string][]byte{
					"text/uri-list": []byte("file:///tmp/music.wav\r\n"),
				})
				a.PumpOnce()
				if taken := off.DropTaken(); taken == nil || *taken {
					t.Fatal("the playlist claimed a drop with no loader")
				}
			})
		}
	}
}
