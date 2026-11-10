package players

import (
	"os"
	"reflect"
	"testing"

	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestFileDropUsesToolkitLocalAndRemoteClassification(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, uris string
		want       []string
	}{
		{"local", "# media\r\nfile:///tmp/a%20b.mp3\r\nfile://localhost/tmp/local.wav\r\n", []string{"/tmp/a b.mp3", "/tmp/local.wav"}},
		{"this hostname", "file://" + hostname + "/tmp/local.wav", []string{"/tmp/local.wav"}},
		{"remote only", "file://remote.example/tmp/local.wav", nil},
		{"mixed", "file:///tmp/local.wav\r\nfile://remote.example/tmp/other.wav", nil},
		{"network protocol", "https://example.test/music.wav", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var received []string
			h := &Host{DropFiles: func(paths []string) { calls++; received = paths }}
			e := widget.NewDropEvent(paint2d.Point{}, "text/uri-list", []byte(tc.uris))
			e.Action = platform.DragCopy
			accepted := h.AcceptFileDrop(e)
			wantCalls := 0
			if tc.want != nil {
				wantCalls = 1
			}
			if accepted != (tc.want != nil) || !reflect.DeepEqual(received, tc.want) || calls != wantCalls {
				t.Fatalf("accepted=%t calls=%d local=%q remote=%q delivered=%q", accepted, calls, e.Paths, e.Remote, received)
			}
			if accepted {
				e.Paths[0] = "changed after delivery"
				if !reflect.DeepEqual(received, tc.want) {
					t.Fatal("deferred loader retained mutable event paths")
				}
			}
		})
	}
}
