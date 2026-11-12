package tags

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"goframework.io/audio-dsp/replaygain"
	id3 "goframework.io/codecs-container-id3"
)

// ReplayGain returns the ReplayGain a file's ID3 and APE tags give: ID3v2's
// TXXX frames (REPLAYGAIN_TRACK_GAIN and the others, as foobar2000 and
// most taggers write them), else its RVA2 frames ("track" and "album"),
// else an APEv2 or APEv1 tag at the file's end (as mp3gain writes them).
// ok is false when none gives a gain.
//
// R97, R98: codecs-container-id3's Tags gives neither TXXX's ReplayGain nor
// RVA2, and no goframework module reads APE tags, so audio-playback, which
// levels a track by its stream's tags, finds no gain in them: the engine
// reads them here and hands audio-playback the gain. Vorbis comments (FLAC,
// Ogg) and MP4's freeform items reach the stream's tags and need nothing.
func ReplayGain(path string) (replaygain.Tags, bool) {
	file, err := os.Open(path)
	if err != nil {
		return replaygain.Tags{}, false
	}
	defer file.Close()
	found := map[string]string{}
	// The weakest first: each later source replaces what it also gives.
	for name, value := range apeItems(file) {
		if strings.HasPrefix(name, "REPLAYGAIN_") {
			found[name] = value
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return replaygain.Tags{}, false
	}
	// id3.Read reads from where the file is.
	if tag, err := id3.Read(file); err == nil {
		for name, value := range rva2(tag) {
			found[name] = value
		}
		for _, name := range []string{"REPLAYGAIN_TRACK_GAIN", "REPLAYGAIN_TRACK_PEAK", "REPLAYGAIN_ALBUM_GAIN", "REPLAYGAIN_ALBUM_PEAK"} {
			if value, ok := tag.UserText(name); ok {
				found[name] = value
			}
		}
	}
	parsed := replaygain.Parse(found)
	return parsed, parsed.HasTrack || parsed.HasAlbum
}

// rva2 reads ID3v2.4's relative volume adjustments of the master volume
// (else the first channel given) as ReplayGain tags: the identification
// "album" gives the album's, any other the track's.
func rva2(tag *id3.Tag) map[string]string {
	found := map[string]string{}
	for _, frame := range tag.Frames {
		if frame.ID != "RVA2" || frame.Compressed || frame.Encrypted {
			continue
		}
		end := 0
		for end < len(frame.Data) && frame.Data[end] != 0 {
			end++
		}
		if end == len(frame.Data) {
			continue
		}
		kind := "TRACK"
		if strings.EqualFold(string(frame.Data[:end]), "album") {
			kind = "ALBUM"
		}
		gain, peak, ok := 0.0, 0.0, false
		for rest := frame.Data[end+1:]; len(rest) >= 4; {
			channel, adjust, bits := rest[0], int16(binary.BigEndian.Uint16(rest[1:])), int(rest[3])
			size := (bits + 7) / 8
			if len(rest) < 4+size {
				break
			}
			if !ok || channel == 1 {
				gain, peak, ok = float64(adjust)/512, peakOf(rest[4:4+size]), true
			}
			rest = rest[4+size:]
			if channel == 1 {
				break
			}
		}
		if ok {
			found["REPLAYGAIN_"+kind+"_GAIN"] = fmt.Sprintf("%.2f dB", gain)
			if peak > 0 {
				found["REPLAYGAIN_"+kind+"_PEAK"] = fmt.Sprintf("%.6f", peak)
			}
		}
	}
	return found
}

// peakOf is an RVA2 peak's value as a fraction of full scale: its bytes,
// big-endian, the most significant bit first, as the bits of a sample
// (at most 32 read).
func peakOf(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	b = b[:min(len(b), 4)]
	value := uint64(0)
	for _, x := range b {
		value = value<<8 | uint64(x)
	}
	return float64(value) / math.Exp2(float64(8*len(b)-1))
}

// apeTagMost is the largest APE tag read: mp3gain's are a few hundred bytes.
const apeTagMost = 1 << 20

// apeItems reads the text items of the APEv2 (or APEv1) tag at a file's
// end, or before an ID3v1 tag there, by upper-case keys. nil when there
// is none.
func apeItems(file io.ReadSeeker) map[string]string {
	end, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil
	}
	footer := make([]byte, 32)
	at := end - 32
	for _, skip := range []int64{0, 128} {
		at = end - skip - 32
		if at < 0 {
			return nil
		}
		if _, err := file.Seek(at, io.SeekStart); err != nil {
			return nil
		}
		if _, err := io.ReadFull(file, footer); err != nil {
			return nil
		}
		if string(footer[:8]) == "APETAGEX" {
			break
		}
		if skip == 128 {
			return nil
		}
	}
	size, count := int64(binary.LittleEndian.Uint32(footer[12:])), int(binary.LittleEndian.Uint32(footer[16:]))
	if size < 32 || size > apeTagMost || size > at+32 {
		return nil
	}
	body := make([]byte, size-32)
	if _, err := file.Seek(at+32-size, io.SeekStart); err != nil {
		return nil
	}
	if _, err := io.ReadFull(file, body); err != nil {
		return nil
	}
	items := map[string]string{}
	for ; count > 0 && len(body) >= 9; count-- {
		length, flags := int(binary.LittleEndian.Uint32(body)), binary.LittleEndian.Uint32(body[4:])
		key := 8
		for key < len(body) && body[key] != 0 {
			key++
		}
		if key == len(body) || length < 0 || length > len(body)-key-1 {
			break
		}
		name, value := strings.ToUpper(string(body[8:key])), body[key+1:key+1+length]
		if flags&6 == 0 { // UTF-8 text
			items[name] = strings.TrimRight(string(value), "\x00")
		}
		body = body[key+1+length:]
	}
	return items
}
