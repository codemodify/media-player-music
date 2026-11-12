package service

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/codemodify/media-player-music/internal/tags/tagstest"
)

// The library shows an MP3's or an AAC file's ID3 tags as soon as it is
// added; a file without tags keeps its name.
func TestLibraryTracksCarryTheirID3Tags(t *testing.T) {
	root := t.TempDir()
	frames := bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0x64}, 64)
	write := func(name string, parts ...[]byte) {
		if err := os.WriteFile(filepath.Join(root, name), bytes.Join(parts, nil), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("01 a.mp3", tagstest.V2(4,
		tagstest.Frame{ID: "TIT2", Data: tagstest.Text(4, "Harbour Lights")},
		tagstest.Frame{ID: "APIC", Data: tagstest.Picture(4, "image/png", tagstest.PNG(300, 300, color.NRGBA{B: 255, A: 255}))},
		tagstest.Frame{ID: "TPE1", Data: tagstest.Text(4, "The Tessellators")}), frames)
	write("02 b.aac", tagstest.V2(2, tagstest.Frame{ID: "TT2", Data: tagstest.Text(2, "Marram")}), frames, tagstest.V1("", "Ostro", "Dune Slack"))
	write("03 c.mp3", frames)
	result, err := Expand(context.Background(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tracks) != 3 {
		t.Fatalf("%d tracks", len(result.Tracks))
	}
	for i, want := range [][3]string{{"Harbour Lights", "The Tessellators", ""}, {"Marram", "Ostro", "Dune Slack"}, {"03 c", "", ""}} {
		if got := result.Tracks[i]; got.Title != want[0] || got.Artist != want[1] || got.Album != want[2] {
			t.Fatalf("track %d: %q, %q, %q; want %q", i, got.Title, got.Artist, got.Album, want)
		}
	}
}
