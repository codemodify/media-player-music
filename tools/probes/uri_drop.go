//go:build ignore

// Public local/remote URI-list decoder check; no app, audio, or native window.
// tools/testenv.sh go run tools/probes/uri_drop.go
package main

import (
	"fmt"
	"os"
	"strings"

	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func main() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	data := strings.Join([]string{
		"# comment",
		"file:///tmp/local%20song.mp3",
		"file://localhost/tmp/local%20two.mp3",
		"file://" + host + "/tmp/local%20three.mp3",
		"file://remote.invalid/tmp/remote%20song.mp3",
		"https://example.invalid/music.mp3",
		"",
	}, "\r\n")
	e := widget.NewDropEvent(paint2d.Point{}, "text/uri-list", []byte(data))
	wantPaths := []string{"/tmp/local song.mp3", "/tmp/local two.mp3", "/tmp/local three.mp3"}
	wantRemote := []string{"file://remote.invalid/tmp/remote%20song.mp3", "https://example.invalid/music.mp3"}
	if fmt.Sprint(e.Paths) != fmt.Sprint(wantPaths) || fmt.Sprint(e.Remote) != fmt.Sprint(wantRemote) {
		panic(fmt.Sprintf("unexpected decoder result Paths=%q Remote=%q", e.Paths, e.Remote))
	}
	fmt.Printf("PASS local paths=%q remote URIs=%q\n", e.Paths, e.Remote)
}
