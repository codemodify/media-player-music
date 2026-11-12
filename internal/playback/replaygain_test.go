package playback

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/codemodify/media-player-music/internal/tags/tagstest"
	"goframework.io/codecs-audio-flac"
	aspec "goframework.io/codecs-audio-spec"
	"goframework.io/system-audio-spec/audiotest"
)

// The ReplayGain tests play files the tests tag themselves (ID3's TXXX and
// RVA2, an APEv2 tag, a FLAC file's Vorbis comments) into audiotest's sound
// system, and compare what was heard with the file played without gain.

// played plays a file to its end with the ReplayGain settings, and returns
// the energy and the peak of what the sound system was given.
func played(t *testing.T, path string, gain ReplayGain) (energy, peak float64) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		b, system := builtinOn(t)
		b.Apply(Command{Op: OpVolume, Volume: 1})
		b.Apply(Command{Op: OpReplayGain, ReplayGain: gain})
		b.Apply(Command{Op: OpLoad, Generation: 1, Path: path, Epoch: 1})
		for _, u := range collect(t, b, 1, UpdateEnded, 10*time.Second) {
			if u.Kind == UpdateError {
				t.Fatalf("%s: %s", filepath.Base(path), u.Error)
			}
		}
		out, err := system.Played(audiotest.Speaker)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range out.F32 {
			energy += float64(s) * float64(s)
			peak = max(peak, math.Abs(float64(s)))
		}
	})
	if energy == 0 {
		t.Fatalf("%s: nothing heard", filepath.Base(path))
	}
	return energy, peak
}

// decibels is the gain from one energy to another.
func decibels(from, to float64) float64 { return 10 * math.Log10(to/from) }

// near fails unless a gain is within 0.3 dB of the one wanted.
func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.3 {
		t.Errorf("%s: %.2f dB, want %.2f dB", what, got, want)
	}
}

func writeFile(t *testing.T, name string, parts ...[]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, bytes.Join(parts, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mp3Audio(t *testing.T) []byte {
	t.Helper()
	audio, err := os.ReadFile(filepath.Join("testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	return audio
}

// flacTone is a second of a 440 Hz tone at amplitude, 44.1 kHz stereo,
// with tags as Vorbis comments.
func flacTone(t *testing.T, amplitude float64, tags map[string]string) string {
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
		v := int16(amplitude * 32767 * math.Sin(2*math.Pi*440*float64(i)/44100))
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

func TestReplayGainFromID3UserTextByMode(t *testing.T) {
	audio := mp3Audio(t)
	plain := writeFile(t, "plain.mp3", audio)
	for _, major := range []int{3, 4} {
		t.Run(fmt.Sprintf("ID3v2.%d", major), func(t *testing.T) {
			tag := tagstest.V2(major,
				tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(major, "REPLAYGAIN_TRACK_GAIN", "-6.00 dB")},
				tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(major, "replaygain_album_gain", "-12.00 dB")},
				tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(major, "REPLAYGAIN_TRACK_PEAK", "0.500000")})
			tagged := writeFile(t, "tagged.mp3", tag, audio)
			reference, _ := played(t, plain, ReplayGain{Mode: ReplayGainOff})
			off, _ := played(t, tagged, ReplayGain{Mode: ReplayGainOff})
			track, _ := played(t, tagged, ReplayGain{Mode: ReplayGainTrack})
			album, _ := played(t, tagged, ReplayGain{Mode: ReplayGainAlbum})
			lifted, _ := played(t, tagged, ReplayGain{Mode: ReplayGainTrack, Preamp: 3})
			near(t, "off", decibels(reference, off), 0)
			near(t, "track", decibels(reference, track), -6)
			near(t, "album", decibels(reference, album), -12)
			near(t, "track with a 3 dB preamp", decibels(reference, lifted), -3)
		})
	}
}

func TestReplayGainFromRVA2AndAPE(t *testing.T) {
	audio := mp3Audio(t)
	reference, _ := played(t, writeFile(t, "plain.mp3", audio), ReplayGain{})
	rva2 := writeFile(t, "rva2.mp3", tagstest.V2(4,
		tagstest.Frame{ID: "RVA2", Data: tagstest.RVA2("track", -4.5, 0.6)},
		tagstest.Frame{ID: "RVA2", Data: tagstest.RVA2("album", -9, 0.6)}), audio)
	ape := writeFile(t, "ape.mp3", audio, tagstest.APE([2]string{"REPLAYGAIN_TRACK_GAIN", "-7.50 dB"},
		[2]string{"Replaygain_Album_Gain", "-2.00 dB"}, [2]string{"REPLAYGAIN_TRACK_PEAK", "0.7"}),
		tagstest.V1("Slow Ferry", "Kite & Tallow", "Low Tide"))
	// TXXX wins over APE where a file has both (foobar2000 writes both).
	both := writeFile(t, "both.mp3", tagstest.V2(3,
		tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(3, "REPLAYGAIN_TRACK_GAIN", "-3.00 dB")}),
		audio, tagstest.APE([2]string{"REPLAYGAIN_TRACK_GAIN", "-9.00 dB"}))
	for _, tc := range []struct {
		name string
		path string
		mode ReplayGainMode
		want float64
	}{
		{"RVA2 track", rva2, ReplayGainTrack, -4.5},
		{"RVA2 album", rva2, ReplayGainAlbum, -9},
		{"APE track", ape, ReplayGainTrack, -7.5},
		{"APE album", ape, ReplayGainAlbum, -2},
		{"TXXX over APE", both, ReplayGainTrack, -3},
		{"album mode, only a track gain", both, ReplayGainAlbum, -3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := played(t, tc.path, ReplayGain{Mode: tc.mode})
			near(t, tc.name, decibels(reference, got), tc.want)
		})
	}
}

func TestReplayGainFromVorbisComments(t *testing.T) {
	// FLAC's comments reach audio-playback's own ReplayGain: nothing of
	// the app's reader.
	plain := flacTone(t, 0.25, nil)
	tagged := flacTone(t, 0.25, map[string]string{"REPLAYGAIN_TRACK_GAIN": "-5.00 dB", "REPLAYGAIN_TRACK_PEAK": "0.25",
		"REPLAYGAIN_ALBUM_GAIN": "+1.50 dB", "REPLAYGAIN_ALBUM_PEAK": "0.25"})
	reference, _ := played(t, plain, ReplayGain{})
	track, _ := played(t, tagged, ReplayGain{Mode: ReplayGainTrack})
	album, _ := played(t, tagged, ReplayGain{Mode: ReplayGainAlbum, Preamp: 1})
	off, _ := played(t, tagged, ReplayGain{Mode: ReplayGainOff, Preamp: 6})
	near(t, "track", decibels(reference, track), -5)
	near(t, "album with a 1 dB preamp", decibels(reference, album), 2.5)
	near(t, "off (the preamp unused)", decibels(reference, off), 0)
}

// TestReplayGainChangesWhilePlaying turns ReplayGain on while a track
// plays, all of it decoded ahead: the rest of it is heard at its gain,
// without restarting it (goframework R100).
func TestReplayGainChangesWhilePlaying(t *testing.T) {
	tagged := flacTone(t, 0.25, map[string]string{"REPLAYGAIN_TRACK_GAIN": "-12.00 dB"})
	synctest.Test(t, func(t *testing.T) {
		b, system := builtinOn(t)
		b.Apply(Command{Op: OpVolume, Volume: 1})
		b.Apply(Command{Op: OpReplayGain, ReplayGain: ReplayGain{Mode: ReplayGainOff}})
		b.Apply(Command{Op: OpLoad, Generation: 1, Path: tagged, Epoch: 1})
		time.Sleep(400 * time.Millisecond)
		b.Apply(Command{Op: OpReplayGain, ReplayGain: ReplayGain{Mode: ReplayGainTrack}})
		var loads int
		for _, u := range collect(t, b, 1, UpdateEnded, 10*time.Second) {
			switch u.Kind {
			case UpdateError:
				t.Fatal(u.Error)
			case UpdateLoaded:
				loads++
			}
		}
		if loads != 1 {
			t.Errorf("the track loaded %d times", loads)
		}
		out, err := system.Played(audiotest.Speaker)
		if err != nil {
			t.Fatal(err)
		}
		channels := out.Format.Channels
		samples := out.F32[:out.Samples*channels]
		start := 0
		for start < len(samples) && samples[start] == 0 {
			start++
		}
		rate := out.Format.Rate * channels
		energy := func(from, to float64) float64 {
			sum := 0.0
			for _, s := range samples[start+int(from*float64(rate)) : start+int(to*float64(rate))] {
				sum += float64(s) * float64(s)
			}
			return sum
		}
		near(t, "after the change", decibels(energy(0.05, 0.25), energy(0.7, 0.9)), -12)
	})
}

func TestReplayGainPeakPreventsClipping(t *testing.T) {
	audio := mp3Audio(t)
	reference, peak := played(t, writeFile(t, "plain.mp3", audio), ReplayGain{})
	if peak <= 0 || peak >= 1 {
		t.Fatalf("the test file's peak: %v", peak)
	}
	// +20 dB would take the peak far past full scale: the tags' peak (the
	// file's own) holds the gain to what brings it to full scale.
	limit := -20 * math.Log10(peak)
	tagged := writeFile(t, "loud.mp3", tagstest.V2(3,
		tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(3, "REPLAYGAIN_TRACK_GAIN", "+20.00 dB")},
		tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(3, "REPLAYGAIN_TRACK_PEAK", fmt.Sprintf("%.6f", peak))}), audio)
	got, loudest := played(t, tagged, ReplayGain{Mode: ReplayGainTrack})
	near(t, "the peak-limited gain", decibels(reference, got), limit)
	if loudest > 1.0001 {
		t.Fatalf("clipped: a sample of %v", loudest)
	}
	// The preamp too stays under the peak's limit.
	lifted, _ := played(t, tagged, ReplayGain{Mode: ReplayGainTrack, Preamp: 6})
	near(t, "the peak-limited gain with a preamp", decibels(reference, lifted), limit)
}

func TestUnsupportedFormatsSayWhatPlays(t *testing.T) {
	path := writeFile(t, "song.wma", []byte("0&\xb2\x75\x8e\x66\xcf\x11 not a format the player reads"))
	synctest.Test(t, func(t *testing.T) {
		b, _ := builtinOn(t)
		b.Apply(Command{Op: OpLoad, Generation: 2, Path: path})
		updates := collect(t, b, 2, UpdateError, 5*time.Second)
		failed, _ := find(updates, UpdateError)
		if !strings.Contains(failed.Error, "song.wma: WMA files are not supported. The player plays MP3, AAC") {
			t.Fatalf("the message: %q", failed.Error)
		}
	})
	if !strings.Contains(UnsupportedMessage("/music/odd.xyz"), "odd.xyz: this format is not supported") {
		t.Fatal("an unknown extension's message")
	}
	if !Playable("a/b/Song.FLAC") || Playable("a/b/song.wma") || Playable("cover.jpg") {
		t.Fatal("Playable")
	}
	if format, ok := UnsupportedFormat("x.APE"); !ok || format != "Monkey's Audio (APE)" {
		t.Fatal("UnsupportedFormat")
	}
}
