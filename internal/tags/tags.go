// Package tags reads what a music file says of itself, for the queue and
// the library: its tags, whichever its container holds, and its length,
// through goframework's demuxers (codecs-container-spec's Open): ID3v1 and
// ID3v2 and APE tags of MP3 and ADTS files (codecs-container-id3,
// codecs-container-apetag), FLAC's and Ogg's Vorbis comments, MP4's and
// M4A's iTunes items, Matroska's tags, WAV's and AIFF's, Monkey's Audio's
// APE tags. Nothing is decoded: a file's first bytes and its headers are
// read.
package tags

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	// every container audio-playback plays, each registering itself
	_ "goframework.io/audio-playback/all"
	_ "goframework.io/codecs-audio-ape" // .ape: its APE tag, and its length
	_ "goframework.io/codecs-container-apetag"
	id3 "goframework.io/codecs-container-id3"
	container "goframework.io/codecs-container-spec"
)

// Info is what a file says of itself.
type Info struct {
	Title, Artist, Album string
	// AlbumArtist is the album's artist where the tags say ("Various
	// Artists" for a compilation).
	AlbumArtist string
	Genre       string
	// Year is the date's year; 0 when none.
	Year int
	// Number and Disc are the track's and the disc's numbers; 0 when none.
	Number, Disc int
	// Length is how long it plays; 0 when the headers do not say.
	Length time.Duration
}

// Read reads a file's tags and length. ok is false for a file that is not
// in a format the player reads, or does not open; an MPEG or ADTS file
// whose audio does not open still gives its ID3 tags, without a length.
func Read(path string) (Info, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Info{}, false
	}
	defer file.Close()
	demuxer, err := container.Open(file)
	if err != nil {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return Info{}, false
		}
		tag, err := id3.Read(file)
		if err != nil {
			return Info{}, false
		}
		got := info(container.Stream{Tags: tag.Tags()})
		return got, got != Info{}
	}
	defer demuxer.Close()
	streams := demuxer.Streams()
	for _, stream := range streams {
		if stream.Kind == container.Audio {
			return info(stream), true
		}
	}
	if len(streams) > 0 {
		return info(streams[0]), true
	}
	return Info{}, false
}

func info(stream container.Stream) Info {
	// Every container names its tags alike (Stream.Tags: a Vorbis comment's
	// TRACKNUMBER, ALBUMARTIST and YEAR come as "track", "album_artist" and
	// "date", as ID3's, APE's and MP4's do).
	named := func(name string) string { return clean(stream.Tags[name]) }
	got := Info{Title: named("title"), Artist: named("artist"), Album: named("album"),
		AlbumArtist: named("album_artist"), Genre: named("genre"),
		Year:   leading(named("date")),
		Number: leading(named("track")), Disc: leading(named("disc"))}
	if got.Year < 1000 || got.Year > 9999 {
		got.Year = 0
	}
	if stream.Duration > 0 {
		got.Length = stream.TimeBase.Duration(stream.Duration)
	}
	return got
}

// clean is a tag's text without the spaces and control characters around
// it, nor any within.
func clean(text string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, text))
}

// leading is the number a tag's text starts with: 3 of "3/12", 2026 of
// "2026-11-12"; 0 when none.
func leading(text string) int {
	text = strings.TrimSpace(text)
	end := 0
	for end < len(text) && end < 9 && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(text[:end])
	return n
}
