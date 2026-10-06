package playback

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Opt in only where a native Pulse server is available. Every audio stream
// targets our own null sink, never the user's speakers or default device.
func TestRealMPG123PulseOutputVolumeAndRestoredSeek(t *testing.T) {
	sink := mpg123PulseNullSink(t)
	path := filepath.Join(t.TempDir(), "variable-rate-48khz.mp3")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=1000:duration=30", "-ar", "48000", "-ac", "2", "-q:a", "2", path).CombinedOutput(); err != nil {
		t.Fatalf("encode VBR fixture: %v: %s", err, out)
	}
	e, err := NewMPG123(EngineOptions{ExtraArgs: []string{"-o", "pulse", "-a", sink}})
	if err != nil {
		t.Fatal(err)
	}
	remoteCleanup(t, e)
	l := remoteEventLog{engine: e}
	remoteApply(t, e, Command{Op: OpVolume, Volume: .4})
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: path, Paused: true, Position: 2375 * time.Millisecond, Epoch: 1})
	// JUMP seeks by MPEG frame; at 48kHz its 24ms frame grid plus the
	// encoder's gapless delay can put SAMPLE up to 48ms before the target.
	l.wait(t, 0, "initial 48kHz VBR seek", func(u Update) bool {
		return u.Kind == UpdateSeeked && u.Generation == 1 && u.Epoch == 1 && math.Abs(u.Position.Seconds()-2.375) < .05
	})
	l.wait(t, 0, "exact VBR duration", func(u Update) bool {
		return u.Kind == UpdateDuration && math.Abs(u.Duration.Seconds()-30) < .02
	})
	quiet := func(label string) {
		t.Helper()
		if rms := mpg123MonitorRMS(t, sink); rms > 1e-7 {
			t.Fatalf("%s emitted audio: RMS=%g", label, rms)
		}
	}
	quiet("paused initial load/seek")
	remoteApply(t, e, Command{Op: OpVolume, Volume: .5})
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	l.wait(t, 0, "resumed playback", func(u Update) bool {
		return u.Kind == UpdatePosition && u.Position > 2700*time.Millisecond
	})
	half := mpg123MonitorRMS(t, sink)
	if half < .015 {
		t.Fatalf("native output remains muted after queued load/seek/resume: RMS=%g", half)
	}
	remoteApply(t, e, Command{Op: OpVolume, Volume: 0})
	quiet("zero volume")
	remoteApply(t, e, Command{Op: OpVolume, Volume: 1})
	full := mpg123MonitorRMS(t, sink)
	if ratio := full / half; ratio < 1.85 || ratio > 2.15 {
		t.Fatalf("native volume did not scale amplitude: full=%g half=%g ratio=%g", full, half, ratio)
	}
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: true})
	quiet("paused playback")
	// Volume changed while paused must survive the next process/load and its
	// initial seek; this follows the shell's saved-state restoration order.
	remoteApply(t, e, Command{Op: OpVolume, Volume: .25})
	mark := len(l.seen)
	remoteApply(t, e, Command{Op: OpLoad, Generation: 2, Path: path, Position: 3110 * time.Millisecond, Epoch: 2})
	l.wait(t, mark, "replacement initial seek", func(u Update) bool {
		return u.Kind == UpdateSeeked && u.Generation == 2 && u.Epoch == 2 && math.Abs(u.Position.Seconds()-3.110) < .05
	})
	l.wait(t, mark, "replacement plays", func(u Update) bool {
		return u.Kind == UpdatePosition && u.Generation == 2 && u.Position > 3400*time.Millisecond
	})
	quarter := mpg123MonitorRMS(t, sink)
	if ratio := full / quarter; ratio < 3.7 || ratio > 4.3 {
		t.Fatalf("saved volume was not restored after replacement/seek: full=%g quarter=%g ratio=%g", full, quarter, ratio)
	}
	t.Logf("48kHz VBR native output RMS: half=%g full=%g restored quarter=%g", half, full, quarter)
}

func TestRealMPG123DefaultOutputWithZeroVolume(t *testing.T) {
	sink := mpg123PulseNullSink(t)
	t.Setenv("PULSE_SINK", sink)
	// ALSA may bypass PULSE_SINK. Hold mpg123's software gain at zero for the
	// entire test so its normal automatic output selection remains inaudible.
	e, err := NewMPG123(EngineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	remoteCleanup(t, e)
	l := remoteEventLog{engine: e}
	remoteApply(t, e, Command{Op: OpVolume, Volume: 0})
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: mp3Fixture(t), Epoch: 1})
	l.wait(t, 0, "automatic output loaded", func(u Update) bool {
		return u.Kind == UpdateLoaded && u.Generation == 1
	})
	u := l.wait(t, 0, "automatic output advances", func(u Update) bool {
		return u.Kind == UpdatePosition && u.Generation == 1 && u.Position > 300*time.Millisecond
	})
	t.Logf("unmodified output selection advances at zero software volume: position=%s", u.Position)
	remoteApply(t, e, Command{Op: OpStop, Generation: 1})
}

func mpg123PulseNullSink(t *testing.T) string {
	t.Helper()
	if os.Getenv("MEDIA_PLAYER_TEST_PULSE") != "1" || runtime.GOOS != "linux" {
		t.Skip("set MEDIA_PLAYER_TEST_PULSE=1 to test an isolated native Pulse null sink")
	}
	for _, executable := range []string{"pactl", "ffmpeg", "mpg123"} {
		if _, err := exec.LookPath(executable); err != nil {
			t.Skipf("%s is required: %v", executable, err)
		}
	}
	sink := fmt.Sprintf("music_player_mpg123_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	out, err := exec.Command("pactl", "load-module", "module-null-sink", "sink_name="+sink, "rate=48000", "channels=2").CombinedOutput()
	if err != nil {
		t.Skipf("cannot create an isolated Pulse null sink: %v: %s", err, out)
	}
	module := strings.TrimSpace(string(out))
	t.Cleanup(func() {
		if out, err := exec.Command("pactl", "unload-module", module).CombinedOutput(); err != nil {
			t.Errorf("remove owned null sink: %v: %s", err, out)
		}
	})
	return sink
}

func mpg123MonitorRMS(t *testing.T, sink string) float64 {
	t.Helper()
	// Let the finite output buffer drain before recording the current state.
	time.Sleep(250 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "pulse", "-fragment_size", "1920", "-i", sink+".monitor", "-t", "0.3", "-ar", "48000", "-ac", "2", "-f", "f32le", "-")
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	data, err := command.Output()
	if err != nil {
		t.Fatalf("record owned null-sink monitor: %v: %s", err, &diagnostics)
	}
	if len(data) < 48000 {
		t.Fatalf("too few captured samples: %d bytes", len(data))
	}
	var squares float64
	for i := 0; i+4 <= len(data); i += 4 {
		value := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i : i+4])))
		squares += value * value
	}
	return math.Sqrt(squares / float64(len(data)/4))
}
