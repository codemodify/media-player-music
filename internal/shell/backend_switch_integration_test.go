package shell

import (
	"errors"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
)

// These observers wrap installed engines rather than synthesize replies. One
// reader records and forwards real events, so it never competes with Shell.
type installedSwitchEngine struct {
	playback.Engine
	id       string
	mu       sync.Mutex
	commands []playback.Command
	updates  []playback.Update
	closed   bool
	events   chan playback.Update
	stop     chan struct{}
	drained  chan struct{}
	once     sync.Once
	err      error
}

func observeInstalledEngine(id string, engine playback.Engine) *installedSwitchEngine {
	e := &installedSwitchEngine{Engine: engine, id: id, events: make(chan playback.Update, 64), stop: make(chan struct{}), drained: make(chan struct{})}
	go func() {
		defer close(e.drained)
		defer close(e.events)
		for update := range engine.Events() {
			e.mu.Lock()
			e.updates = append(e.updates, update)
			e.mu.Unlock()
			select {
			case e.events <- update:
			case <-e.stop:
			}
		}
	}()
	return e
}

func (e *installedSwitchEngine) Apply(c playback.Command) error {
	if err := e.Engine.Apply(c); err != nil {
		return err
	}
	e.mu.Lock()
	e.commands = append(e.commands, c)
	e.mu.Unlock()
	return nil
}

func (e *installedSwitchEngine) Events() <-chan playback.Update { return e.events }

func (e *installedSwitchEngine) Close() error {
	e.once.Do(func() {
		close(e.stop)
		e.err = e.Engine.Close()
		<-e.drained
		e.mu.Lock()
		e.closed = true
		e.mu.Unlock()
	})
	return e.err
}

func (e *installedSwitchEngine) snapshot() ([]playback.Command, []playback.Update, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]playback.Command(nil), e.commands...), append([]playback.Update(nil), e.updates...), e.closed
}

type installedSwitchFactory struct {
	mu      sync.Mutex
	engines []*installedSwitchEngine
}

func (f *installedSwitchFactory) open(info playback.BackendInfo) (playback.Engine, error) {
	args := map[string][]string{"mpv": {"--ao=null"}, "vlc": {"--aout=dummy"}, "mpg123": {"-o", "sleep"}}[info.ID]
	opts := playback.EngineOptions{Executable: info.Executable, ExtraArgs: args}
	var engine playback.Engine
	var err error
	switch info.ID {
	case "mpv":
		engine, err = playback.NewMPVEngine(opts)
	case "vlc":
		engine, err = playback.NewVLC(opts)
	case "mpg123":
		engine, err = playback.NewMPG123(opts)
	default:
		return nil, errors.New("unexpected installed test backend")
	}
	if err != nil {
		return nil, err
	}
	recorded := observeInstalledEngine(info.ID, engine)
	f.mu.Lock()
	f.engines = append(f.engines, recorded)
	f.mu.Unlock()
	return recorded, nil
}

func (f *installedSwitchFactory) snapshot() []*installedSwitchEngine {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*installedSwitchEngine(nil), f.engines...)
}

func installedSwitchPlayer(t *testing.T, target string) (*ui.Application, *Shell, *installedSwitchFactory) {
	t.Helper()
	if runtime.GOOS == "plan9" {
		t.Skip("the installed mpv adapter is unavailable on Plan9")
	}
	infos := playback.Detect()
	for _, id := range []string{"mpv", target} {
		available := false
		for _, info := range infos {
			if info.ID == id {
				available = info.Available
			}
		}
		if !available {
			t.Skip("installed " + id + " is unavailable")
		}
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(players.DefaultSkin))
	f := &installedSwitchFactory{}
	t.Cleanup(func() {
		for _, engine := range f.snapshot() {
			if err := engine.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	a := ui.New(ui.Options{Headless: true, Scale: 1, DisableLookWatch: true})
	s, err := New(a, Options{Skin: players.DefaultSkin, Headless: true, AudioBackend: "mpv", DetectBackends: func() []playback.BackendInfo { return infos }, OpenBackend: f.open})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	if s.engine == nil || s.backendError != "" {
		t.Fatalf("installed initial mpv could not start: %s", s.backendError)
	}
	return a, s, f
}

func switchMP3(t *testing.T, name string, seconds float64) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is unavailable for the local cross-decoder MP3 fixture")
	}
	input := integrationWAV(t, "source.wav", seconds)
	output := filepath.Join(t.TempDir(), name)
	command := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", input, "-c:a", "libmp3lame", "-b:a", "128k", "-metadata", "title="+name, "-y", output)
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create original silent MP3 fixture: %v: %s", err, data)
	}
	return output
}

func assertInstalledCapabilities(t *testing.T, s *Shell, id string) {
	t.Helper()
	caps, model := s.engine.Capabilities(), s.Host.Model
	if !caps.Seek || model.BackendEqualizer != caps.Equalizer || model.BackendBalance != caps.Balance || model.BackendSpectrum != caps.Spectrum {
		t.Fatalf("%s capabilities and player controls disagree: %+v", id, caps)
	}
	wantSpectrum := runtime.GOOS != "windows"
	if id == "mpv" && (!caps.Equalizer || !caps.Balance || caps.Spectrum != wantSpectrum) {
		t.Fatalf("returning to mpv did not restore its capabilities: %+v", caps)
	}
	if id != "mpv" && (caps.Equalizer || caps.Balance || caps.Spectrum) {
		t.Fatalf("%s advertised unavailable application DSP/analysis: %+v", id, caps)
	}
	for _, backend := range s.settingsState().Backends {
		if backend.ID == id && (backend.Equalizer != caps.Equalizer || backend.Balance != caps.Balance || backend.Visualizer != caps.Spectrum) {
			t.Fatalf("settings advertised different %s capabilities: %+v", id, backend)
		}
	}
}

func TestInstalledBackendSwitchPreservesPausedAndPlayingSession(t *testing.T) {
	for _, target := range []string{"vlc", "mpg123"} {
		for _, paused := range []bool{true, false} {
			state := "playing"
			if paused {
				state = "paused"
			}
			t.Run(target+"/"+state, func(t *testing.T) {
				a, s, factory := installedSwitchPlayer(t, target)
				first := switchMP3(t, "spare.mp3", 30)
				selected := switchMP3(t, "selected track.mp3", 30)
				if err := s.LoadFiles([]string{first, selected}, false); err != nil {
					t.Fatal(err)
				}
				s.Host.SelectTrack(1)
				tr := s.Host.Model.Transport
				integrationWait(t, a, s, "initial real input", func() bool { return !s.playback.loading && tr.Track().Length > 29*time.Second })
				s.Host.Command(players.CmdPlayPause)
				s.Host.Seek(.1)
				integrationWait(t, a, s, "initial paused seek", func() bool { return !s.playback.seeking && math.Abs(tr.Pos.Seconds()-3) < .3 })
				s.Host.SetVolume(.37)
				s.Host.SetPan(.4)
				s.Host.Model.Equaliser.SetOn(true)
				s.Host.Model.Equaliser.Set(3, 3)
				// A real mpv filter rebuild can update its buffered paused
				// decoder clock after the exact seek observation. Drain those
				// native positions before measuring the switch handoff.
				integrationPumpFor(a, s, 450*time.Millisecond)
				if !paused {
					s.Host.Play()
					integrationWait(t, a, s, "initial resumed real clock", func() bool { return tr.Pos > 3200*time.Millisecond })
				}
				list, model := tr.List, s.Host.Model
				for _, id := range []string{target, "mpv"} {
					before := tr.Pos
					engines := factory.snapshot()
					old := engines[len(engines)-1]
					_, oldUpdates, _ := old.snapshot()
					nativeBaseline := false
					for _, update := range oldUpdates {
						if update.Kind == playback.UpdatePosition && update.Generation == s.playback.generation && update.Position == before {
							nativeBaseline = true
						}
					}
					if !nativeBaseline {
						t.Fatalf("handoff baseline %s has no current-generation native clock observation", before)
					}
					if err := s.SetBackend(id); err != nil {
						t.Fatal(err)
					}
					integrationWait(t, a, s, "switch to installed "+id, func() bool {
						return !s.backendSwitching && s.BackendID() == id && !s.playback.loading && !s.playback.seeking
					})
					engines = factory.snapshot()
					next := engines[len(engines)-1]
					commands, _, _ := next.snapshot()
					var load *playback.Command
					volume, dsp := false, false
					for i := range commands {
						c := &commands[i]
						if c.Op == playback.OpLoad {
							load = c
						}
						volume = volume || c.Op == playback.OpVolume && c.Volume == .37
						dsp = dsp || c.Op == playback.OpDSP && c.DSP.Balance == .4 && c.DSP.Enabled && c.DSP.Gains[3] == 3
						if id != "mpv" && c.Op == playback.OpDSP {
							t.Fatal("unsupported DSP reached the limited real engine")
						}
					}
					if load == nil || load.Path != selected || load.Paused != paused || !volume || id == "mpv" && !dsp {
						t.Fatalf("%s did not receive the preserved session: load=%+v volume=%t dsp=%t", id, load, volume, dsp)
					}
					if load.Position < before-300*time.Millisecond || load.Position > before+2*time.Second || tr.Pos < before-500*time.Millisecond {
						t.Fatalf("%s reset or jumped the position: before=%s load=%s actual=%s", id, before, load.Position, tr.Pos)
					}
					if paused && math.Abs((load.Position-before).Seconds()) > .15 {
						t.Fatalf("%s lost the immediate paused handoff position: before=%s load=%s", id, before, load.Position)
					}
					if _, updates, closed := old.snapshot(); !closed {
						t.Fatal("new real engine started playback before previous engine closed")
					} else {
						for _, update := range updates {
							if update.Kind == playback.UpdateEnded {
								t.Fatal("backend replacement emitted a natural EOF")
							}
						}
					}
					wantState := players.Playing
					if paused {
						wantState = players.Paused
					}
					if s.Host.Model != model || tr.List != list || list.Index() != 1 || list.Len() != 2 || tr.Track().Path != selected || tr.State != wantState || tr.Volume != .37 || tr.Pan != .4 || model.Equaliser.Gains[3] != 3 {
						t.Fatalf("%s changed the frontend session", id)
					}
					assertInstalledCapabilities(t, s, id)
					position := tr.Pos
					if paused {
						integrationPumpFor(a, s, 300*time.Millisecond)
						if math.Abs((tr.Pos - position).Seconds()) > .15 {
							t.Fatalf("%s advanced its paused clock: %s -> %s", id, position, tr.Pos)
						}
					} else {
						integrationWait(t, a, s, id+" continuing playback", func() bool { return tr.Pos > position+150*time.Millisecond })
					}
				}
				s.Close()
				for _, engine := range factory.snapshot() {
					if _, _, closed := engine.snapshot(); !closed || !errors.Is(engine.Apply(playback.Command{Op: playback.OpVolume, Volume: .2}), playback.ErrClosed) {
						t.Fatal("shell shutdown left an installed engine active")
					}
				}
			})
		}
	}
}

func TestInstalledBackendNaturalEOFAdvancesOnceAndStopDoesNot(t *testing.T) {
	for _, target := range []string{"vlc", "mpg123"} {
		t.Run(target, func(t *testing.T) {
			a, s, factory := installedSwitchPlayer(t, target)
			if err := s.SetBackend(target); err != nil {
				t.Fatal(err)
			}
			integrationWait(t, a, s, "idle switch to "+target, func() bool { return !s.backendSwitching && s.BackendID() == target })
			short := switchMP3(t, "natural end.mp3", 2)
			long := switchMP3(t, "next.mp3", 20)
			if err := s.LoadFiles([]string{short, long}, false); err != nil {
				t.Fatal(err)
			}
			tr := s.Host.Model.Transport
			generation := s.playback.generation
			integrationWait(t, a, s, "actual "+target+" EOF advancing the app queue", func() bool { return tr.List.Index() == 1 && !s.playback.loading && tr.Pos > 100*time.Millisecond })
			engine := factory.snapshot()[len(factory.snapshot())-1]
			_, updates, _ := engine.snapshot()
			ends := 0
			for _, update := range updates {
				if update.Kind == playback.UpdateEnded && update.Generation == generation {
					ends++
				}
			}
			if ends != 1 || tr.Track().Path != long || tr.State != players.Playing {
				t.Fatalf("natural EOF count=%d state=%s path=%s", ends, tr.State, tr.Track().Path)
			}
			current := s.playback.generation
			s.Host.Command(players.CmdStop)
			integrationPumpFor(a, s, 350*time.Millisecond)
			_, updates, _ = engine.snapshot()
			for _, update := range updates {
				if update.Kind == playback.UpdateEnded && update.Generation == current {
					t.Fatal("explicit Stop was misreported as natural EOF")
				}
			}
			if tr.List.Index() != 1 || tr.State != players.Stopped {
				t.Fatal("explicit Stop advanced the queue")
			}
		})
	}
}
