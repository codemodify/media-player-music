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
	"encoding/binary"
	"image"
	_ "image/jpeg" // covers are JPEG or PNG
	_ "image/png"
	"os"
	"path/filepath"
	"slices"
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
	if tag.Version == 4 {
		// R94: codecs-container-id3 reads an ID3v2.4 frame of 128 bytes
		// or more by a plain size, so a cover (and every frame after it)
		// is lost. Read the tag again with its frames' sizes written as
		// plain integers, which it reads right; ID3v1's frames as Read
		// joins them.
		if fixed := readV24(file); fixed != nil {
			for _, frame := range tag.Frames {
				if !slices.ContainsFunc(fixed.Frames, func(f id3.Frame) bool { return f.ID == frame.ID }) {
					fixed.Frames = append(fixed.Frames, frame)
				}
			}
			tag = fixed
		}
	}
	return tag, true
}

// readV24 reads the ID3v2.4 tag at a file's start, its frames' syncsafe
// sizes made plain first (R94).
func readV24(file *os.File) *id3.Tag {
	head := make([]byte, 10)
	if _, err := file.ReadAt(head, 0); err != nil {
		return nil
	}
	size := id3.Size(head)
	if size == 0 || head[3] != 4 || head[5]&0xc0 != 0 || size > 64<<20 {
		return nil // no 2.4 tag, or one unsynchronised or with an extended header: left as Read reads it
	}
	block := make([]byte, size)
	if _, err := file.ReadAt(block, 0); err != nil {
		return nil
	}
	for at := 10; at+10 <= len(block) && block[at] != 0; {
		b := block[at+4 : at+8]
		if (b[0]|b[1]|b[2]|b[3])&0x80 != 0 {
			return nil
		}
		frame := int(b[0])<<21 | int(b[1])<<14 | int(b[2])<<7 | int(b[3])
		binary.BigEndian.PutUint32(b, uint32(frame))
		at += 10 + frame
	}
	tag, err := id3.Parse(block)
	if err != nil {
		return nil
	}
	return tag
}
