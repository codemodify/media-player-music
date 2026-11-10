package shell

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

type neutralFixture struct {
	mu       sync.Mutex
	commands []playback.Command
	events   chan playback.Update
	closed   bool
	caps     playback.Capabilities
}

func newNeutralFixture(caps playback.Capabilities) *neutralFixture {
	return &neutralFixture{events: make(chan playback.Update, 100), caps: caps}
}
func (e *neutralFixture) Apply(c playback.Command) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return playback.ErrClosed
	}
	e.commands = append(e.commands, c)
	return nil
}
func (e *neutralFixture) Events() <-chan playback.Update      { return e.events }
func (e *neutralFixture) Capabilities() playback.Capabilities { return e.caps }
func (e *neutralFixture) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.closed {
		e.closed = true
		close(e.events)
	}
	return nil
}
func (e *neutralFixture) snapshot() ([]playback.Command, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]playback.Command(nil), e.commands...), e.closed
}

func switchFixture(t *testing.T) (*ui.Application, *Shell, *neutralFixture, *neutralFixture) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(players.DefaultSkin))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	first := newNeutralFixture(playback.Capabilities{Seek: true, Equalizer: true, Balance: true, Spectrum: true})
	next := newNeutralFixture(playback.Capabilities{Seek: true})
	infos := []playback.BackendInfo{{ID: "mpv", Name: "mpv", Executable: "test-mpv", Available: true}, {ID: "vlc", Name: "VLC", Executable: "test-vlc", Available: true}, {ID: "mplayer", Name: "MPlayer"}, {ID: "mpg123", Name: "mpg123", Executable: "test-mpg123", Available: true}}
	s, err := New(a, Options{Headless: true, Remember: true, Engine: first, DetectBackends: func() []playback.BackendInfo { return infos }, OpenBackend: func(info playback.BackendInfo) (playback.Engine, error) {
		if info.ID == "mpg123" {
			return nil, errors.New("control interface failed")
		}
		return next, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	return a, s, first, next
}

func pumpSwitch(t *testing.T, a *ui.Application, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		a.PumpOnce()
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("backend switch did not complete")
}

func TestBackendSwitchRestoresPausedSessionAndRejectsOldEvents(t *testing.T) {
	a, s, old, next := switchFixture(t)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	tpt := s.Host.Model.Transport
	list := tpt.List
	s.engineEvent(playback.Update{Kind: playback.UpdateLoaded, Generation: s.playback.generation})
	s.engineEvent(playback.Update{Kind: playback.UpdatePosition, Generation: s.playback.generation, Position: 13 * time.Second})
	s.Host.Command(players.CmdPlayPause)
	s.Host.SetVolume(.37)
	s.Host.SetPan(.4)
	s.Host.Model.Equaliser.SetOn(true)
	s.Host.Model.Equaliser.Set(3, 4)
	oldEpoch, oldGeneration := s.engineEpoch, s.playback.generation
	if err := s.SetBackend("vlc"); err != nil {
		t.Fatal(err)
	}
	pumpSwitch(t, a, func() bool { return !s.backendSwitching })
	commands, _ := next.snapshot()
	var load *playback.Command
	for i := range commands {
		if commands[i].Op == playback.OpLoad {
			load = &commands[i]
		}
	}
	if load == nil || !load.Paused || load.Position != 13*time.Second || load.Path != tpt.Track().Path {
		t.Fatalf("new engine lost paused load: %#v", load)
	}
	if s.engineEpoch <= oldEpoch || s.playback.generation == oldGeneration {
		t.Fatal("switch did not replace event/load identity")
	}
	if _, closed := old.snapshot(); !closed {
		t.Fatal("old process was not closed before resume")
	}
	if tpt.List != list || tpt.State != players.Paused || tpt.Pos != 13*time.Second || tpt.Volume != .37 || tpt.Pan != .4 || s.Host.Model.Equaliser.Gains[3] != 4 {
		t.Fatal("switch changed frontend session state")
	}
	if s.Host.Model.BackendEqualizer || s.Host.Model.BackendBalance || s.Host.Model.BackendSpectrum {
		t.Fatal("unavailable features were presented as supported")
	}
	for _, c := range commands {
		if c.Op == playback.OpDSP {
			t.Fatal("unsupported DSP was sent to new engine")
		}
	}
	s.engineEvent(playback.Update{Kind: playback.UpdateEnded, Generation: oldGeneration})
	s.engineEvent(playback.Update{Kind: playback.UpdateMetadata, Generation: oldGeneration, Info: playback.TrackInfo{Title: "old engine"}})
	if tpt.List.Index() != 0 || tpt.Track().Title == "old engine" {
		t.Fatal("old engine update changed current session")
	}
	if players.LoadSettings().AudioBackend != "vlc" {
		t.Fatal("selected backend was not persisted")
	}
}

func TestMissingOrFailedBackendKeepsCurrentPlayback(t *testing.T) {
	a, s, old, _ := switchFixture(t)
	if err := s.SetBackend("mplayer"); err == nil {
		t.Fatal("missing executable accepted")
	}
	if s.engine != old || s.BackendID() != "mpv" {
		t.Fatal("missing executable replaced active backend")
	}
	if err := s.SetBackend("mpg123"); err != nil {
		t.Fatal(err)
	}
	pumpSwitch(t, a, func() bool { return !s.backendSwitching })
	if s.engine != old || s.BackendID() != "mpv" || s.backendError != "control interface failed" {
		t.Fatal("failed constructor changed active backend")
	}
	if _, closed := old.snapshot(); closed {
		t.Fatal("startup failure stopped working engine")
	}
	if players.LoadSettings().AudioBackend == "mpg123" {
		t.Fatal("failed choice became saved preference")
	}
}

func TestSettingsShortcutWorksAndClosingDialogKeepsPlayer(t *testing.T) {
	a, s, _, _ := switchFixture(t)
	if !s.Keys(s.MainWindow(), widget.KeyEvent{Key: platform.KeyComma, Mods: platform.ModCtrl}) {
		t.Fatal("Ctrl+, not handled")
	}
	a.PumpOnce()
	if s.settingsWindow == nil || s.settingsPanel == nil {
		t.Fatal("settings dialog not opened")
	}
	w := s.settingsWindow
	w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyEscape})
	a.PumpOnce()
	if !w.Closed() || s.closed || s.MainWindow().Closed() {
		t.Fatal("closing settings closed the player")
	}
}

func TestTransportKeepsControllingOldEngineDuringFailedBackendStartup(t *testing.T) {
	a, s, old, _ := switchFixture(t)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	oldGeneration := s.playback.generation
	started, release := make(chan struct{}), make(chan struct{})
	s.opts.OpenBackend = func(playback.BackendInfo) (playback.Engine, error) {
		close(started)
		<-release
		return nil, errors.New("delayed startup failed")
	}
	if err := s.SetBackend("vlc"); err != nil {
		t.Fatal(err)
	}
	<-started
	s.Host.Command(players.CmdPlayPause)
	commands, _ := old.snapshot()
	if commands[len(commands)-1].Op != playback.OpPause || !commands[len(commands)-1].Paused {
		t.Fatal("pause during candidate startup did not reach active engine")
	}
	s.Host.Command(players.CmdNext)
	if s.playback.generation == oldGeneration || s.playback.index != s.Host.Model.Transport.List.Index() {
		t.Fatal("track change during startup kept obsolete load identity")
	}
	s.engineEvent(playback.Update{Kind: playback.UpdateMetadata, Generation: oldGeneration, Info: playback.TrackInfo{Title: "stale title"}})
	s.engineEvent(playback.Update{Kind: playback.UpdateEnded, Generation: oldGeneration})
	if s.Host.Model.Transport.Track().Title == "stale title" || s.Host.Model.Transport.List.Index() != 1 {
		t.Fatal("old-track events changed replacement track during startup")
	}
	s.Host.Command(players.CmdStop)
	commands, _ = old.snapshot()
	if commands[len(commands)-1].Op != playback.OpStop {
		t.Fatal("stop during candidate startup did not reach active engine")
	}
	close(release)
	pumpSwitch(t, a, func() bool { return !s.backendSwitching })
	if s.engine != old || s.Host.Model.Transport.State != players.Stopped {
		t.Fatal("startup failure did not preserve latest transport intent")
	}
}

func TestCloseJoinsPendingBackendConstructorAndClosesCandidate(t *testing.T) {
	a, s, old, next := switchFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	s.opts.OpenBackend = func(playback.BackendInfo) (playback.Engine, error) {
		close(started)
		<-release
		return next, nil
	}
	if err := s.SetBackend("vlc"); err != nil {
		t.Fatal(err)
	}
	<-started
	go func() { <-s.lifecycleDone; close(release) }()
	s.Close()
	if _, closed := old.snapshot(); !closed {
		t.Fatal("active engine survived shutdown")
	}
	if _, closed := next.snapshot(); !closed {
		t.Fatal("unaccepted candidate survived shutdown")
	}
	a.PumpOnce()
	if s.engine != old {
		t.Fatal("queued acceptance installed an engine after shutdown")
	}
}

type gatedCloseEngine struct {
	*neutralFixture
	started chan struct{}
	release chan struct{}
}

func (e *gatedCloseEngine) Close() error {
	close(e.started)
	<-e.release
	return e.neutralFixture.Close()
}

func TestCloseJoinsOldEngineCleanupAfterCommittedBackendSwitch(t *testing.T) {
	a, s, old, next := switchFixture(t)
	gated := &gatedCloseEngine{neutralFixture: old, started: make(chan struct{}), release: make(chan struct{})}
	s.engine = gated
	if err := s.SetBackend("vlc"); err != nil {
		t.Fatal(err)
	}
	pumpSwitch(t, a, func() bool { return s.backendTransition })
	<-gated.started
	go func() { <-s.lifecycleDone; close(gated.release) }()
	s.Close()
	if _, closed := old.snapshot(); !closed {
		t.Fatal("old process survived committed switch shutdown")
	}
	if _, closed := next.snapshot(); !closed {
		t.Fatal("new process survived committed switch shutdown")
	}
}

func TestFatalBackendErrorAllowsSelectedBackendRetry(t *testing.T) {
	a, s, old, next := switchFixture(t)
	s.engineEvent(playback.Update{Kind: playback.UpdateError, Error: "player process exited"})
	if s.backendError == "" {
		t.Fatal("fatal engine failure was not exposed in Settings")
	}
	if err := s.SetBackend("mpv"); err != nil {
		t.Fatal(err)
	}
	pumpSwitch(t, a, func() bool { return !s.backendSwitching })
	if s.engine != next || s.backendError != "" {
		t.Fatal("selected engine was not restarted after fatal error")
	}
	if _, closed := old.snapshot(); !closed {
		t.Fatal("failed engine was not closed on retry")
	}
}

func TestMissingSavedBackendKeepsFrontendAndCanSwitch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(players.DefaultSkin))
	if err := (players.Settings{Skin: players.DefaultSkin, AudioBackend: "mplayer"}).Save(); err != nil {
		t.Fatal(err)
	}
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	engine := newNeutralFixture(playback.Capabilities{Seek: true})
	s, err := New(a, Options{Headless: true, Remember: true, DetectBackends: func() []playback.BackendInfo {
		return []playback.BackendInfo{{ID: "mplayer", Name: "MPlayer"}, {ID: "vlc", Name: "VLC", Executable: "test-vlc", Available: true}}
	}, OpenBackend: func(info playback.BackendInfo) (playback.Engine, error) {
		if !info.Available {
			return nil, errors.New("saved backend is no longer installed")
		}
		return engine, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	if s.engine != nil || s.backendError == "" || s.BackendID() != "mplayer" || s.MainWindow().Closed() {
		t.Fatal("missing saved backend prevented usable frontend or silently replaced preference")
	}
	if err := s.SetBackend("vlc"); err != nil {
		t.Fatal(err)
	}
	pumpSwitch(t, a, func() bool { return !s.backendSwitching })
	if s.engine != engine || players.LoadSettings().AudioBackend != "vlc" {
		t.Fatal("available backend did not replace missing saved backend")
	}
}

// A rejected load has not entered the engine, so its generation cannot be
// used to stop the audio that was playing before the failed request.
type rejectingLoadFixture struct {
	*neutralFixture
	active  uint64
	stopped bool
}

func (e *rejectingLoadFixture) Apply(c playback.Command) error {
	if c.Op == playback.OpLoad {
		if e.active != 0 {
			return errors.New("mpg123 supports MP3/MPEG only; use mpv or VLC for .mp4")
		}
		e.active = c.Generation
	}
	if c.Op == playback.OpStop && (c.Generation == 0 || c.Generation == e.active) {
		e.stopped = true
	}
	return e.neutralFixture.Apply(c)
}

func TestRejectedLoadStopsPreviousAudioAndShowsErrorInSettings(t *testing.T) {
	a, s, fixture, _ := switchFixture(t)
	e := &rejectingLoadFixture{neutralFixture: fixture}
	s.engine = e
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	oldGeneration := e.active
	s.openSettings()
	a.PumpOnce()
	s.Host.Command(players.CmdNext)
	if !e.stopped || s.Host.Model.Transport.State != players.Stopped || s.playback.generation == oldGeneration {
		t.Fatal("rejected load left old generation audible")
	}
	if s.settingsState().Error != s.Host.Model.Error || s.settingsState().Error == "" {
		t.Fatal("Settings hid track-specific playback failure")
	}
	if s.settingsPanel == nil {
		t.Fatal("Settings was not kept open after failed load")
	}
}
