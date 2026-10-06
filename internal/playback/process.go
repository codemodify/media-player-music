package playback

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// updateBroker keeps process readers independent of the UI. Close seals input;
// the output remains available until the caller has drained all accepted events.
type updateBroker struct {
	mu     sync.Mutex
	queue  []Update
	closed bool
	wake   chan struct{}
	out    chan Update
}

func newUpdateBroker() *updateBroker {
	b := &updateBroker{wake: make(chan struct{}, 1), out: make(chan Update, 32)}
	go b.run()
	return b
}

func (b *updateBroker) offer(u Update) {
	b.mu.Lock()
	if !b.closed {
		u.Bands = append([]float32(nil), u.Bands...)
		// A busy UI needs the latest clock/picture. Removing an earlier
		// transient and appending its replacement keeps critical event order.
		if u.Kind == UpdatePosition || u.Kind == UpdateSpectrum {
			for i := len(b.queue) - 1; i >= 0; i-- {
				if old := b.queue[i]; old.Kind == u.Kind && old.Generation == u.Generation && old.Epoch == u.Epoch {
					copy(b.queue[i:], b.queue[i+1:])
					b.queue[len(b.queue)-1] = Update{}
					b.queue = b.queue[:len(b.queue)-1]
					break
				}
			}
		}
		b.queue = append(b.queue, u)
	}
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *updateBroker) close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *updateBroker) run() {
	defer close(b.out)
	for {
		b.mu.Lock()
		if len(b.queue) == 0 {
			closed := b.closed
			b.mu.Unlock()
			if closed {
				return
			}
			<-b.wake
			continue
		}
		u := b.queue[0]
		b.queue[0] = Update{}
		b.queue = b.queue[1:]
		b.mu.Unlock()
		b.out <- u
	}
}

type processLine struct {
	text string
	err  error
}

// lineProcess only launches an executable and transports newline commands.
// There is no shell, native library, inherited frontend stdin, or audio parsing.
type lineProcess struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	readPipe    io.ReadCloser
	lines       chan processLine
	writes      chan string
	done        chan struct{}
	stop        chan struct{}
	readerDone  chan struct{}
	writerDone  chan struct{}
	waitErr     error
	diagnostics *tailBuffer
	closeOnce   sync.Once
}

func startLineProcess(executable string, args []string, protocolStderr bool, profile string) (*lineProcess, error) {
	p := &lineProcess{cmd: exec.Command(executable, args...), lines: make(chan processLine, 128),
		writes: make(chan string, 128), done: make(chan struct{}), stop: make(chan struct{}),
		readerDone: make(chan struct{}), writerDone: make(chan struct{}), diagnostics: &tailBuffer{limit: 16 * 1024}}
	configureCommand(p.cmd)
	if profile != "" {
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "MPLAYER_HOME=") {
				p.cmd.Env = append(p.cmd.Env, v)
			}
		}
		p.cmd.Env = append(p.cmd.Env, "MPLAYER_HOME="+profile)
	}
	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if protocolStderr {
		p.readPipe, err = p.cmd.StderrPipe()
		p.cmd.Stdout = p.diagnostics
	} else {
		p.readPipe, err = p.cmd.StdoutPipe()
		p.cmd.Stderr = &processDiagnostics{process: p}
	}
	if err != nil {
		_ = p.stdin.Close()
		return nil, err
	}
	p.cmd.WaitDelay = 500 * time.Millisecond
	if err = p.cmd.Start(); err != nil {
		_ = p.stdin.Close()
		_ = p.readPipe.Close()
		return nil, err
	}
	go p.read()
	go p.write()
	go func() {
		// Finish reading before Wait closes StdoutPipe/StderrPipe so the final
		// EOF notification is not lost when an executable exits naturally.
		<-p.readerDone
		p.waitErr = p.cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// MPlayer reports failed loads on stderr even though slave answers are on
// stdout. Forward complete diagnostic lines too so those errors arrive promptly.
type processDiagnostics struct {
	process *lineProcess
	pending []byte
}

func (w *processDiagnostics) Write(data []byte) (int, error) {
	n := len(data)
	_, _ = w.process.diagnostics.Write(data)
	w.pending = append(w.pending, data...)
	for {
		index := strings.IndexByte(string(w.pending), '\n')
		if index < 0 {
			break
		}
		line := strings.TrimSuffix(string(w.pending[:index]), "\r")
		w.pending = w.pending[index+1:]
		select {
		case w.process.lines <- processLine{text: line}:
		case <-w.process.stop:
			return n, nil
		}
	}
	// Logs without newline must not grow memory indefinitely.
	if len(w.pending) > 1024*1024 {
		w.pending = append(w.pending[:0], w.pending[len(w.pending)-16384:]...)
	}
	return n, nil
}

func (p *lineProcess) read() {
	defer close(p.readerDone)
	scanner := bufio.NewScanner(p.readPipe)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		_, _ = fmt.Fprintln(p.diagnostics, line)
		select {
		case p.lines <- processLine{text: line}:
		case <-p.stop:
			return
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case p.lines <- processLine{err: err}:
		case <-p.stop:
		}
	}
}

func (p *lineProcess) write() {
	defer close(p.writerDone)
	for {
		select {
		case <-p.stop:
			return
		case line := <-p.writes:
			if _, err := io.WriteString(p.stdin, line+"\n"); err != nil {
				select {
				case p.lines <- processLine{err: err}:
				case <-p.stop:
				}
				return
			}
		}
	}
}

func (p *lineProcess) send(lines ...string) error {
	for _, line := range lines {
		if strings.ContainsAny(line, "\r\n\x00") {
			return errors.New("playback: invalid protocol command")
		}
		select {
		case <-p.done:
			return ErrClosed
		case <-p.stop:
			return ErrClosed
		default:
		}
		select {
		case p.writes <- line:
		default:
			return ErrQueueFull
		}
	}
	return nil
}

func (p *lineProcess) close(quit string) {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		_ = p.send(quit)
		select {
		case <-p.done:
		case <-time.After(500 * time.Millisecond):
			_ = p.cmd.Process.Kill()
			_ = p.readPipe.Close()
		}
		close(p.stop)
		_ = p.stdin.Close()
		_ = p.readPipe.Close()
		<-p.done
		<-p.writerDone
	})
}

type remoteSession struct {
	process               *lineProcess
	generation            uint64
	epoch                 int64
	path                  string
	info                  TrackInfo
	loaded, failed, ended bool
	wantPaused, paused    bool
	volume                float32
	initialPosition       time.Duration
	position, duration    time.Duration
	deadline              time.Time
	seeks                 []remoteSeek
	queries               []remoteQuery
	metadataKeys          map[string]string
}

type remoteSeek struct {
	epoch        int64
	position     time.Duration
	acknowledged bool
	queried      bool
}

type remoteQuery struct {
	epoch int64
	seek  bool
}

func remotePoll(s *remoteSession) {
	q := remoteQuery{epoch: s.epoch}
	for i := range s.seeks {
		if !s.seeks[i].queried {
			s.seeks[i].queried = true
			q.epoch, q.seek = s.seeks[i].epoch, true
			break
		}
	}
	s.queries = append(s.queries, q)
}

func remoteClock(s *remoteSession, b *updateBroker, d remoteDialect) []string {
	if len(s.queries) == 0 {
		return nil
	}
	q := s.queries[0]
	s.queries = s.queries[1:]
	b.offer(Update{Kind: UpdatePosition, Generation: s.generation, Epoch: q.epoch, Position: s.position})
	if q.seek && len(s.seeks) > 0 && s.seeks[0].epoch == q.epoch && (d.name() != "mpg123" || s.seeks[0].acknowledged) {
		return remoteSeeked(s, b, d)
	}
	return nil
}

type remoteDialect interface {
	name() string
	args([]string) []string
	ready(string) bool
	quit() string
	load(*remoteSession) []string
	pause(bool) string
	seek(time.Duration) string
	volume(float32) string
	poll(*remoteSession) []string
	parse(*remoteSession, string, *updateBroker) []string
}

type processEngine struct {
	opts      EngineOptions
	dialect   remoteDialect
	caps      Capabilities
	commands  chan Command
	stop      chan struct{}
	done      chan struct{}
	broker    *updateBroker
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
	initial   *lineProcess
	profile   string
}

func newProcessEngine(opts EngineOptions, dialect remoteDialect, caps Capabilities) (Engine, error) {
	if opts.Executable == "" {
		opts.Executable = dialect.name()
	}
	path, err := exec.LookPath(opts.Executable)
	if err != nil {
		return nil, fmt.Errorf("playback: find %s: %w", dialect.name(), err)
	}
	opts.Executable = path
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = 5 * time.Second
	}
	p := &processEngine{opts: opts, dialect: dialect, caps: caps, commands: make(chan Command, commandCapacity),
		stop: make(chan struct{}), done: make(chan struct{}), broker: newUpdateBroker()}
	if dialect.name() == "mplayer" {
		p.profile, err = os.MkdirTemp("", "music-player-mplayer-")
		if err != nil {
			p.broker.close()
			return nil, fmt.Errorf("playback: create MPlayer profile: %w", err)
		}
	}
	p.initial, err = p.start()
	if err != nil {
		p.broker.close()
		if p.profile != "" {
			_ = os.RemoveAll(p.profile)
		}
		return nil, err
	}
	go p.run()
	return p, nil
}

func (p *processEngine) start() (*lineProcess, error) {
	// Both protocols are read from stderr for mpg123 (its documented remote
	// error stream), and stdout for MPlayer.
	child, err := startLineProcess(p.opts.Executable, p.dialect.args(p.opts.ExtraArgs), p.dialect.name() == "mpg123", p.profile)
	if err != nil {
		return nil, fmt.Errorf("playback: start %s: %w", p.dialect.name(), err)
	}
	timer := time.NewTimer(p.opts.StartupTimeout)
	defer timer.Stop()
	for {
		select {
		case line := <-child.lines:
			if line.err == nil && p.dialect.ready(line.text) {
				return child, nil
			}
		case <-child.done:
			child.close(p.dialect.quit())
			return nil, fmt.Errorf("playback: %s exited during startup: %s", p.dialect.name(), strings.TrimSpace(child.diagnostics.String()))
		case <-timer.C:
			child.close(p.dialect.quit())
			return nil, fmt.Errorf("playback: %s startup timed out: %s", p.dialect.name(), strings.TrimSpace(child.diagnostics.String()))
		case <-p.stop:
			child.close(p.dialect.quit())
			return nil, ErrClosed
		}
	}
}

func (p *processEngine) Capabilities() Capabilities { return p.caps }
func (p *processEngine) Events() <-chan Update      { return p.broker.out }

func (p *processEngine) Apply(c Command) error {
	if c.Op < OpLoad || c.Op > OpDSP {
		return errors.New("playback: unknown operation")
	}
	if c.Op == OpLoad && (c.Generation == 0 || c.Path == "" || strings.ContainsAny(c.Path, "\r\n\x00")) {
		return errors.New("playback: load requires a generation and a path without newline or NUL")
	}
	if c.Op == OpLoad {
		if validator, ok := p.dialect.(interface{ validateLoad(string) error }); ok {
			if err := validator.validateLoad(c.Path); err != nil {
				return err
			}
		}
	}
	if (c.Op == OpSeek || c.Op == OpLoad) && c.Position < 0 {
		return errors.New("playback: negative seek position")
	}
	if c.Op == OpVolume && (math.IsNaN(float64(c.Volume)) || math.IsInf(float64(c.Volume), 0) || c.Volume < 0 || c.Volume > 1) {
		return errors.New("playback: volume must be between zero and one")
	}
	if c.Op == OpDSP && meaningfulDSP(c.DSP) {
		return fmt.Errorf("playback: %s does not support this equalizer or balance: %w", p.dialect.name(), ErrUnsupported)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.commands <- c:
		return nil
	default:
		return ErrQueueFull
	}
}

func meaningfulDSP(d DSP) bool {
	if d.Balance != 0 {
		return true
	}
	if !d.Enabled {
		return false
	}
	if d.Preamp != 0 {
		return true
	}
	for _, gain := range d.Gains {
		if gain != 0 {
			return true
		}
	}
	return false
}

func (p *processEngine) run() {
	child := p.initial
	var s *remoteSession
	volume := float32(1)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		child.close(p.dialect.quit())
		if p.profile != "" {
			_ = os.RemoveAll(p.profile)
		}
		p.broker.close()
		close(p.done)
	}()
	fail := func(err error) {
		generation := uint64(0)
		if s != nil {
			generation = s.generation
			s.failed = true
		}
		p.broker.offer(Update{Kind: UpdateError, Generation: generation, Error: err.Error()})
	}
	send := func(lines ...string) {
		if child != nil {
			if err := child.send(lines...); err != nil {
				fail(err)
			}
		}
	}
	for {
		var lines <-chan processLine
		var exited <-chan struct{}
		if child != nil {
			lines = child.lines
			exited = child.done
		}
		select {
		case <-p.stop:
			return
		case c := <-p.commands:
			if c.Op == OpLoad {
				child.close(p.dialect.quit())
				child = nil
				s = &remoteSession{generation: c.Generation, epoch: c.Epoch, path: c.Path, wantPaused: c.Paused, volume: volume,
					paused: true, initialPosition: c.Position, deadline: time.Now().Add(p.opts.StartupTimeout), metadataKeys: make(map[string]string)}
				var err error
				child, err = p.start()
				if err != nil {
					if !errors.Is(err, ErrClosed) {
						fail(err)
					}
					continue
				}
				s.process = child
				// MPlayer can briefly decode while entering its pause loop.
				// Keep every paused load/seek inaudible until playback is wanted.
				send(p.dialect.volume(0))
				send(p.dialect.load(s)...)
				continue
			}
			if c.Op == OpVolume {
				volume = c.Volume
				if s != nil {
					s.volume = volume
				}
				if s != nil && s.wantPaused {
					send(p.dialect.volume(0))
				} else {
					send(p.dialect.volume(volume))
				}
				continue
			}
			if s == nil || (c.Generation != 0 && c.Generation != s.generation) {
				continue
			}
			switch c.Op {
			case OpStop:
				child.close(p.dialect.quit())
				child = nil
				s = nil
			case OpPause:
				s.wantPaused = c.Paused
				if s.loaded && !s.ended && !s.failed && s.paused != c.Paused {
					if c.Paused {
						send(p.dialect.volume(0))
					} else {
						send(p.dialect.volume(s.volume))
					}
					send(p.dialect.pause(c.Paused))
					s.paused = c.Paused
				}
			case OpSeek:
				s.epoch = c.Epoch
				if !s.loaded {
					s.initialPosition = c.Position
					continue
				}
				s.seeks = append(s.seeks, remoteSeek{epoch: c.Epoch, position: c.Position})
				send(p.dialect.seek(c.Position))
				send(p.dialect.poll(s)...)
			}
		case line := <-lines:
			if line.err != nil {
				fail(line.err)
				continue
			}
			if s != nil && !s.failed {
				send(p.dialect.parse(s, line.text, p.broker)...)
			}
		case <-exited:
			// Scanner output precedes process exit; drain it before deciding
			// whether exit was a failure or a documented EOF.
			for len(child.lines) > 0 {
				line := <-child.lines
				if s != nil && !s.failed && line.err == nil {
					_ = p.dialect.parse(s, line.text, p.broker)
				}
			}
			if s == nil || (!s.ended && !s.failed) {
				fail(fmt.Errorf("playback: %s exited unexpectedly: %s", p.dialect.name(), strings.TrimSpace(child.diagnostics.String())))
			}
			child.close(p.dialect.quit())
			child = nil
		case <-ticker.C:
			if s != nil && !s.loaded && !s.failed && time.Now().After(s.deadline) {
				fail(fmt.Errorf("playback: %s did not load %s", p.dialect.name(), s.path))
			}
			if s != nil && s.loaded && !s.failed && !s.ended {
				send(p.dialect.poll(s)...)
			}
		}
	}
}

func (p *processEngine) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		close(p.stop)
		<-p.done
	})
	return nil
}

func remoteUpdate(s *remoteSession, b *updateBroker, kind UpdateKind) {
	b.offer(Update{Kind: kind, Generation: s.generation, Epoch: s.epoch, Position: s.position, Duration: s.duration, Info: s.info})
}

func remoteLoaded(s *remoteSession, b *updateBroker, dialect remoteDialect) []string {
	if s.loaded {
		return nil
	}
	s.loaded = true
	remoteUpdate(s, b, UpdateLoaded)
	if s.initialPosition > 0 {
		s.seeks = append(s.seeks, remoteSeek{epoch: s.epoch, position: s.initialPosition})
		return append([]string{dialect.seek(s.initialPosition)}, dialect.poll(s)...)
	}
	if !s.wantPaused {
		s.paused = false
		return []string{dialect.volume(s.volume), dialect.pause(false)}
	}
	return nil
}

func remoteSeeked(s *remoteSession, b *updateBroker, dialect remoteDialect) []string {
	if len(s.seeks) == 0 {
		return nil
	}
	seek := s.seeks[0]
	s.seeks = s.seeks[1:]
	b.offer(Update{Kind: UpdateSeeked, Generation: s.generation, Epoch: seek.epoch, Position: s.position})
	if len(s.seeks) == 0 && s.paused != s.wantPaused {
		s.paused = s.wantPaused
		volume := s.volume
		if s.wantPaused {
			volume = 0
		}
		return []string{dialect.volume(volume), dialect.pause(s.wantPaused)}
	}
	return nil
}
