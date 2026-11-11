package shell

import (
	"errors"
	"log"
	"math"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/tags"
)

// Transport intent and queue state belong to the frontend. Engine adapters
// supply the actual audio clock and translate their own command/event protocol.
type playbackState struct {
	initialized   bool
	path          string
	list          *players.Playlist
	index         int
	state         players.State
	position      time.Duration
	volume        float32
	dsp           playback.DSP
	generation    uint64
	seeking       bool
	loading       bool
	seekOnLoad    time.Duration
	loadPosition  time.Duration
	syncing       bool
	spectrumEpoch int64
}

func (s *Shell) startPlaybackEvents() {
	s.eventStop, s.eventDone = make(chan struct{}), make(chan struct{})
	engine, stop, done, epoch := s.engine, s.eventStop, s.eventDone, s.engineEpoch
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				for range engine.Events() {
				}
				return
			case event, ok := <-engine.Events():
				if !ok {
					return
				}
				s.App.Post(func() {
					if !s.closed && s.engineEpoch == epoch {
						s.engineEvent(event)
					}
				})
			}
		}
	}()
}

func (s *Shell) apply(command playback.Command) bool {
	if s.engine == nil {
		return false
	}
	if err := s.engine.Apply(command); err != nil {
		if errors.Is(err, playback.ErrClosed) {
			s.backendError = err.Error()
			s.refreshSettings()
		}
		s.playbackError(err.Error())
		return false
	}
	return true
}

func (s *Shell) syncPlayback() {
	if s.closed || s.backendTransition || s.playback.syncing {
		return
	}
	if s.engine == nil {
		if !s.Host.Model.Live {
			return
		}
		if s.Host.Model.Transport.State != players.Stopped {
			s.Host.Model.Transport.State = players.Stopped
			s.Host.Model.Error = "Choose an installed playback backend in Settings"
			s.refresh()
		}
		return
	}
	p := &s.playback
	p.syncing = true
	defer func() { p.syncing = false }()
	m, t := s.Host.Model, s.Host.Model.Transport
	if !p.initialized || p.volume != t.Volume {
		if !s.apply(playback.Command{Op: playback.OpVolume, Volume: t.Volume, Generation: p.generation}) {
			return
		}
		p.volume = t.Volume
	}
	path := t.Track().Path
	newLoad := t.State != players.Stopped && path != "" && (path != p.path || p.list != t.List || p.index != t.List.Index())
	newSeek := t.State != players.Stopped && path != "" && !newLoad && !p.loading && p.position != t.Pos
	if newLoad || newSeek {
		s.resetSpectrum()
	}
	if !s.syncAudioFilters() {
		return
	}
	p.initialized = true
	if t.State == players.Stopped || path == "" {
		if p.path != "" {
			s.apply(playback.Command{Op: playback.OpStop, Generation: p.generation})
			p.generation++
		}
		p.path, p.list, p.state, p.position, p.seeking, p.loading = "", t.List, players.Stopped, 0, false, false
		m.Spectrum.Reset()
		return
	}
	if newLoad {
		m.Error = ""
		s.loadCover(path)
		p.generation++
		p.path, p.list, p.index, p.state, p.position = path, t.List, t.List.Index(), t.State, t.Pos
		p.loading, p.seeking, p.seekOnLoad, p.loadPosition = true, t.Pos > 0, t.Pos, t.Pos
		if !s.apply(playback.Command{Op: playback.OpLoad, Generation: p.generation, Path: path, Position: t.Pos, Paused: t.State == players.Paused, Epoch: p.spectrumEpoch}) {
			return
		}
	} else {
		if p.state != t.State {
			if !s.apply(playback.Command{Op: playback.OpPause, Generation: p.generation, Paused: t.State == players.Paused}) {
				return
			}
			p.state = t.State
		}
		if p.position != t.Pos {
			if p.loading {
				p.seekOnLoad, p.position = t.Pos, t.Pos
				return
			}
			if !s.apply(playback.Command{Op: playback.OpSeek, Generation: p.generation, Position: t.Pos, Epoch: p.spectrumEpoch}) {
				return
			}
			p.position, p.seeking = t.Pos, true
		}
	}
	if s.presentation != nil {
		s.pulse.Start(s.MainWindow())
	}
}

func (s *Shell) syncAudioFilters() bool {
	p, m := &s.playback, s.Host.Model
	dsp := playback.DSP{Balance: m.Transport.Pan, Enabled: m.Equaliser.On, Preamp: m.Equaliser.Preamp}
	copy(dsp.Gains[:], m.Equaliser.Gains)
	if p.initialized && p.dsp == dsp {
		return true
	}
	intended := dsp
	caps := s.engine.Capabilities()
	if !caps.Balance {
		dsp.Balance = 0
	}
	if !caps.Equalizer {
		dsp.Enabled = false
		dsp.Preamp = 0
		dsp.Gains = [10]float32{}
	}
	if caps.Equalizer || caps.Balance || caps.Spectrum {
		if !s.apply(playback.Command{Op: playback.OpDSP, Generation: p.generation, DSP: dsp, Epoch: p.spectrumEpoch}) {
			return false
		}
	}
	// Preserve intended DSP across engines, even if one cannot apply it.
	p.dsp = intended
	return true
}

// loadCover reads the front cover of the file starting off the UI thread,
// for the now-playing display (ID3's APIC: MP3 and AAC files).
func (s *Shell) loadCover(path string) {
	m := s.Host.Model
	if m.CoverPath == path {
		return
	}
	m.CoverPath, m.Cover = path, nil
	if !tags.Tagged(path) {
		return
	}
	s.backendWorkers.Add(1)
	go func() {
		defer s.backendWorkers.Done()
		cover := tags.Cover(path, coverSide)
		if cover == nil {
			return
		}
		s.App.Post(func() {
			if !s.closed && s.Host.Model.CoverPath == path {
				s.Host.Model.Cover = cover
				s.refresh()
			}
		})
	}()
}

// coverSide is the largest side a cover is kept at: the display draws it
// at about 100 pixels, 175 at 1.75 times.
const coverSide = 256

func (s *Shell) resetSpectrum() { s.playback.spectrumEpoch++; s.Host.Model.Spectrum.Reset() }

func (s *Shell) engineEvent(event playback.Update) {
	p, t := &s.playback, s.Host.Model.Transport
	if event.Generation != 0 && event.Generation != p.generation {
		return
	}
	if event.Kind == playback.UpdateError {
		if event.Generation == 0 {
			s.backendError = event.Error
			s.refreshSettings()
		}
		s.playbackError(event.Error)
		return
	}
	if p.path == "" || p.generation == 0 {
		return
	}
	i := t.List.Index()
	if i < 0 {
		return
	}
	track := &t.List.Tracks[i]
	switch event.Kind {
	case playback.UpdateLoaded:
		p.loading = false
		if p.seekOnLoad != p.loadPosition {
			s.resetSpectrum()
			if s.apply(playback.Command{Op: playback.OpSeek, Generation: p.generation, Position: p.seekOnLoad, Epoch: p.spectrumEpoch}) {
				p.seeking = true
			}
		}
	case playback.UpdateSeeked:
		if event.Epoch != 0 && event.Epoch != p.spectrumEpoch {
			return
		}
		p.seeking = false
	case playback.UpdatePosition:
		if !p.loading && !p.seeking && (event.Epoch == 0 || event.Epoch == p.spectrumEpoch) && event.Position >= 0 {
			t.Pos, p.position = event.Position, event.Position
		}
	case playback.UpdateDuration:
		if event.Duration >= 0 {
			track.Length = event.Duration
		}
	case playback.UpdateMetadata:
		if event.Info.Title != "" {
			track.Title = event.Info.Title
		}
		track.Artist, track.Album = event.Info.Artist, event.Info.Album
	case playback.UpdateAudioFormat:
		if event.Info.SampleRate > 0 {
			track.Rate = event.Info.SampleRate / 1000
		}
		if event.Info.Channels > 0 {
			track.Channels = event.Info.Channels
		}
		if event.Info.Bitrate > 0 {
			track.Bitrate = event.Info.Bitrate
		}
	case playback.UpdateSpectrum:
		if t.State == players.Playing && !p.loading && !p.seeking && event.Epoch == p.spectrumEpoch && len(event.Bands) > 0 && math.Abs((event.Position-t.Pos).Seconds()) <= 1.5 {
			s.Host.Model.Spectrum.SetLevels(event.Bands)
		}
	case playback.UpdateEnded:
		t.Pos = 0
		if t.State != players.Playing {
			t.State = players.Stopped
		} else if t.Repeat != players.RepeatOne {
			if t.List.Advance(1) && t.Repeat == players.RepeatOff {
				t.State = players.Stopped
			}
		}
		p.path = ""
		s.syncPlayback()
	}
	s.refresh()
}

func (s *Shell) playbackError(message string) {
	log.Printf("playback (%s): %s", s.backendID, message)
	if s.engine != nil {
		// A rejected load may never have adopted the frontend generation.
		// Stop the active audio regardless of that failed request's identity.
		_ = s.engine.Apply(playback.Command{Op: playback.OpStop})
	}
	s.playback.path, s.playback.state, s.playback.position, s.playback.loading = "", players.Stopped, 0, false
	s.playback.initialized = false
	s.playback.generation++
	s.Host.Model.Error = message
	s.Host.Model.Transport.State = players.Stopped
	s.Host.Model.Transport.Pos = 0
	s.Host.Model.Spectrum.Reset()
	s.refreshSettings()
	s.refresh()
}
