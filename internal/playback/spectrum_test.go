package playback

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func toneWAV(t *testing.T, frequency, seconds float64, channels int) (string, []byte) {
	t.Helper()
	const rate = 48000
	frames := int(seconds * rate)
	pcm := make([]byte, frames*channels*2)
	for frame := 0; frame < frames; frame++ {
		var sample int16
		if frequency > 0 {
			sample = int16(18000 * math.Sin(2*math.Pi*frequency*float64(frame)/rate))
		}
		for channel := 0; channel < channels; channel++ {
			binary.LittleEndian.PutUint16(pcm[(frame*channels+channel)*2:], uint16(sample))
		}
	}
	header := make([]byte, 44)
	copy(header, "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(len(pcm)+36))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], uint16(channels))
	binary.LittleEndian.PutUint32(header[24:], rate)
	binary.LittleEndian.PutUint32(header[28:], uint32(rate*channels*2))
	binary.LittleEndian.PutUint16(header[32:], uint16(channels*2))
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(len(pcm)))
	path := filepath.Join(t.TempDir(), "tone.wav")
	if err := os.WriteFile(path, append(header, pcm...), 0600); err != nil {
		t.Fatal(err)
	}
	return path, pcm
}

func installSpectrum(t *testing.T, p *MPV, generation int64) {
	t.Helper()
	filter := p.SpectrumFilter(generation)
	if filter == nil {
		t.Fatal("native metadata FIFO is unavailable")
	}
	send(t, p, "set_property", "af", []any{filter})
}

func (l *eventLog) spectrum(t *testing.T, from int, generation int64, matches func(SpectrumData) bool) SpectrumData {
	t.Helper()
	var found SpectrumData
	l.wait(t, from, "real spectrum", func(e Event) bool {
		var data SpectrumData
		if e.Event != "spectrum" || json.Unmarshal(e.Data, &data) != nil || data.Generation != generation || len(data.Bands) != SpectrumBands {
			return false
		}
		for _, level := range data.Bands {
			if math.IsNaN(float64(level)) || level < 0 || level > 1 {
				t.Fatalf("invalid measured level: %v", data)
			}
		}
		if !matches(data) {
			return false
		}
		found = data
		return true
	})
	return found
}

func peakFrequency(data SpectrumData) (int, float64) {
	index := 0
	for i, value := range data.Bands {
		if value > data.Bands[index] {
			index = i
		}
	}
	return index, 32 * math.Pow(500, float64(index)/47)
}

func TestRealMPVSpectrumTracksFrequencySilencePauseAndSeek(t *testing.T) {
	p := realMPV(t)
	l := &eventLog{p: p}
	low, _ := toneWAV(t, 440, 5, 2)
	installSpectrum(t, p, 10)
	send(t, p, "loadfile", low, "replace")
	data := l.spectrum(t, 0, 10, func(data SpectrumData) bool { return data.Time > .4 })
	lowIndex, frequency := peakFrequency(data)
	if frequency < 350 || frequency > 550 || data.Bands[lowIndex] < .75 {
		t.Fatalf("440 Hz did not dominate its frequency band: peak=%g levels=%v", frequency, data.Bands)
	}
	mark := len(l.seen)
	send(t, p, "set_property", "pause", true)
	l.wait(t, mark, "pause", property("pause", func(raw json.RawMessage) bool { return string(raw) == "true" }))
	position := l.getNumber(t, "time-pos")
	// Drain the already decoded prebuffer, then the meter must go quiet.
	until := time.Now().Add(350 * time.Millisecond)
	for time.Now().Before(until) {
		select {
		case e := <-p.Events():
			l.seen = append(l.seen, e)
		case <-time.After(10 * time.Millisecond):
		}
	}
	until = time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(until) {
		select {
		case e := <-p.Events():
			l.seen = append(l.seen, e)
			if e.Event == "spectrum" {
				t.Fatal("paused audio kept generating fresh spectrum frames")
			}
		case <-time.After(10 * time.Millisecond):
		}
	}
	if got := l.getNumber(t, "time-pos"); math.Abs(got-position) > .01 {
		t.Fatalf("paused analyzer advanced playback: %g -> %g", position, got)
	}
	mark = len(l.seen)
	installSpectrum(t, p, 11)
	send(t, p, "seek", 1.5, "absolute+exact")
	send(t, p, "set_property", "pause", false)
	l.spectrum(t, mark, 11, func(data SpectrumData) bool { return data.Time >= 1.5 && data.Time < 2.3 })

	mark = len(l.seen)
	high, _ := toneWAV(t, 6000, 3, 2)
	send(t, p, "loadfile", high, "replace")
	l.wait(t, mark, "high-tone file loaded", func(e Event) bool { return e.Event == "file-loaded" })
	l.wait(t, mark, "high-tone source path", property("path", func(raw json.RawMessage) bool {
		var path string
		return json.Unmarshal(raw, &path) == nil && path == high
	}))
	installSpectrum(t, p, 12)
	data = l.spectrum(t, mark, 12, func(data SpectrumData) bool { return data.Time > .4 && data.Time < 1.4 })
	highIndex, frequency := peakFrequency(data)
	if frequency < 4500 || frequency > 7500 || highIndex-lowIndex < 15 {
		t.Fatalf("new 6 kHz track retained old bands: peak=%g levels=%v", frequency, data.Bands)
	}
	mark = len(l.seen)
	silence, _ := toneWAV(t, 0, 1.3, 2)
	send(t, p, "loadfile", silence, "replace")
	l.wait(t, mark, "silent file loaded", func(e Event) bool { return e.Event == "file-loaded" })
	l.wait(t, mark, "silent source path", property("path", func(raw json.RawMessage) bool {
		var path string
		return json.Unmarshal(raw, &path) == nil && path == silence
	}))
	// A filter installed before replacing the file can measure the old audio
	// with the new generation. Label only the confirmed replacement source;
	// old/new media timestamps overlap and cannot identify the file by themselves.
	installSpectrum(t, p, 13)
	data = l.spectrum(t, mark, 13, func(data SpectrumData) bool { return data.Time > .3 && data.Time < 1 })
	for _, level := range data.Bands {
		if level > .001 {
			t.Fatalf("silence generated nonzero bands at %gs in generation%d: %v", data.Time, data.Generation, data.Bands)
		}
	}
	l.wait(t, mark, "EOF with analyzer installed", func(e Event) bool { return e.Event == "end-file" && e.Reason == "eof" })
}

func TestRealMPVSpectrumPreservesChannelsAndSamples(t *testing.T) {
	requireSpectrumHost(t)
	for _, channels := range []int{1, 2, 6} {
		t.Run(string(rune('0'+channels))+" channels", func(t *testing.T) {
			p := realMPV(t)
			l := &eventLog{p: p}
			path, _ := toneWAV(t, 1000, 3, channels)
			send(t, p, "loadfile", path, "replace")
			l.wait(t, 0, "file loaded", func(e Event) bool { return e.Event == "file-loaded" })
			l.wait(t, 0, "audio output ready", property("time-pos", func(raw json.RawMessage) bool { return number(raw) > .05 }))
			before := getAudioOutput(t, l)
			mark := len(l.seen)
			installSpectrum(t, p, 20)
			l.spectrum(t, mark, 20, func(data SpectrumData) bool { return data.Time > .2 })
			after := getAudioOutput(t, l)
			if before["channel-count"] != float64(channels) || before["channels"] != after["channels"] || before["samplerate"] != after["samplerate"] || before["format"] != after["format"] {
				t.Fatalf("analysis rerouted original audio: before=%v after=%v", before, after)
			}
		})
	}
	t.Run("PCM unchanged", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "output.wav")
		p, err := New(Options{ExtraArgs: []string{"--ao=pcm", "--ao-pcm-file=" + output}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = p.Close()
			for range p.Events() {
			}
		})
		l := &eventLog{p: p}
		path, pcm := toneWAV(t, 1000, .3, 2)
		installSpectrum(t, p, 30)
		send(t, p, "loadfile", path, "replace")
		l.wait(t, 0, "PCM EOF", func(e Event) bool { return e.Event == "end-file" && e.Reason == "eof" })
		_ = p.Close()
		encoded, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		var audio []byte
		for at := 12; at+8 <= len(encoded); {
			size := int(binary.LittleEndian.Uint32(encoded[at+4:]))
			end := at + 8 + size
			if end > len(encoded) {
				t.Fatal("truncated PCM WAV")
			}
			if string(encoded[at:at+4]) == "data" {
				audio = encoded[at+8 : end]
				break
			}
			at = end + (size & 1)
		}
		if string(audio) != string(pcm) {
			t.Fatalf("passive analyzer changed PCM: source=%d output=%d", len(pcm), len(audio))
		}
	})
}

func getAudioOutput(t *testing.T, l *eventLog) map[string]any {
	t.Helper()
	mark := len(l.seen)
	send(t, l.p, "get_property", "audio-out-params")
	e := l.wait(t, mark, "audio output", func(e Event) bool {
		return e.Event == "command-reply" && string(e.Command) == `["get_property","audio-out-params"]`
	})
	var params map[string]any
	if json.Unmarshal(e.Data, &params) != nil || params == nil {
		t.Fatalf("missing audio output: %s", e.Data)
	}
	return params
}

func TestRealMPVSpectrumDoesNotBackUpControlEvents(t *testing.T) {
	p := realMPV(t)
	l := &eventLog{p: p}
	path, _ := toneWAV(t, 1000, 5, 2)
	installSpectrum(t, p, 40)
	send(t, p, "loadfile", path, "replace")
	l.spectrum(t, 0, 40, func(data SpectrumData) bool { return data.Time > .2 })
	for i := 0; i < 180; i++ {
		send(t, p, "get_property", "volume")
	}
	// A full public event buffer must retain control replies and coalesce
	// changing spectrum pictures rather than adding them to the backlog.
	time.Sleep(700 * time.Millisecond)
	send(t, p, "stop")
	_ = p.Close()
	replies, ends, pictures := 0, 0, 0
	for e := range p.Events() {
		if e.Event == "command-reply" && string(e.Command) == `["get_property","volume"]` {
			replies++
		}
		if e.Event == "end-file" {
			ends++
		}
		if e.Event == "spectrum" {
			pictures++
		}
	}
	if replies != 180 || ends != 1 || pictures > 3 {
		t.Fatalf("bad backlog: replies=%d end=%d pictures=%d", replies, ends, pictures)
	}
}

func TestRealMPVSpectrumEscapesPrivateFIFOPath(t *testing.T) {
	requireSpectrumHost(t)
	dir, err := os.MkdirTemp("/tmp", "meter 'colon:")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("TMPDIR", dir)
	p := realMPV(t)
	if !strings.Contains(p.dir, "'") || !strings.Contains(p.dir, ":") {
		t.Fatal("fixture did not exercise filter escaping")
	}
	path, _ := toneWAV(t, 1000, 2, 2)
	installSpectrum(t, p, 50)
	send(t, p, "loadfile", path, "replace")
	l := &eventLog{p: p}
	l.spectrum(t, 0, 50, func(data SpectrumData) bool { return data.Time > .2 })
}

func requireSpectrumHost(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("Unix IPC and metadata FIFO")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("system mpv is not installed")
	}
}
