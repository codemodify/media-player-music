// Package tags reads what a music file says of itself for the playlist
// (the library) and the now-playing display: ID3 tags (ID3v1 and v1.1,
// ID3v2.2, 2.3 and 2.4) through goframework's codecs-container-id3, and
// their front cover (APIC, ID3v2.2's PIC).
//
// Only the formats whose tags are ID3 are read: MPEG audio files and ADTS
// (.aac) files. Others keep their file name until an engine reports their
// tags as it plays them.
package tags

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

// Info is a file's text for the library.
type Info struct {
	Title, Artist, Album string
}

// Tagged reports whether a file of this name carries ID3 tags.
func Tagged(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".mp2", ".mp1", ".mpga", ".aac":
		return true
	}
	return false
}

// Read reads a file's ID3 tags: ID3v2 at its start and ID3v1 at its end,
// joined. ok is false for a file that is not Tagged or carries no tag.
func Read(path string) (Info, bool) {
	tag, ok := read(path)
	if !ok {
		return Info{}, false
	}
	text := tag.Tags()
	info := Info{Title: text["title"], Artist: text["artist"], Album: text["album"]}
	return info, info != Info{}
}

// Cover returns a file's front cover (else its first picture), decoded,
// and the largest side it is kept at: a cover larger than that is scaled
// down. nil when there is none or it cannot be decoded.
func Cover(path string, largest int) *paint2d.Image {
	tag, ok := read(path)
	if !ok {
		return nil
	}
	picture, ok := tag.Cover()
	if !ok || picture.MIME == "-->" || len(picture.Data) == 0 {
		return nil
	}
	// A tag is the file's to say: a picture past 4096 × 4096 is not
	// decoded, so a crafted one cannot take the memory of a huge image.
	config, _, err := image.DecodeConfig(bytes.NewReader(picture.Data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width*config.Height > maxCoverPixels {
		return nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(picture.Data))
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
