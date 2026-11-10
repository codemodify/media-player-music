package standard

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestNativePlaylistDropsReachTheApplicationAsCopies(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist()
			p.Refresh()
			a.PumpOnce()
			var received [][]string
			p.Host.DropFiles = func(paths []string) { received = append(received, paths) }
			off := p.Main.Surface().(*platform.Offscreen)
			q := p.body.queue
			heading := widget.DeviceOrigin(q)
			heading.X += q.Bounds().Dx() / 2
			heading.Y += 10 * scale
			for _, point := range []paint2d.Point{
				widget.DeviceBounds(q.list).Center(), heading, widget.DeviceBounds(q.add).Center(),
			} {
				off.SimulateDropAction(point, map[string][]byte{
					"text/uri-list": []byte("# local music\r\nfile:///tmp/song%20name.mp3\r\nfile://localhost/tmp/music-folder\r\n"),
					"text/plain":    []byte("must choose URI data"),
				}, platform.DragCopy|platform.DragMove, platform.DragMove)
				a.PumpOnce()
				if taken := off.DropTaken(); taken == nil || !*taken {
					t.Fatalf("playlist drop at %v was refused: %v", point, taken)
				}
				if mime, action := off.DragAccepted(); mime != "text/uri-list" || action != platform.DragCopy {
					t.Fatalf("enqueue drop negotiated %q/%v instead of URI copy", mime, action)
				}
			}
			want := []string{"/tmp/song name.mp3", "/tmp/music-folder"}
			if len(received) != 3 {
				t.Fatalf("application received %d drops, want 3", len(received))
			}
			for _, paths := range received {
				if !reflect.DeepEqual(paths, want) {
					t.Fatalf("application got %v, want decoded files and folders %v", paths, want)
				}
			}
			if p.Transport.List.Len() != 0 || p.Transport.State != players.Stopped {
				t.Fatal("presentation changed the queue or playback before the application loaded the drop")
			}
		})
	}
}

func TestNativePlaylistDropsRefuseUnsupportedOffersAndRemoteFiles(t *testing.T) {
	a, p := open(t, 1)
	var calls int
	p.Host.DropFiles = func([]string) { calls++ }
	off := p.Main.Surface().(*platform.Offscreen)
	point := widget.DeviceBounds(p.body.queue.list).Center()
	for _, offer := range []struct {
		name   string
		mime   string
		data   string
		action platform.DragAction
	}{
		{"text", "text/plain", "/tmp/music.mp3", platform.DragCopy},
		{"move", "text/uri-list", "file:///tmp/music.mp3", platform.DragMove},
		{"remote authority", "text/uri-list", "file://remote.example/tmp/music.mp3", platform.DragCopy},
		{"no local paths", "text/uri-list", "https://example.com/music.mp3", platform.DragCopy},
	} {
		t.Run(offer.name, func(t *testing.T) {
			off.SimulateDropAction(point, map[string][]byte{offer.mime: []byte(offer.data)}, offer.action, offer.action)
			a.PumpOnce()
			if taken := off.DropTaken(); taken == nil || *taken {
				t.Fatalf("unsupported drop was accepted: %v", taken)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("unsupported drops reached the application %d times", calls)
	}
	p.Host.DropFiles = nil
	off.SimulateDrop(point, map[string][]byte{"text/uri-list": []byte("file:///tmp/music.mp3")})
	a.PumpOnce()
	if taken := off.DropTaken(); taken == nil || *taken {
		t.Fatal("a presentation without a loader accepted a file drop")
	}
}

func TestFileDropOwnsItsPathsForDeferredLoading(t *testing.T) {
	var received []string
	h := &players.Host{DropFiles: func(paths []string) { received = paths }}
	e := widget.NewDropEvent(paint2d.Point{}, "text/uri-list", []byte("file:///tmp/song.mp3"))
	e.Action = platform.DragCopy
	if !h.AcceptFileDrop(e) {
		t.Fatal("local URI copy was rejected")
	}
	e.Paths[0] = "/tmp/changed.mp3"
	if !reflect.DeepEqual(received, []string{"/tmp/song.mp3"}) {
		t.Fatalf("deferred loader shares the event's mutable paths: %v", received)
	}
}
