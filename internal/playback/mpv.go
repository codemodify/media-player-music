// Package playback controls separately installed media-player executables.
// It owns no playlist or UI state. Commands are queued and events are delivered
// in wire order; callers apply events to their model on their UI thread.
package playback

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrClosed      = errors.New("playback: backend is closed")
	ErrQueueFull   = errors.New("playback: command queue is full")
	ErrUnsupported = errors.New("playback: operation is not supported by this backend or platform")
)

// Backend is the low-level MPV transport used by MPVEngine and protocol tests.
// Send queues a native command without waiting for mpv. A rejected command returns an error immediately; an
// accepted command's execution error arrives as a command-error event.
type Backend interface {
	Send(command ...any) error
	Events() <-chan Event
	Close() error
}

type Options struct {
	Executable     string
	ExtraArgs      []string
	StartupTimeout time.Duration
}

// Event preserves mpv's JSON fields. Error and FileError are human-readable mpv
// strings, not numeric libmpv error codes. PlaylistEntryID associates start-file
// and end-file even when one load replaces another before events are consumed.
//
// Replies gain Event="command-reply" or "command-error", Name (the command),
// and Command (the original argument array). Data remains the reply's result.
// A backend-error means the process or connection died unexpectedly. Consumers
// should drain Events until it closes, including after calling Close.
type Event struct {
	Event                    string          `json:"event"`
	Name                     string          `json:"name,omitempty"`
	Data                     json.RawMessage `json:"data,omitempty"`
	Reason                   string          `json:"reason,omitempty"`
	Error                    string          `json:"error,omitempty"`
	FileError                string          `json:"file_error,omitempty"`
	RequestID                int64           `json:"request_id,omitempty"`
	ID                       int64           `json:"id,omitempty"`
	PlaylistEntryID          int64           `json:"playlist_entry_id,omitempty"`
	PlaylistInsertID         int64           `json:"playlist_insert_id,omitempty"`
	PlaylistInsertNumEntries int64           `json:"playlist_insert_num_entries,omitempty"`
	Path                     string          `json:"path,omitempty"`
	Command                  json.RawMessage `json:"-"`
}

type request struct {
	id      int64
	name    string
	command json.RawMessage
	wire    []byte
}

// MPV is one process and one persistent IPC connection. It is safe to call Send
// and Close concurrently. The controller, rather than a skin or window, owns it.
type MPV struct {
	cmd         *exec.Cmd
	conn        net.Conn
	dir         string
	diagnostics *tailBuffer
	processDone chan struct{}
	readDone    chan struct{}
	finished    chan struct{}
	waitErr     error // protected by processDone
	commands    chan request
	quit        chan struct{}
	stopIO      chan struct{}
	ioWG        sync.WaitGroup
	mu          sync.Mutex
	closing     bool
	nextID      int64
	outstanding int
	pending     map[int64]request
	ioError     error
	closeOnce   sync.Once
	closeErr    error
	eventsIn    chan Event
	events      chan Event
	spectrumIn  chan Event
	spectrum    *spectrumMeter
}

const commandCapacity = 256

// New starts the installed mpv with no user config, no video, and a private
// IPC endpoint. ExtraArgs permits engine options such as --ao=null in tests.
// IPC uses Unix sockets on Unix and named pipes on Windows.
func New(opts Options) (*MPV, error) {
	if runtime.GOOS == "plan9" {
		return nil, ErrUnsupported
	}
	executable := opts.Executable
	if executable == "" {
		executable = "mpv"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return nil, fmt.Errorf("playback: find mpv: %w", err)
	}
	dir, err := os.MkdirTemp("", "music-player-mpv-")
	if err != nil {
		return nil, fmt.Errorf("playback: create IPC directory: %w", err)
	}
	socket := ipcAddress(dir)
	// macOS TMPDIR and user-selected temp roots can exceed sockaddr_un's
	// small path limit. A fresh mode-0700 directory in /tmp remains private.
	if len(socket) >= 100 {
		_ = os.RemoveAll(dir)
		dir, err = os.MkdirTemp("/tmp", "music-player-mpv-")
		if err != nil {
			return nil, fmt.Errorf("playback: create short IPC directory: %w", err)
		}
		socket = ipcAddress(dir)
	}
	args := append([]string{}, opts.ExtraArgs...)
	args = append(args, "--no-config", "--idle=yes", "--no-video", "--audio-display=no",
		"--terminal=no", "--input-terminal=no", "--input-default-bindings=no",
		"--input-vo-keyboard=no", "--osc=no", "--load-scripts=no", "--input-ipc-server="+socket)
	p := &MPV{
		cmd: exec.Command(path, args...), dir: dir,
		diagnostics: &tailBuffer{limit: 16 * 1024},
		processDone: make(chan struct{}), readDone: make(chan struct{}), finished: make(chan struct{}),
		commands: make(chan request, commandCapacity), quit: make(chan struct{}), stopIO: make(chan struct{}),
		pending: make(map[int64]request), eventsIn: make(chan Event), events: make(chan Event, 64),
		spectrumIn: make(chan Event, 1),
	}
	p.cmd.Stdout, p.cmd.Stderr = p.diagnostics, p.diagnostics
	configureCommand(p.cmd)
	// A subprocess inheriting mpv's diagnostic pipes must not make Wait or
	// startup-failure cleanup hang after the mpv process itself has exited.
	p.cmd.WaitDelay = 500 * time.Millisecond
	if err := p.cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("playback: start mpv: %w", err)
	}
	go func() { p.waitErr = p.cmd.Wait(); close(p.processDone) }()
	timeout := opts.StartupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	p.conn, err = connect(socket, timeout, p.processDone)
	if err != nil {
		_ = p.cmd.Process.Kill()
		<-p.processDone
		_ = os.RemoveAll(dir)
		if p.waitErr != nil {
			err = fmt.Errorf("%w (%v)", err, p.waitErr)
		}
		return nil, fmt.Errorf("playback: start mpv IPC: %w%s", err, p.diagnosticSuffix())
	}
	// The enclosing directory is already mode 0700. Restrict the socket too.
	if err := restrictIPC(socket); err != nil {
		_ = p.conn.Close()
		_ = p.cmd.Process.Kill()
		<-p.processDone
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("playback: restrict mpv IPC: %w", err)
	}
	// Meter metadata has a separate bounded path: a stalled UI may skip
	// pictures, while EOF, errors and command replies remain lossless.
	p.spectrum, err = newSpectrumMeter(dir, p.offerSpectrum)
	if err != nil {
		_, _ = fmt.Fprintf(p.diagnostics, "visualizer unavailable: %v\n", err)
	}
	go p.deliverEvents()
	p.ioWG.Add(2)
	go p.read()
	go p.write()
	go p.supervise()
	for i, name := range []string{"time-pos", "duration", "pause", "metadata", "audio-params", "path", "volume", "audio-bitrate", "media-title"} {
		if err := p.Send("observe_property", i+1, name); err != nil {
			_ = p.Close()
			return nil, fmt.Errorf("playback: observe %s: %w", name, err)
		}
	}
	return p, nil
}

func connect(socket string, timeout time.Duration, processDone <-chan struct{}) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("startup timed out: %w", last)
		}
		conn, err := dialIPC(socket, min(100*time.Millisecond, time.Until(deadline)))
		if err == nil {
			return conn, nil
		}
		last = err
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("startup timed out: %w", last)
		}
		select {
		case <-processDone:
			return nil, errors.New("mpv exited before opening IPC")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// Send snapshots the argument array as JSON and queues it in FIFO order. It
// neither waits for the process nor blocks when its finite command queue fills.
func (p *MPV) Send(command ...any) error {
	if p == nil {
		return ErrClosed
	}
	if len(command) == 0 {
		return errors.New("playback: empty command")
	}
	name, ok := command[0].(string)
	if !ok || name == "" {
		return errors.New("playback: command name must be a nonempty string")
	}
	args, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("playback: encode command: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return ErrClosed
	}
	select {
	case <-p.processDone:
		return ErrClosed
	default:
	}
	if p.outstanding >= commandCapacity {
		return ErrQueueFull
	}
	p.nextID++
	wire, err := json.Marshal(struct {
		Command   json.RawMessage `json:"command"`
		RequestID int64           `json:"request_id"`
	}{args, p.nextID})
	if err != nil {
		return err
	}
	r := request{p.nextID, name, args, append(wire, '\n')}
	select {
	case p.commands <- r:
		p.outstanding++
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *MPV) Events() <-chan Event { return p.events }

func (p *MPV) write() {
	defer p.ioWG.Done()
	for {
		// Prefer shutdown over queued work, so Close is bounded even after a
		// burst of seeks or a command that blocks the remote IPC loop.
		select {
		case <-p.quit:
			p.writeQuit()
			return
		default:
		}
		select {
		case <-p.stopIO:
			return
		case <-p.quit:
			p.writeQuit()
			return
		case r := <-p.commands:
			p.mu.Lock()
			p.pending[r.id] = r
			p.mu.Unlock()
			_ = p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if _, err := io.Copy(p.conn, strings.NewReader(string(r.wire))); err != nil {
				p.failIO(fmt.Errorf("write mpv IPC: %w", err))
				return
			}
		}
	}
}

func (p *MPV) writeQuit() {
	_ = p.conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
	_, _ = io.WriteString(p.conn, "{\"command\":[\"quit\"]}\n")
}

func (p *MPV) read() {
	defer p.ioWG.Done()
	defer close(p.readDone)
	scanner := bufio.NewScanner(p.conn)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			p.failIO(fmt.Errorf("decode mpv IPC: %w", err))
			return
		}
		if e.Event == "" {
			p.mu.Lock()
			r, found := p.pending[e.RequestID]
			delete(p.pending, e.RequestID)
			if found {
				p.outstanding--
			}
			p.mu.Unlock()
			e.Event = "command-reply"
			if e.Error != "" && e.Error != "success" {
				e.Event = "command-error"
			}
			if found {
				e.Name, e.Command = r.name, r.command
			}
		}
		p.eventsIn <- e
	}
	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	p.failIO(fmt.Errorf("read mpv IPC: %w", err))
}

func (p *MPV) failIO(err error) {
	p.mu.Lock()
	closing := p.closing
	if p.ioError == nil {
		p.ioError = err
	}
	p.mu.Unlock()
	if !closing {
		_ = p.cmd.Process.Kill()
	}
}

func (p *MPV) supervise() {
	<-p.processDone
	close(p.stopIO)
	// The process may exit before the reader drains the final end-file and
	// shutdown bytes. Read to EOF first; an inherited/unresponsive socket gets
	// a bounded deadline rather than leaving cleanup blocked forever.
	_ = p.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	<-p.readDone
	_ = p.conn.Close()
	p.ioWG.Wait()
	if p.spectrum != nil {
		p.spectrum.close()
	}
	p.mu.Lock()
	closing, ipcErr := p.closing, p.ioError
	p.mu.Unlock()
	if !closing {
		message := "mpv exited unexpectedly"
		if p.waitErr != nil {
			message += ": " + p.waitErr.Error()
		}
		if ipcErr != nil {
			message += ": " + ipcErr.Error()
		}
		p.eventsIn <- Event{Event: "backend-error", Error: message + p.diagnosticSuffix()}
	}
	// Cleanup also happens on crashes, even if the controller has not yet
	// reacted. Closing the stream preserves queued EOF and error events.
	_ = os.RemoveAll(p.dir)
	close(p.eventsIn)
	close(p.finished)
}

// deliverEvents decouples socket reads from a temporarily busy UI. It never
// silently drops an end-file or an error because a fixed event buffer filled.
// Consumers must keep draining this channel until it closes.
func (p *MPV) deliverEvents() {
	in := p.eventsIn
	var queue []Event
	var picture *Event
	defer close(p.events)
	for in != nil || len(queue) > 0 {
		var out chan Event
		var next Event
		if len(queue) > 0 {
			out, next = p.events, queue[0]
		} else if picture != nil {
			out, next = p.events, *picture
		}
		select {
		case e, ok := <-in:
			if !ok {
				in = nil
			} else {
				queue = append(queue, e)
			}
		case e := <-p.spectrumIn:
			picture = &e
		case out <- next:
			if len(queue) > 0 {
				queue[0] = Event{}
				queue = queue[1:]
			} else {
				picture = nil
			}
		}
	}
}

// Close requests a graceful quit, then kills an unresponsive child. It always
// waits for process/IPC cleanup, but never waits for the event consumer.
func (p *MPV) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closing = true
		p.mu.Unlock()
		close(p.quit)
		select {
		case <-p.processDone:
		case <-time.After(750 * time.Millisecond):
			_ = p.cmd.Process.Kill()
			<-p.processDone
		}
		<-p.finished
		p.closeErr = os.RemoveAll(p.dir)
	})
	return p.closeErr
}

func (p *MPV) diagnosticSuffix() string {
	if text := strings.TrimSpace(p.diagnostics.String()); text != "" {
		return ": " + text
	}
	return ""
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *tailBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(data)
	if len(data) >= b.limit {
		b.data = append(b.data[:0], data[len(data)-b.limit:]...)
	} else {
		if over := len(b.data) + len(data) - b.limit; over > 0 {
			b.data = b.data[over:]
		}
		b.data = append(b.data, data...)
	}
	return n, nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}
