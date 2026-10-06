package playback

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// MPVEngine isolates JSON IPC, entry correlation and FFmpeg filter graphs from
// the frontend. The wire backend remains injectable for protocol regression
// tests, while every production engine uses the same typed contract.
type MPVEngine struct {
	raw          Backend
	broker       *updateBroker
	done         chan struct{}
	mu           sync.Mutex
	generation   uint64
	epoch        int64
	entry        int64
	loaded       int64
	entries      map[int64]uint64
	pendingEnds  map[int64]Event
	queries      map[string][]mpvQuery
	commands     map[string][]uint64
	loading      bool
	failed       bool
	fallbackLoad bool
	seeking      bool
	seekOnLoad   time.Duration
	audioReady   bool
	dsp          DSP
	activeSeek   *mpvSeek
	queuedSeek   *mpvSeek
}

type mpvQuery struct {
	generation uint64
	epoch      int64
}
type mpvSeek struct {
	position time.Duration
	epoch    int64
	started  bool
}

func NewMPVEngine(opts EngineOptions) (Engine, error) {
	raw, err := New(Options{Executable: opts.Executable, ExtraArgs: opts.ExtraArgs, StartupTimeout: opts.StartupTimeout})
	if err != nil {
		return nil, err
	}
	return WrapMPV(raw), nil
}

func WrapMPV(raw Backend) *MPVEngine {
	p := &MPVEngine{raw: raw, broker: newUpdateBroker(), done: make(chan struct{}), entries: make(map[int64]uint64), pendingEnds: make(map[int64]Event), queries: make(map[string][]mpvQuery), commands: make(map[string][]uint64)}
	go func() {
		defer close(p.done)
		defer p.broker.close()
		for event := range raw.Events() {
			for _, update := range p.Translate(event) {
				p.broker.offer(update)
			}
		}
	}()
	return p
}

func (p *MPVEngine) Events() <-chan Update { return p.broker.out }
func (p *MPVEngine) Capabilities() Capabilities {
	_, spectrum := p.raw.(SpectrumSource)
	if raw, ok := p.raw.(*MPV); ok {
		spectrum = raw.spectrum != nil
	}
	return Capabilities{Seek: true, Equalizer: true, Balance: true, Spectrum: spectrum, Formats: "Most audio formats"}
}
func (p *MPVEngine) Close() error { err := p.raw.Close(); <-p.done; return err }

func (p *MPVEngine) send(args ...any) error {
	if err := p.raw.Send(args...); err != nil {
		return err
	}
	key, _ := json.Marshal(args)
	p.commands[string(key)] = append(p.commands[string(key)], p.generation)
	return nil
}
func (p *MPVEngine) query(name string) error {
	if err := p.send("get_property", name); err != nil {
		return err
	}
	p.queries[name] = append(p.queries[name], mpvQuery{generation: p.generation, epoch: p.epoch})
	return nil
}

func (p *MPVEngine) Apply(command Command) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch command.Op {
	case OpLoad:
		// Without a returned entry ID, a replaced unidentified load cannot
		// safely contribute a failure to its successor. The old-version
		// empty-snapshot fallback is restricted to an uncontested load.
		p.fallbackLoad = !p.loading && len(p.pendingEnds) == 0 && len(p.queries["playlist"]) == 0
		p.generation, p.epoch, p.loading, p.seeking, p.audioReady, p.failed = command.Generation, command.Epoch, true, false, false, false
		p.activeSeek, p.queuedSeek = nil, nil
		p.seekOnLoad = command.Position
		if err := p.filters(); err != nil {
			return err
		}
		if err := p.send("set_property", "pause", command.Paused); err != nil {
			return err
		}
		if err := p.send("loadfile", command.Path, "replace"); err != nil {
			return err
		}
		return p.query("playlist")
	case OpPause:
		if command.Generation != 0 && command.Generation != p.generation {
			return nil
		}
		return p.send("set_property", "pause", command.Paused)
	case OpStop:
		if command.Generation != 0 && command.Generation != p.generation {
			return nil
		}
		err := p.send("stop")
		p.generation, p.loading, p.seeking = 0, false, false
		p.activeSeek, p.queuedSeek = nil, nil
		return err
	case OpSeek:
		if command.Generation != 0 && command.Generation != p.generation {
			return nil
		}
		if command.Position < 0 {
			return fmt.Errorf("mpv: negative seek position")
		}
		p.epoch, p.seeking = command.Epoch, true
		if p.loading {
			p.seekOnLoad = command.Position
			return nil
		}
		seek := &mpvSeek{position: command.Position, epoch: command.Epoch}
		if p.activeSeek != nil {
			p.queuedSeek = seek
			return nil
		}
		return p.beginSeek(seek)
	case OpVolume:
		return p.send("set_property", "volume", command.Volume*100)
	case OpDSP:
		p.dsp, p.epoch = command.DSP, command.Epoch
		return p.filters()
	default:
		return fmt.Errorf("mpv: unsupported operation %d", command.Op)
	}
}

// Only one native seek is in flight. MPV's seek/restart notifications have no
// request ID; serializing them lets a restart finish its actual operation rather
// than borrowing the epoch of a newer request. Intermediate queued targets can
// be replaced by the latest frontend intent before they reach the decoder.
func (p *MPVEngine) beginSeek(seek *mpvSeek) error {
	p.activeSeek, p.seeking = seek, true
	if err := p.filters(); err != nil {
		return err
	}
	return p.send("seek", seek.position.Seconds(), "absolute+exact")
}

func (p *MPVEngine) recordEntry(id int64, generation uint64) []Update {
	p.entries[id] = generation
	if end, ok := p.pendingEnds[id]; ok {
		delete(p.pendingEnds, id)
		return p.translate(end)
	}
	return nil
}

func (p *MPVEngine) filters() error {
	chain := []any{}
	if graph := dspGraph(p.dsp); graph != "" {
		chain = append(chain, map[string]any{"name": "lavfi", "params": map[string]string{"graph": graph}})
	}
	if source, ok := p.raw.(SpectrumSource); ok {
		if analyzer := source.SpectrumFilter(p.epoch); analyzer != nil {
			chain = append(chain, analyzer)
		}
	}
	return p.send("set_property", "af", chain)
}

func dspGraph(dsp DSP) string {
	var filters []string
	if dsp.Balance != 0 {
		left, right := float32(1), float32(1)
		if dsp.Balance > 0 {
			left -= dsp.Balance
		} else {
			right += dsp.Balance
		}
		filters = append(filters, fmt.Sprintf("aformat=channel_layouts=stereo,pan=stereo|c0=%.6f*c0|c1=%.6f*c1", left, right))
	}
	if dsp.Enabled {
		if dsp.Preamp != 0 {
			filters = append(filters, fmt.Sprintf("volume=%.4fdB", dsp.Preamp))
		}
		frequencies := [10]int{31, 62, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}
		for i, frequency := range frequencies {
			if dsp.Gains[i] != 0 {
				filters = append(filters, fmt.Sprintf("equalizer=f=%d:t=o:w=1:g=%.4f", frequency, dsp.Gains[i]))
			}
		}
	}
	return strings.Join(filters, ",")
}

func (p *MPVEngine) current() bool {
	return p.generation != 0 && !p.failed && p.entries[p.entry] == p.generation
}

// Translate is the protocol boundary. It is also used by deterministic tests
// to deliver native events without a process or a GUI event-loop race.
func (p *MPVEngine) Translate(event Event) []Update {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.translate(event)
}

func (p *MPVEngine) translate(event Event) []Update {
	var updates []Update
	errorUpdate := func(err error) {
		updates = append(updates, Update{Kind: UpdateError, Generation: p.generation, Error: err.Error()})
	}
	switch event.Event {
	case "backend-error", "shutdown":
		message := event.Error
		if message == "" {
			message = "mpv exited; select a backend in Settings to reconnect"
		}
		return []Update{{Kind: UpdateError, Error: message}}
	case "command-reply", "command-error":
		generation := p.generation
		if pending := p.commands[string(event.Command)]; len(pending) > 0 {
			generation = pending[0]
			p.commands[string(event.Command)] = pending[1:]
			if len(pending) == 1 {
				delete(p.commands, string(event.Command))
			}
		}
		if event.Name == "loadfile" && event.Event == "command-reply" {
			// Modern mpv returns this ID even when a failed file is removed
			// before the subsequent playlist snapshot. The command itself is
			// tagged when queued, so rapid replacements retain load identity.
			var result struct {
				ID *int64 `json:"playlist_entry_id"`
			}
			if json.Unmarshal(event.Data, &result) == nil && result.ID != nil {
				updates = append(updates, p.recordEntry(*result.ID, generation)...)
				updates = append(updates, p.ready()...)
			}
		} else if event.Name == "get_property" {
			var args []json.RawMessage
			var name string
			if json.Unmarshal(event.Command, &args) != nil || len(args) < 2 || json.Unmarshal(args[1], &name) != nil {
				return nil
			}
			pending := p.queries[name]
			if len(pending) == 0 {
				return nil
			}
			query := pending[0]
			generation = query.generation
			p.queries[name] = pending[1:]
			if event.Event == "command-error" {
				return nil
			}
			if name == "playlist" {
				var entries []struct {
					ID int64 `json:"id"`
				}
				if json.Unmarshal(event.Data, &entries) != nil {
					return nil
				}
				// Older mpv returns null from loadfile. If its current-load
				// snapshot is empty after a reported failed entry, no requested
				// track survived. Report that negative result without guessing
				// which untagged entry (possibly replaced) supplied the error.
				if len(entries) == 0 && generation == p.generation && p.loading && !p.failed && p.fallbackLoad {
					for _, end := range p.pendingEnds {
						if end.Reason == "error" {
							p.failed, p.loading, p.seeking = true, false, false
							p.activeSeek, p.queuedSeek = nil, nil
							return []Update{{Kind: UpdateError, Generation: generation, Error: "mpv could not load the requested file"}}
						}
					}
				}
				for _, entry := range entries {
					updates = append(updates, p.recordEntry(entry.ID, generation)...)
				}
				updates = append(updates, p.ready()...)
			} else if generation == p.generation && p.current() && !p.loading && (name != "time-pos" || query.epoch == p.epoch) {
				updates = append(updates, p.property(name, event.Data)...)
			}
		} else if event.Event == "command-error" && generation == p.generation {
			return []Update{{Kind: UpdateError, Generation: generation, Error: "mpv: " + event.Error}}
		}
	case "start-file":
		p.entry = event.PlaylistEntryID
	case "file-loaded":
		p.loaded = p.entry
		if p.loading && !p.current() {
			if err := p.query("playlist"); err != nil {
				errorUpdate(err)
			}
		}
		updates = append(updates, p.ready()...)
	case "seek":
		if p.current() && p.activeSeek != nil {
			p.activeSeek.started = true
		}
	case "playback-restart":
		if p.current() {
			if seek := p.activeSeek; seek != nil && seek.started {
				p.activeSeek = nil
				updates = append(updates, Update{Kind: UpdateSeeked, Generation: p.generation, Epoch: seek.epoch})
				if next := p.queuedSeek; next != nil {
					p.queuedSeek = nil
					if err := p.beginSeek(next); err != nil {
						errorUpdate(err)
					}
				} else {
					p.seeking = false
				}
			}
			if !p.loading && !p.audioReady {
				if err := p.query("audio-params"); err != nil {
					errorUpdate(err)
				}
			}
		}
	case "property-change":
		if p.current() && !p.loading {
			updates = append(updates, p.property(event.Name, event.Data)...)
		}
	case "spectrum":
		var data SpectrumData
		if p.current() && !p.loading && !p.seeking && json.Unmarshal(event.Data, &data) == nil && data.Generation == p.epoch {
			updates = append(updates, Update{Kind: UpdateSpectrum, Generation: p.generation, Epoch: data.Generation, Position: time.Duration(data.Time * float64(time.Second)), Bands: data.Bands})
		}
	case "end-file":
		generation, known := p.entries[event.PlaylistEntryID]
		if !known {
			p.pendingEnds[event.PlaylistEntryID] = event
			return nil
		}
		delete(p.entries, event.PlaylistEntryID)
		if generation != p.generation || p.generation == 0 {
			return nil
		}
		if event.Reason == "eof" {
			updates = append(updates, Update{Kind: UpdateEnded, Generation: generation})
		}
		if event.Reason == "error" {
			if p.failed {
				return nil
			}
			p.failed = true
			message := event.FileError
			if message == "" {
				message = event.Error
			}
			if message == "" {
				message = "the file could not be decoded"
			}
			updates = append(updates, Update{Kind: UpdateError, Generation: generation, Error: message})
		}
	}
	return updates
}

func (p *MPVEngine) ready() []Update {
	if !p.loading || p.loaded != p.entry || !p.current() {
		return nil
	}
	p.loading = false
	if p.seekOnLoad > 0 {
		if err := p.beginSeek(&mpvSeek{position: p.seekOnLoad, epoch: p.epoch}); err != nil {
			return []Update{{Kind: UpdateError, Generation: p.generation, Error: err.Error()}}
		}
	}
	updates := []Update{{Kind: UpdateLoaded, Generation: p.generation}}
	for _, name := range []string{"time-pos", "duration", "metadata", "audio-params", "audio-bitrate"} {
		if err := p.query(name); err != nil {
			updates = append(updates, Update{Kind: UpdateError, Generation: p.generation, Error: err.Error()})
			break
		}
	}
	return updates
}

func (p *MPVEngine) property(name string, data json.RawMessage) []Update {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	u := Update{Generation: p.generation, Epoch: p.epoch}
	switch name {
	case "time-pos", "duration", "audio-bitrate":
		var seconds float64
		if json.Unmarshal(data, &seconds) != nil || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
			return nil
		}
		if name == "time-pos" {
			if p.seeking {
				return nil
			}
			u.Kind, u.Position = UpdatePosition, time.Duration(seconds*float64(time.Second))
		}
		if name == "duration" {
			u.Kind, u.Duration = UpdateDuration, time.Duration(seconds*float64(time.Second))
		}
		if name == "audio-bitrate" {
			u.Kind, u.Info.Bitrate = UpdateAudioFormat, int(seconds)
		}
	case "metadata":
		var metadata map[string]string
		if json.Unmarshal(data, &metadata) != nil {
			return nil
		}
		u.Kind = UpdateMetadata
		for key, value := range metadata {
			switch strings.ToLower(key) {
			case "title":
				u.Info.Title = value
			case "artist":
				u.Info.Artist = value
			case "album":
				u.Info.Album = value
			}
		}
	case "audio-params":
		var params struct {
			Rate     int `json:"samplerate"`
			Channels int `json:"channel-count"`
		}
		if json.Unmarshal(data, &params) != nil || params.Rate <= 0 || params.Channels <= 0 {
			return nil
		}
		p.audioReady = true
		u.Kind, u.Info.SampleRate, u.Info.Channels = UpdateAudioFormat, params.Rate, params.Channels
	default:
		return nil
	}
	return []Update{u}
}
