package service

import (
	"log"
	"math"
	"time"

	"github.com/codemodify/media-player-music/internal/model"
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/musicapi"
)

// playbackState is what the engine was last told: the transport's intent
// and the queue belong to the service, the engine supplies the actual
// audio clock. Everything here is the service's, under its lock.
type playbackState struct {
	initialized   bool
	path          string
	list          *model.Playlist
	index         int
	restart       bool // the track chosen plays from its start, as a new load, even when it is the one loaded
	state         model.State
	position      time.Duration
	volume        float32
	dsp           playback.DSP
	generation    uint64
	seeking       bool
	loading       bool
	seekOnLoad    time.Duration
	loadPosition  time.Duration
	spectrumEpoch int64
}

func (thisRef *Service) apply(command playback.Command) bool {
	if err := thisRef.engine.Apply(command); err != nil {
		thisRef.playbackError(err.Error())
		return false
	}
	return true
}

// sync tells the engine what the transport now asks of it: a load, a
// pause, a seek, the levels and the filters, whatever changed since it was
// last told.
func (thisRef *Service) sync() {
	if thisRef.closed {
		return
	}
	p, t := &thisRef.playback, thisRef.transport
	if !p.initialized || p.volume != t.Volume {
		if !thisRef.apply(playback.Command{Op: playback.OpVolume, Volume: t.Volume, Generation: p.generation}) {
			return
		}
		p.volume = t.Volume
	}
	path := t.Track().Path
	newLoad := t.State != model.Stopped && path != "" && (p.restart || path != p.path || p.list != t.List || p.index != t.List.Index())
	newSeek := t.State != model.Stopped && path != "" && !newLoad && !p.loading && p.position != t.Pos
	p.restart = false
	if newLoad || newSeek {
		thisRef.resetSpectrum()
	}
	if !thisRef.syncFilters() {
		return
	}
	p.initialized = true
	if t.State == model.Stopped || path == "" {
		if p.path != "" {
			thisRef.apply(playback.Command{Op: playback.OpStop, Generation: p.generation})
			p.generation++
		}
		p.path, p.list, p.state, p.position, p.seeking, p.loading = "", t.List, model.Stopped, 0, false, false
		thisRef.bands = nil
		return
	}
	if newLoad {
		thisRef.err = ""
		thisRef.equalizer.AutoForTrack(t.List.Index())
		thisRef.syncFilters()
		p.generation++
		p.path, p.list, p.index, p.state, p.position = path, t.List, t.List.Index(), t.State, t.Pos
		p.loading, p.seeking, p.seekOnLoad, p.loadPosition = true, t.Pos > 0, t.Pos, t.Pos
		thisRef.apply(playback.Command{Op: playback.OpLoad, Generation: p.generation, Path: path, Position: t.Pos, Paused: t.State == model.Paused, Epoch: p.spectrumEpoch})
		return
	}
	if p.state != t.State {
		thisRef.bands = nil // what was measured before a pause is not shown after it
		if !thisRef.apply(playback.Command{Op: playback.OpPause, Generation: p.generation, Paused: t.State == model.Paused}) {
			return
		}
		p.state = t.State
	}
	if p.position != t.Pos {
		if p.loading {
			p.seekOnLoad, p.position = t.Pos, t.Pos
			return
		}
		if !thisRef.apply(playback.Command{Op: playback.OpSeek, Generation: p.generation, Position: t.Pos, Epoch: p.spectrumEpoch}) {
			return
		}
		p.position, p.seeking = t.Pos, true
	}
}

func (thisRef *Service) syncFilters() bool {
	p, t, e := &thisRef.playback, thisRef.transport, thisRef.equalizer
	dsp := playback.DSP{Balance: t.Pan, Enabled: e.On, Preamp: e.Preamp}
	copy(dsp.Gains[:], e.Gains)
	if p.initialized && p.dsp == dsp {
		return true
	}
	if !thisRef.apply(playback.Command{Op: playback.OpDSP, Generation: p.generation, DSP: dsp, Epoch: p.spectrumEpoch}) {
		return false
	}
	p.dsp = dsp
	return true
}

// applyReplayGain gives the engine the ReplayGain settings.
func (thisRef *Service) applyReplayGain() {
	command := playback.Command{Op: playback.OpReplayGain, ReplayGain: playback.ReplayGain{Preamp: thisRef.gain.Preamp}}
	switch thisRef.gain.Mode {
	case musicapi.ReplayGainTrack:
		command.ReplayGain.Mode = playback.ReplayGainTrack
	case musicapi.ReplayGainAlbum:
		command.ReplayGain.Mode = playback.ReplayGainAlbum
	}
	thisRef.apply(command)
}

func (thisRef *Service) resetSpectrum() {
	thisRef.playback.spectrumEpoch++
	thisRef.bands = nil
}

// engineEvent takes an update of the engine's, under the lock: the clock,
// what the decoder found, the end of a track.
func (thisRef *Service) engineEvent(event playback.Update) {
	p, t := &thisRef.playback, thisRef.transport
	if event.Generation != 0 && event.Generation != p.generation {
		return
	}
	if event.Kind == playback.UpdateError {
		thisRef.playbackError(event.Error)
		thisRef.publishLocked()
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
			thisRef.resetSpectrum()
			if thisRef.apply(playback.Command{Op: playback.OpSeek, Generation: p.generation, Position: p.seekOnLoad, Epoch: p.spectrumEpoch}) {
				p.seeking = true
			}
		}
		return
	case playback.UpdateSeeked:
		if event.Epoch == 0 || event.Epoch == p.spectrumEpoch {
			p.seeking = false
		}
		return
	case playback.UpdatePosition:
		if !p.loading && !p.seeking && (event.Epoch == 0 || event.Epoch == p.spectrumEpoch) && event.Position >= 0 {
			t.Pos, p.position = event.Position, event.Position
			thisRef.publishNowLocked()
			thisRef.keepPosition()
		}
		return
	case playback.UpdateSpectrum:
		if t.State == model.Playing && !p.loading && !p.seeking && event.Epoch == p.spectrumEpoch && len(event.Bands) > 0 && math.Abs((event.Position-t.Pos).Seconds()) <= 1.5 {
			bands := make([]float64, len(event.Bands))
			for i, band := range event.Bands {
				bands[i] = float64(band)
			}
			thisRef.bands = bands
			thisRef.publishNowLocked()
		}
		return
	case playback.UpdateDuration:
		if event.Duration < 0 {
			return
		}
		track.Length = event.Duration
	case playback.UpdateMetadata:
		if event.Info.Title != "" {
			track.Title = event.Info.Title
		}
		if event.Info.Artist != "" || event.Info.Album != "" {
			track.Artist, track.Album = event.Info.Artist, event.Info.Album
		}
	case playback.UpdateAudioFormat:
		if event.Info.SampleRate > 0 {
			track.Rate, track.SampleRate = event.Info.SampleRate/1000, event.Info.SampleRate
		}
		if event.Info.Channels > 0 {
			track.Channels = event.Info.Channels
		}
		if event.Info.Bitrate > 0 {
			track.Bitrate = event.Info.Bitrate
		}
	case playback.UpdateEnded:
		t.Pos = 0
		if t.State != model.Playing {
			t.State = model.Stopped
		} else if t.Repeat != model.RepeatOne {
			if t.List.Advance(1) && t.Repeat == model.RepeatOff {
				t.State = model.Stopped
			}
		}
		p.path = ""
		thisRef.sync()
		thisRef.savedAt = -1
		if thisRef.changed != nil {
			go thisRef.changed() // never under the lock
		}
	default:
		return
	}
	thisRef.publishLocked()
}

// keepPosition says what is kept changed when the track has moved five
// seconds since it was last kept: a run cut short starts again near where
// it was, without a write at each tick.
func (thisRef *Service) keepPosition() {
	if d := thisRef.transport.Pos - thisRef.savedAt; d > 5*time.Second || d < -5*time.Second {
		thisRef.savedAt = thisRef.transport.Pos
		if thisRef.changed != nil {
			go thisRef.changed()
		}
	}
}

func (thisRef *Service) playbackError(message string) {
	log.Printf("playback: %s", message)
	// A rejected load may never have adopted the generation: stop the
	// active audio regardless of that failed request's identity.
	_ = thisRef.engine.Apply(playback.Command{Op: playback.OpStop})
	p := &thisRef.playback
	p.path, p.state, p.position, p.loading = "", model.Stopped, 0, false
	p.initialized = false
	p.generation++
	thisRef.err = message
	thisRef.transport.State = model.Stopped
	thisRef.transport.Pos = 0
	thisRef.bands = nil
}
