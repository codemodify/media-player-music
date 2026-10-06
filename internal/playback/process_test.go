package playback

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type remoteEventLog struct {
	engine Engine
	seen   []Update
}

func slaveFixtureName(extension string) string {
	if runtime.GOOS == "windows" {
		return "music spaced ; pause." + extension
	}
	return `music "quoted" \ ; pause.` + extension
}

func (l *remoteEventLog) wait(t *testing.T, from int, description string, match func(Update) bool) Update {
	t.Helper()
	timer := time.NewTimer(7 * time.Second)
	defer timer.Stop()
	for {
		for _, u := range l.seen[from:] {
			if match(u) {
				return u
			}
		}
		from = len(l.seen)
		select {
		case u, ok := <-l.engine.Events():
			if !ok {
				t.Fatalf("events closed waiting for %s: %+v", description, l.seen)
			}
			l.seen = append(l.seen, u)
			if u.Kind == UpdateError && !match(u) {
				t.Fatalf("waiting for %s: %s", description, u.Error)
			}
		case <-timer.C:
			t.Fatalf("timeout waiting for %s: %+v", description, l.seen[max(0, len(l.seen)-20):])
		}
	}
}

func remoteApply(t *testing.T, e Engine, c Command) {
	t.Helper()
	if err := e.Apply(c); err != nil {
		t.Fatal(err)
	}
}
func remoteCleanup(t *testing.T, e Engine) {
	t.Helper()
	t.Cleanup(func() {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
		for range e.Events() {
		}
	})
}

// The helper is a real child executable, not an in-process replacement for
// transport/lifecycle behavior. Protocol details are verified separately with
// installed executables below.
func TestSlaveProcessHelper(t *testing.T) {
	mode := os.Getenv("MEDIA_PLAYER_SLAVE_HELPER")
	if mode == "" {
		return
	}
	out := os.Stdout
	if mode == "mpg123" {
		out = os.Stderr
		fmt.Fprintln(out, "@R MPG123 (ThOr) v11")
	} else {
		fmt.Fprintln(out, "MPlayer helper")
	}
	logPath := os.Getenv("MEDIA_PLAYER_SLAVE_LOG")
	paused := true
	pos := 0.0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if logPath != "" {
			f, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			fmt.Fprintln(f, line)
			f.Close()
		}
		command := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "pausing_keep_force "), "pausing_keep "), "pausing ")
		switch {
		case command == "QUIT" || command == "quit":
			if os.Getenv("MEDIA_PLAYER_SLAVE_STALL") == "1" {
				for {
					time.Sleep(time.Hour)
				}
			}
			// Deliberately offer EOF on replacement/Close. The parent must not
			// interpret teardown output as natural completion of its playlist.
			if mode == "mpg123" {
				fmt.Fprintln(out, "@P 3")
			} else {
				fmt.Fprintln(out, "ID_EXIT=EOF")
			}
			os.Exit(0)
		case strings.HasPrefix(command, "LOADPAUSED "):
			fmt.Fprintln(out, "@P 1")
		case strings.HasPrefix(command, "loadfile "):
			fmt.Fprintln(out, "ID_AUDIO_RATE=44100\nID_AUDIO_NCH=2\nID_LENGTH=4\nStarting playback...")
		case command == "FORMAT":
			fmt.Fprintln(out, "@FORMAT 44100 2")
		case command == "SAMPLE":
			fmt.Fprintf(out, "@SAMPLE %d 176400\n", int(pos*44100))
		case command == "get_property length":
			fmt.Fprintln(out, "ANS_length=4")
		case command == "get_property time_pos":
			fmt.Fprintf(out, "ANS_time_pos=%.6f\n", pos)
		case command == "PAUSE" || command == "pause":
			paused = !paused
			if mode == "mpg123" {
				if paused {
					fmt.Fprintln(out, "@P 1")
				} else {
					fmt.Fprintln(out, "@P 2")
				}
			}
		case strings.HasPrefix(command, "JUMP "):
			pos, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(command, "JUMP "), "s"), 64)
			fmt.Fprintln(out, "@J 42")
		case strings.HasPrefix(command, "seek "):
			pos, _ = strconv.ParseFloat(strings.Fields(command)[1], 64)
		}
	}
	os.Exit(0)
}

func helperEngine(t *testing.T, mode string) (Engine, string) {
	t.Helper()
	t.Setenv("MEDIA_PLAYER_SLAVE_HELPER", mode)
	log := filepath.Join(t.TempDir(), "commands.log")
	t.Setenv("MEDIA_PLAYER_SLAVE_LOG", log)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts := EngineOptions{Executable: exe, ExtraArgs: []string{"-test.run=^TestSlaveProcessHelper$", "--"}, StartupTimeout: 2 * time.Second}
	var e Engine
	if mode == "mpg123" {
		e, err = NewMPG123(opts)
	} else {
		e, err = NewMPlayer(opts)
	}
	if err != nil {
		t.Fatal(err)
	}
	remoteCleanup(t, e)
	return e, log
}

func TestSlaveAdaptersPreserveIntentGenerationsAndPaths(t *testing.T) {
	for _, mode := range []string{"mplayer", "mpg123"} {
		t.Run(mode, func(t *testing.T) {
			e, log := helperEngine(t, mode)
			l := remoteEventLog{engine: e}
			path := filepath.Join(t.TempDir(), `track "quote" \ ; PAUSE.mp3`)
			remoteApply(t, e, Command{Op: OpLoad, Generation: 11, Path: path, Paused: true, Position: time.Second, Epoch: 7})
			l.wait(t, 0, "initial seek", func(u Update) bool {
				return u.Kind == UpdateSeeked && u.Generation == 11 && u.Epoch == 7 && u.Position == time.Second
			})
			remoteApply(t, e, Command{Op: OpPause, Generation: 11, Paused: false})
			remoteApply(t, e, Command{Op: OpPause, Generation: 11, Paused: false})
			remoteApply(t, e, Command{Op: OpSeek, Generation: 11, Position: 2 * time.Second, Epoch: 8})
			l.wait(t, 0, "explicit seek", func(u Update) bool { return u.Kind == UpdateSeeked && u.Epoch == 8 && u.Position == 2*time.Second })
			remoteApply(t, e, Command{Op: OpLoad, Generation: 12, Path: path, Paused: true, Epoch: 9})
			l.wait(t, 0, "replacement loaded", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 12 })
			// A queued old-generation pause must not toggle the replacement.
			remoteApply(t, e, Command{Op: OpPause, Generation: 11, Paused: false})
			remoteApply(t, e, Command{Op: OpSeek, Generation: 12, Position: 3 * time.Second, Epoch: 10})
			l.wait(t, 0, "new generation seek", func(u Update) bool { return u.Kind == UpdateSeeked && u.Generation == 12 && u.Epoch == 10 })
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
			for u := range e.Events() {
				l.seen = append(l.seen, u)
			}
			for _, u := range l.seen {
				if u.Kind == UpdateEnded {
					t.Fatalf("replacement/Close generated natural EOF: %+v", u)
				}
			}
			contents, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			commands := strings.Split(strings.TrimSpace(string(contents)), "\n")
			toggles := 0
			loads := 0
			for _, c := range commands {
				if c == "pause" || c == "PAUSE" {
					toggles++
				}
				if mode == "mpg123" && strings.HasPrefix(c, "LOADPAUSED ") {
					loads++
					if c != "LOADPAUSED "+path {
						t.Fatalf("path altered: %q", c)
					}
				}
				if mode == "mplayer" && strings.HasPrefix(c, "pausing loadfile ") {
					loads++
					if c != "pausing loadfile "+mplayerQuote(path)+" 0" {
						t.Fatalf("path altered: %q", c)
					}
				}
			}
			if toggles != 1 || loads != 2 {
				t.Fatalf("wanted one idempotent resume and two loads; toggles=%d loads=%d\n%s", toggles, loads, contents)
			}
			if !errors.Is(e.Apply(Command{Op: OpStop}), ErrClosed) {
				t.Fatal("closed adapter accepted command")
			}
		})
	}
}

func TestSlaveAdaptersRejectInjectionAndUnsupportedDSP(t *testing.T) {
	for _, mode := range []string{"mplayer", "mpg123"} {
		t.Run(mode, func(t *testing.T) {
			e, _ := helperEngine(t, mode)
			for _, path := range []string{"file\nQUIT", "file\rQUIT", "file\x00QUIT"} {
				if e.Apply(Command{Op: OpLoad, Generation: 1, Path: path}) == nil {
					t.Fatalf("accepted injected path %q", path)
				}
			}
			if c := e.Capabilities(); !c.Seek || c.Equalizer || c.Balance || c.Spectrum {
				t.Fatalf("unsupported capabilities advertised: %+v", c)
			}
			if !errors.Is(e.Apply(Command{Op: OpDSP, DSP: DSP{Balance: .5}}), ErrUnsupported) {
				t.Fatal("unsupported balance accepted")
			}
			if !errors.Is(e.Apply(Command{Op: OpDSP, DSP: DSP{Enabled: true, Preamp: 3}}), ErrUnsupported) {
				t.Fatal("unsupported EQ accepted")
			}
			if err := e.Apply(Command{Op: OpDSP, DSP: DSP{Enabled: true}}); err != nil {
				t.Fatalf("flat DSP should be harmless: %v", err)
			}
			if e.Apply(Command{Op: OpVolume, Volume: float32(math.NaN())}) == nil {
				t.Fatal("accepted NaN volume")
			}
		})
	}
}

func TestSlaveAdapterCloseKillsUnresponsiveChildWithoutEventConsumer(t *testing.T) {
	t.Setenv("MEDIA_PLAYER_SLAVE_STALL", "1")
	e, _ := helperEngine(t, "mpg123")
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: "fake.mp3", Paused: true})
	started := time.Now()
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("Close waited for child or event consumer")
	}
	for range e.Events() {
	}
}

func TestRemoteClockKeepsQueryEpochAcrossSeek(t *testing.T) {
	for _, d := range []remoteDialect{mplayerDialect{}, mpg123Dialect{}} {
		t.Run(d.name(), func(t *testing.T) {
			b := newUpdateBroker()
			s := &remoteSession{generation: 4, epoch: 1, loaded: true, info: TrackInfo{SampleRate: 44100}}
			d.poll(s) // Query already outstanding when the seek is accepted.
			s.epoch = 2
			s.seeks = append(s.seeks, remoteSeek{epoch: 2, position: time.Second})
			d.poll(s)
			if d.name() == "mpg123" {
				d.parse(s, "@SAMPLE 0 88200", b)
				d.parse(s, "@J 38", b)
				d.parse(s, "@SAMPLE 44100 88200", b)
			} else {
				d.parse(s, "ANS_time_pos=0", b)
				d.parse(s, "ANS_time_pos=1", b)
			}
			b.close()
			var positions []Update
			var seeks []Update
			for u := range b.out {
				if u.Kind == UpdatePosition {
					positions = append(positions, u)
				}
				if u.Kind == UpdateSeeked {
					seeks = append(seeks, u)
				}
			}
			if len(positions) != 2 || positions[0].Epoch != 1 || positions[1].Epoch != 2 || len(seeks) != 1 || seeks[0].Epoch != 2 || seeks[0].Position != time.Second {
				t.Fatalf("old query mislabeled as new seek: positions=%+v seeks=%+v", positions, seeks)
			}
		})
	}
}

func TestUpdateBrokerDrainsCriticalEventsAndSnapshotsSpectrum(t *testing.T) {
	b := newUpdateBroker()
	bands := []float32{.25}
	b.offer(Update{Kind: UpdateSpectrum, Bands: bands})
	bands[0] = 1
	for i := uint64(1); i <= 200; i++ {
		b.offer(Update{Kind: UpdateLoaded, Generation: i})
		b.offer(Update{Kind: UpdateEnded, Generation: i})
	}
	b.close()
	first := <-b.out
	if first.Bands[0] != .25 {
		t.Fatal("spectrum input was not snapshotted")
	}
	count := 0
	for u := range b.out {
		wantKind := UpdateLoaded
		if count%2 == 1 {
			wantKind = UpdateEnded
		}
		if u.Kind != wantKind || u.Generation != uint64(count/2+1) {
			t.Fatalf("critical order lost at%d: %+v", count, u)
		}
		count++
	}
	if count != 400 {
		t.Fatalf("dropped critical events: %d", count)
	}
}
