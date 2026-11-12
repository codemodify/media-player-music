package tags

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/codemodify/media-player-music/internal/tags/tagstest"
)

func tagged(t *testing.T, parts ...[]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(path, bytes.Join(parts, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReplayGainReadsTXXXRVA2AndAPE(t *testing.T) {
	frames := bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0x00}, 64) // stands for audio
	for _, tc := range []struct {
		name                    string
		path                    string
		track, album, trackPeak float64
		hasTrack, hasAlbum      bool
	}{
		{"TXXX in ID3v2.3 (UTF-16)", tagged(t, tagstest.V2(3,
			tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(3, "REPLAYGAIN_TRACK_GAIN", "-6.48 dB")},
			tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(3, "REPLAYGAIN_TRACK_PEAK", "0.988525")},
			tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(3, "replaygain_album_gain", "-7.10 dB")}), frames),
			-6.48, -7.10, 0.988525, true, true},
		{"TXXX in ID3v2.4 (UTF-8)", tagged(t, tagstest.V2(4,
			tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(4, "REPLAYGAIN_ALBUM_GAIN", "+2.5 dB")}), frames),
			0, 2.5, 0, false, true},
		{"RVA2", tagged(t, tagstest.V2(4,
			tagstest.Frame{ID: "RVA2", Data: tagstest.RVA2("track", -3.25, 0.5)},
			tagstest.Frame{ID: "RVA2", Data: tagstest.RVA2("ALBUM", 1.5, 0.75)}), frames),
			-3.25, 1.5, 0.5, true, true},
		{"APEv2 before ID3v1", tagged(t, frames, tagstest.APE([2]string{"REPLAYGAIN_TRACK_GAIN", "-1.00 dB"},
			[2]string{"Cover Art (Front)", "x"}, [2]string{"replaygain_track_peak", "0.25"}), tagstest.V1("a", "b", "c")),
			-1, 0, 0.25, true, false},
		{"APEv2 at the end", tagged(t, frames, tagstest.APE([2]string{"REPLAYGAIN_ALBUM_GAIN", "-4 dB"})),
			0, -4, 0, false, true},
		{"TXXX over RVA2 over APE", tagged(t, tagstest.V2(4,
			tagstest.Frame{ID: "RVA2", Data: tagstest.RVA2("track", -2, 0.5)},
			tagstest.Frame{ID: "TXXX", Data: tagstest.UserText(4, "REPLAYGAIN_TRACK_GAIN", "-1 dB")}),
			frames, tagstest.APE([2]string{"REPLAYGAIN_TRACK_GAIN", "-9 dB"}, [2]string{"REPLAYGAIN_ALBUM_GAIN", "-8 dB"})),
			-1, -8, 0.5, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ReplayGain(tc.path)
			if !ok || got.HasTrack != tc.hasTrack || got.HasAlbum != tc.hasAlbum ||
				math.Abs(got.TrackGain-tc.track) > 0.01 || math.Abs(got.AlbumGain-tc.album) > 0.01 || math.Abs(got.TrackPeak-tc.trackPeak) > 0.001 {
				t.Fatalf("got %+v, %v", got, ok)
			}
		})
	}
}

func TestReplayGainWithoutTagsOrWithBrokenOnes(t *testing.T) {
	frames := bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0x00}, 64)
	plain := tagged(t, tagstest.V2(3, tagstest.Frame{ID: "TIT2", Data: tagstest.Text(3, "Untitled")}), frames)
	// An APE footer whose size runs past the file's start.
	broken := tagstest.APE([2]string{"REPLAYGAIN_TRACK_GAIN", "-1 dB"})
	footer := broken[len(broken)-32:]
	binary.LittleEndian.PutUint32(footer[12:], 1<<19)
	for _, path := range []string{plain, tagged(t, frames, footer), filepath.Join(t.TempDir(), "missing.mp3")} {
		if got, ok := ReplayGain(path); ok {
			t.Fatalf("%s: a gain from nothing: %+v", filepath.Base(path), got)
		}
	}
}
