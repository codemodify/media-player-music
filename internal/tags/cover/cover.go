// Package cover decodes a music file's front cover for the now-playing
// display: the picture its ID3 tag holds (APIC, ID3v2.2's PIC), through
// goframework's codecs-container-id3, JPEG or PNG, kept small. It is the
// desktop UI's: the daemon draws nothing, and reads tags with
// internal/tags.
package cover

import (
	"bytes"
	"image"
	_ "image/jpeg" // covers are JPEG or PNG
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	id3 "goframework.io/codecs-container-id3"
	"goframework.io/graphics-paint2d"
)

const maxCoverPixels = 4096 * 4096

// Of returns a file's front cover (else its first picture), decoded,
// and the largest side it is kept at: a cover larger than that is scaled
// down. nil when there is none or it cannot be decoded.
func Of(path string, largest int) *paint2d.Image {
	data, ok := Picture(path)
	if !ok {
		return nil
	}
	// A tag is the file's to say: a picture past 4096 × 4096 is not
	// decoded, so a crafted one cannot take the memory of a huge image.
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width*config.Height > maxCoverPixels {
		return nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	cover := paint2d.NewImageFromNRGBA(decoded)
	if largest > 0 && (cover.Width > largest || cover.Height > largest) {
		scale := float64(largest) / float64(max(cover.Width, cover.Height))
		scaled := paint2d.NewImage(max(1, int(float64(cover.Width)*scale+.5)), max(1, int(float64(cover.Height)*scale+.5)))
		paint2d.Resample(scaled, cover, paint2d.ResampleCatmullRom)
		cover = scaled
	}
	return cover
}

// Tagged reports whether a file of this name carries ID3 tags.
func Tagged(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".mp2", ".mp1", ".mpga", ".aac":
		return true
	}
	return false
}

// Picture returns a file's front cover (else its first picture) as its
// tag holds it, JPEG or PNG most often: false when there is none.
func Picture(path string) ([]byte, bool) {
	tag, ok := read(path)
	if !ok {
		return nil, false
	}
	picture, ok := tag.Cover()
	if !ok || picture.MIME == "-->" || len(picture.Data) == 0 {
		return nil, false
	}
	return picture.Data, true
}

func read(path string) (*id3.Tag, bool) {
	if !Tagged(path) {
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	tag, err := id3.Read(file)
	if err != nil {
		return nil, false
	}
	return tag, true
}
