package shell

import (
	"context"
	"log"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/tags/cover"
	"github.com/codemodify/media-player-music/musicapi"
)

// The player is media-player-musicd's: the shell's model is a copy of it,
// which the faces draw and change. A daemon's state is put into the model
// as it comes (ShowState, ShowQueue, ShowNow), and what a face changed in
// the model since is sent back as the daemon's calls (syncPlayback); the
// daemon's answer is the state that follows.

// synced is the model as the daemon last told it: what a face changes is
// what differs from it.
type synced struct {
	seq     uint64 // the State shown
	queue   uint64 // the Queue shown
	index   int
	pos     time.Duration
	volume  float32
	pan     float32
	eq      musicapi.Equalizer
	state   musicapi.State
	formats string
}

// callTimeout bounds a call to the daemon from the UI's thread.
const callTimeout = 5 * time.Second

func (s *Shell) call(do func(ctx context.Context) (musicapi.State, error)) bool {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	state, err := do(ctx)
	if err != nil {
		s.daemonError(err)
		return false
	}
	s.ShowState(state)
	return true
}

// ShowLink shows how the connection to the daemon stands: text, when it
// is a problem ("can't reach the music daemon…"), in place of the
// daemon's own error; "" once connected.
func (s *Shell) ShowLink(text string, problem bool) {
	if s.closed {
		return
	}
	if problem {
		s.Host.Model.Error = text
	} else {
		s.Host.Model.Error = s.daemon.state.Error
	}
	s.refreshSettings()
	s.refresh()
}

// daemonError shows why a call failed.
func (s *Shell) daemonError(err error) {
	log.Printf("media-player-musicd: %v", err)
	s.Host.Model.Error = err.Error()
	s.refreshSettings()
	s.refresh()
}

// Show puts a daemon's state into the model, by its event's name: for a
// renderer's link and the tests' services, on the UI's thread.
func (s *Shell) Show(event string, value any) {
	switch event {
	case musicapi.StateEvent:
		s.ShowState(value.(musicapi.State))
	case musicapi.QueueEvent:
		s.ShowQueue(value.(musicapi.Queue))
	case musicapi.NowEvent:
		s.ShowNow(value.(musicapi.Now))
	}
}

// ShowState puts the daemon's state into the model; an older one than the
// one shown is let go. A state naming a queue the shell has not had yet
// fetches it.
func (s *Shell) ShowState(state musicapi.State) {
	if s.closed || s.player == nil || state.Seq <= s.daemon.seq {
		return
	}
	s.daemon.seq = state.Seq
	s.daemon.state = state
	s.daemon.formats = state.Formats
	if state.Queue.Version > s.daemon.queue && !s.applying {
		s.fetchQueue()
	}
	s.apply()
}

// fetchQueue asks the daemon for its queue, and shows it.
func (s *Shell) fetchQueue() {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	queue, err := s.player.Queue(ctx)
	if err != nil {
		s.daemonError(err)
		return
	}
	s.ShowQueue(queue)
}

// ShowQueue puts the daemon's queue into the model.
func (s *Shell) ShowQueue(queue musicapi.Queue) {
	if s.closed || s.player == nil || queue.Version <= s.daemon.queue {
		return
	}
	s.daemon.queue = queue.Version
	tracks := make([]players.Track, len(queue.Tracks))
	for i, each := range queue.Tracks {
		tracks[i] = modelTrack(each)
	}
	t := s.Host.Model.Transport
	t.List = players.NewPlaylist(tracks...)
	t.List.Shuffle(t.Random)
	s.apply()
}

// apply puts the daemon's last state into the model, without a face's
// hooks sending it back.
func (s *Shell) apply() {
	s.applying = true
	defer func() { s.applying = false }()
	state, m := s.daemon.state, s.Host.Model
	t := m.Transport
	before, beforeState := t.Track().Path, t.State
	switch state.Status {
	case musicapi.StatusPlaying:
		t.State = players.Playing
	case musicapi.StatusPaused:
		t.State = players.Paused
	default:
		t.State = players.Stopped
	}
	if t.Random != state.Shuffle {
		t.Random = state.Shuffle
		t.List.Shuffle(state.Shuffle)
	}
	if state.Index >= 0 && state.Index < t.List.Len() {
		t.List.Select(state.Index)
		if current := &t.List.Tracks[state.Index]; current.Path == state.Track.Path {
			*current = modelTrack(state.Track)
		}
	}
	t.Pos = time.Duration(state.PositionMS) * time.Millisecond
	t.Volume, t.Pan = float32(state.Volume), float32(state.Balance)
	switch state.Repeat {
	case musicapi.RepeatAll:
		t.Repeat = players.RepeatAll
	case musicapi.RepeatOne:
		t.Repeat = players.RepeatOne
	default:
		t.Repeat = players.RepeatOff
	}
	e := m.Equaliser
	e.On, e.Auto, e.Preamp, e.Preset = state.Equalizer.On, state.Equalizer.Auto, float32(state.Equalizer.Preamp), state.Equalizer.Preset
	for i := range e.Gains {
		if i < len(state.Equalizer.Gains) {
			e.Gains[i] = float32(state.Equalizer.Gains[i])
		}
	}
	m.Error = state.Error
	if path := t.Track().Path; path != before || t.State != beforeState && t.State == players.Stopped {
		m.Spectrum.Reset()
	}
	if path := t.Track().Path; path != "" && t.State != players.Stopped {
		s.loadCover(path)
	}
	s.daemon.index, s.daemon.pos, s.daemon.volume, s.daemon.pan = t.List.Index(), t.Pos, t.Volume, t.Pan
	s.daemon.eq = equalizerOf(e)
	s.refreshSettings()
	s.refresh()
	if s.presentation != nil && t.State == players.Playing {
		s.pulse.Start(s.MainWindow())
	}
}

// ShowNow moves the track's clock and the analyser as the daemon hears
// them; one of an older state than the one shown is let go.
func (s *Shell) ShowNow(now musicapi.Now) {
	if s.closed || s.player == nil || now.Seq < s.daemon.seq {
		return
	}
	m := s.Host.Model
	t := m.Transport
	if now.Index != t.List.Index() || now.Index < 0 {
		return
	}
	t.Pos = time.Duration(now.PositionMS) * time.Millisecond
	s.daemon.pos = t.Pos
	if now.LengthMS > 0 {
		t.List.Tracks[now.Index].Length = time.Duration(now.LengthMS) * time.Millisecond
	}
	if t.State == players.Playing && len(now.Bands) > 0 {
		bands := make([]float32, len(now.Bands))
		for i, band := range now.Bands {
			bands[i] = float32(band)
		}
		m.Spectrum.SetLevels(bands)
	}
	s.refresh()
}

// syncPlayback sends the daemon what a face changed in the model since the
// daemon last told it: a track chosen, a seek, the levels, the equaliser.
func (s *Shell) syncPlayback() {
	if s.closed || s.player == nil || s.applying {
		return
	}
	// what the faces asked, taken whole first: each answer is the daemon's
	// state, put into the model, and would undo what is not sent yet
	m, t := s.Host.Model, s.Host.Model.Transport
	index, pos, volume, pan, eq := t.List.Index(), t.Pos, t.Volume, t.Pan, equalizerOf(m.Equaliser)
	chose, sought := index >= 0 && index != s.daemon.index, pos != s.daemon.pos
	levels := volume != s.daemon.volume || pan != s.daemon.pan
	equalised := !sameEqualizer(eq, s.daemon.eq)
	if chose {
		if !s.call(func(ctx context.Context) (musicapi.State, error) { return s.player.Play(ctx, index) }) {
			return
		}
	} else if sought {
		if !s.call(func(ctx context.Context) (musicapi.State, error) { return s.player.Seek(ctx, pos.Milliseconds()) }) {
			return
		}
	}
	if levels {
		given := musicapi.Levels{Volume: float64(volume), Balance: float64(pan)}
		if !s.call(func(ctx context.Context) (musicapi.State, error) { return s.player.SetLevels(ctx, given) }) {
			return
		}
	}
	if equalised {
		s.call(func(ctx context.Context) (musicapi.State, error) { return s.player.SetEqualizer(ctx, eq) })
	}
}

// command is a model command's name in the daemon's API.
func command(c players.Command) string {
	switch c {
	case players.CmdPlay:
		return musicapi.CommandPlay
	case players.CmdPlayPause:
		return musicapi.CommandPlayPause
	case players.CmdStop:
		return musicapi.CommandStop
	case players.CmdNext:
		return musicapi.CommandNext
	case players.CmdPrev:
		return musicapi.CommandPrevious
	case players.CmdSeekBack:
		return musicapi.CommandSeekBack
	case players.CmdSeekForward:
		return musicapi.CommandSeekForward
	case players.CmdVolumeUp:
		return musicapi.CommandVolumeUp
	case players.CmdVolumeDown:
		return musicapi.CommandVolumeDown
	case players.CmdMute:
		return musicapi.CommandMute
	case players.CmdShuffle:
		return musicapi.CommandShuffle
	case players.CmdRepeat:
		return musicapi.CommandRepeat
	}
	return ""
}

func modelTrack(t musicapi.Track) players.Track {
	return players.Track{Path: t.Path, Title: t.Title, Artist: t.Artist, Album: t.Album, Length: time.Duration(t.LengthMS) * time.Millisecond,
		Kind: "audio", Rate: t.Rate / 1000, SampleRate: t.Rate, Channels: t.Channels, Bitrate: t.Bitrate,
		AlbumArtist: t.AlbumArtist, Genre: t.Genre, Year: t.Year, Number: t.Number, Disc: t.Disc}
}

func equalizerOf(e *players.Equalizer) musicapi.Equalizer {
	gains := make([]float64, len(e.Gains))
	for i, gain := range e.Gains {
		gains[i] = float64(gain)
	}
	return musicapi.Equalizer{On: e.On, Auto: e.Auto, Preamp: float64(e.Preamp), Gains: gains, Preset: e.Preset}
}

func sameEqualizer(a, b musicapi.Equalizer) bool {
	if a.On != b.On || a.Auto != b.Auto || a.Preamp != b.Preamp || a.Preset != b.Preset || len(a.Gains) != len(b.Gains) {
		return false
	}
	for i := range a.Gains {
		if a.Gains[i] != b.Gains[i] {
			return false
		}
	}
	return true
}

// loadCover reads the front cover of the file playing off the UI thread,
// for the now-playing display (ID3's APIC: MP3 and AAC files). The file is
// read where the shell runs: a daemon's on another machine has none here.
func (s *Shell) loadCover(path string) {
	m := s.Host.Model
	if m.CoverPath == path {
		return
	}
	m.CoverPath, m.Cover = path, nil
	if !cover.Tagged(path) {
		return
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		picture := cover.Of(path, coverSide)
		if picture == nil {
			return
		}
		s.App.Post(func() {
			if !s.closed && s.Host.Model.CoverPath == path {
				s.Host.Model.Cover = picture
				s.refresh()
			}
		})
	}()
}

// coverSide is the largest side a cover is kept at: the display draws it
// at about 100 pixels, 175 at 1.75 times.
const coverSide = 256
