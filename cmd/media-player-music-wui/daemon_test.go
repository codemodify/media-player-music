package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"goframework.io/app-plugin/client/ensure"
)

// The web UI starts media-player-musicd (a stand-in for ensure.Daemon
// here: none of the user's is started, no service manager reached), says
// "Starting…" meanwhile and why it could not after; a page that cannot
// reach it starts it again at most every again, the page's Retry at once
// (R115).
func TestStarter(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	ensureDaemon = func(ctx context.Context, options ensure.Options) (ensure.Result, error) {
		calls.Add(1)
		if options.Name != "media-player-musicd" || options.Address != DefaultDaemon || options.Lock != "" || options.Service == nil || options.Logs != "/logs" {
			t.Errorf("ensure's options: %+v", options)
		}
		options.Starting(ensure.Program)
		<-release
		return ensure.Result{}, &ensure.Error{Err: ensure.ErrEnded, Text: "media-player-musicd ended before it was ready (exit status 1)"}
	}
	defer func() { ensureDaemon = ensure.Daemon }()
	daemons := &starter{ctx: context.Background(), address: DefaultDaemon, logs: "/logs", log: func(string, ...any) {}, again: time.Hour}
	daemons.Start()
	wait := func(want string, problem bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			if text, wrong := daemons.Status(); text == want && wrong == problem {
				return
			}
			if time.Now().After(deadline) {
				text, wrong := daemons.Status()
				t.Fatalf("%q %v, want %q %v", text, wrong, want, problem)
			}
		}
	}
	wait("Starting media-player-musicd…", false)
	daemons.Start() // under way: not again
	close(release)
	wait("media-player-musicd ended before it was ready (exit status 1)", true)
	daemons.Unreachable() // too soon
	if calls.Load() != 1 {
		t.Fatalf("%d starts", calls.Load())
	}
	daemons.Start() // the page's Retry: at once
	for deadline := time.Now().Add(5 * time.Second); calls.Load() != 2; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("not started again")
		}
	}
	wait("media-player-musicd ended before it was ready (exit status 1)", true)
	daemons.again = 0
	daemons.Unreachable()
	for deadline := time.Now().Add(5 * time.Second); calls.Load() != 3; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("not started again when unreachable")
		}
	}
}
