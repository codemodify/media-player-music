// Package playback plays the queue's tracks in the player's own process, on
// goframework's audio-playback (Builtin). The frontend owns the playlist and
// the transport's intent; the engine owns decoding and the sound stream, and
// reports what is heard as Updates.
package playback

import (
	"errors"
	"time"
)

// ErrClosed is a command given to a closed engine.
var ErrClosed = errors.New("playback: the engine is closed")

// Engine is what the shell drives: Builtin, or a test's fake. Apply queues
// work without blocking the UI.
type Engine interface {
	Apply(Command) error
	Events() <-chan Update
	Close() error
}

type Operation uint8

const (
	OpLoad Operation = iota + 1
	OpPause
	OpStop
	OpSeek
	OpVolume
	OpDSP
	OpReplayGain // the ReplayGain settings, heard from the next load
)

// Generation identifies a load, including repeated loads of the same path.
// Epoch identifies analyzer/seek state within that load. Load and seek report
// completion asynchronously; old generations must never affect a new track.
// Stop with Generation zero stops the active audio regardless of load identity.
type Command struct {
	Op         Operation
	Generation uint64
	Path       string
	Position   time.Duration
	Paused     bool
	Volume     float32 // 0..1
	DSP        DSP
	ReplayGain ReplayGain
	Epoch      int64
}

type DSP struct {
	Balance float32 // -1..1
	Enabled bool
	Preamp  float32     // dB
	Gains   [10]float32 // 31,62,125,250,500,1000,2000,4000,8000,16000 Hz
}

// ReplayGainMode chooses the tags' gain a track plays at.
type ReplayGainMode uint8

const (
	ReplayGainOff ReplayGainMode = iota
	ReplayGainTrack
	ReplayGainAlbum
)

// ReplayGain is how tracks are levelled by their ReplayGain tags: the mode
// (a track without the mode's gain takes the other one), and a preamp added
// to the tags' gain. The tags' peak always keeps the gain from clipping.
type ReplayGain struct {
	Mode   ReplayGainMode
	Preamp float64 // dB
}

type UpdateKind uint8

const (
	UpdateLoaded UpdateKind = iota + 1
	UpdatePosition
	UpdateDuration
	UpdateMetadata
	UpdateAudioFormat
	UpdateSeeked
	UpdateEnded // natural EOF only; Stop and replacement do not advance the queue
	UpdateError
	UpdateSpectrum
)

type TrackInfo struct {
	Title      string
	Artist     string
	Album      string
	SampleRate int // Hz
	Channels   int
	Bitrate    int
}

type Update struct {
	Kind       UpdateKind
	Generation uint64
	Position   time.Duration
	Duration   time.Duration
	Info       TrackInfo
	Bands      []float32
	Epoch      int64
	Error      string
}
