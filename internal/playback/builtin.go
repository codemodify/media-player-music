package playback

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"goframework.io/audio-dsp/eq"
	"goframework.io/audio-dsp/spectrum"
	inproc "goframework.io/audio-playback"
	// Every format audio-playback plays: MP3 (codecs-audio-mp3: .mp3,
	// .mp2, and MP3 in M4A and MKV), AAC (codecs-audio-aac: .aac, and AAC
	// LC, HE-AAC and HE-AAC v2 in M4A and MKV), FLAC, Ogg Vorbis and Opus,
	// ALAC, WAV, AIFF, AU.
	_ "goframework.io/audio-playback/all"
	// ID3 tags (v1, v2.2 to 2.4) of MP3 and ADTS files in the stream's
	// Tags: the now-playing title, artist and album.
	_ "goframework.io/codecs-container-id3"
	sound "goframework.io/system-audio-spec"
)

// BuiltinID names the engine that plays in the player's own process, on
// goframework's audio-playback, through the system's sound
// (system-audio-all): nothing to install.
const BuiltinID = "builtin"

// BuiltinCapabilities are the built-in engine's: audio-playback's
// equaliser, balance and analyser.
var BuiltinCapabilities = Capabilities{Seek: true, Equalizer: true, Balance: true, Spectrum: true,
	Formats: "MP3, AAC (M4A, .aac), FLAC, Ogg Vorbis and Opus, ALAC, WAV, AIFF, AU"}

// BuiltinOptions make a built-in engine.
type BuiltinOptions struct {
	// System is the sound system to play through: the running system's
	// (system-audio-all) when nil. Tests give audiotest's.
	System sound.System
	// Name names the sound stream in the system's mixer.
	Name string
}

// Builtin is the Engine on goframework's audio-playback. Each load makes a
// player of one track, as the frontend owns the queue: a load replaces the
// player, so nothing of an old load reaches the new one. Commands run in
// order on one goroutine of the engine's, off the UI thread (opening the
// sound stream takes a moment).
type Builtin struct {
	opts    BuiltinOptions
	broker  *updateBroker
	lock    sync.Mutex
	work    []Command
	closed  bool
	wake    chan struct{}
	quit    chan struct{}
	done    chan struct{}
	current atomic.Pointer[builtinLoad] // the load whose events count

	// The worker's own.
	load   *builtinLoad
	volume float32
	dsp    DSP
	levels []float64
	bars   []float32
}

// builtinLoad is one load: its player and what is waiting for it.
type builtinLoad struct {
	generation uint64
	player     *inproc.Player
	epoch      atomic.Int64
	// The last seek asked (the load's position too): its time and epoch,
	// for the player's Seeked event, which comes as it is heard.
	seekAt    atomic.Int64 // a time.Duration
	seekEpoch atomic.Int64
	// The worker's own.
	seekTarget time.Duration // the last seek made, until it is heard
	seekSince  time.Time
	bitrate    atomic.Bool // a bitrate was reported
}

// NewBuiltin returns a built-in engine; nothing is opened until a load.
func NewBuiltin(opts BuiltinOptions) *Builtin {
	if opts.Name == "" {
		opts.Name = "Music Player"
	}
	b := &Builtin{opts: opts, broker: newUpdateBroker(), wake: make(chan struct{}, 1), quit: make(chan struct{}), done: make(chan struct{}), volume: 1}
	go b.run()
	return b
}

func (b *Builtin) Events() <-chan Update      { return b.broker.out }
func (b *Builtin) Capabilities() Capabilities { return BuiltinCapabilities }

// Apply queues a command for the engine's goroutine; it never waits.
func (b *Builtin) Apply(command Command) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed {
		return ErrClosed
	}
	b.work = append(b.work, command)
	select {
	case b.wake <- struct{}{}:
	default:
	}
	return nil
}

// Close stops what plays and closes the sound stream.
func (b *Builtin) Close() error {
	b.lock.Lock()
	if b.closed {
		b.lock.Unlock()
		return nil
	}
	b.closed = true
	b.lock.Unlock()
	close(b.quit)
	<-b.done
	return nil
}

func (b *Builtin) run() {
	defer close(b.done)
	defer b.broker.close()
	defer b.drop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-b.quit:
			return
		case <-b.wake:
			b.lock.Lock()
			work := b.work
			b.work = nil
			b.lock.Unlock()
			for _, command := range work {
				if err := b.execute(command); err != nil {
					b.broker.offer(Update{Kind: UpdateError, Generation: command.Generation, Error: err.Error()})
				}
			}
		case <-ticker.C:
			b.poll()
		}
	}
}

// mine reports whether a command concerns the load playing: generation 0
// concerns whatever plays.
func (b *Builtin) mine(command Command) bool {
	return b.load != nil && (command.Generation == 0 || command.Generation == b.load.generation)
}

func (b *Builtin) execute(command Command) error {
	switch command.Op {
	case OpLoad:
		b.drop()
		l := &builtinLoad{generation: command.Generation}
		l.epoch.Store(command.Epoch)
		p, err := inproc.New(inproc.Settings{System: b.opts.System, Name: b.opts.Name, Events: func(event inproc.Event) { b.event(l, event) }})
		if err != nil {
			return err
		}
		l.player = p
		p.SetVolume(gain(b.volume))
		if err := b.filters(p); err != nil {
			p.Close()
			return err
		}
		p.Add(inproc.File(command.Path))
		b.load = l
		b.current.Store(l)
		// The track from the load's position (0 its start), Seeked
		// reported as it is heard there.
		at := max(0, command.Position)
		l.seekAt.Store(int64(at))
		l.seekEpoch.Store(command.Epoch)
		if at > 0 {
			l.seekTarget, l.seekSince = at, time.Now()
		}
		if err := p.JumpAt(0, at); err != nil {
			b.drop()
			return err
		}
		if command.Paused {
			p.Pause()
		}
		return nil
	case OpPause:
		if !b.mine(command) {
			return nil
		}
		if command.Paused {
			return b.load.player.Pause()
		}
		return b.load.player.Play()
	case OpStop:
		if command.Generation == 0 || b.mine(command) {
			b.drop()
		}
		return nil
	case OpSeek:
		if !b.mine(command) {
			return nil
		}
		if command.Position < 0 {
			return errors.New("built-in: negative seek position")
		}
		l := b.load
		l.epoch.Store(command.Epoch)
		l.seekAt.Store(int64(command.Position))
		l.seekEpoch.Store(command.Epoch)
		// The track the load opens, though it has not opened yet.
		if err := l.player.Seek(command.Position); err != nil {
			if errors.Is(err, inproc.ErrNoTrack) {
				return nil // it failed to open: its own event says so
			}
			return err
		}
		l.seekTarget, l.seekSince = command.Position, time.Now()
		return nil
	case OpVolume:
		b.volume = max(0, min(1, command.Volume))
		if b.load != nil {
			b.load.player.SetVolume(gain(b.volume))
		}
		return nil
	case OpDSP:
		b.dsp = command.DSP
		if b.load != nil {
			b.load.epoch.Store(command.Epoch)
			return b.filters(b.load.player)
		}
		return nil
	default:
		return errors.New("built-in: unsupported operation")
	}
}

// gain is the volume's: the cube of the slider, as mpv's volume is, so
// that a position sounds as loud as it did with mpv.
func gain(volume float32) float64 {
	v := float64(volume)
	return v * v * v
}

// filters sets the balance and the equaliser.
func (b *Builtin) filters(p *inproc.Player) error {
	p.SetBalance(float64(b.dsp.Balance))
	if !b.dsp.Enabled {
		return p.SetEqualizer(0, nil)
	}
	var gains [10]float64
	for i, g := range b.dsp.Gains {
		gains[i] = float64(g)
	}
	return p.SetEqualizer(float64(b.dsp.Preamp), eq.Graphic(gains))
}

// drop closes the load's player.
func (b *Builtin) drop() {
	if b.load == nil {
		return
	}
	b.current.Store(nil)
	b.load.player.Close()
	b.load = nil
}

// poll reports where the track is heard and the analyser's bands.
func (b *Builtin) poll() {
	l := b.load
	if l == nil {
		return
	}
	position := l.player.Position()
	if position.Track != 0 {
		return
	}
	if !l.seekSince.IsZero() {
		// What was heard before a seek is not reported as where it is.
		if d := position.At - l.seekTarget; (d < -100*time.Millisecond || d > 500*time.Millisecond) && time.Since(l.seekSince) < time.Second {
			return
		}
		l.seekSince = time.Time{}
	}
	epoch := l.epoch.Load()
	b.broker.offer(Update{Kind: UpdatePosition, Generation: l.generation, Epoch: epoch, Position: position.At})
	if position.Bitrate > 0 && !l.bitrate.Swap(true) {
		b.broker.offer(Update{Kind: UpdateAudioFormat, Generation: l.generation, Info: TrackInfo{Bitrate: position.Bitrate}})
	}
	if !position.Playing {
		return
	}
	b.levels = l.player.Spectrum(b.levels[:0])
	if len(b.levels) == 0 {
		return
	}
	// The analyser's dBov onto 0..1 from -60 dB, as mpv's bands were.
	b.bars = spectrum.Normalize(b.bars, b.levels, -60)
	b.broker.offer(Update{Kind: UpdateSpectrum, Generation: l.generation, Epoch: epoch, Position: position.At, Bands: b.bars})
}

// event translates a player's event, on its goroutine; a replaced load's
// are dropped.
func (b *Builtin) event(l *builtinLoad, event inproc.Event) {
	if b.current.Load() != l {
		return
	}
	switch event.Kind {
	case inproc.Loaded:
		info := event.Info
		b.broker.offer(Update{Kind: UpdateLoaded, Generation: l.generation})
		if info.Duration > 0 {
			b.broker.offer(Update{Kind: UpdateDuration, Generation: l.generation, Duration: info.Duration})
		}
		if info.Bitrate > 0 {
			l.bitrate.Store(true) // the header's stated bitrate, kept
		}
		b.broker.offer(Update{Kind: UpdateAudioFormat, Generation: l.generation, Info: TrackInfo{SampleRate: info.Format.Rate, Channels: info.Format.Channels, Bitrate: info.Bitrate}})
		if meta := (TrackInfo{Title: info.Tags["title"], Artist: info.Tags["artist"], Album: info.Tags["album"]}); meta != (TrackInfo{}) {
			b.broker.offer(Update{Kind: UpdateMetadata, Generation: l.generation, Info: meta})
		}
	case inproc.Seeked:
		// What the last seek asked is heard (one superseded before it was
		// heard gives none).
		if int64(event.At) == l.seekAt.Load() {
			b.broker.offer(Update{Kind: UpdateSeeked, Generation: l.generation, Epoch: l.seekEpoch.Load(), Position: event.At})
		}
	case inproc.Ended:
		b.broker.offer(Update{Kind: UpdateEnded, Generation: l.generation})
	case inproc.Failed:
		message := "the file could not be played"
		if event.Err != nil {
			message = event.Err.Error()
		}
		if event.Track < 0 {
			// The sound stream itself: the engine has to be chosen again.
			b.broker.offer(Update{Kind: UpdateError, Error: "the sound output stopped: " + message})
			return
		}
		b.broker.offer(Update{Kind: UpdateError, Generation: l.generation, Error: message})
	}
}
