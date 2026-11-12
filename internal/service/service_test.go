package service

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/model"
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/musicapi"
)

// engine records the service's commands; a test plays the engine's part
// through Inject.
type engine struct {
	lock     sync.Mutex
	commands []playback.Command
	events   chan playback.Update
	closed   bool
}

func newEngine() *engine { return &engine{events: make(chan playback.Update)} }

func (e *engine) Apply(command playback.Command) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return playback.ErrClosed
	}
	e.commands = append(e.commands, command)
	return nil
}
func (e *engine) Events() <-chan playback.Update { return e.events }
func (e *engine) Close() error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if !e.closed {
		e.closed = true
		close(e.events)
	}
	return nil
}

func (e *engine) count(op playback.Operation) int {
	e.lock.Lock()
	defer e.lock.Unlock()
	n := 0
	for _, command := range e.commands {
		if command.Op == op {
			n++
		}
	}
	return n
}

func (e *engine) last(op playback.Operation) playback.Command {
	e.lock.Lock()
	defer e.lock.Unlock()
	for i := len(e.commands) - 1; i >= 0; i-- {
		if e.commands[i].Op == op {
			return e.commands[i]
		}
	}
	return playback.Command{}
}

// told keeps the states the service told, by event.
type told struct {
	lock   sync.Mutex
	states map[string][]any
}

func (thisRef *told) publish(event string, value any) {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	thisRef.states[event] = append(thisRef.states[event], value)
}

func (thisRef *told) count(event string) int {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	return len(thisRef.states[event])
}

func (thisRef *told) lastState() musicapi.State {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	states := thisRef.states[musicapi.StateEvent]
	return states[len(states)-1].(musicapi.State)
}

func start(t *testing.T, saved Saved) (*Service, *engine, *told) {
	t.Helper()
	e, heard := newEngine(), &told{states: map[string][]any{}}
	s := New(Options{Engine: e, Saved: saved, Publish: heard.publish})
	t.Cleanup(func() { s.Close() })
	return s, e, heard
}

func files(t *testing.T, names ...string) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("fixture; decoding is the engine's"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func inject(s *Service, u playback.Update) {
	u.Generation = s.Generation()
	s.Inject(u)
}

func TestAddPlaysAndEveryChangeIsTold(t *testing.T) {
	s, e, heard := start(t, Saved{})
	ctx := context.Background()
	paths := files(t, "a.mp3", "b.flac", "c.ogg")
	added, err := s.Add(ctx, musicapi.Add{Paths: paths[:2]})
	if err != nil || added.Tracks != 2 || added.State.Status != musicapi.StatusPlaying || added.State.Index != 0 || e.last(playback.OpLoad).Path != paths[0] {
		t.Fatalf("an empty queue's first add did not play: %+v %v", added, err)
	}
	queues := heard.count(musicapi.QueueEvent)
	if _, err := s.Add(ctx, musicapi.Add{Paths: paths[2:]}); err != nil {
		t.Fatal(err)
	}
	if e.count(playback.OpLoad) != 1 || heard.count(musicapi.QueueEvent) != queues+1 || heard.lastState().Queue.Length != 3 {
		t.Fatal("an add to a queue playing restarted it, or was not told")
	}
	// the clock and the analyser as the engine hears them
	inject(s, playback.Update{Kind: playback.UpdateLoaded})
	inject(s, playback.Update{Kind: playback.UpdateDuration, Duration: time.Minute})
	inject(s, playback.Update{Kind: playback.UpdatePosition, Position: 1500 * time.Millisecond})
	inject(s, playback.Update{Kind: playback.UpdateSpectrum, Epoch: s.Epoch(), Position: 1500 * time.Millisecond, Bands: []float32{.5, .25}})
	heard.lock.Lock()
	nows := heard.states[musicapi.NowEvent]
	now := nows[len(nows)-1].(musicapi.Now)
	heard.lock.Unlock()
	if now.PositionMS != 1500 || now.LengthMS != 60000 || !slices.Equal(now.Bands, []float64{.5, .25}) || now.Seq != heard.lastState().Seq {
		t.Fatalf("now: %+v", now)
	}
	// a track chosen plays from its start; next and previous follow the order
	if state, err := s.Play(ctx, 2); err != nil || state.Index != 2 || e.last(playback.OpLoad).Path != paths[2] {
		t.Fatalf("Play: %+v %v", state, err)
	}
	if _, err := s.Play(ctx, 7); err == nil {
		t.Fatal("a track not in the queue played")
	}
	if state, _ := s.Command(ctx, musicapi.CommandPrevious); state.Index != 1 {
		t.Fatalf("previous: %d", state.Index)
	}
	if _, err := s.Command(ctx, "dance"); err == nil {
		t.Fatal("an unknown command was taken")
	}
	// a seek, the levels, the mode, the equaliser
	inject(s, playback.Update{Kind: playback.UpdateLoaded})
	if state, _ := s.Seek(ctx, 30000); state.PositionMS != 30000 || e.last(playback.OpSeek).Position != 30*time.Second {
		t.Fatalf("seek: %+v", state)
	}
	if state, _ := s.SetLevels(ctx, musicapi.Levels{Volume: 3, Balance: -.5}); state.Volume != 1 || state.Balance != -.5 || e.last(playback.OpVolume).Volume != 1 {
		t.Fatalf("levels: %+v", state)
	}
	if state, _ := s.SetMode(ctx, musicapi.Mode{Repeat: musicapi.RepeatOne, Shuffle: true}); state.Repeat != musicapi.RepeatOne || !state.Shuffle {
		t.Fatalf("mode: %+v", state)
	}
	state, _ := s.SetEqualizer(ctx, musicapi.Equalizer{On: true, Preset: "Vocal"})
	if !state.Equalizer.On || state.Equalizer.Preset != "Vocal" || !e.last(playback.OpDSP).DSP.Enabled || e.last(playback.OpDSP).DSP.Gains[4] != 5.5 {
		t.Fatalf("equaliser: %+v", state.Equalizer)
	}
	state, _ = s.SetEqualizer(ctx, musicapi.Equalizer{On: true, Gains: []float64{20, 0, 0, 0, 0, 0, 0, 0, 0, -1}})
	if state.Equalizer.Gains[0] != musicapi.EqualizerRange || state.Equalizer.Preset != "Custom" {
		t.Fatalf("a band out of range: %+v", state.Equalizer)
	}
	if state, _ := s.SetReplayGain(ctx, musicapi.ReplayGain{Mode: "album", Preamp: 99}); state.ReplayGain != (musicapi.ReplayGain{Mode: musicapi.ReplayGainAlbum, Preamp: 15}) ||
		e.last(playback.OpReplayGain).ReplayGain != (playback.ReplayGain{Mode: playback.ReplayGainAlbum, Preamp: 15}) {
		t.Fatalf("ReplayGain: %+v", state.ReplayGain)
	}
	// each state newer than the one before
	heard.lock.Lock()
	var last uint64
	for _, each := range heard.states[musicapi.StateEvent] {
		if seq := each.(musicapi.State).Seq; seq <= last {
			t.Fatalf("a state %d after %d", seq, last)
		} else {
			last = seq
		}
	}
	heard.lock.Unlock()
}

func TestRemoveAndMoveKeepTheTrackPlaying(t *testing.T) {
	s, e, _ := start(t, Saved{})
	ctx := context.Background()
	paths := files(t, "a.mp3", "b.mp3", "c.mp3", "d.mp3")
	if _, err := s.Add(ctx, musicapi.Add{Paths: paths}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Play(ctx, 2); err != nil {
		t.Fatal(err)
	}
	loads := e.count(playback.OpLoad)
	state, _ := s.Remove(ctx, []int{0, 0, 9})
	if state.Index != 1 || state.Track.Path != paths[2] || state.Queue.Length != 3 || e.count(playback.OpLoad) != loads {
		t.Fatalf("removing a track before the one playing: %+v, %d loads", state, e.count(playback.OpLoad)-loads)
	}
	state, _ = s.Move(ctx, musicapi.Move{From: 1, To: 0})
	if state.Index != 0 || state.Track.Path != paths[2] || e.count(playback.OpLoad) != loads {
		t.Fatalf("moving the track playing: %+v", state)
	}
	state, _ = s.Move(ctx, musicapi.Move{From: 2, To: 0})
	queue, _ := s.Queue(ctx)
	if state.Index != 1 || queue.Tracks[0].Path != paths[3] || queue.Tracks[1].Path != paths[2] || e.count(playback.OpLoad) != loads {
		t.Fatalf("moving a track before the one playing: %+v %+v", state, queue)
	}
	state, _ = s.Remove(ctx, []int{1})
	if state.Track.Path != paths[1] || e.count(playback.OpLoad) != loads+1 {
		t.Fatalf("removing the track playing did not play the next: %+v", state)
	}
	state, _ = s.Clear(ctx)
	if state.Status != musicapi.StatusStopped || state.Index != -1 || e.count(playback.OpStop) == 0 {
		t.Fatalf("clear: %+v", state)
	}
}

func TestTheEndOfATrackPlaysTheNextAndTheQueuesEndStops(t *testing.T) {
	s, e, heard := start(t, Saved{})
	ctx := context.Background()
	paths := files(t, "a.mp3", "b.mp3")
	if _, err := s.Add(ctx, musicapi.Add{Paths: paths}); err != nil {
		t.Fatal(err)
	}
	inject(s, playback.Update{Kind: playback.UpdateLoaded})
	inject(s, playback.Update{Kind: playback.UpdateMetadata, Info: playback.TrackInfo{Title: "Sung", Artist: "Singer"}})
	inject(s, playback.Update{Kind: playback.UpdateEnded})
	if state := heard.lastState(); state.Index != 1 || state.Status != musicapi.StatusPlaying || e.last(playback.OpLoad).Path != paths[1] {
		t.Fatalf("the next track: %+v", state)
	}
	if queue, _ := s.Queue(ctx); queue.Tracks[0].Title != "Sung" || queue.Tracks[0].Artist != "Singer" {
		t.Fatalf("what the decoder told was not kept: %+v", queue.Tracks[0])
	}
	inject(s, playback.Update{Kind: playback.UpdateLoaded})
	inject(s, playback.Update{Kind: playback.UpdateEnded})
	if state := heard.lastState(); state.Status != musicapi.StatusStopped {
		t.Fatalf("the queue's end: %+v", state)
	}
	// an update of a load gone is let go
	before := heard.count(musicapi.StateEvent)
	s.Inject(playback.Update{Kind: playback.UpdateEnded, Generation: s.Generation() - 1})
	if heard.count(musicapi.StateEvent) != before {
		t.Fatal("an old load's update was taken")
	}
	// an engine's error stops, and says why
	if _, err := s.Play(ctx, 0); err != nil {
		t.Fatal(err)
	}
	inject(s, playback.Update{Kind: playback.UpdateError, Error: "a.mp3 could not be played"})
	if state := heard.lastState(); state.Status != musicapi.StatusStopped || !strings.Contains(state.Error, "could not be played") {
		t.Fatalf("an error: %+v", state)
	}
	if state, _ := s.Command(ctx, musicapi.CommandPlay); state.Error != "" || state.Status != musicapi.StatusPlaying {
		t.Fatalf("play after an error: %+v", state)
	}
}

func TestWhatIsKeptComesBackStopped(t *testing.T) {
	s, _, _ := start(t, Saved{})
	ctx := context.Background()
	paths := files(t, "a.mp3", "b.mp3", "c.mp3")
	if _, err := s.Add(ctx, musicapi.Add{Paths: paths}); err != nil {
		t.Fatal(err)
	}
	s.Play(ctx, 1)
	inject(s, playback.Update{Kind: playback.UpdateLoaded})
	inject(s, playback.Update{Kind: playback.UpdateDuration, Duration: time.Minute})
	s.Seek(ctx, 9250)
	s.SetLevels(ctx, musicapi.Levels{Volume: .5, Balance: .25})
	s.SetMode(ctx, musicapi.Mode{Repeat: musicapi.RepeatAll})
	s.SetEqualizer(ctx, musicapi.Equalizer{On: true, Preset: "Late night"})
	s.SetReplayGain(ctx, musicapi.ReplayGain{Mode: musicapi.ReplayGainOff})
	saved := s.Saved()
	if saved.Index != 1 || saved.PositionMS != 9250 || len(saved.Queue) != 3 || saved.Queue[1].LengthMS != 60000 || *saved.Volume != .5 ||
		saved.Repeat != musicapi.RepeatAll || saved.Equalizer == nil || saved.ReplayGain.Mode != musicapi.ReplayGainOff {
		t.Fatalf("kept: %+v", saved)
	}
	// a file gone since is left out; the rest comes back as it was, stopped
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	again, e, _ := start(t, saved)
	state, _ := again.State(ctx)
	if state.Status != musicapi.StatusStopped || state.Index != 0 || state.Track.Path != paths[1] || state.PositionMS != 9250 || state.Volume != .5 ||
		state.Balance != .25 || state.Repeat != musicapi.RepeatAll || state.Equalizer.Preset != "Late night" || state.ReplayGain.Mode != musicapi.ReplayGainOff || e.count(playback.OpLoad) != 0 {
		t.Fatalf("restored: %+v", state)
	}
	again.Command(ctx, musicapi.CommandPlay)
	if load := e.last(playback.OpLoad); load.Path != paths[1] || load.Position != 9250*time.Millisecond {
		t.Fatalf("play after a restore: %+v", load)
	}
	// nothing chosen is nothing kept: track gain, the default volume
	if fresh, _, _ := start(t, Saved{}); fresh.Saved().Volume != nil || fresh.Saved().ReplayGain.Mode != "" || fresh.Saved().Equalizer != nil {
		t.Fatalf("defaults kept: %+v", fresh.Saved())
	}
}

func TestPresetsAreTheModels(t *testing.T) {
	var names []string
	for _, preset := range model.EqPresets() {
		names = append(names, preset.Name)
	}
	if !slices.Equal(names, musicapi.Presets) || !slices.Equal(model.EqBands, musicapi.Bands) {
		t.Fatalf("presets %q, bands %q", names, model.EqBands)
	}
}

func TestUnsupportedFilesAreRefusedAndAFoldersAreSaid(t *testing.T) {
	s, e, _ := start(t, Saved{})
	ctx := context.Background()
	paths := files(t, "song.wma", "album.ape", "song.mp3")
	if _, err := s.Add(ctx, musicapi.Add{Paths: paths[:1]}); err == nil || !strings.Contains(err.Error(), "song.wma: WMA files are not supported") || e.count(playback.OpLoad) != 0 {
		t.Fatalf("an explicit WMA file: %v", err)
	}
	added, err := s.Add(ctx, musicapi.Add{Paths: []string{filepath.Dir(paths[0])}})
	if err != nil || added.Tracks != 1 || !strings.Contains(added.Skipped, "2 files in formats the player does not play") || !strings.Contains(added.State.Error, "Monkey's Audio") {
		t.Fatalf("a folder's: %+v %v", added, err)
	}
}
