// Package link is the Go renderers' connection to media-player-musicd: the
// player's API, callable at once, and the player's states as they change,
// whether the daemon is up yet or comes and goes.
//
// Starting the daemon when none answers is goframework's "ensure daemon"
// facility's (release-gaps.md F1), which this package adopts once it
// lands: Options.Ensure is where it goes. Until then a renderer says it
// cannot reach the daemon and keeps dialling.
package link

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/musicapi"
	client "goframework.io/app-plugin/client"
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
	// Ensure, when set, is called before the first dial: it is where
	// goframework's ensure-daemon facility (F1) starts media-player-musicd
	// when none answers. Nil until that lands.
	Ensure func(ctx context.Context) error
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
	if options.Ensure != nil {
		if err := options.Ensure(ctx); err != nil && options.Failed != nil {
			options.Failed(err)
		}
	}
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
	connection.Start()
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
