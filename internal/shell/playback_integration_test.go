package shell

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/style"
)

func TestRealPlaybackControlsAndAppearanceChanges(t *testing.T) {
	a, s, engine := integrationPlayer(t)
	m, host := s.Host.Model, s.Host
	tr := m.Transport
	if !m.Live || tr.List.Len() != 0 || tr.State != players.Stopped {
		t.Fatal("an injected real backend did not start with an empty live player")
	}
	// This test changes every appearance while audio continues to play.
	// Leave enough audio for slower painting without changing wait deadlines.
	first := integrationWAV(t, "first track.wav", 60)
	second := integrationWAV(t, "second track.wav", 12)
	if err := s.LoadFiles([]string{first}, false); err != nil {
		t.Fatal(err)
	}
	integrationWait(t, a, s, "duration, audio metadata and real playback position", func() bool {
		return !s.playback.loading && tr.Track().Length == 60*time.Second &&
			tr.Track().Rate == 44 && tr.Track().Channels == 2 && tr.Pos > 100*time.Millisecond
	})
	if tr.Track().Path != first || tr.State != players.Playing {
		t.Fatal("opening a file did not start the requested track")
	}
	livePosition := tr.Pos
	s.tick(time.Second)
	if tr.Pos != livePosition {
		t.Fatal("the UI pulse advanced the live clock independently of mpv")
	}
	for _, values := range [][]float32{m.Spectrum.Bars, m.Spectrum.Peaks} {
		for _, value := range values {
			if value != 0 {
				t.Fatal("silent audio produced nonzero spectrum levels")
			}
		}
	}

	generation := s.playback.generation
	position := tr.Pos
	host.Play()
	host.Play()
	if s.playback.generation != generation || tr.State != players.Playing || tr.Pos != position {
		t.Fatal("pressing Play while playing paused or reloaded the current file")
	}
	if raw := integrationProperty(t, a, s, engine, "pause"); string(raw) != "false" {
		t.Fatalf("Play left mpv paused: %s", raw)
	}
	host.Command(players.CmdPlayPause)
	if raw := integrationProperty(t, a, s, engine, "pause"); string(raw) != "true" || tr.State != players.Paused {
		t.Fatalf("Pause did not pause mpv and the model: %s, %s", raw, tr.State)
	}
	before := integrationNumber(t, integrationProperty(t, a, s, engine, "time-pos"))
	integrationPumpFor(a, s, 100*time.Millisecond)
	if after := integrationNumber(t, integrationProperty(t, a, s, engine, "time-pos")); math.Abs(after-before) > .02 {
		t.Fatalf("the paused backend moved from %.3f to %.3f", before, after)
	}
	seekMark := len(engine.snapshot())
	host.Seek(.05) // Three seconds into the one-minute fixture.
	integrationWait(t, a, s, "the paused seek reaching its requested target", func() bool {
		for _, event := range engine.snapshot()[seekMark:] {
			var position float64
			if event.Event == "property-change" && event.Name == "time-pos" &&
				json.Unmarshal(event.Data, &position) == nil && math.Abs(position-3) < .03 {
				return !s.playback.seeking
			}
		}
		return false
	})
	// ao=null can report the paused audio clock one buffered block behind
	// its exact seek-target observation. Verify that clock too, then require
	// resumed playback to advance beyond the target below.
	if got := integrationNumber(t, integrationProperty(t, a, s, engine, "time-pos")); math.Abs(got-3) > .25 {
		t.Fatalf("the backend's paused seek clock is %.3f, want near 3 seconds", got)
	}
	host.SetVolume(.37)
	if got := integrationNumber(t, integrationProperty(t, a, s, engine, "volume")); math.Abs(got-37) > .01 {
		t.Fatalf("the backend volume is %.3f, want 37", got)
	}
	host.SetPan(.35)
	m.Equaliser.SetOn(true)
	m.Equaliser.SetPreamp(-2)
	m.Equaliser.Set(3, 4)
	raw := integrationProperty(t, a, s, engine, "af")
	if !strings.Contains(string(raw), "lavfi") || !strings.Contains(string(raw), "pan=stereo") ||
		!strings.Contains(string(raw), "volume=-2.0000dB") || !strings.Contains(string(raw), "equalizer=f=250") {
		t.Fatalf("mpv did not retain the requested balance/preamp/EQ filter graph: %s", raw)
	}
	host.Play()
	integrationWait(t, a, s, "playback resuming after seek and DSP changes", func() bool { return tr.Pos > 3100*time.Millisecond })
	if tr.State != players.Playing || m.Error != "" {
		t.Fatalf("the backend rejected playback or DSP: %s", m.Error)
	}

	list := tr.List
	for _, skin := range players.AppearanceIDs() {
		at := tr.Pos
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		integrationWait(t, a, s, "clock continuing with "+skin, func() bool { return tr.Pos > at+20*time.Millisecond })
		if s.Host.Model != m || tr.List != list || s.playback.generation != generation ||
			tr.Track().Path != first || tr.State != players.Playing || tr.Volume != .37 || tr.Pan != .35 ||
			m.Equaliser.Preamp != -2 || m.Equaliser.Gains[3] != 4 {
			t.Fatalf("%s replaced or reset live playback state", skin)
		}
	}

	host.Command(players.CmdPlayPause)
	if raw := integrationProperty(t, a, s, engine, "pause"); string(raw) != "true" {
		t.Fatalf("could not pause before modifying the queue: %s", raw)
	}
	at := tr.Pos
	if err := s.LoadFiles([]string{second}, true); err != nil {
		t.Fatal(err)
	}
	if tr.List != list || tr.List.Len() != 2 || tr.List.Index() != 0 || tr.Pos != at || tr.State != players.Paused {
		t.Fatal("appending a file replaced the queue or changed paused playback")
	}
	host.Command(players.CmdNext)
	integrationWait(t, a, s, "Next loading the appended track", func() bool {
		return tr.List.Index() == 1 && !s.playback.loading && tr.Track().Length == 12*time.Second && tr.Track().Rate == 44
	})
	integrationAssertPath(t, a, s, engine, second)
	host.Command(players.CmdPrev)
	integrationWait(t, a, s, "Previous returning to the first track", func() bool {
		return tr.List.Index() == 0 && !s.playback.loading
	})
	integrationAssertPath(t, a, s, engine, first)
	host.RemoveTrack()
	integrationWait(t, a, s, "removing the current track and loading its replacement", func() bool {
		return tr.List.Len() == 1 && tr.Track().Path == second && !s.playback.loading
	})
	integrationAssertPath(t, a, s, engine, second)
	host.RemoveTrack()
	if tr.List.Len() != 0 || tr.State != players.Stopped || tr.Pos != 0 {
		t.Fatal("removing the last track did not empty and stop the live player")
	}
	if raw := integrationProperty(t, a, s, engine, "idle-active"); string(raw) != "true" {
		t.Fatalf("the empty player left mpv playing: %s", raw)
	}

	windows := s.Windows()
	s.Close()
	a.DrainPosted()
	if s.Pulse().Running() || !errors.Is(engine.Send("get_property", "pause"), playback.ErrClosed) {
		t.Fatal("closing the shell left its timer or backend active")
	}
	for _, window := range windows {
		if !window.Window.Closed() {
			t.Fatal("closing live playback left a presentation window open")
		}
	}
}

func TestRealSpectrumFollowsAudioAcrossSkinsSeeksAndSilence(t *testing.T) {
	a, s, _ := integrationPlayer(t)
	low := integrationTone(t, "1000 Hz.wav", 12, 1000)
	high := integrationTone(t, "8000 Hz.wav", 4, 8000)
	silent := integrationWAV(t, "silence.wav", 4)
	peak := func() (int, float32) {
		index, value := 0, float32(0)
		for i, band := range s.Host.Model.Spectrum.Bars {
			if band > value {
				index, value = i, band
			}
		}
		return index, value
	}
	if err := s.LoadFiles([]string{low}, false); err != nil {
		t.Fatal(err)
	}
	integrationWait(t, a, s, "a measured 1000 Hz peak", func() bool {
		i, value := peak()
		return i >= 24 && i <= 28 && value > .4
	})
	model := s.Host.Model
	for _, skin := range players.AppearanceIDs() {
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		integrationWait(t, a, s, "measured spectrum in "+skin, func() bool {
			_, value := peak()
			return s.Host.Model == model && value > .4
		})
	}
	s.Host.Command(players.CmdPlayPause)
	integrationPumpFor(a, s, 100*time.Millisecond)
	s.tick(3 * time.Second)
	if _, value := peak(); value != 0 {
		t.Fatal("paused audio kept the visualizer active")
	}
	s.Host.Seek(.5)
	s.Host.Play()
	integrationWait(t, a, s, "spectrum resuming after a paused seek", func() bool {
		_, value := peak()
		return !s.playback.seeking && s.Host.Model.Transport.Pos >= 5900*time.Millisecond && value > .4
	})
	if err := s.LoadFiles([]string{high}, false); err != nil {
		t.Fatal(err)
	}
	integrationWait(t, a, s, "the peak moving to 8000 Hz", func() bool {
		i, value := peak()
		return i >= 40 && i <= 44 && value > .4
	})
	if err := s.LoadFiles([]string{silent}, false); err != nil {
		t.Fatal(err)
	}
	integrationWait(t, a, s, "silence replacing the previous spectrum", func() bool {
		_, value := peak()
		return !s.playback.loading && s.Host.Model.Transport.Pos > 100*time.Millisecond && value == 0
	})
}

func TestRealPlaybackEOFRepeatModes(t *testing.T) {
	for _, repeat := range []players.Repeat{players.RepeatOff, players.RepeatAll, players.RepeatOne} {
		t.Run(repeat.String(), func(t *testing.T) {
			a, s, _ := integrationPlayer(t)
			tr := s.Host.Model.Transport
			for tr.Repeat != repeat {
				s.Host.Command(players.CmdRepeat)
			}
			first := integrationWAV(t, "short first.wav", 1)
			second := integrationWAV(t, "short second.wav", 1.2)
			if err := s.LoadFiles([]string{first, second}, false); err != nil {
				t.Fatal(err)
			}
			switch repeat {
			case players.RepeatOff:
				integrationWait(t, a, s, "natural EOF advancing to the second track", func() bool {
					return tr.List.Index() == 1 && tr.Track().Length == 1200*time.Millisecond
				})
				integrationWait(t, a, s, "the final EOF stopping Repeat Off", func() bool {
					return tr.State == players.Stopped && s.playback.path == ""
				})
				if tr.Pos != 0 {
					t.Fatal("the stopped EOF position was not reset")
				}
			case players.RepeatAll:
				integrationWait(t, a, s, "Repeat All wrapping after the second track", func() bool {
					return s.playback.generation >= 3 && tr.List.Index() == 0 && tr.State == players.Playing &&
						!s.playback.loading && tr.Pos > 0
				})
			case players.RepeatOne:
				integrationWait(t, a, s, "Repeat One reloading the same file twice", func() bool {
					return s.playback.generation >= 3 && tr.List.Index() == 0 && tr.State == players.Playing &&
						!s.playback.loading && tr.Pos > 0
				})
				if tr.List.At(1).Length != 0 {
					t.Fatal("Repeat One played the second file")
				}
			}
		})
	}
}

// Record without competing for the shell's event stream: this is the only
// reader of real mpv events and forwards them to the controller in wire order.
type integrationMPV struct {
	*playback.MPV
	mu      sync.Mutex
	seen    []playback.Event
	events  chan playback.Event
	stop    chan struct{}
	drained chan struct{}
	once    sync.Once
	err     error
}

func (p *integrationMPV) Events() <-chan playback.Event { return p.events }

func (p *integrationMPV) Close() error {
	p.once.Do(func() {
		close(p.stop)
		p.err = p.MPV.Close()
		<-p.drained
	})
	return p.err
}

func (p *integrationMPV) snapshot() []playback.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]playback.Event(nil), p.seen...)
}

func integrationPlayer(t *testing.T) (*uitoolkit.Application, *Shell, *integrationMPV) {
	t.Helper()
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("real mpv integration requires the Unix IPC backend")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("system mpv is not installed; presentation tests remain available")
	}
	mpv, err := playback.New(playback.Options{ExtraArgs: []string{"--ao=null"}})
	if err != nil {
		t.Fatal(err)
	}
	engine := &integrationMPV{
		MPV: mpv, events: make(chan playback.Event, 64),
		stop: make(chan struct{}), drained: make(chan struct{}),
	}
	go func() {
		defer close(engine.drained)
		defer close(engine.events)
		for event := range mpv.Events() {
			engine.mu.Lock()
			engine.seen = append(engine.seen, event)
			engine.mu.Unlock()
			select {
			case engine.events <- event:
			case <-engine.stop:
			}
		}
	}()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(players.DefaultSkin))
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: 1, DisableLookWatch: true})
	s, err := New(a, Options{Skin: players.DefaultSkin, Headless: true, Scale: 1, Backend: engine})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	return a, s, engine
}

func integrationWait(t *testing.T, a *uitoolkit.Application, s *Shell, description string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for {
		s.Pulse().Stop()
		a.DrainPosted()
		s.Pulse().Stop()
		if message := s.Host.Model.Error; message != "" {
			t.Fatalf("waiting for %s: %s", description, message)
		}
		if ready() {
			return
		}
		if time.Now().After(deadline) {
			tr := s.Host.Model.Transport
			t.Fatalf("timed out waiting for %s: state=%s index=%d pos=%s duration=%s loading=%t generation=%d", description,
				tr.State, tr.List.Index(), tr.Pos, tr.Length(), s.playback.loading, s.playback.generation)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func integrationPumpFor(a *uitoolkit.Application, s *Shell, duration time.Duration) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		s.Pulse().Stop()
		a.DrainPosted()
		time.Sleep(5 * time.Millisecond)
	}
}

func integrationProperty(t *testing.T, a *uitoolkit.Application, s *Shell, engine *integrationMPV, name string) json.RawMessage {
	t.Helper()
	mark := len(engine.snapshot())
	if err := engine.Send("get_property", name); err != nil {
		t.Fatal(err)
	}
	var result playback.Event
	integrationWait(t, a, s, "mpv property "+name, func() bool {
		for _, event := range engine.snapshot()[mark:] {
			var command []string
			if event.Name == "get_property" && json.Unmarshal(event.Command, &command) == nil && len(command) == 2 && command[1] == name {
				result = event
				return true
			}
		}
		return false
	})
	if result.Event != "command-reply" || string(result.Data) == "null" {
		t.Fatalf("mpv property %s failed: %+v", name, result)
	}
	return result.Data
}

func integrationNumber(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		t.Fatalf("mpv numeric property %q: %v", raw, err)
	}
	return number
}

func integrationAssertPath(t *testing.T, a *uitoolkit.Application, s *Shell, engine *integrationMPV, want string) {
	t.Helper()
	var path string
	if raw := integrationProperty(t, a, s, engine, "path"); json.Unmarshal(raw, &path) != nil || path != want {
		t.Fatalf("mpv loaded %q, want %q", path, want)
	}
}

// Silent stereo PCM needs no external fixture generator and no audible output.
func integrationWAV(t *testing.T, name string, seconds float64) string {
	t.Helper()
	const rate, channels, bits = 44100, 2, 16
	size := int(seconds*rate) * channels * bits / 8
	data := make([]byte, 44+size)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+size))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], channels)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*channels*bits/8)
	binary.LittleEndian.PutUint16(data[32:], channels*bits/8)
	binary.LittleEndian.PutUint16(data[34:], bits)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(size))
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func integrationTone(t *testing.T, name string, seconds, frequency float64) string {
	t.Helper()
	path := integrationWAV(t, name, seconds)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for offset := 44; offset+4 <= len(data); offset += 4 {
		frame := (offset - 44) / 4
		sample := int16(12000 * math.Sin(2*math.Pi*frequency*float64(frame)/44100))
		binary.LittleEndian.PutUint16(data[offset:], uint16(sample))
		binary.LittleEndian.PutUint16(data[offset+2:], uint16(sample))
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
