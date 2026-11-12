package link

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/app-plugin/client/ensure"
)

// With Ensure, a renderer starts media-player-musicd first (here a
// stand-in for ensure.Daemon: no daemon of the user's is started, no
// service manager reached) and says so; when it cannot be, says why with
// Retry, and dials nothing; Retry starts it again, and once it runs the
// connection dials, once (R115).
func TestEnsureStartsTheDaemonThenDials(t *testing.T) {
	address := "unix://" + filepath.Join(t.TempDir(), "absent.sock")
	answers := make(chan error)
	var calls atomic.Int32
	ensureDaemon = func(ctx context.Context, options ensure.Options) (ensure.Result, error) {
		calls.Add(1)
		if options.Name != "media-player-musicd" || options.Address != address || options.Service == nil || options.Lock != "" {
			t.Errorf("ensure's options: %+v", options)
		}
		options.Starting(ensure.Program)
		return ensure.Result{}, <-answers
	}
	defer func() { ensureDaemon = ensure.Daemon }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	told := make(chan Daemon, 16)
	statuses := make(chan string, 64)
	Start(ctx, Options{
		Address: address, Name: "test", Version: "test", Ensure: true,
		State:  func(musicapi.State) {},
		Daemon: func(daemon Daemon) { told <- daemon },
		Status: func(text string, _ bool) {
			select {
			case statuses <- text:
			default:
			}
		},
	})
	next := func() Daemon {
		t.Helper()
		select {
		case daemon := <-told:
			return daemon
		case <-time.After(10 * time.Second):
			t.Fatal("Daemon not told")
		}
		return Daemon{}
	}
	if daemon := next(); !daemon.Starting || daemon.Text() != "Starting media-player-musicd…" || daemon.Problem() {
		t.Fatalf("%+v %q", daemon, daemon.Text())
	}
	answers <- &ensure.Error{Name: "media-player-musicd", Err: ensure.ErrNotFound, Text: "media-player-musicd not found next to this program or on PATH"}
	failed := next()
	if !failed.Problem() || failed.Retry == nil || failed.Text() != "media-player-musicd not found next to this program or on PATH" || !errors.Is(failed.Err, ensure.ErrNotFound) {
		t.Fatalf("%+v %q", failed, failed.Text())
	}
	select {
	case text := <-statuses:
		t.Fatalf("dialled before the daemon ran: %q", text)
	case <-time.After(200 * time.Millisecond):
	}

	failed.Retry()
	if daemon := next(); !daemon.Starting {
		t.Fatalf("%+v", daemon)
	}
	answers <- nil
	if daemon := next(); daemon.Text() != "" || daemon.Problem() || daemon.Retry != nil {
		t.Fatalf("%+v", daemon)
	}
	select {
	case text := <-statuses:
		if text == "" {
			t.Fatal("connected to nothing")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("not dialled once the daemon ran")
	}
	if calls.Load() != 2 {
		t.Fatalf("%d starts", calls.Load())
	}
}

// Without Ensure (an address given), nothing is started and the
// connection dials at once.
func TestNoEnsureStartsNothing(t *testing.T) {
	ensureDaemon = func(context.Context, ensure.Options) (ensure.Result, error) {
		t.Error("ensure called")
		return ensure.Result{}, nil
	}
	defer func() { ensureDaemon = ensure.Daemon }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	statuses := make(chan string, 16)
	Start(ctx, Options{
		Address: "unix://" + filepath.Join(t.TempDir(), "absent.sock"), Name: "test", Version: "test",
		Daemon: func(Daemon) { t.Error("Daemon told") },
		Status: func(text string, _ bool) {
			select {
			case statuses <- text:
			default:
			}
		},
	})
	select {
	case <-statuses:
	case <-time.After(10 * time.Second):
		t.Fatal("not dialled")
	}
}

// EnsureOptions: the daemon's name, the user's service first, its output
// in the music's log folder, ensure's lock by the daemon's name.
func TestEnsureOptions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	options := EnsureOptions("ws://127.0.0.1:50130/clients")
	if options.Name != "media-player-musicd" || options.Address != "ws://127.0.0.1:50130/clients" || options.Service == nil || options.Lock != "" || options.Logs == "" {
		t.Fatalf("%+v", options)
	}
}
