package cover

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/codemodify/media-player-music/internal/tags"
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
	if info, ok := tags.Read(path); !ok || info != (tags.Info{Artist: "Ruth Alder", Album: "Field Notes"}) {
		t.Fatalf("the frame after the cover: %+v, %v", info, ok)
	}
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
