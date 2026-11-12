package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"github.com/codemodify/media-player-music/internal/service"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// fakeEngine records the daemon's commands; a test plays the engine's part
// by handing the service Updates (update). Its commands come under the
// service's lock, from the test's goroutine.
type fakeEngine struct {
	lock     sync.Mutex
	commands []playback.Command
	events   chan playback.Update
	closed   bool
}

func newFakeEngine() *fakeEngine { return &fakeEngine{events: make(chan playback.Update, 100)} }

func (e *fakeEngine) Apply(command playback.Command) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return playback.ErrClosed
	}
	e.commands = append(e.commands, command)
	return nil
}
func (e *fakeEngine) Events() <-chan playback.Update { return e.events }
func (e *fakeEngine) Close() error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if !e.closed {
		e.closed = true
		close(e.events)
	}
	return nil
}

// last returns the last command of an operation.
func (e *fakeEngine) last(op playback.Operation) (playback.Command, bool) {
	e.lock.Lock()
	defer e.lock.Unlock()
	for i := len(e.commands) - 1; i >= 0; i-- {
		if e.commands[i].Op == op {
			return e.commands[i], true
		}
	}
	return playback.Command{}, false
}

func countCommand(e *fakeEngine, op playback.Operation) int {
	e.lock.Lock()
	defer e.lock.Unlock()
	count := 0
	for _, command := range e.commands {
		if command.Op == op {
			count++
		}
	}
	return count
}

// daemon is a shell's player in the test's own process: the service
// media-player-musicd serves, on a fake engine (or audiotest's), its states
// posted to the shell's UI thread as the daemon's link posts them.
type daemon struct {
	app     *ui.Application
	service *service.Service
	shell   *Shell
}

var (
	daemonsLock sync.Mutex
	daemons     = map[*Shell]*daemon{}
)

// startDaemon makes a service on engine as saved, its states for the shell
// that newShell makes with it; the service is closed at the test's end.
func startDaemon(t *testing.T, a *ui.Application, engine playback.Engine, saved service.Saved, newShell func(Options) (*Shell, error), options Options) (*Shell, *daemon, error) {
	t.Helper()
	d := &daemon{app: a}
	d.service = service.New(service.Options{Engine: engine, Saved: saved, Publish: func(event string, value any) {
		a.Post(func() {
			if d.shell != nil {
				d.shell.Show(event, value)
			}
		})
	}})
	t.Cleanup(func() { d.service.Close() })
	options.Player = d.service
	s, err := newShell(options)
	if err != nil {
		return nil, nil, err
	}
	d.shell = s
	daemonsLock.Lock()
	daemons[s] = d
	daemonsLock.Unlock()
	a.DrainPosted()
	return s, d, nil
}

// serviceOf is the service a shell shows.
func serviceOf(s *Shell) *daemon {
	daemonsLock.Lock()
	defer daemonsLock.Unlock()
	return daemons[s]
}

func liveShell(t *testing.T, skin string) (*ui.Application, *Shell, *fakeEngine) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(skin))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	e := newFakeEngine()
	s, _, err := startDaemon(t, a, e, service.Saved{}, func(o Options) (*Shell, error) { return New(a, o) }, Options{Skin: skin, Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	return a, s, e
}

func mediaPaths(t *testing.T, count int) []string {
	t.Helper()
	var paths []string
	for i := 0; i < count; i++ {
		path := filepath.Join(t.TempDir(), "music.wav")
		if err := os.WriteFile(path, []byte("fixture; decoding belongs to the engine's tests"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

// update hands the daemon an update of the current load, and the shell
// what the daemon then told it.
func update(s *Shell, u playback.Update) {
	d := serviceOf(s)
	u.Generation = d.service.Generation()
	d.service.Inject(u)
	d.app.DrainPosted()
}

// updateOf hands the daemon an update as it is, of whichever load it
// names.
func updateOf(s *Shell, u playback.Update) {
	d := serviceOf(s)
	d.service.Inject(u)
	d.app.DrainPosted()
}

// pauseAt pauses the daemon at a time of its track, and shows it.
func pauseAt(t *testing.T, s *Shell, at time.Duration) {
	t.Helper()
	d, ctx := serviceOf(s), context.Background()
	if state, _ := d.service.State(ctx); state.Status == musicapi.StatusPlaying {
		if _, err := d.service.Command(ctx, musicapi.CommandPlayPause); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.service.Seek(ctx, at.Milliseconds()); err != nil {
		t.Fatal(err)
	}
	d.app.DrainPosted()
	if tr := s.Host.Model.Transport; tr.State != players.Paused || tr.Pos != at {
		t.Fatalf("not paused at %v: %v at %v", at, tr.State, tr.Pos)
	}
}

// placeAt puts the daemon's transport in a state at a time of its track,
// and shows it.
func placeAt(t *testing.T, s *Shell, state players.State, at time.Duration) {
	t.Helper()
	d, ctx := serviceOf(s), context.Background()
	command := map[players.State]string{players.Stopped: musicapi.CommandStop, players.Playing: musicapi.CommandPlay}[state]
	if state == players.Paused {
		if _, err := d.service.Command(ctx, musicapi.CommandPlay); err != nil {
			t.Fatal(err)
		}
		command = musicapi.CommandPlayPause
	}
	if _, err := d.service.Command(ctx, command); err != nil {
		t.Fatal(err)
	}
	if _, err := d.service.Seek(ctx, at.Milliseconds()); err != nil {
		t.Fatal(err)
	}
	d.app.DrainPosted()
	if tr := s.Host.Model.Transport; tr.State != state || tr.Pos != at {
		t.Fatalf("not %v at %v: %v at %v", state, at, tr.State, tr.Pos)
	}
}

// generation is the daemon's current load.
func generation(s *Shell) uint64 { return serviceOf(s).service.Generation() }

// loaded says the current load opened.
func loaded(s *Shell) { update(s, playback.Update{Kind: playback.UpdateLoaded}) }

// seeked says the last seek is heard.
func seeked(s *Shell) {
	update(s, playback.Update{Kind: playback.UpdateSeeked, Epoch: serviceOf(s).service.Epoch()})
}

func TestLiveClockAndEventsRejectSupersededTrack(t *testing.T) {
	_, s, e := liveShell(t, players.DefaultSkin)
	paths := mediaPaths(t, 2)
	if err := s.LoadFiles(paths, false); err != nil {
		t.Fatal(err)
	}
	if load, ok := e.last(playback.OpLoad); !ok || load.Path != paths[0] || load.Generation != generation(s) {
		t.Fatalf("the first track was not loaded: %+v", e.commands)
	}
	loaded(s)
	update(s, playback.Update{Kind: playback.UpdateDuration, Duration: 90 * time.Second})
	update(s, playback.Update{Kind: playback.UpdatePosition, Position: 12500 * time.Millisecond})
	pos := s.Host.Model.Transport.Pos
	if pos != 12500*time.Millisecond || s.Host.Model.Transport.Track().Length != 90*time.Second {
		t.Fatal("the engine's clock or duration was not applied")
	}
	s.tick(time.Second)
	if s.Host.Model.Transport.Pos != pos {
		t.Fatal("UI pulse drove the actual audio clock")
	}
	for _, v := range s.Host.Model.Spectrum.Bars {
		if v != 0 {
			t.Fatal("live playback fabricated spectrum data")
		}
	}
	old := generation(s)
	s.Host.Command(players.CmdNext)
	if generation(s) == old || countCommand(e, playback.OpLoad) != 2 {
		t.Fatal("Next did not load the next track as a new generation")
	}
	for _, u := range []playback.Update{
		{Kind: playback.UpdatePosition, Generation: old, Position: 78 * time.Second},
		{Kind: playback.UpdateMetadata, Generation: old, Info: playback.TrackInfo{Title: "old title"}},
		{Kind: playback.UpdateAudioFormat, Generation: old, Info: playback.TrackInfo{SampleRate: 96000, Channels: 6}},
		{Kind: playback.UpdateEnded, Generation: old},
	} {
		updateOf(s, u)
	}
	if track := s.Host.Model.Transport.Track(); s.Host.Model.Transport.List.Index() != 1 || s.Host.Model.Transport.Pos != 0 || track.Title == "old title" || track.Rate != 0 {
		t.Fatal("an old load's update changed the newly selected track")
	}
	loaded(s)
	update(s, playback.Update{Kind: playback.UpdateDuration, Duration: 100 * time.Second})
	update(s, playback.Update{Kind: playback.UpdateMetadata, Info: playback.TrackInfo{Title: "new title", Artist: "artist"}})
	update(s, playback.Update{Kind: playback.UpdateAudioFormat, Info: playback.TrackInfo{SampleRate: 48000, Channels: 2, Bitrate: 320000}})
	update(s, playback.Update{Kind: playback.UpdatePosition, Position: 2 * time.Second})
	if track := s.Host.Model.Transport.Track(); track.Title != "new title" || track.Artist != "artist" || track.Rate != 48 || track.Channels != 2 || track.Bitrate != 320000 || s.Host.Model.Transport.Pos != 2*time.Second {
		t.Fatal("fresh metadata, format or clock not applied")
	}
	s.Host.SetVolume(.23)
	if countCommand(e, playback.OpSeek) != 0 {
		t.Fatal("volume edit sought to a stale UI position")
	}
	if volume, ok := e.last(playback.OpVolume); !ok || volume.Volume != .23 {
		t.Fatal("the volume did not reach the engine")
	}
}

func TestEndWhilePausedStopsAndPlayReloads(t *testing.T) {
	_, s, e := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	loaded(s)
	update(s, playback.Update{Kind: playback.UpdateEnded})
	if s.Host.Model.Transport.List.Index() != 1 || s.Host.Model.Transport.State != players.Playing || countCommand(e, playback.OpLoad) != 2 {
		t.Fatal("the end of a track did not play the next one")
	}
	loaded(s)
	s.Host.Command(players.CmdPlayPause)
	if pause, ok := e.last(playback.OpPause); !ok || !pause.Paused {
		t.Fatal("pause did not reach the engine")
	}
	update(s, playback.Update{Kind: playback.UpdateEnded})
	if s.Host.Model.Transport.State != players.Stopped {
		t.Fatal("a paused end did not stop")
	}
	before := countCommand(e, playback.OpLoad)
	s.Host.Play()
	if countCommand(e, playback.OpLoad) != before+1 {
		t.Fatal("Play did not reload the ended file")
	}
}

func TestSeekWaitsForLoadAndErrorsStopAudio(t *testing.T) {
	_, s, e := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	s.Host.Model.Transport.List.Tracks[0].Length = time.Minute
	s.Host.Seek(.5)
	if countCommand(e, playback.OpSeek) != 0 {
		t.Fatal("seek was sent before the engine loaded the file")
	}
	loaded(s)
	if seek, ok := e.last(playback.OpSeek); !ok || countCommand(e, playback.OpSeek) != 1 || seek.Position != 30*time.Second {
		t.Fatal("pending seek not applied after load")
	}
	seeked(s)
	update(s, playback.Update{Kind: playback.UpdateError, Error: "the file could not be played"})
	if s.Host.Model.Transport.State != players.Stopped || !strings.Contains(s.Host.Model.Error, "could not be played") || countCommand(e, playback.OpStop) == 0 {
		t.Fatal("an engine error did not stop actual audio and show its error")
	}
	s.Host.Play()
	if s.Host.Model.Error != "" {
		t.Fatal("retry did not clear error")
	}
}

func TestPlaylistValidationAppendRemoveAndShutdown(t *testing.T) {
	_, s, e := liveShell(t, players.DefaultSkin)
	paths := mediaPaths(t, 2)
	if err := s.LoadFiles(paths[:1], false); err != nil {
		t.Fatal(err)
	}
	list := s.Host.Model.Transport.List
	if err := s.LoadFiles([]string{filepath.Join(t.TempDir(), "missing")}, false); err == nil || s.Host.Model.Transport.List != list {
		t.Fatal("invalid file replaced the working playlist")
	}
	if err := s.LoadFiles([]string{t.TempDir()}, false); err == nil {
		t.Fatal("a directory was accepted as a music file")
	}
	before := countCommand(e, playback.OpLoad)
	if err := s.LoadFiles(paths[1:], true); err != nil {
		t.Fatal(err)
	}
	if s.Host.Model.Transport.List.Len() != 2 || countCommand(e, playback.OpLoad) != before {
		t.Fatal("Add interrupted the current file")
	}
	s.Host.RemoveTrack()
	if s.Host.Model.Transport.Track().Path != paths[1] || countCommand(e, playback.OpLoad) != before+1 {
		t.Fatal("removing current track did not load its successor")
	}
	s.Host.RemoveTrack()
	if s.Host.Model.Transport.List.Len() != 0 || s.Host.Model.Transport.State != players.Stopped || countCommand(e, playback.OpStop) == 0 {
		t.Fatal("removing last track did not stop audio")
	}
	s.Close()
	s.Close()
	if e.closed {
		t.Fatal("the shell closed the daemon's engine")
	}
}

func TestUnsupportedFormatsAreRefusedWithAClearMessage(t *testing.T) {
	_, s, e := liveShell(t, players.DefaultSkin)
	dir := t.TempDir()
	wma, wv, mp3 := filepath.Join(dir, "song.wma"), filepath.Join(dir, "album.wv"), filepath.Join(dir, "song.mp3")
	for _, path := range []string{wma, wv, mp3} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := s.LoadFiles([]string{wma}, false)
	if err == nil || !strings.Contains(err.Error(), "song.wma: WMA files are not supported") || s.Host.Model.Transport.List.Len() != 0 || countCommand(e, playback.OpLoad) != 0 {
		t.Fatalf("an explicit WMA file was not refused clearly: %v", err)
	}
	if err := s.LoadFiles([]string{dir}, false); err != nil {
		t.Fatal(err)
	}
	if list := s.Host.Model.Transport.List; list.Len() != 1 || list.At(0).Path != mp3 {
		t.Fatal("a folder's unsupported files were queued")
	}
	if !strings.Contains(s.Host.Model.Error, "2 files in formats the player does not play") || !strings.Contains(s.Host.Model.Error, "WavPack") || !strings.Contains(s.Host.Model.Error, "WMA") {
		t.Fatalf("the skipped files were not reported: %q", s.Host.Model.Error)
	}
	// A file that turns out not to play says so when the engine fails it.
	update(s, playback.Update{Kind: playback.UpdateError, Error: playback.UnsupportedMessage("/music/odd.xyz")})
	if !strings.Contains(s.Host.Model.Error, "odd.xyz: this format is not supported. The player plays MP3") {
		t.Fatalf("the engine's message did not reach the player: %q", s.Host.Model.Error)
	}
}

func TestReplayGainSettingsReachTheEngineAndAreSaved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(players.DefaultSkin))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	e := newFakeEngine()
	saved := service.Saved{ReplayGain: musicapi.ReplayGain{Mode: musicapi.ReplayGainAlbum, Preamp: -2}}
	s, d, err := startDaemon(t, a, e, saved, func(o Options) (*Shell, error) { return New(a, o) }, Options{Headless: true, Remember: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if gain, ok := e.last(playback.OpReplayGain); !ok || gain.ReplayGain != (playback.ReplayGain{Mode: playback.ReplayGainAlbum, Preamp: -2}) {
		t.Fatalf("the saved ReplayGain did not reach the engine at start: %+v", e.commands)
	}
	s.SetReplayGain(players.ReplayGain{Mode: players.ReplayGainTrack, Preamp: 3.5})
	if gain, ok := e.last(playback.OpReplayGain); !ok || gain.ReplayGain != (playback.ReplayGain{Mode: playback.ReplayGainTrack, Preamp: 3.5}) {
		t.Fatal("a ReplayGain change did not reach the engine")
	}
	if kept := d.service.Saved().ReplayGain; kept != (musicapi.ReplayGain{Preamp: 3.5}) {
		t.Fatalf("the ReplayGain change was not kept (track gain as none): %+v", kept)
	}
	s.SetReplayGain(players.ReplayGain{Mode: "loudest", Preamp: 40})
	if gain, _ := e.last(playback.OpReplayGain); gain.ReplayGain != (playback.ReplayGain{Mode: playback.ReplayGainOff, Preamp: players.MaxPreamp}) {
		t.Fatalf("an unknown mode or a preamp out of range was not normalised: %+v", gain.ReplayGain)
	}
	// The settings window shows them, and its controls change them.
	s.openSettings()
	a.PumpOnce()
	a.PumpOnce()
	if s.settingsPanel == nil {
		t.Fatal("no settings window")
	}
	if state := s.settingsPanel.State(); state.ReplayGain.Preamp != players.MaxPreamp || !strings.Contains(state.Formats, "MP3") {
		t.Fatalf("the settings window shows %+v", state)
	}
	s.settingsPanel.SetState(players.SettingsState{Formats: playback.Formats, ReplayGain: players.ReplayGain{Mode: players.ReplayGainAlbum}})
	if s.settingsPanel.State().ReplayGain.Mode != players.ReplayGainAlbum {
		t.Fatal("the settings window did not take its state")
	}
}

func TestFileChooserFitsAllPresentationsAndLoadsPickedFile(t *testing.T) {
	for _, skin := range []string{players.DefaultSkin, winamp.Skin, winamp.SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, s, _ := liveShell(t, skin)
			path := mediaPaths(t, 1)[0]
			s.Host.OpenFiles(s.MainWindow().Content(), false)
			a.PumpOnce()
			w := s.fileWindow
			if w == nil || w.Closed() {
				t.Fatal("no file chooser window")
			}
			var field *widgets.TextField
			var open *widgets.Button
			var visit func(widget.Component)
			visit = func(c widget.Component) {
				switch x := c.(type) {
				case *widgets.TextField:
					field = x
				case *widgets.Button:
					if x.Text == "Open" {
						open = x
					}
				}
				for _, child := range c.Children() {
					visit(child)
				}
			}
			visit(w.Content())
			if field == nil || open == nil {
				t.Fatal("missing file path and Open controls")
			}
			bounds := widget.LocalToWindow(open, open.LocalBounds())
			width, height := w.PixelSize()
			if bounds.Min.Y < 0 || bounds.Max.Y > float32(height) || bounds.Max.X > float32(width) {
				t.Fatalf("chooser Open button clipped: %v in %dx%d", bounds, width, height)
			}
			field.SetText(path)
			if !open.AccessibleAction(0, a11y.ActionDefault) {
				t.Fatal("could not activate chooser Open")
			}
			waitFileLoad(t, a, "picked file load", func() bool {
				return s.Host.Model.Transport.Track().Path == path && s.Host.Model.Transport.State == players.Playing
			})
			if !w.Closed() || s.Host.Model.Transport.Track().Path != path || s.Host.Model.Transport.State != players.Playing {
				t.Fatal("picked file was not loaded and started")
			}
			// A temporary file chooser must never become part of the window rack.
			if skin != players.DefaultSkin && len(s.Windows()) != 3 {
				t.Fatal("file chooser entered the skin rack")
			}
		})
	}
}
