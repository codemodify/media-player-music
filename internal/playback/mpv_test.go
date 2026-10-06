package playback

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
)

func realMPV(t *testing.T) *MPV {
	t.Helper()
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("Unix IPC backend")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("system mpv is not installed")
	}
	p, err := New(Options{ExtraArgs: []string{"--ao=null"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		for range p.Events() {
		}
	})
	return p
}

type eventLog struct {
	p    *MPV
	seen []Event
}

func (l *eventLog) wait(t *testing.T, from int, description string, matches func(Event) bool) Event {
	t.Helper()
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	for {
		for _, e := range l.seen[from:] {
			if matches(e) {
				return e
			}
		}
		from = len(l.seen)
		select {
		case e, ok := <-l.p.Events():
			if !ok {
				t.Fatalf("events closed while waiting for %s; seen %v", description, l.seen)
			}
			l.seen = append(l.seen, e)
			if e.Event == "backend-error" {
				t.Fatalf("waiting for %s: %s", description, e.Error)
			}
		case <-deadline.C:
			start := max(0, len(l.seen)-15)
			t.Fatalf("timed out waiting for %s; recent events %v", description, l.seen[start:])
		}
	}
}

func send(t *testing.T, p *MPV, command ...any) {
	t.Helper()
	if err := p.Send(command...); err != nil {
		t.Fatal(err)
	}
}

func property(name string, matches func(json.RawMessage) bool) func(Event) bool {
	return func(e Event) bool { return e.Event == "property-change" && e.Name == name && matches(e.Data) }
}

func number(raw json.RawMessage) float64 {
	var value float64
	if json.Unmarshal(raw, &value) != nil || string(raw) == "null" {
		return math.NaN()
	}
	return value
}

func (l *eventLog) getNumber(t *testing.T, name string) float64 {
	t.Helper()
	mark := len(l.seen)
	send(t, l.p, "get_property", name)
	e := l.wait(t, mark, "get_property "+name, func(e Event) bool {
		var args []string
		return e.Event == "command-reply" && e.Error == "success" &&
			json.Unmarshal(e.Command, &args) == nil && len(args) == 2 && args[0] == "get_property" && args[1] == name
	})
	value := number(e.Data)
	if math.IsNaN(value) {
		t.Fatalf("%s is not a number: %s", name, e.Data)
	}
	return value
}

func TestRealMPVPlaybackPauseSeekVolumeAndErrors(t *testing.T) {
	p := realMPV(t)
	l := &eventLog{p: p}
	// Quotes and a newline exercise JSON parameter encoding, rather than shell
	// escaping or human terminal output parsing.
	path := wav(t, "music \"quoted\"\ntrack.wav", 1.8)
	send(t, p, "set_property", "pause", true)
	send(t, p, "loadfile", path, "replace")
	start := l.wait(t, 0, "start-file", func(e Event) bool { return e.Event == "start-file" })
	if start.PlaylistEntryID == 0 {
		t.Fatal("start-file has no playlist identity")
	}
	l.wait(t, 0, "file-loaded", func(e Event) bool { return e.Event == "file-loaded" })
	l.wait(t, 0, "duration", property("duration", func(raw json.RawMessage) bool { return math.Abs(number(raw)-1.8) < 0.01 }))
	l.wait(t, 0, "source path", property("path", func(raw json.RawMessage) bool { var v string; return json.Unmarshal(raw, &v) == nil && v == path }))
	l.wait(t, 0, "audio parameters", property("audio-params", func(raw json.RawMessage) bool {
		var params struct {
			SampleRate   int `json:"samplerate"`
			ChannelCount int `json:"channel-count"`
		}
		return json.Unmarshal(raw, &params) == nil && params.SampleRate == 44100 && params.ChannelCount == 2
	}))
	mark := len(l.seen)
	send(t, p, "set_property", "pause", false)
	l.wait(t, mark, "playing position", property("time-pos", func(raw json.RawMessage) bool { return number(raw) > 0.12 }))
	mark = len(l.seen)
	send(t, p, "set_property", "pause", true)
	l.wait(t, mark, "paused", property("pause", func(raw json.RawMessage) bool { return string(raw) == "true" }))
	before := l.getNumber(t, "time-pos")
	time.Sleep(100 * time.Millisecond)
	if after := l.getNumber(t, "time-pos"); math.Abs(after-before) > 0.01 {
		t.Fatalf("paused engine advanced: %v -> %v", before, after)
	}
	mark = len(l.seen)
	send(t, p, "set_property", "volume", 27)
	l.wait(t, mark, "volume", property("volume", func(raw json.RawMessage) bool { return number(raw) == 27 }))
	if got := l.getNumber(t, "volume"); got != 27 {
		t.Fatalf("volume=%v", got)
	}
	mark = len(l.seen)
	send(t, p, "seek", 1.4, "absolute+exact")
	l.wait(t, mark, "exact paused seek", property("time-pos", func(raw json.RawMessage) bool { return math.Abs(number(raw)-1.4) < 0.03 }))
	mark = len(l.seen)
	send(t, p, "set_property", "pause", false)
	end := l.wait(t, mark, "natural EOF", func(e Event) bool { return e.Event == "end-file" && e.Reason == "eof" })
	if end.PlaylistEntryID != start.PlaylistEntryID {
		t.Fatalf("start/end IDs differ: %d vs %d", start.PlaylistEntryID, end.PlaylistEntryID)
	}
	mark = len(l.seen)
	send(t, p, "loadfile", filepath.Join(t.TempDir(), "missing.wav"), "replace")
	bad := l.wait(t, mark, "missing file error", func(e Event) bool { return e.Event == "end-file" && e.Reason == "error" })
	if bad.FileError == "" && bad.Error == "" {
		t.Fatal("file error lost diagnostic")
	}
	if bad.PlaylistEntryID == start.PlaylistEntryID {
		t.Fatal("replacement reused playlist identity")
	}
	mark = len(l.seen)
	send(t, p, "a-command-that-does-not-exist", "argument")
	failure := l.wait(t, mark, "command error", func(e Event) bool { return e.Event == "command-error" })
	if failure.Name != "a-command-that-does-not-exist" || failure.RequestID == 0 || failure.Error == "" || string(failure.Command) != `["a-command-that-does-not-exist","argument"]` {
		t.Fatalf("command error lost correlation: %+v", failure)
	}
	// A bad media path or command does not kill the healthy idle backend.
	if got := l.getNumber(t, "volume"); got != 27 {
		t.Fatalf("engine after error: volume=%v", got)
	}
}

func TestRealMPVPreservesEventsWhileConsumerIsBusyAndOnQuit(t *testing.T) {
	p := realMPV(t)
	path := wav(t, "short.wav", 0.08)
	send(t, p, "loadfile", path, "replace")
	// More replies than the public event channel's capacity. IPC must keep
	// draining while the UI is busy; start/end/error events cannot be dropped.
	for i := 0; i < 180; i++ {
		send(t, p, "get_property", "volume")
	}
	time.Sleep(500 * time.Millisecond)
	send(t, p, "quit")
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	var startID, endID int64
	replies, backendError := 0, false
	for {
		select {
		case e, ok := <-p.Events():
			if !ok {
				if startID == 0 || startID != endID || replies != 180 || !backendError {
					t.Fatalf("lost events: start=%d end=%d replies=%d backendError=%v", startID, endID, replies, backendError)
				}
				return
			}
			switch e.Event {
			case "start-file":
				startID = e.PlaylistEntryID
			case "end-file":
				if e.Reason == "eof" {
					endID = e.PlaylistEntryID
				}
			case "command-reply":
				if e.Name == "get_property" {
					replies++
				}
			case "backend-error":
				backendError = true
			}
		case <-deadline.C:
			t.Fatal("event stream did not finish after quit")
		}
	}
}

func TestRealMPVCrashIsReportedAndCleaned(t *testing.T) {
	p := realMPV(t)
	if err := p.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	reported := false
	for {
		select {
		case e, ok := <-p.Events():
			if !ok {
				if !reported {
					t.Fatal("crash did not produce backend-error")
				}
				if _, err := os.Stat(p.dir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("IPC directory survived crash: %v", err)
				}
				if err := p.Send("stop"); !errors.Is(err, ErrClosed) {
					t.Fatalf("send after crash=%v", err)
				}
				return
			}
			if e.Event == "backend-error" {
				reported = strings.Contains(e.Error, "mpv exited unexpectedly")
			}
		case <-deadline.C:
			t.Fatal("crashed child or stream did not finish")
		}
	}
}

func TestRealMPVConcurrentSendClose(t *testing.T) {
	p := realMPV(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 80; j++ {
				if err := p.Send("get_property", "volume"); err != nil && !errors.Is(err, ErrClosed) && !errors.Is(err, ErrQueueFull) {
					t.Errorf("send: %v", err)
				}
			}
		}()
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if _, err := os.Stat(p.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("IPC directory survived Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatal("second close:", err)
	}
}

func TestStartupFailuresAndTimeoutCleanPrivateDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("Unix IPC backend")
	}
	// Keep the socket below sockaddr_un's path limit, including mpv's own
	// private subdirectory. Go's verbose test-name directories can exceed it.
	dir, err := os.MkdirTemp("", "mpv-startup-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMPDIR", dir)
	if _, err := New(Options{Executable: filepath.Join(dir, "missing-mpv")}); err == nil {
		t.Fatal("missing executable accepted")
	}
	if _, err := exec.LookPath("mpv"); err == nil {
		if _, err := New(Options{ExtraArgs: []string{"--this-option-does-not-exist"}}); err == nil || !strings.Contains(err.Error(), "exit status") {
			t.Fatalf("missing startup diagnostic: %v", err)
		}
	}
	// A real process that never creates IPC must be killed and reaped. exec
	// replaces the shell so no orphan sleep child inherits diagnostic pipes.
	if _, err := exec.LookPath("sh"); err == nil {
		if _, err := New(Options{Executable: "sh", ExtraArgs: []string{"-c", "echo startup-diagnostic >&2; exit 7"}}); err == nil || !strings.Contains(err.Error(), "startup-diagnostic") || !strings.Contains(err.Error(), "exit status 7") {
			t.Fatalf("captured startup diagnostic=%v", err)
		}
		started := time.Now()
		if _, err := New(Options{Executable: "sh", ExtraArgs: []string{"-c", "exec sleep 30"}, StartupTimeout: 80 * time.Millisecond}); err == nil || !strings.Contains(err.Error(), "startup timed out") {
			t.Fatalf("startup timeout=%v", err)
		}
		if time.Since(started) > 2*time.Second {
			t.Fatal("startup timeout did not terminate child promptly")
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("startup failures leaked private directories: %v", entries)
	}
}

func TestRealMPVLongTempDirectoryUsesPrivateShortSocket(t *testing.T) {
	configured := filepath.Join(t.TempDir(), strings.Repeat("long-temp-path-", 6))
	if err := os.MkdirAll(configured, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", configured)
	p := realMPV(t)
	if strings.HasPrefix(p.dir, configured+string(filepath.Separator)) || len(filepath.Join(p.dir, "ipc.sock")) >= 100 {
		t.Fatalf("long configured temp path did not fall back: %q", p.dir)
	}
	for _, path := range []string{p.dir, filepath.Join(p.dir, "ipc.sock")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("IPC is accessible to other users: %s mode=%v", path, info.Mode())
		}
	}
	// The failed long-path directory is removed before the short one starts.
	entries, err := os.ReadDir(configured)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("fallback leaked long-path directories: %v", entries)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("short IPC directory survived Close: %v", err)
	}
}

func TestSendValidationAndQueueBackpressure(t *testing.T) {
	// No worker: this stands for a stalled peer. The UI must get an explicit
	// rejection instead of blocking or silently discarding an accepted command.
	p := &MPV{processDone: make(chan struct{}), commands: make(chan request, commandCapacity)}
	for _, args := range [][]any{nil, {17}, {""}, {"loadfile", make(chan int)}} {
		if err := p.Send(args...); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
	for i := 0; i < commandCapacity; i++ {
		send(t, p, "seek", i, "absolute")
	}
	started := time.Now()
	if err := p.Send("stop"); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("full queue error=%v", err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("full queue blocked caller")
	}
	for i := 0; i < commandCapacity; i++ {
		r := <-p.commands
		var args []any
		if err := json.Unmarshal(r.command, &args); err != nil {
			t.Fatal(err)
		}
		if number, ok := args[1].(float64); !ok || int(number) != i {
			t.Fatalf("FIFO command %d: %s", i, r.command)
		}
	}
}

// wav generates original PCM rather than depending on a music fixture, network,
// decoder download, or audible device. All integration tests use --ao=null.
func wav(t *testing.T, name string, seconds float64) string {
	t.Helper()
	const rate, channels, bits = 44100, 2, 16
	frames := int(seconds * rate)
	size := frames * channels * bits / 8
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
	for i := 0; i < frames; i++ {
		sample := int16(3000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		for ch := 0; ch < channels; ch++ {
			binary.LittleEndian.PutUint16(data[44+(i*channels+ch)*2:], uint16(sample))
		}
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
