// Package service is the player media-player-musicd serves: musicapi.Player
// over one queue and transport (internal/model) every renderer shares,
// played by one engine (internal/playback), its changes told to every
// renderer and kept. The renderers' tests run it in their own process.
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/codemodify/media-player-music/internal/model"
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/musicapi"
)

// Options make a Service.
type Options struct {
	// Engine plays: the built-in engine on the system's sound in the
	// daemon, a fake one or the built-in engine on audiotest's sound
	// system in tests. The service owns it and closes it.
	Engine playback.Engine
	// Saved is what was kept of an earlier run (Service.Saved's).
	Saved Saved
	// Publish tells every renderer a state (musicapi.StateEvent,
	// QueueEvent, NowEvent), called under the service's lock so that the
	// states follow the changes: it must not wait (clienthost's SetState
	// never does) and must not call the service.
	Publish func(event string, value any)
	// Changed hears that what is kept changed, for the daemon to save it
	// (Saved); nil for none.
	Changed func()
}

// Service is the player every renderer shares.
type Service struct {
	lock      sync.Mutex
	engine    playback.Engine
	publish   func(string, any)
	changed   func()
	transport *model.Transport
	equalizer *model.Equalizer
	gain      musicapi.ReplayGain
	err       string
	clients   int
	seq       uint64
	version   uint64 // the queue's
	published uint64 // the queue's version last published
	bands     []float64
	playback  playbackState
	savedAt   time.Duration // the position last kept
	closed    bool
	done      chan struct{}

	adding sync.Mutex // one Add at a time, in the order they came
}

// New makes the service as saved, and starts hearing its engine.
func New(options Options) *Service {
	publish := options.Publish
	if publish == nil {
		publish = func(string, any) {}
	}
	s := &Service{engine: options.Engine, publish: publish, changed: options.Changed, done: make(chan struct{})}
	s.restore(options.Saved)
	go s.hear()
	s.lock.Lock()
	s.applyReplayGain()
	s.sync()
	s.lock.Unlock()
	return s
}

// hear takes the engine's updates as they come, until it closes.
func (thisRef *Service) hear() {
	defer close(thisRef.done)
	for update := range thisRef.engine.Events() {
		thisRef.Inject(update)
	}
}

// Inject handles an engine's update as if the engine sent it: for the
// renderers' tests, whose fake engine sends none.
func (thisRef *Service) Inject(update playback.Update) {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	if thisRef.closed {
		return
	}
	thisRef.engineEvent(update)
}

// Generation is the current load's (a test's fake updates carry it), and
// Epoch its analyser's and seeks' epoch.
func (thisRef *Service) Generation() uint64 {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	return thisRef.playback.generation
}

// Epoch is the current load's epoch: a seek's or the analyser's.
func (thisRef *Service) Epoch() int64 {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	return thisRef.playback.spectrumEpoch
}

// Close stops what plays and closes the engine.
func (thisRef *Service) Close() error {
	thisRef.lock.Lock()
	if thisRef.closed {
		thisRef.lock.Unlock()
		return nil
	}
	thisRef.closed = true
	thisRef.lock.Unlock()
	err := thisRef.engine.Close()
	<-thisRef.done
	return err
}

// Publish tells every renderer the player as it is now: the daemon calls
// it as it starts.
func (thisRef *Service) Publish() {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	thisRef.published = 0
	thisRef.publishLocked()
}

// Connected counts a renderer coming (+1) or going (−1), and tells them
// all.
func (thisRef *Service) Connected(delta int) {
	thisRef.change(func() { thisRef.clients += delta })
}

// change makes a change, plays it and tells every renderer under the lock,
// so that two changes' states never cross, then says what is kept changed.
func (thisRef *Service) change(change func()) musicapi.State {
	thisRef.lock.Lock()
	change()
	thisRef.sync()
	state := thisRef.publishLocked()
	thisRef.lock.Unlock()
	if thisRef.changed != nil {
		thisRef.changed()
	}
	return state
}

// publishLocked tells the renderers the state, the queue first when it
// changed since it was last told.
func (thisRef *Service) publishLocked() musicapi.State {
	if thisRef.published != thisRef.version || thisRef.published == 0 {
		thisRef.published = thisRef.version
		thisRef.publish(musicapi.QueueEvent, thisRef.queueLocked())
	}
	state := thisRef.stateLocked()
	thisRef.publish(musicapi.StateEvent, state)
	thisRef.publishNowLocked()
	return state
}

func (thisRef *Service) publishNowLocked() {
	thisRef.publish(musicapi.NowEvent, thisRef.nowLocked())
}

// queueChanged gives the queue a new version.
func (thisRef *Service) queueChanged() { thisRef.version++ }

func (thisRef *Service) stateLocked() musicapi.State {
	thisRef.seq++
	t := thisRef.transport
	state := musicapi.State{
		Seq: thisRef.seq, Status: status(t.State), Index: t.List.Index(), Track: track(t.Track()),
		PositionMS: t.Pos.Milliseconds(), Volume: float64(t.Volume), Balance: float64(t.Pan),
		Repeat: repeat(t.Repeat), Shuffle: t.Random, Equalizer: equalizer(thisRef.equalizer), ReplayGain: thisRef.gain,
		Queue:   musicapi.QueueInfo{Version: thisRef.version, Length: t.List.Len(), LengthMS: t.List.Total().Milliseconds()},
		Error:   thisRef.err,
		Formats: playback.Formats,
		Clients: thisRef.clients,
	}
	return state
}

func (thisRef *Service) nowLocked() musicapi.Now {
	t := thisRef.transport
	now := musicapi.Now{Seq: thisRef.seq, Index: t.List.Index(), Status: status(t.State), PositionMS: t.Pos.Milliseconds(), LengthMS: t.Length().Milliseconds()}
	if t.State == model.Playing {
		now.Bands = thisRef.bands
	}
	return now
}

func (thisRef *Service) queueLocked() musicapi.Queue {
	tracks := make([]musicapi.Track, thisRef.transport.List.Len())
	for i, each := range thisRef.transport.List.Tracks {
		tracks[i] = track(each)
	}
	return musicapi.Queue{Version: thisRef.version, Tracks: tracks}
}

// ---- musicapi.Player ----------------------------------------------------------

func (thisRef *Service) State(context.Context) (musicapi.State, error) {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	return thisRef.stateLocked(), nil
}

func (thisRef *Service) Queue(context.Context) (musicapi.Queue, error) {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	return thisRef.queueLocked(), nil
}

// commands are the commands by their names.
var commands = map[string]model.Command{
	musicapi.CommandPlay: model.CmdPlay, musicapi.CommandPlayPause: model.CmdPlayPause, musicapi.CommandStop: model.CmdStop,
	musicapi.CommandNext: model.CmdNext, musicapi.CommandPrevious: model.CmdPrev,
	musicapi.CommandSeekBack: model.CmdSeekBack, musicapi.CommandSeekForward: model.CmdSeekForward,
	musicapi.CommandVolumeUp: model.CmdVolumeUp, musicapi.CommandVolumeDown: model.CmdVolumeDown, musicapi.CommandMute: model.CmdMute,
	musicapi.CommandShuffle: model.CmdShuffle, musicapi.CommandRepeat: model.CmdRepeat,
}

// CommandName is a model command's name in the API; "" for none.
func CommandName(command model.Command) string {
	for name, each := range commands {
		if each == command {
			return name
		}
	}
	return ""
}

func (thisRef *Service) Command(_ context.Context, name string) (musicapi.State, error) {
	command, ok := commands[name]
	if !ok {
		state, _ := thisRef.State(context.Background())
		return state, fmt.Errorf("no command %q", name)
	}
	return thisRef.change(func() {
		before := thisRef.transport.Random
		thisRef.transport.Do(command)
		if command == model.CmdPlay || command == model.CmdPlayPause {
			thisRef.err = ""
		}
		if before != thisRef.transport.Random {
			thisRef.queueChanged() // the play order, not the queue: told for a renderer that shows it
		}
	}), nil
}

// ErrNoTrack is Play's error for an index that is not in the queue.
var ErrNoTrack = errors.New("no such track in the queue")

func (thisRef *Service) Play(_ context.Context, index int) (musicapi.State, error) {
	var err error
	state := thisRef.change(func() {
		t := thisRef.transport
		if index < 0 || index >= t.List.Len() {
			err = ErrNoTrack
			return
		}
		t.List.Select(index)
		t.Pos, t.State, thisRef.err = 0, model.Playing, ""
		thisRef.playback.restart = true
	})
	return state, err
}

func (thisRef *Service) Seek(_ context.Context, positionMS int64) (musicapi.State, error) {
	return thisRef.change(func() {
		t := thisRef.transport
		at := time.Duration(max(positionMS, 0)) * time.Millisecond
		if length := t.Length(); length > 0 {
			at = min(at, length)
		}
		t.Pos = at
	}), nil
}

func (thisRef *Service) SetLevels(_ context.Context, levels musicapi.Levels) (musicapi.State, error) {
	return thisRef.change(func() {
		if !math.IsNaN(levels.Volume) {
			thisRef.transport.SetVolume(float32(levels.Volume))
		}
		if !math.IsNaN(levels.Balance) {
			thisRef.transport.SetPan(float32(levels.Balance))
		}
	}), nil
}

func (thisRef *Service) SetMode(_ context.Context, mode musicapi.Mode) (musicapi.State, error) {
	return thisRef.change(func() {
		t := thisRef.transport
		t.Repeat = repeatOf(mode.Repeat)
		if t.Random != mode.Shuffle {
			t.SetRandom(mode.Shuffle)
			thisRef.queueChanged()
		}
	}), nil
}

func (thisRef *Service) SetEqualizer(_ context.Context, given musicapi.Equalizer) (musicapi.State, error) {
	return thisRef.change(func() { setEqualizer(thisRef.equalizer, given, thisRef.transport.List.Index()) }), nil
}

func (thisRef *Service) SetReplayGain(_ context.Context, gain musicapi.ReplayGain) (musicapi.State, error) {
	return thisRef.change(func() {
		gain = NormalReplayGain(gain)
		if gain != thisRef.gain {
			thisRef.gain = gain
			thisRef.applyReplayGain()
		}
	}), nil
}

func (thisRef *Service) Add(ctx context.Context, add musicapi.Add) (musicapi.Added, error) {
	thisRef.adding.Lock()
	defer thisRef.adding.Unlock()
	if len(add.Paths) == 0 {
		state, _ := thisRef.State(ctx)
		return musicapi.Added{State: state}, nil
	}
	result, err := Expand(ctx, add.Paths)
	if err != nil {
		state := thisRef.change(func() { thisRef.err = err.Error() })
		return musicapi.Added{State: state}, err
	}
	state := thisRef.change(func() {
		t := thisRef.transport
		start := add.Replace || t.List.Len() == 0
		if add.Replace {
			t.List = model.NewPlaylist(result.Tracks...)
			t.List.Shuffle(t.Random)
		} else {
			t.List.Append(result.Tracks, t.Random)
		}
		thisRef.queueChanged()
		thisRef.err = ""
		if start {
			t.Pos, t.State = 0, model.Playing
			thisRef.playback.restart = true
		}
	})
	if skipped := result.Skipped(); skipped != "" {
		// after the load, which clears an error
		state = thisRef.change(func() { thisRef.err = fmt.Sprintf("Added %d tracks; %s", len(result.Tracks), skipped) })
	}
	return musicapi.Added{Tracks: len(result.Tracks), Skipped: result.Skipped(), State: state}, nil
}

func (thisRef *Service) Remove(_ context.Context, indexes []int) (musicapi.State, error) {
	return thisRef.change(func() {
		t := thisRef.transport
		indexes = slices.Clone(indexes)
		slices.Sort(indexes)
		indexes = slices.Compact(indexes)
		current := t.List.Index()
		removedCurrent := false
		for i := len(indexes) - 1; i >= 0; i-- {
			index := indexes[i]
			if index < 0 || index >= t.List.Len() {
				continue
			}
			if index == current {
				removedCurrent = true
			}
			t.List.Remove(index, t.Random)
		}
		thisRef.queueChanged()
		if removedCurrent {
			t.Pos = 0
			thisRef.playback.restart = true // the track after it, from its start
		} else {
			thisRef.playback.index = t.List.Index() // the same track, at its new place
		}
		if t.List.Len() == 0 {
			t.State = model.Stopped
		}
		thisRef.err = ""
	}), nil
}

func (thisRef *Service) Move(_ context.Context, move musicapi.Move) (musicapi.State, error) {
	return thisRef.change(func() {
		t := thisRef.transport
		n := t.List.Len()
		if move.From < 0 || move.From >= n || move.To < 0 || move.To >= n || move.From == move.To {
			return
		}
		current := t.List.Index()
		tracks := slices.Clone(t.List.Tracks)
		moved := tracks[move.From]
		tracks = slices.Delete(tracks, move.From, move.From+1)
		tracks = slices.Insert(tracks, move.To, moved)
		// where the track playing went
		switch {
		case current == move.From:
			current = move.To
		case move.From < current && move.To >= current:
			current--
		case move.From > current && move.To <= current:
			current++
		}
		t.List = model.NewPlaylist(tracks...)
		t.List.Shuffle(t.Random)
		t.List.Select(current)
		thisRef.playback.list, thisRef.playback.index = t.List, current
		thisRef.queueChanged()
	}), nil
}

func (thisRef *Service) Clear(context.Context) (musicapi.State, error) {
	return thisRef.change(func() {
		t := thisRef.transport
		t.List = model.NewPlaylist()
		t.Pos, t.State, thisRef.err = 0, model.Stopped, ""
		thisRef.queueChanged()
	}), nil
}

// ---- between the API's types and the model's -------------------------------

func status(state model.State) string {
	switch state {
	case model.Playing:
		return musicapi.StatusPlaying
	case model.Paused:
		return musicapi.StatusPaused
	}
	return musicapi.StatusStopped
}

func repeat(mode model.Repeat) string {
	switch mode {
	case model.RepeatAll:
		return musicapi.RepeatAll
	case model.RepeatOne:
		return musicapi.RepeatOne
	}
	return musicapi.RepeatOff
}

func repeatOf(name string) model.Repeat {
	switch name {
	case musicapi.RepeatAll:
		return model.RepeatAll
	case musicapi.RepeatOne:
		return model.RepeatOne
	}
	return model.RepeatOff
}

func track(t model.Track) musicapi.Track {
	rate := t.SampleRate
	if rate == 0 {
		rate = t.Rate * 1000
	}
	return musicapi.Track{Path: t.Path, Title: t.Title, Artist: t.Artist, Album: t.Album, LengthMS: t.Length.Milliseconds(),
		Rate: rate, Channels: t.Channels, Bitrate: t.Bitrate}
}

func modelTrack(t musicapi.Track) model.Track {
	return model.Track{Path: t.Path, Title: t.Title, Artist: t.Artist, Album: t.Album, Length: time.Duration(t.LengthMS) * time.Millisecond,
		Kind: "audio", Rate: t.Rate / 1000, SampleRate: t.Rate, Channels: t.Channels, Bitrate: t.Bitrate}
}

func equalizer(e *model.Equalizer) musicapi.Equalizer {
	gains := make([]float64, len(e.Gains))
	for i, gain := range e.Gains {
		gains[i] = float64(gain)
	}
	return musicapi.Equalizer{On: e.On, Auto: e.Auto, Preamp: float64(e.Preamp), Gains: gains, Preset: e.Preset}
}

// setEqualizer gives e the settings given, without telling anyone: a
// preset by its name when no gains are given.
func setEqualizer(e *model.Equalizer, given musicapi.Equalizer, track int) {
	changed := e.Changed
	e.Changed = nil
	defer func() { e.Changed = changed }()
	e.SetOn(given.On)
	if !math.IsNaN(given.Preamp) {
		e.SetPreamp(float32(given.Preamp))
	}
	if len(given.Gains) == 0 && given.Preset != "" {
		e.Apply(given.Preset)
	} else {
		for i, gain := range given.Gains {
			if !math.IsNaN(gain) {
				e.Set(i, float32(gain))
			}
		}
		if given.Preset != "" && slices.Contains(musicapi.Presets, given.Preset) || given.Preset == "Custom" {
			e.Preset = given.Preset
		}
	}
	e.SetAuto(given.Auto, track)
}

// NormalReplayGain returns gain with a mode the player does not know as
// off, none as track gain (the default), and the preamp within its range.
func NormalReplayGain(gain musicapi.ReplayGain) musicapi.ReplayGain {
	switch gain.Mode {
	case "":
		gain.Mode = musicapi.ReplayGainTrack
	case musicapi.ReplayGainOff, musicapi.ReplayGainTrack, musicapi.ReplayGainAlbum:
	default:
		gain.Mode = musicapi.ReplayGainOff
	}
	if !(gain.Preamp >= musicapi.MinReplayGainPreamp) { // NaN too
		gain.Preamp = musicapi.MinReplayGainPreamp
	}
	gain.Preamp = min(gain.Preamp, musicapi.MaxReplayGainPreamp)
	return gain
}
