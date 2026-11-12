package cover

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
	cover := Of(path, 256)
	if cover == nil || cover.Width != 256 || cover.Height != 128 {
		t.Fatalf("a 600 × 300 cover kept at 256: %+v", cover)
	}
	if c := cover.NRGBAAt(100, 60); c.G < 160 || c.R > 30 {
		t.Fatalf("the JPEG cover's colour: %v", c)
	}
	// A picture that is no image is no cover.
	path = write(t, "junk.mp3", tagstest.V2(4, tagstest.Frame{ID: "APIC", Data: tagstest.Picture(4, "image/png", []byte("not a picture"))}), frames)
	if Of(path, 0) != nil {
		t.Fatal("junk decoded as a cover")
	}
}

func TestPicturesOfEveryID3Version(t *testing.T) {
	red := color.NRGBA{R: 200, A: 255}
	for _, major := range []int{2, 3, 4} {
		id := map[int]string{2: "PIC", 3: "APIC", 4: "APIC"}[major]
		path := write(t, "song.mp3", tagstest.V2(major, tagstest.Frame{ID: id, Data: tagstest.Picture(major, "image/png", tagstest.PNG(4, 2, red))}), frames)
		picture := Of(path, 0)
		if picture == nil || picture.Width != 4 || picture.Height != 2 {
			t.Fatalf("ID3v2.%d: the cover was not read: %+v", major, picture)
		}
		if c := picture.NRGBAAt(1, 1); c.R != 200 || c.G != 0 || c.A != 255 {
			t.Fatalf("ID3v2.%d: the cover's colour: %v", major, c)
		}
	}
	if Tagged("song.flac") || !Tagged("song.AAC") {
		t.Fatal("Tagged")
	}
	if _, ok := Picture(write(t, "plain.mp3", frames)); ok {
		t.Fatal("a cover from a file without a tag")
	}
}
