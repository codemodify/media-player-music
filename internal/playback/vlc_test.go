package playback

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func silentVLC(t *testing.T) *VLC {
	return testVLC(t, []string{"--aout=dummy"})
}

func testVLC(t *testing.T, args []string) *VLC {
	t.Helper()
	if _, err := exec.LookPath("vlc"); err != nil {
		t.Skip("installed VLC executable is unavailable")
	}
	engine, err := NewVLC(EngineOptions{ExtraArgs: args})
	if err != nil {
		t.Fatal(err)
	}
	p := engine.(*VLC)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		for range p.Events() {
		}
	})
	return p
}

func TestVLCInstalledVolumeWithSilentALSAOutput(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("silent ALSA null output is Linux-specific")
	}
	if _, err := exec.LookPath("vlc"); err != nil {
		t.Skip("installed VLC executable is unavailable")
	}
	modules, err := exec.Command("vlc", "--list").Output()
	if err != nil || !strings.Contains(string(modules), "alsa") {
		t.Skip("installed VLC has no ALSA output module")
	}
	// ALSA's built-in null device discards audio, while VLC's ALSA module
	// exposes actual software volume (unlike its dummy output module).
	p := testVLC(t, []string{"--aout=alsa", "--alsa-audio-device=null"})
	vlcApply(t, p, Command{Op: OpVolume, Volume: .45})
	vlcApply(t, p, Command{Op: OpLoad, Generation: 15, Path: wav(t, "volume.wav", 5), Paused: true})
	vlcUpdate(t, p, 5*time.Second, func(u Update) bool { return u.Kind == UpdateLoaded })
	var snapshot vlcStatus
	if err := p.request("status.json", nil, &snapshot); err != nil || snapshot.Volume != 115 {
		t.Fatalf("pre-load volume was not reapplied to actual VLC output: %d, %v", snapshot.Volume, err)
	}
	vlcApply(t, p, Command{Op: OpVolume, Volume: .8})
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := p.request("status.json", nil, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Volume == 205 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("actual VLC output volume did not change: %d", snapshot.Volume)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func vlcApply(t *testing.T, p *VLC, c Command) {
	t.Helper()
	if err := p.Apply(c); err != nil {
		t.Fatal(err)
	}
}

func vlcUpdate(t *testing.T, p *VLC, timeout time.Duration, predicate func(Update) bool) Update {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case update, ok := <-p.Events():
			if !ok {
				t.Fatal("VLC events closed before expected update")
			}
			if update.Kind == UpdateError {
				t.Fatalf("VLC error: %s", update.Error)
			}
			if update.Kind == UpdateSpectrum {
				t.Fatal("VLC must not fabricate spectrum measurements")
			}
			if predicate(update) {
				return update
			}
		case <-timer.C:
			t.Fatal("timed out waiting for VLC update")
		}
	}
}

func vlcNoEnd(t *testing.T, p *VLC, duration time.Duration) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case update := <-p.Events():
			if update.Kind == UpdateEnded || update.Kind == UpdateError || update.Kind == UpdateSpectrum {
				t.Fatalf("unexpected VLC update: %+v", update)
			}
		case <-timer.C:
			return
		}
	}
}

func vlcWaitState(t *testing.T, p *VLC, want string) vlcStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var snapshot vlcStatus
		if err := p.request("status.json", nil, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.State == want {
			return snapshot
		}
		if time.Now().After(deadline) {
			t.Fatalf("VLC state stayed %q, want %q", snapshot.State, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestVLCInstalledTransportAndInitialSeek(t *testing.T) {
	p := silentVLC(t)
	// The fresh loopback control endpoint must not accept unauthenticated
	// requests, even though it belongs only to this child process.
	res, err := p.client.Get(p.endpoint + "status.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("VLC control endpoint accepted missing authentication: %s", res.Status)
	}
	path := wav(t, "unicode 音樂 # ? & + track.wav", 8)
	vlcApply(t, p, Command{Op: OpLoad, Generation: 11, Path: path, Position: 2 * time.Second, Paused: true, Epoch: 4})
	loaded := vlcUpdate(t, p, 5*time.Second, func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 11 })
	if delta := loaded.Position - 2*time.Second; delta < -150*time.Millisecond || delta > 150*time.Millisecond {
		t.Fatalf("initial position was not applied before Loaded: %+v", loaded)
	}
	if loaded.Duration != 8*time.Second || loaded.Info.SampleRate != 44100 || loaded.Info.Bitrate != 1411000 {
		t.Fatalf("actual VLC duration/format was not decoded: %+v", loaded)
	}
	vlcNoEnd(t, p, 350*time.Millisecond)
	var snapshot vlcStatus
	if err := p.request("status.json", nil, &snapshot); err != nil || snapshot.State != "paused" {
		t.Fatalf("initial input is not paused: %+v, %v", snapshot, err)
	}
	vlcApply(t, p, Command{Op: OpVolume, Volume: .45})
	vlcApply(t, p, Command{Op: OpPause, Generation: 11, Paused: false})
	vlcUpdate(t, p, 3*time.Second, func(u Update) bool {
		return u.Kind == UpdatePosition && u.Epoch == 4 && u.Position > 2200*time.Millisecond
	})
	// VLC's dummy audio module deliberately has no volume_set callback;
	// command delivery is checked below without claiming silent-output gain.
	vlcApply(t, p, Command{Op: OpPause, Generation: 11, Paused: true})
	vlcWaitState(t, p, "paused")
	vlcNoEnd(t, p, 300*time.Millisecond)
	if err := p.request("status.json", nil, &snapshot); err != nil || snapshot.State != "paused" {
		t.Fatalf("explicit pause failed: %+v, %v", snapshot, err)
	}
	vlcApply(t, p, Command{Op: OpSeek, Generation: 11, Position: 6 * time.Second, Epoch: 5})
	seeked := vlcUpdate(t, p, 3*time.Second, func(u Update) bool { return u.Kind == UpdateSeeked && u.Epoch == 5 })
	if delta := seeked.Position - 6*time.Second; delta < -150*time.Millisecond || delta > 150*time.Millisecond {
		t.Fatalf("seek acknowledgement used the old clock: %+v", seeked)
	}
	position := vlcUpdate(t, p, time.Second, func(u Update) bool { return u.Kind == UpdatePosition && u.Epoch == 5 })
	if position.Position < 5850*time.Millisecond {
		t.Fatalf("post-seek position reverted: %+v", position)
	}
	vlcApply(t, p, Command{Op: OpSeek, Generation: 10, Position: time.Second, Epoch: 6})
	vlcApply(t, p, Command{Op: OpStop}) // global stop after a rejected new load
	vlcWaitState(t, p, "stopped")
	vlcNoEnd(t, p, 350*time.Millisecond)
	if err := p.request("status.json", nil, &snapshot); err != nil || snapshot.State != "stopped" {
		t.Fatalf("explicit stop failed: %+v, %v", snapshot, err)
	}
	if caps := p.Capabilities(); !caps.Seek || caps.Equalizer || caps.Balance || caps.Spectrum {
		t.Fatalf("VLC advertised unsupported features: %+v", caps)
	}
}

func TestVLCInstalledNaturalEndAndReplacementIsolation(t *testing.T) {
	p := silentVLC(t)
	long := wav(t, "old.wav", 8)
	short := wav(t, "new.wav", 2)
	vlcApply(t, p, Command{Op: OpLoad, Generation: 20, Path: long})
	vlcUpdate(t, p, 5*time.Second, func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 20 })
	// An already active input must not emit a replacement EOF. The two
	// queued loads also exercise independent playlist-item identities.
	vlcApply(t, p, Command{Op: OpLoad, Generation: 21, Path: long})
	vlcApply(t, p, Command{Op: OpLoad, Generation: 22, Path: short})
	vlcUpdate(t, p, 5*time.Second, func(u Update) bool {
		if u.Kind == UpdateEnded && u.Generation != 22 {
			t.Fatalf("replacement emitted a false EOF: %+v", u)
		}
		return u.Kind == UpdateLoaded && u.Generation == 22
	})
	vlcUpdate(t, p, 5*time.Second, func(u Update) bool {
		if u.Generation != 22 {
			t.Fatalf("old input event leaked after latest Loaded: %+v", u)
		}
		return u.Kind == UpdateEnded
	})
	vlcNoEnd(t, p, 250*time.Millisecond) // completion is emitted exactly once
}

func TestVLCInstalledInvalidMediaAndLifecycle(t *testing.T) {
	p := silentVLC(t)
	path := filepath.Join(t.TempDir(), "broken.wav")
	if err := os.WriteFile(path, []byte("this is not a media file"), 0600); err != nil {
		t.Fatal(err)
	}
	vlcApply(t, p, Command{Op: OpLoad, Generation: 31, Path: path})
	timer := time.NewTimer(7 * time.Second)
	defer timer.Stop()
	for {
		select {
		case update := <-p.Events():
			if update.Kind == UpdateLoaded || update.Kind == UpdateEnded {
				t.Fatalf("broken media was treated as successful playback: %+v", update)
			}
			if update.Kind == UpdateError {
				if update.Generation != 31 || update.Error == "" || strings.Contains(update.Error, p.password) {
					t.Fatalf("invalid-media error lacks identity or leaks authentication: %+v", update)
				}
				if err := p.Close(); err != nil {
					t.Fatal(err)
				}
				if err := p.Apply(Command{Op: OpPause}); !errors.Is(err, ErrClosed) {
					t.Fatalf("Apply after Close = %v", err)
				}
				select {
				case <-p.processDone:
				default:
					t.Fatal("Close left VLC process running")
				}
				return
			}
		case <-timer.C:
			t.Fatal("invalid media did not produce an error")
		}
	}
}

func TestVLCStatusMetadataAndCapabilityValidation(t *testing.T) {
	var status vlcStatus
	if err := json.Unmarshal([]byte(`{"information":{"category":{"meta":{"title":"Song","artist":"Artist","album":"Album"},"Stream 0":{"Type":"Audio","Sample_rate":"48000 Hz","Bitrate":"192 kb/s","Channels":"Stereo"}}}}`), &status); err != nil {
		t.Fatal(err)
	}
	if got := vlcTrackInfo(status); got != (TrackInfo{Title: "Song", Artist: "Artist", Album: "Album", SampleRate: 48000, Channels: 2, Bitrate: 192000}) {
		t.Fatalf("metadata = %+v", got)
	}
	delete(status.Information.Category["Stream 0"], "Channels")
	status.Information.Category["Stream 0"]["Decoded_channels"] = "Stereo"
	if got := vlcTrackInfo(status); got.Channels != 2 {
		t.Fatalf("decoded output channels = %d", got.Channels)
	}
	p := &VLC{}
	if err := p.Apply(Command{Op: OpDSP}); err == nil {
		t.Fatal("VLC silently accepted unsupported exact DSP")
	}
	if err := p.Apply(Command{Op: OpVolume, Volume: float32(math.NaN())}); err == nil {
		t.Fatal("VLC accepted NaN volume")
	}
}

func TestVLCFileURIEscapesNativePaths(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/music # ? & +.wav": "file:///tmp/music%20%23%20%3F%20&%20+.wav",
		"C:/Music/a # b.wav":     "file:///C:/Music/a%20%23%20b.wav",
		"//server/share/a.wav":   "file://server/share/a.wav",
	} {
		if got := vlcFileURI(path); got != want {
			t.Errorf("file URI for %q = %q, want %q", path, got, want)
		}
	}
}

func TestVLCVolumeUsesAuthenticatedHTTPCommand(t *testing.T) {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "" || password != "test-secret" {
			t.Error("VLC request omitted authentication")
		}
		if r.URL.Path != "/status.json" || r.URL.Query().Get("command") != "volume" || r.URL.Query().Get("val") != "115" {
			t.Errorf("wrong VLC volume protocol: %s", r.URL.String())
		}
		called.Store(true)
		_, _ = w.Write([]byte(`{"version":"3.0.23","volume":115}`))
	}))
	defer server.Close()
	p := &VLC{ctx: context.Background(), client: server.Client(), endpoint: server.URL + "/", password: "test-secret"}
	if err := p.setVolume(.45); err != nil {
		t.Fatal(err)
	}
	if !called.Load() {
		t.Fatal("VLC volume command was not sent")
	}
}

type vlcVolumeDelay struct {
	base    http.RoundTripper
	enabled atomic.Bool
	entered chan struct{}
	release chan struct{}
	final   chan struct{}
	mu      sync.Mutex
	values  []string
}

func (d *vlcVolumeDelay) RoundTrip(r *http.Request) (*http.Response, error) {
	if !d.enabled.Load() || r.URL.Query().Get("command") != "volume" {
		return d.base.RoundTrip(r)
	}
	value := r.URL.Query().Get("val")
	d.mu.Lock()
	first := len(d.values) == 0
	d.values = append(d.values, value)
	d.mu.Unlock()
	if first {
		close(d.entered)
		select {
		case <-d.release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	response, err := d.base.RoundTrip(r)
	if err == nil && value == "0" {
		select {
		case d.final <- struct{}{}:
		default:
		}
	}
	return response, err
}

func TestVLCVolumeFloodKeepsLatestMuteAndTransportResponsive(t *testing.T) {
	p := silentVLC(t)
	// Install interception before the idle worker can issue any requests;
	// subsequent toggling is atomic, leaving the HTTP client's fields fixed.
	delay := &vlcVolumeDelay{base: p.client.Transport, entered: make(chan struct{}), release: make(chan struct{}), final: make(chan struct{}, 1)}
	p.client.Transport = delay
	vlcApply(t, p, Command{Op: OpLoad, Generation: 50, Path: wav(t, "backpressure.wav", 12), Paused: true})
	vlcUpdate(t, p, 5*time.Second, func(u Update) bool { return u.Kind == UpdateLoaded })
	delay.enabled.Store(true)
	vlcApply(t, p, Command{Op: OpVolume, Volume: .5})
	select {
	case <-delay.entered:
	case <-time.After(time.Second):
		t.Fatal("volume request did not reach controlled HTTP backpressure")
	}
	for i := 0; i < 2000; i++ {
		if err := p.Apply(Command{Op: OpVolume, Volume: float32(i%101) / 100}); err != nil {
			t.Fatalf("slider flood filled the command queue: %v", err)
		}
	}
	vlcApply(t, p, Command{Op: OpVolume, Volume: 0})
	vlcApply(t, p, Command{Op: OpStop})
	started := time.Now()
	close(delay.release)
	select {
	case <-delay.final:
	case <-time.After(time.Second):
		t.Fatal("latest mute was left behind stale volume requests")
	}
	vlcWaitState(t, p, "stopped")
	if time.Since(started) > time.Second {
		t.Fatal("transport command was held behind the slider flood")
	}
	delay.mu.Lock()
	values := append([]string(nil), delay.values...)
	delay.mu.Unlock()
	if len(values) != 2 || values[0] != "128" || values[1] != "0" {
		t.Fatalf("volume flood replayed stale requests instead of latest mute: %v", values)
	}
	vlcNoEnd(t, p, 100*time.Millisecond)
}

type vlcPulseInput struct {
	Index      int               `json:"index"`
	Sink       int               `json:"sink"`
	Properties map[string]string `json:"properties"`
	Volume     map[string]struct {
		Value int `json:"value"`
	} `json:"volume"`
}

// This exercises the default Linux output path through a temporary null sink,
// preserving every other sink/input and unloading only our own module.
func TestVLCInstalledPulseVolumeMatchesPerceptualGain(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native PulseAudio/PipeWire validation is Linux-specific")
	}
	if _, err := exec.LookPath("vlc"); err != nil {
		t.Skip("installed VLC is unavailable")
	}
	pactl, err := exec.LookPath("pactl")
	if err != nil {
		t.Skip("pactl is unavailable")
	}
	if err := exec.Command(pactl, "info").Run(); err != nil {
		t.Skip("PulseAudio/PipeWire server is unavailable in this test environment")
	}
	name := "music_player_vlc_test_" + strconv.Itoa(os.Getpid()) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	module, err := exec.Command(pactl, "load-module", "module-null-sink", "sink_name="+name, "sink_properties=device.description="+name).Output()
	if err != nil {
		t.Skip("PulseAudio/PipeWire cannot provide a private null sink")
	}
	t.Cleanup(func() {
		if data, err := exec.Command(pactl, "unload-module", strings.TrimSpace(string(module))).CombinedOutput(); err != nil {
			t.Errorf("remove private null sink: %v: %s", err, data)
		}
	})
	t.Setenv("PULSE_SINK", name)
	p := testVLC(t, []string{"--aout=pulse"})
	vlcApply(t, p, Command{Op: OpLoad, Generation: 60, Path: wav(t, "private-volume.wav", 30)})
	vlcUpdate(t, p, 5*time.Second, func(u Update) bool { return u.Kind == UpdateLoaded })
	var sinkID int
	var sinks []struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
	}
	data, err := exec.Command(pactl, "-f", "json", "list", "sinks").Output()
	if err != nil || json.Unmarshal(data, &sinks) != nil {
		t.Fatalf("read private sink: %v", err)
	}
	found := false
	for _, sink := range sinks {
		if sink.Name == name {
			sinkID, found = sink.Index, true
		}
	}
	if !found {
		t.Fatal("private null sink did not appear")
	}
	input := func() (vlcPulseInput, bool) {
		data, err := exec.Command(pactl, "-f", "json", "list", "sink-inputs").Output()
		var inputs []vlcPulseInput
		if err != nil || json.Unmarshal(data, &inputs) != nil {
			t.Fatalf("read private stream: %v", err)
		}
		for _, row := range inputs {
			if row.Properties["application.process.id"] == strconv.Itoa(p.cmd.Process.Pid) {
				if row.Sink != sinkID {
					t.Fatal("owned VLC was not routed to the private null sink")
				}
				return row, true
			}
		}
		return vlcPulseInput{}, false
	}
	for _, volume := range []float32{.05, .25, .5, .75, 1, 0, .5, 1} {
		vlcApply(t, p, Command{Op: OpVolume, Volume: volume})
		deadline := time.Now().Add(time.Second)
		for {
			row, ok := input()
			matches := ok && len(row.Volume) == 2
			wantGain := math.Pow(float64(volume), 3)
			for _, channel := range row.Volume {
				gain := math.Pow(float64(channel.Value)/65536, 3)
				if volume == 0 {
					matches = matches && channel.Value == 0
				} else if volume == 1 {
					matches = matches && channel.Value == 65536
				} else {
					matches = matches && math.Abs(gain-wantGain) < wantGain*.08
				}
			}
			if matches {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("slider %.2f did not reach expected actual Pulse gain: %+v", volume, row.Volume)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	// An external mixer can change our own stream. The next slider update
	// remains absolute and recovers the requested gain rather than accumulating.
	row, _ := input()
	if err := exec.Command(pactl, "set-sink-input-volume", strconv.Itoa(row.Index), "10%").Run(); err != nil {
		t.Fatal(err)
	}
	vlcApply(t, p, Command{Op: OpVolume, Volume: .5})
	deadline := time.Now().Add(time.Second)
	for {
		row, _ = input()
		matched := len(row.Volume) == 2
		for _, channel := range row.Volume {
			matched = matched && math.Abs(math.Pow(float64(channel.Value)/65536, 3)-.125) < .002
		}
		if matched {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("absolute slider update did not recover after owned-stream mixer change: %+v", row.Volume)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
