package tags

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/tags/tagstest"
	"goframework.io/codecs-audio-flac"
	aspec "goframework.io/codecs-audio-spec"
	"goframework.io/codecs-container-ogg"
	container "goframework.io/codecs-container-spec"
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

// audio is a second of real MP3 (the engine's test file), which opens.
func audio(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "playback", "testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func text(info Info) [3]string { return [3]string{info.Title, info.Artist, info.Album} }

func TestReadsEveryID3Version(t *testing.T) {
	for _, major := range []int{2, 3, 4} {
		ids := map[string]string{"title": "TIT2", "artist": "TPE1", "album": "TALB", "band": "TPE2", "genre": "TCON", "track": "TRCK", "disc": "TPOS", "year": "TYER"}
		if major == 2 {
			ids = map[string]string{"title": "TT2", "artist": "TP1", "album": "TAL", "band": "TP2", "genre": "TCO", "track": "TRK", "disc": "TPA", "year": "TYE"}
		}
		if major == 4 {
			ids["year"] = "TDRC"
		}
		tag := tagstest.V2(major,
			tagstest.Frame{ID: ids["title"], Data: tagstest.Text(major, "Harbour Lights")},
			tagstest.Frame{ID: ids["artist"], Data: tagstest.Text(major, "Tessellators")},
			tagstest.Frame{ID: ids["album"], Data: tagstest.Text(major, "Low Tide")},
			tagstest.Frame{ID: ids["band"], Data: tagstest.Text(major, "Various Artists")},
			tagstest.Frame{ID: ids["genre"], Data: tagstest.Text(major, "Ambient")},
			tagstest.Frame{ID: ids["track"], Data: tagstest.Text(major, "3/12")},
			tagstest.Frame{ID: ids["disc"], Data: tagstest.Text(major, "2/2")},
			tagstest.Frame{ID: ids["year"], Data: tagstest.Text(major, "2019")})
		info, ok := Read(write(t, "song.mp3", tag, audio(t)))
		want := Info{Title: "Harbour Lights", Artist: "Tessellators", Album: "Low Tide", AlbumArtist: "Various Artists", Genre: "Ambient", Year: 2019, Number: 3, Disc: 2}
		if info.Length < 900*time.Millisecond || info.Length > 1100*time.Millisecond {
			t.Fatalf("ID3v2.%d: the length: %+v", major, info)
		}
		info.Length = 0
		if !ok || info != want {
			t.Fatalf("ID3v2.%d: %+v, %v", major, info, ok)
		}
	}
}

func TestReadsID3v1AndJoinsItBehindID3v2(t *testing.T) {
	path := write(t, "old.mp3", frames, tagstest.V1("Marram", "Ostro", "Dune Slack"))
	if info, ok := Read(path); !ok || text(info) != [3]string{"Marram", "Ostro", "Dune Slack"} {
		t.Fatalf("ID3v1: %+v, %v", info, ok)
	}
	// ID3v2's frames win; ID3v1 fills what ID3v2 does not say.
	path = write(t, "both.mp3", tagstest.V2(3, tagstest.Frame{ID: "TIT2", Data: tagstest.Text(3, "Beacon — live")}), frames, tagstest.V1("Beacon", "Ruth Alder", "Field Notes"))
	if info, ok := Read(path); !ok || text(info) != [3]string{"Beacon — live", "Ruth Alder", "Field Notes"} {
		t.Fatalf("ID3v2 and v1: %+v, %v", info, ok)
	}
}

// flacTone is a second of silence-near tone, 44.1 kHz stereo, with tags as
// Vorbis comments.
func flacTone(t *testing.T, tags map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tone.flac")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	format := aspec.Format{Rate: 44100, Channels: 2, Sample: aspec.S16}
	w, err := flac.Create(file, flac.Settings{Format: format, Tags: tags})
	if err != nil {
		t.Fatal(err)
	}
	frame := aspec.Frame{Format: format}
	frame.Resize(44100)
	for i := 0; i < 44100; i++ {
		v := int16(800 * math.Sin(2*math.Pi*440*float64(i)/44100))
		frame.S16[2*i], frame.S16[2*i+1] = v, v
	}
	if err := w.WriteFrame(&frame); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadsVorbisCommentsAndTheLength(t *testing.T) {
	path := flacTone(t, map[string]string{"TITLE": "Copper Wire", "ARTIST": "Ostro", "ALBUM": "Dune Slack", "ALBUMARTIST": "Ostro & Friends",
		"GENRE": "Folk", "DATE": "2021-04-30", "TRACKNUMBER": "07", "DISCNUMBER": "1"})
	info, ok := Read(path)
	want := Info{Title: "Copper Wire", Artist: "Ostro", Album: "Dune Slack", AlbumArtist: "Ostro & Friends", Genre: "Folk", Year: 2021, Number: 7, Disc: 1, Length: time.Second}
	if !ok || info != want {
		t.Fatalf("FLAC: %+v, %v", info, ok)
	}
}

// The names other taggers write (TRACKNUMBER, ALBUM ARTIST, YEAR) are read
// as the framework names them for every container (R114): in FLAC, and in
// Ogg, where the stream is remuxed with its comments.
func TestReadsOtherTaggersVorbisNames(t *testing.T) {
	tagged := map[string]string{"TITLE": "Salt Road", "ALBUM ARTIST": "Various Artists",
		"YEAR": "1998", "TRACKNUMBER": "4/11", "DISCNUMBER": "2/2"}
	want := Info{Title: "Salt Road", AlbumArtist: "Various Artists", Year: 1998, Number: 4, Disc: 2, Length: time.Second}
	path := flacTone(t, tagged)
	if info, ok := Read(path); !ok || info != want {
		t.Fatalf("FLAC: %+v, %v", info, ok)
	}
	if info, ok := Read(oggOf(t, path, map[string]string{"TITLE": "Salt Road", "ALBUMARTIST": "Various Artists",
		"YEAR": "1998", "TRACKNUMBER": "4", "DISCNUMBER": "2"})); !ok || info != want {
		t.Fatalf("Ogg: %+v, %v", info, ok)
	}
}

// oggOf remuxes a FLAC file's stream into an Ogg file with tags as its
// comments.
func oggOf(t *testing.T, path string, tags map[string]string) string {
	t.Helper()
	in, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	demuxer, err := container.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer demuxer.Close()
	out := filepath.Join(t.TempDir(), "tone.oga")
	file, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	w, err := ogg.Create(file, ogg.Settings{})
	if err != nil {
		t.Fatal(err)
	}
	stream := demuxer.Streams()[0]
	stream.Tags = tags
	index, err := w.Add(stream)
	if err != nil {
		t.Fatal(err)
	}
	var packet container.Packet
	for {
		if err := demuxer.Read(&packet); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		packet.Stream = index
		if err := w.Write(packet); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestWhatIsNotMusicGivesNothing(t *testing.T) {
	if _, ok := Read(write(t, "notes.txt", []byte("not music at all"))); ok {
		t.Fatal("a text file gave tags")
	}
	if _, ok := Read(filepath.Join(t.TempDir(), "missing.mp3")); ok {
		t.Fatal("a missing file gave tags")
	}
	// control characters and spaces around a tag's text are not kept
	tag := tagstest.V2(3, tagstest.Frame{ID: "TIT2", Data: tagstest.Text(3, "  Nine\x01 Volt\t ")})
	if info, ok := Read(write(t, "song.mp3", tag, frames)); !ok || info.Title != "Nine Volt" {
		t.Fatalf("%q, %v", info.Title, ok)
	}
}

// A Monkey's Audio file (R101) gives its APE tag and its length to the
// library.
func TestMonkeysAudioTagsAndLength(t *testing.T) {
	audio, err := os.ReadFile(filepath.Join("..", "playback", "testdata", "stereo-c3000.ape"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "song.ape")
	tag := tagstest.APE([2]string{"Title", "Slow Ferry"}, [2]string{"Artist", "Kite & Tallow"}, [2]string{"Album", "Low Tide"},
		[2]string{"Genre", "Ambient"}, [2]string{"Year", "2011"}, [2]string{"Track", "3/9"})
	if err := os.WriteFile(path, append(audio, tag...), 0o600); err != nil {
		t.Fatal(err)
	}
	info, ok := Read(path)
	if !ok || info.Title != "Slow Ferry" || info.Artist != "Kite & Tallow" || info.Album != "Low Tide" || info.Genre != "Ambient" || info.Year != 2011 || info.Number != 3 {
		t.Fatalf("%+v %v", info, ok)
	}
	if info.Length < 550*time.Millisecond || info.Length > 650*time.Millisecond {
		t.Fatalf("the length: %v", info.Length)
	}
}
