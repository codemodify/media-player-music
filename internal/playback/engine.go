package playback

import "time"

// Engine controls a separately installed playback executable. Implementations
// own process startup, protocol translation and decoding; the frontend owns the
// playlist and transport intent. Apply queues work without blocking the UI.
type Engine interface {
	Apply(Command) error
	Events() <-chan Update
	Capabilities() Capabilities
	Close() error
}

type EngineOptions struct {
	Executable     string
	ExtraArgs      []string
	StartupTimeout time.Duration
}

type Capabilities struct {
	Seek      bool
	Equalizer bool // the player's ten frequency bands and preamp
	Balance   bool
	Spectrum  bool   // passive measurements for our own visualizer
	Formats   string // human-readable supported format scope
}

type Operation uint8

const (
	OpLoad Operation = iota + 1
	OpPause
	OpStop
	OpSeek
	OpVolume
	OpDSP
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
	Epoch      int64
}

type DSP struct {
	Balance float32 // -1..1
	Enabled bool
	Preamp  float32     // dB
	Gains   [10]float32 // 31,62,125,250,500,1000,2000,4000,8000,16000 Hz
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
