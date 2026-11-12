// Package link is the Go renderers' connection to media-player-musicd: the
// player's API, callable at once, and the player's states as they change,
// whether the daemon is up yet or comes and goes.
//
// Starting the daemon when none answers is goframework's "ensure daemon"
// facility's (release-gaps.md F1, ledger R115: app-plugin's client/ensure):
// with Options.Ensure, Start starts media-player-musicd first when it does
// not answer, says so (Options.Daemon), and says why when it cannot, with
// a Retry.
package link

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codemodify/media-player-music/musicapi"
	client "goframework.io/app-plugin/client"
	"goframework.io/app-plugin/client/ensure"
	"goframework.io/app-plugin/client/ensure/userservice"
	"goframework.io/app-spec/protocol/transport"
	disk "goframework.io/vfs-source-disk"

	// ws:// addresses too: --daemon ws://host:50130/clients
	_ "goframework.io/app-spec/protocol/transport/websocket"
)

// DefaultAddress is where the daemon listens on this machine by default.
func DefaultAddress() (string, error) {
	places, err := disk.UserPlaces(musicapi.Name)
	if err != nil {
		return "", err
	}
	return "unix://" + filepath.ToSlash(filepath.Join(places.Runtime, musicapi.SocketName)), nil
}

// Token is the token to present to the daemon at address: given, when
// one is; none for its socket, which only the user reaches and which
// admits the user's programs without one; for a ws:// or wss:// address,
// the one in media-player-musicd.token in the user's runtime folder, which
// the daemon admits (made when missing: the renderer may start first).
func Token(address, given string) (string, error) {
	if given != "" || !strings.HasPrefix(address, "ws://") && !strings.HasPrefix(address, "wss://") {
		return given, nil
	}
	places, err := disk.UserPlaces(musicapi.Name)
	if err != nil {
		return "", err
	}
	return transport.TokenFile(filepath.Join(places.Runtime, musicapi.TokenFile))
}

// Options are what a renderer hears; each is called from the connection's
// goroutines, so a renderer posts them to its UI's.
type Options struct {
	Address, Name, Version string
	// Token is presented to the daemon (Token's): none on its socket.
	Token string
	// State, Queue, Now and Library are told the player's states each
	// time they change, and as each connection is made; nil for one not
	// wanted.
	State   func(musicapi.State)
	Queue   func(musicapi.Queue)
	Now     func(musicapi.Now)
	Library func(musicapi.Library)
	// Connected is told when a connection is made (true) and ends (false).
	Connected func(bool)
	// Failed is told why connecting stopped for good (a refusal, an
	// address that does not parse).
	Failed func(error)
	// Status is told how the connection stands, in words for the person
	// (StatusText's), each time that changes, and "" once connected.
	Status func(text string, problem bool)
	// Ensure starts media-player-musicd before the first dial when it does
	// not answer (goframework R115's ensure: the user's service, else the
	// program next to this one or on PATH, detached, its output in the
	// log folder's media-player-musicd.log), telling Daemon how that goes;
	// the connection is dialled once it runs. Only for the default
	// address, where the daemon listens when started with no arguments.
	Ensure bool
	// Daemon is told, with Ensure, how starting the daemon stands:
	// starting, why it could not be started (with Retry), or nothing to
	// say (the zero Daemon).
	Daemon func(Daemon)
}

// DaemonName is the daemon's program, which a renderer starts when it does
// not answer at the default address (Options.Ensure).
const DaemonName = musicapi.DaemonName

// Daemon is how starting media-player-musicd stands, for a renderer to
// show in its window: the zero Daemon when there is nothing to say.
type Daemon struct {
	// Starting: ensure is starting it ("Starting media-player-musicd…").
	Starting bool
	// Err is why it could not be started, in words to show as they are
	// ("media-player-musicd not found next to this program or on PATH").
	Err error
	// Retry, with Err, starts it again (then dials).
	Retry func()
}

// Text is what a renderer says of it: "Starting media-player-musicd…",
// why it could not be started, or "".
func (thisRef Daemon) Text() string {
	switch {
	case thisRef.Err != nil:
		return thisRef.Err.Error()
	case thisRef.Starting:
		return "Starting " + DaemonName + "…"
	}
	return ""
}

// Problem tells whether it could not be started.
func (thisRef Daemon) Problem() bool { return thisRef.Err != nil }

// ensureDaemon is ensure.Daemon; the tests replace it, so that none starts
// a daemon of the user's or reaches a service manager.
var ensureDaemon = ensure.Daemon

// EnsureOptions are ensure's options for the daemon at address: the
// user's service first, else the program; its output in the log folder.
// The lock is ensure's default, by the daemon's name: every renderer's,
// the web UI's too.
func EnsureOptions(address string) ensure.Options {
	options := ensure.Options{Name: DaemonName, Address: address, Service: userservice.Starter(DaemonName)}
	if places, err := disk.UserPlaces(musicapi.Name); err == nil {
		options.Logs = places.Logs
	}
	return options
}

// starter starts the daemon for Start (Options.Ensure), one attempt at a
// time, then dials once: dial is the connection's Start.
type starter struct {
	ctx     context.Context
	options Options
	dial    func()
	once    sync.Once
	lock    sync.Mutex
	busy    bool
}

func (thisRef *starter) tell(daemon Daemon) {
	if thisRef.options.Daemon != nil {
		thisRef.options.Daemon(daemon)
	}
}

// attempt starts the daemon, unless an attempt is under way, and dials
// once it runs; when it cannot be started, says why with Retry.
func (thisRef *starter) attempt() {
	thisRef.lock.Lock()
	if thisRef.busy {
		thisRef.lock.Unlock()
		return
	}
	thisRef.busy = true
	thisRef.lock.Unlock()
	go func() {
		settings := EnsureOptions(thisRef.options.Address)
		settings.Starting = func(ensure.How) { thisRef.tell(Daemon{Starting: true}) }
		_, err := ensureDaemon(thisRef.ctx, settings)
		thisRef.lock.Lock()
		thisRef.busy = false
		thisRef.lock.Unlock()
		if thisRef.ctx.Err() != nil {
			return
		}
		if err != nil {
			thisRef.tell(Daemon{Err: err, Retry: thisRef.attempt})
			return
		}
		thisRef.tell(Daemon{})
		thisRef.once.Do(thisRef.dial)
	}()
}

// StatusText is a connection's status in words for the person, and
// whether it is a problem: "" once connected.
func StatusText(status client.Status) (string, bool) {
	switch status.Phase {
	case client.Connecting:
		if status.Attempt > 1 {
			return "can't reach the music daemon, retrying…", true
		}
		return "connecting to the music daemon…", false
	case client.Waiting:
		retry := "retrying in " + strconv.Itoa(int((status.Retry+time.Second-1)/time.Second)) + "s"
		if status.Attempt == 0 {
			return "lost the music daemon, " + retry, true
		}
		return "can't reach the music daemon (media-player-musicd), " + retry, true
	case client.Closed:
		return "not connected to the music daemon", status.Err != nil
	}
	return "", false
}

// Start connects, again whenever the connection ends, until ctx ends, and
// returns the player's API at once: its calls fail with
// client.ErrNotConnected while there is no connection.
func Start(ctx context.Context, options Options) musicapi.Player {
	// the newest state at each connection, then each newer one, never an
	// older one after a newer one (app-spec's states, R13)
	parts := []client.Part{}
	if options.State != nil {
		parts = append(parts, client.OnState(musicapi.StateEvent, func(_ context.Context, state musicapi.State) { options.State(state) }))
	}
	if options.Queue != nil {
		parts = append(parts, client.OnState(musicapi.QueueEvent, func(_ context.Context, queue musicapi.Queue) { options.Queue(queue) }))
	}
	if options.Now != nil {
		parts = append(parts, client.OnState(musicapi.NowEvent, func(_ context.Context, now musicapi.Now) { options.Now(now) }))
	}
	if options.Library != nil {
		parts = append(parts, client.OnState(musicapi.LibraryEvent, func(_ context.Context, library musicapi.Library) { options.Library(library) }))
	}
	connection, err := client.New(client.Options{
		Address: options.Address, Name: options.Name, Version: options.Version, Token: options.Token, Reconnect: true,
		Status: func(status client.Status) {
			if options.Status != nil {
				options.Status(StatusText(status))
			}
		},
		Connected: func(ctx context.Context, _ *client.Connection) {
			if options.Connected != nil {
				options.Connected(true)
			}
			<-ctx.Done()
			if options.Connected != nil {
				options.Connected(false)
			}
		},
	}, parts...)
	if err != nil {
		if options.Failed != nil {
			options.Failed(err)
		}
		return musicapi.NewPlayer(failing{err})
	}
	player := musicapi.NewPlayer(connection)
	if options.Ensure {
		// the daemon started first, if it must be, then dialled; the
		// renderer says "Starting media-player-musicd…" meanwhile, or why
		// it could not be, with Retry
		(&starter{ctx: ctx, options: options, dial: connection.Start}).attempt()
	} else {
		connection.Start()
	}
	go func() {
		select {
		case <-ctx.Done():
		case <-connection.Done():
			// a refusal before the first connection ends the dialling
			if err := connection.Err(); err != nil && !errors.Is(err, context.Canceled) && options.Failed != nil {
				options.Failed(err)
			}
		}
		connection.Close()
	}()
	return player
}

// failing is the caller of a connection that could not be made: every call
// fails with why.
type failing struct{ err error }

func (failing) Use(string, string)                                     {}
func (thisRef failing) Invoke(context.Context, string, any, any) error { return thisRef.err }
