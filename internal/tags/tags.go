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
	"os"
	"path/filepath"
	"strings"

	id3 "goframework.io/codecs-container-id3"
)

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
