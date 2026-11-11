package tags

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/codemodify/media-player-music/internal/tags/tagstest"
)

// frames stands for an MP3's audio: tags are read without decoding it.
var frames = bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0x64}, 256)

func write(t *testing.T, name string, parts ...[]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, bytes.Join(parts, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadsEveryID3Version(t *testing.T) {
	red := color.NRGBA{R: 200, A: 255}
	for _, major := range []int{2, 3, 4} {
		ids := map[string]string{"title": "TIT2", "artist": "TPE1", "album": "TALB", "picture": "APIC"}
		if major == 2 {
			ids = map[string]string{"title": "TT2", "artist": "TP1", "album": "TAL", "picture": "PIC"}
		}
		tag := tagstest.V2(major,
			tagstest.Frame{ID: ids["title"], Data: tagstest.Text(major, "Harbour Lights")},
			tagstest.Frame{ID: ids["artist"], Data: tagstest.Text(major, "Tessellators")},
			tagstest.Frame{ID: ids["album"], Data: tagstest.Text(major, "Low Tide")},
			tagstest.Frame{ID: ids["picture"], Data: tagstest.Picture(major, "image/png", tagstest.PNG(4, 2, red))})
		path := write(t, "song.mp3", tag, frames)
		info, ok := Read(path)
		if !ok || info != (Info{Title: "Harbour Lights", Artist: "Tessellators", Album: "Low Tide"}) {
			t.Fatalf("ID3v2.%d: %+v, %v", major, info, ok)
		}
		cover := Cover(path, 0)
		if cover == nil || cover.Width != 4 || cover.Height != 2 {
			t.Fatalf("ID3v2.%d: the cover was not read: %+v", major, cover)
		}
		if c := cover.NRGBAAt(1, 1); c.R != 200 || c.G != 0 || c.A != 255 {
			t.Fatalf("ID3v2.%d: the cover's colour: %v", major, c)
		}
	}
}

func TestReadsID3v1AndJoinsItBehindID3v2(t *testing.T) {
	path := write(t, "old.mp3", frames, tagstest.V1("Marram", "Ostro", "Dune Slack"))
	if info, ok := Read(path); !ok || info != (Info{Title: "Marram", Artist: "Ostro", Album: "Dune Slack"}) {
		t.Fatalf("ID3v1: %+v, %v", info, ok)
	}
	// ID3v2's frames win; ID3v1 fills what ID3v2 does not say.
	path = write(t, "both.mp3", tagstest.V2(3, tagstest.Frame{ID: "TIT2", Data: tagstest.Text(3, "Beacon — live")}), frames, tagstest.V1("Beacon", "Ruth Alder", "Field Notes"))
	if info, ok := Read(path); !ok || info != (Info{Title: "Beacon — live", Artist: "Ruth Alder", Album: "Field Notes"}) {
		t.Fatalf("ID3v2 and v1: %+v, %v", info, ok)
	}
	if Cover(path, 0) != nil {
		t.Fatal("a cover from a tag without one")
	}
}

func TestCoverIsJPEGOrPNGAndKeptSmall(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 600, 300))
	for i := 0; i < len(picture.Pix); i += 4 {
		picture.Pix[i+1], picture.Pix[i+3] = 180, 255
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, picture, nil); err != nil {
		t.Fatal(err)
	}
	// ID3v2.4 with a frame past 127 bytes, and one after it (goframework R94, read by codecs-container-id3 itself).
	path := write(t, "big.mp3", tagstest.V2(4,
		tagstest.Frame{ID: "APIC", Data: tagstest.Picture(4, "image/jpeg", data.Bytes())},
		tagstest.Frame{ID: "TALB", Data: tagstest.Text(4, "Field Notes")}), frames, tagstest.V1("", "Ruth Alder", ""))
	if info, ok := Read(path); !ok || info != (Info{Artist: "Ruth Alder", Album: "Field Notes"}) {
		t.Fatalf("the frame after the cover: %+v, %v", info, ok)
	}
	cover := Cover(path, 256)
	if cover == nil || cover.Width != 256 || cover.Height != 128 {
		t.Fatalf("a 600 × 300 cover kept at 256: %+v", cover)
	}
	if c := cover.NRGBAAt(100, 60); c.G < 160 || c.R > 30 {
		t.Fatalf("the JPEG cover's colour: %v", c)
	}
	// A picture that is no image is no cover.
	path = write(t, "junk.mp3", tagstest.V2(4, tagstest.Frame{ID: "APIC", Data: tagstest.Picture(4, "image/png", []byte("not a picture"))}), frames)
	if Cover(path, 0) != nil {
		t.Fatal("junk decoded as a cover")
	}
}

func TestOnlyID3FormatsAreRead(t *testing.T) {
	tag := tagstest.V2(4, tagstest.Frame{ID: "TIT2", Data: tagstest.Text(4, "Nine Volt")})
	if info, ok := Read(write(t, "song.aac", tag, frames)); !ok || info.Title != "Nine Volt" {
		t.Fatalf(".aac: %+v, %v", info, ok)
	}
	if _, ok := Read(write(t, "song.flac", tag, frames)); ok {
		t.Fatal("a FLAC file's tags are its own, not ID3")
	}
	if _, ok := Read(write(t, "plain.mp3", frames)); ok {
		t.Fatal("a file without tags gave some")
	}
	if _, ok := Read(filepath.Join(t.TempDir(), "missing.mp3")); ok {
		t.Fatal("a missing file gave tags")
	}
}
