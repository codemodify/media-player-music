// Package cover decodes a music file's front cover for the now-playing
// display: the picture its tags hold (internal/tags), JPEG or PNG, kept
// small. It is the desktop UI's: the daemon draws nothing.
package cover

import (
	"bytes"
	"image"
	_ "image/jpeg" // covers are JPEG or PNG
	_ "image/png"

	"github.com/codemodify/media-player-music/internal/tags"
	"goframework.io/graphics-paint2d"
)

const maxCoverPixels = 4096 * 4096

// Of returns a file's front cover (else its first picture), decoded,
// and the largest side it is kept at: a cover larger than that is scaled
// down. nil when there is none or it cannot be decoded.
func Of(path string, largest int) *paint2d.Image {
	data, ok := tags.Picture(path)
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
