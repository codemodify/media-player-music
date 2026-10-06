package playback

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// VLC uses the installed executable's authenticated Lua HTTP interface. Its
// command and JSON schema are documented by VideoLAN:
// https://github.com/videolan/vlc/blob/3.0.x/share/lua/http/requests/README.txt
// No libVLC or additional audio process is linked or started.
type VLC struct {
	cmd           *exec.Cmd
	client        *http.Client
	endpoint      string
	password      string
	diagnostics   *tailBuffer
	processDone   chan struct{}
	waitErr       error // published by closing processDone
	commands      chan Command
	volumeWake    chan struct{}
	stop          chan struct{}
	done          chan struct{}
	broker        *updateBroker
	mu            sync.Mutex
	closed        bool
	pendingVolume float32 // protected by mu; latest slider intent wins
	volumePending bool
	closeOnce     sync.Once
	cancel        context.CancelFunc
	ctx           context.Context
	volume        float32 // worker-owned desired volume, reapplied after each load
}

type vlcStatus struct {
	Version     string  `json:"version"`
	State       string  `json:"state"`
	CurrentID   int64   `json:"currentplid"`
	Time        float64 `json:"time"`
	Position    float64 `json:"position"`
	Length      float64 `json:"length"`
	Volume      int     `json:"volume"`
	Information struct {
		Category map[string]map[string]any `json:"category"`
	} `json:"information"`
}

type vlcItem struct {
	ID       string    `json:"id"`
	URI      string    `json:"uri"`
	Children []vlcItem `json:"children"`
}

type vlcSession struct {
	generation uint64
	epoch      int64
	itemID     int64
	load       Command
	deadline   time.Time
	loaded     bool
	active     bool
	playing    bool
	wantPaused bool
	position   time.Duration
	duration   time.Duration
	info       TrackInfo
	seek       *Command
}

// NewVLC starts a private, video-free VLC process. All local HTTP traffic is
// authenticated on a fresh 127.0.0.1 port; user configuration is not changed.
// ExtraArgs can select a silent audio output such as --aout=dummy in tests.
func NewVLC(opts EngineOptions) (Engine, error) {
	name := opts.Executable
	if name == "" {
		name = "vlc"
	}
	executable, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("playback: find VLC: %w", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("playback: reserve VLC HTTP port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("playback: generate VLC HTTP password: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &VLC{ctx: ctx, cancel: cancel, password: hex.EncodeToString(secret), volume: 1,
		endpoint:    fmt.Sprintf("http://127.0.0.1:%d/requests/", port),
		client:      &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 2 * time.Second},
		diagnostics: &tailBuffer{limit: 16 * 1024}, processDone: make(chan struct{}),
		commands: make(chan Command, commandCapacity), volumeWake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	args := append([]string{}, opts.ExtraArgs...)
	args = append(args, "--ignore-config", "--no-one-instance", "-I", "http", "--http-host=127.0.0.1",
		"--http-port="+strconv.Itoa(port), "--http-password="+p.password,
		"--no-video", "--no-media-library", "--no-metadata-network-access", "--no-loop", "--no-repeat",
		"--start-paused", "--play-and-pause", "--file-caching=50")
	p.cmd = exec.Command(executable, args...)
	configureCommand(p.cmd)
	p.cmd.Stdout, p.cmd.Stderr = p.diagnostics, p.diagnostics
	p.cmd.WaitDelay = 500 * time.Millisecond
	// Stream field names and units are parsed from the English JSON schema.
	p.cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := p.cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("playback: start VLC: %w", err)
	}
	go func() { p.waitErr = p.cmd.Wait(); close(p.processDone) }()
	timeout := opts.StartupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	p.client.Timeout = min(2*time.Second, timeout)
	deadline := time.Now().Add(timeout)
	for {
		var state vlcStatus
		if err = p.request("status.json", nil, &state); err == nil && state.Version != "" {
			break
		}
		if err == nil {
			err = errors.New("VLC HTTP endpoint returned an invalid status")
		}
		select {
		case <-p.processDone:
			err = fmt.Errorf("VLC exited before opening HTTP: %v", p.waitErr)
		case <-time.After(20 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			err = fmt.Errorf("VLC HTTP startup timed out: %w", err)
		}
		select {
		case <-p.processDone:
		default:
			if time.Now().Before(deadline) {
				continue
			}
		}
		cancel()
		_ = p.cmd.Process.Kill()
		<-p.processDone
		p.client.CloseIdleConnections()
		return nil, fmt.Errorf("playback: %w%s", err, p.diagnosticSuffix())
	}
	p.client.Timeout = 2 * time.Second
	p.broker = newUpdateBroker()
	go p.run()
	return p, nil
}

func (p *VLC) Capabilities() Capabilities {
	return Capabilities{Seek: true, Formats: "Most audio formats supported by the installed VLC build"}
}

func (p *VLC) Events() <-chan Update { return p.broker.out }

func (p *VLC) Apply(c Command) error {
	if p == nil {
		return ErrClosed
	}
	if c.Op < OpLoad || c.Op > OpDSP {
		return errors.New("playback: unknown VLC operation")
	}
	if c.Op == OpDSP {
		return errors.New("playback: VLC does not expose this player's exact equalizer, balance or spectrum")
	}
	if c.Op == OpLoad && (c.Path == "" || strings.ContainsRune(c.Path, 0)) {
		return errors.New("playback: invalid VLC path")
	}
	if c.Op == OpVolume && (math.IsNaN(float64(c.Volume)) || math.IsInf(float64(c.Volume), 0)) {
		return errors.New("playback: invalid VLC volume")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case <-p.processDone:
		return ErrClosed
	default:
	}
	if c.Op == OpVolume {
		// Pointer motion must not queue hundreds of slow HTTP snapshots.
		// Keep one latest value independently of ordered load/transport work.
		p.pendingVolume, p.volumePending = max(0, min(1, c.Volume)), true
		select {
		case p.volumeWake <- struct{}{}:
		default:
		}
		return nil
	}
	select {
	case p.commands <- c:
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *VLC) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.stop)
		p.mu.Unlock()
		p.cancel()
		_ = p.cmd.Process.Kill()
		<-p.processDone
		<-p.done
		p.client.CloseIdleConnections()
	})
	return nil
}

func (p *VLC) request(endpoint string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(p.ctx, http.MethodGet, p.endpoint+endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("", p.password)
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return fmt.Errorf("VLC HTTP: %s", res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4*1024*1024)).Decode(out)
}

func (p *VLC) command(name string, params url.Values) (vlcStatus, error) {
	if params == nil {
		params = make(url.Values)
	}
	params.Set("command", name)
	var s vlcStatus
	err := p.request("status.json", params, &s)
	return s, err
}

func (p *VLC) run() {
	defer close(p.done)
	defer p.broker.close()
	defer func() {
		_ = p.cmd.Process.Kill()
		<-p.processDone
	}()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var session *vlcSession
	for {
		select {
		case <-p.stop:
			return
		default:
		}
		if p.takeVolume() {
			if err := p.setVolume(p.volume); err != nil {
				if p.ctx.Err() == nil {
					p.broker.offer(Update{Kind: UpdateError, Error: fmt.Sprintf("VLC volume: %v", err)})
				}
				return
			}
		}
		select {
		case <-p.stop:
			return
		case <-p.volumeWake:
			continue
		case <-p.processDone:
			if p.ctx.Err() != nil {
				return
			}
			p.broker.offer(Update{Kind: UpdateError, Error: fmt.Sprintf("VLC exited: %v%s", p.waitErr, p.diagnosticSuffix())})
			return
		case c := <-p.commands:
			var err error
			if c.Op == OpLoad {
				session, err = p.load(c)
			} else if session != nil && (c.Generation == session.generation || c.Op == OpStop && c.Generation == 0) {
				switch c.Op {
				case OpStop:
					session.active = false // explicit Stop never emits EOF
					_, err = p.command("pl_stop", nil)
				case OpPause:
					session.wantPaused = c.Paused
					if session.loaded && session.active {
						err = p.pause(session)
					}
				case OpSeek:
					session.seek = &c
					if session.loaded && session.active {
						err = p.seek(session, c)
					}
				}
			}
			if err != nil {
				if p.ctx.Err() != nil {
					return
				}
				p.broker.offer(Update{Kind: UpdateError, Generation: c.Generation, Error: fmt.Sprintf("VLC: %v", err)})
			}
		case <-ticker.C:
			if session == nil || !session.active {
				continue
			}
			var snapshot vlcStatus
			if err := p.request("status.json", nil, &snapshot); err != nil {
				if p.ctx.Err() != nil {
					return
				}
				p.broker.offer(Update{Kind: UpdateError, Error: fmt.Sprintf("VLC HTTP connection: %v", err)})
				return
			}
			if err := p.observe(session, snapshot); err != nil {
				session.active = false
				p.broker.offer(Update{Kind: UpdateError, Generation: session.generation, Error: fmt.Sprintf("VLC: %v%s", err, p.diagnosticSuffix())})
			}
		}
	}
}

func (p *VLC) load(c Command) (*vlcSession, error) {
	if _, err := p.command("pl_stop", nil); err != nil {
		return nil, err
	}
	if _, err := p.command("pl_empty", nil); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(c.Path)
	if err != nil {
		return nil, err
	}
	uri := vlcFileURI(path)
	if _, err := p.command("in_enqueue", url.Values{"input": {uri}}); err != nil {
		return nil, err
	}
	var playlist vlcItem
	if err := p.request("playlist.json", nil, &playlist); err != nil {
		return nil, err
	}
	var itemID int64 = -1
	var find func(vlcItem)
	find = func(item vlcItem) {
		if item.URI == uri {
			itemID, _ = strconv.ParseInt(item.ID, 10, 64)
		}
		for _, child := range item.Children {
			find(child)
		}
	}
	find(playlist)
	if itemID < 0 {
		return nil, errors.New("VLC did not enqueue the requested file")
	}
	if _, err := p.command("pl_play", url.Values{"id": {strconv.FormatInt(itemID, 10)}}); err != nil {
		return nil, err
	}
	return &vlcSession{generation: c.Generation, epoch: c.Epoch, itemID: itemID, load: c,
		active: true, wantPaused: c.Paused, deadline: time.Now().Add(5 * time.Second)}, nil
}

func (p *VLC) pause(s *vlcSession) error {
	name := "pl_forceresume"
	if s.wantPaused {
		name = "pl_forcepause"
	}
	state, err := p.command(name, nil)
	if err == nil && state.CurrentID == s.itemID && state.State == "playing" {
		s.playing = true
	}
	return err
}

func (p *VLC) setVolume(volume float32) error {
	// Match mpv's normalized perceptual volume: both backends cube the
	// normalized setting before applying sample gain. VLC's API uses 256
	// for unity gain rather than mpv's 100.
	// https://github.com/videolan/vlc/blob/3.0.x/modules/audio_output/volume.h
	native := int(math.Round(float64(max(0, min(1, volume))) * 256))
	_, err := p.command("volume", url.Values{"val": {strconv.Itoa(native)}})
	return err
}

func (p *VLC) takeVolume() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.volumePending {
		return false
	}
	p.volume, p.volumePending = p.pendingVolume, false
	return true
}

func (p *VLC) seek(s *vlcSession, c Command) error {
	if s.duration <= 0 {
		return errors.New("VLC has not reported a seekable duration")
	}
	position := max(0, min(s.duration, c.Position))
	percent := float64(position) / float64(s.duration) * 100
	_, err := p.command("seek", url.Values{"val": {strconv.FormatFloat(percent, 'f', 6, 64) + "%"}})
	if err != nil {
		return err
	}
	// The HTTP command queues an input-thread seek. Do not acknowledge it
	// from its immediate reply: that reply can still contain the old clock.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if p.takeVolume() {
			if err := p.setVolume(p.volume); err != nil {
				return err
			}
		}
		var state vlcStatus
		if err := p.request("status.json", nil, &state); err != nil {
			return err
		}
		if state.CurrentID != s.itemID {
			return errors.New("VLC seek targeted a replaced input")
		}
		actual := time.Duration(state.Position * float64(s.duration))
		if delta := actual - position; delta >= -150*time.Millisecond && delta <= 150*time.Millisecond {
			position = actual
			break
		}
		if time.Now().After(deadline) {
			return errors.New("VLC did not acknowledge the requested seek")
		}
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	s.epoch = c.Epoch
	s.position = position
	s.seek = nil
	p.broker.offer(Update{Kind: UpdateSeeked, Generation: s.generation, Position: position, Epoch: s.epoch})
	return nil
}

func (p *VLC) observe(s *vlcSession, state vlcStatus) error {
	if state.CurrentID != s.itemID || state.State == "stopped" {
		if s.loaded {
			return errors.New("playback stopped unexpectedly")
		}
		if time.Now().After(s.deadline) {
			return errors.New("could not open this media file")
		}
		return nil
	}
	duration := time.Duration(max(0, state.Length) * float64(time.Second))
	if duration > 0 && duration != s.duration {
		s.duration = duration
		p.broker.offer(Update{Kind: UpdateDuration, Generation: s.generation, Duration: duration})
	}
	info := vlcTrackInfo(state)
	if info != s.info {
		s.info = info
		p.broker.offer(Update{Kind: UpdateMetadata, Generation: s.generation, Info: info})
		p.broker.offer(Update{Kind: UpdateAudioFormat, Generation: s.generation, Info: info})
	}
	if !s.loaded {
		if state.State != "paused" {
			return nil // --start-paused establishes a quiet initial input
		}
		if !vlcHasAudio(state) {
			if time.Now().After(s.deadline) {
				return errors.New("this media file has no decodable audio stream")
			}
			return nil
		}
		if s.load.Position > 0 {
			if s.duration <= 0 {
				if time.Now().After(s.deadline) {
					return errors.New("VLC could not seek to the requested start position")
				}
				return nil
			}
			if err := p.seek(s, s.load); err != nil {
				return err
			}
		}
		s.loaded = true
		p.takeVolume() // apply even a volume change made during initial seek
		if err := p.setVolume(p.volume); err != nil {
			return err
		}
		p.broker.offer(Update{Kind: UpdateLoaded, Generation: s.generation, Position: s.position, Duration: s.duration, Info: s.info, Epoch: s.epoch})
		if s.seek != nil {
			if err := p.seek(s, *s.seek); err != nil {
				return err
			}
		}
		return p.pause(s)
	}
	if !s.wantPaused && s.playing && state.State == "paused" {
		// --play-and-pause retains the input at its natural end. Every
		// app-issued pause updates wantPaused before the HTTP command.
		s.active = false
		p.broker.offer(Update{Kind: UpdateEnded, Generation: s.generation, Epoch: s.epoch})
		_, err := p.command("pl_stop", nil)
		return err
	}
	if state.State == "playing" {
		s.playing = true
	}
	position := time.Duration(max(0, state.Time) * float64(time.Second))
	if s.duration > 0 && state.Position >= 0 && state.Position <= 1 {
		position = time.Duration(state.Position * float64(s.duration))
	}
	s.position = position
	p.broker.offer(Update{Kind: UpdatePosition, Generation: s.generation, Position: position, Epoch: s.epoch})
	return nil
}

func vlcTrackInfo(s vlcStatus) TrackInfo {
	var info TrackInfo
	meta := s.Information.Category["meta"]
	text := func(v any) string {
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
	info.Title, info.Artist, info.Album = text(meta["title"]), text(meta["artist"]), text(meta["album"])
	for name, stream := range s.Information.Category {
		if name == "meta" || !strings.EqualFold(text(stream["Type"]), "audio") {
			continue
		}
		info.SampleRate = vlcLeadingNumber(text(stream["Sample_rate"]))
		info.Bitrate = vlcLeadingNumber(text(stream["Bitrate"])) * 1000
		channels := strings.ToLower(text(stream["Channels"]))
		if channels == "" {
			channels = strings.ToLower(text(stream["Decoded_channels"]))
		}
		switch channels {
		case "stereo":
			info.Channels = 2
		case "mono":
			info.Channels = 1
		default:
			info.Channels = vlcLeadingNumber(channels)
		}
		break
	}
	return info
}

func vlcHasAudio(s vlcStatus) bool {
	for name, stream := range s.Information.Category {
		if name != "meta" && strings.EqualFold(fmt.Sprint(stream["Type"]), "audio") {
			return true
		}
	}
	return false
}

func vlcLeadingNumber(s string) int {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[0])
	return n
}

func vlcFileURI(path string) string {
	path = filepath.ToSlash(path)
	uri := url.URL{Scheme: "file", Path: path}
	if strings.HasPrefix(path, "//") {
		if host, rest, ok := strings.Cut(strings.TrimPrefix(path, "//"), "/"); ok {
			uri.Host, uri.Path = host, "/"+rest
		}
	} else if len(path) >= 2 && path[1] == ':' {
		uri.Path = "/" + path // Windows drives use file:///C:/..., not a host.
	}
	return uri.String()
}

func (p *VLC) diagnosticSuffix() string {
	if text := strings.TrimSpace(p.diagnostics.String()); text != "" {
		return " (" + strings.ReplaceAll(text, p.password, "[redacted]") + ")"
	}
	return ""
}
