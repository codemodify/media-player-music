package playback

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const SpectrumBands = 48
const spectrumLabel = "music-player-spectrum"
const spectrumGenerationKey = "music_player_generation"

// SpectrumSource optionally supplies a real, passive analyzer for the audio
// filter chain. The caller includes this entry alongside its playback filters
// and changes generation when loading or seeking, to reject queued old samples.
type SpectrumSource interface {
	SpectrumFilter(generation int64) map[string]any
}

// SpectrumData is carried by Event{Event: "spectrum"}. Bands are logarithmic
// frequency measurements from 32 Hz to 16 kHz, normalized from -60..0 dB RMS
// to 0..1. Time is the decoded frame's media position, not a wall clock.
type SpectrumData struct {
	Bands      []float32 `json:"bands"`
	Time       float64   `json:"time"`
	Generation int64     `json:"generation"`
}

// SpectrumFilter uses the same mpv process and decoder as playback. Its
// original audio branch is untouched; the analysis branch is downmixed,
// measured, then discarded. No PCM is exported and no extra program runs.
// A nil result means this platform could not create the private metadata FIFO.
func (p *MPV) SpectrumFilter(generation int64) map[string]any {
	if p == nil || p.spectrum == nil {
		return nil
	}
	return map[string]any{
		"name": "lavfi", "label": spectrumLabel,
		"params": map[string]string{"graph": spectrumGraph(p.spectrum.path, generation)},
	}
}

func spectrumGraph(path string, generation int64) string {
	var graph strings.Builder
	graph.WriteString("[in]asplit=2[main][analysis];[analysis]aformat=sample_fmts=fltp:channel_layouts=mono,aresample=48000,asetnsamples=n=1024:p=0,asplit=48")
	for i := 0; i < SpectrumBands; i++ {
		fmt.Fprintf(&graph, "[b%d]", i)
	}
	graph.WriteByte(';')
	width := math.Log2(16000.0/32) / (SpectrumBands - 1)
	for i := 0; i < SpectrumBands; i++ {
		frequency := 32 * math.Pow(16000.0/32, float64(i)/(SpectrumBands-1))
		// Explicit mono layout after each band is necessary for amerge's
		// format negotiation, including modern FFmpeg's arbitrary layouts.
		fmt.Fprintf(&graph, "[b%d]bandpass=f=%.6f:t=o:w=%.6f,aformat=sample_fmts=fltp:channel_layouts=mono[v%d];", i, frequency, width, i)
	}
	for i := 0; i < SpectrumBands; i++ {
		fmt.Fprintf(&graph, "[v%d]", i)
	}
	fmt.Fprintf(&graph, "amerge=inputs=48,astats=metadata=1:reset=1:measure_perchannel=RMS_level:measure_overall=none,ametadata=mode=delete:key=%s,ametadata=mode=add:key=%s:value=%d,ametadata=mode=print:file=%s:direct=1,anullsink;[main]anull[out]", spectrumGenerationKey, spectrumGenerationKey, generation, quoteFilterValue(path))
	return graph.String()
}

// FFmpeg has two escaping layers: graph syntax and option values. Escape
// option punctuation first, then escape that result for the outer graph parser.
func quoteFilterValue(value string) string {
	escape := func(text, special string) string {
		var out strings.Builder
		for _, char := range text {
			if strings.ContainsRune(special, char) {
				out.WriteByte('\\')
			}
			out.WriteRune(char)
		}
		return out.String()
	}
	return escape(escape(value, "\\': \t\r\n"), "\\'[];, \t\r\n")
}

func (p *MPV) offerSpectrum(e Event) {
	select {
	case p.spectrumIn <- e:
	default:
		select {
		case <-p.spectrumIn:
		default:
		}
		select {
		case p.spectrumIn <- e:
		default:
		}
	}
}

type spectrumMeter struct {
	path   string
	file   *os.File
	emit   func(Event)
	stop   chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
	mu     sync.Mutex
	latest SpectrumData
	dirty  bool
}

func newSpectrumMeter(dir string, emit func(Event)) (*spectrumMeter, error) {
	path := filepath.Join(dir, "spectrum.fifo")
	file, err := openSpectrumFIFO(path)
	if err != nil {
		return nil, err
	}
	m := &spectrumMeter{path: path, file: file, emit: emit, stop: make(chan struct{})}
	m.wg.Add(2)
	go m.read()
	go m.publish()
	return m, nil
}

func (m *spectrumMeter) close() {
	m.once.Do(func() {
		close(m.stop)
		_ = m.file.Close()
		m.wg.Wait()
	})
}

func (m *spectrumMeter) read() {
	defer m.wg.Done()
	scanner := bufio.NewScanner(m.file)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	var frame spectrumFrame
	for scanner.Scan() {
		if frame.line(scanner.Text()) {
			m.mu.Lock()
			m.latest = SpectrumData{Bands: append([]float32(nil), frame.bands[:]...), Time: frame.time, Generation: frame.generation}
			m.dirty = true
			m.mu.Unlock()
		}
	}
}

func (m *spectrumMeter) publish() {
	defer m.wg.Done()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			data, dirty := m.latest, m.dirty
			m.dirty = false
			m.mu.Unlock()
			if dirty {
				encoded, _ := json.Marshal(data)
				m.emit(Event{Event: "spectrum", Data: encoded})
			}
		}
	}
}

type spectrumFrame struct {
	bands      [SpectrumBands]float32
	seen       [SpectrumBands]bool
	count      int
	time       float64
	timeKnown  bool
	generation int64
	complete   bool
}

func (f *spectrumFrame) line(line string) bool {
	if strings.HasPrefix(line, "frame:") {
		*f = spectrumFrame{}
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "pts_time:") {
				var err error
				f.time, err = strconv.ParseFloat(strings.TrimPrefix(field, "pts_time:"), 64)
				f.timeKnown = err == nil && !math.IsNaN(f.time) && !math.IsInf(f.time, 0) && f.time >= 0
			}
		}
		return false
	}
	key, value, ok := strings.Cut(line, "=")
	if !ok || f.complete {
		return false
	}
	if key == spectrumGenerationKey {
		generation, err := strconv.ParseInt(value, 10, 64)
		if err != nil || !f.timeKnown || f.count != SpectrumBands {
			return false
		}
		f.generation, f.complete = generation, true
		return true
	}
	if !strings.HasPrefix(key, "lavfi.astats.") || !strings.HasSuffix(key, ".RMS_level") {
		return false
	}
	i, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(key, "lavfi.astats."), ".RMS_level"))
	if err != nil || i < 1 || i > SpectrumBands {
		return false
	}
	level, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(level) {
		return false
	}
	if !f.seen[i-1] {
		f.count++
		f.seen[i-1] = true
	}
	f.bands[i-1] = float32(max(0, min(1, (level+60)/60)))
	return false
}
