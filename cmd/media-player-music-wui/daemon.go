package main

import (
	"context"
	"sync"
	"time"

	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/app-plugin/client/ensure"
	"goframework.io/app-plugin/client/ensure/userservice"
)

// ensureDaemon is ensure.Daemon; the tests replace it, so that none starts
// a daemon of the user's or reaches a service manager.
var ensureDaemon = ensure.Daemon

// starter starts media-player-musicd for the page when it does not answer
// at the default address (goframework R115), as office-calculator's web UI
// does: at start, again, at most every again, whenever a page's connection
// cannot reach it, and at once when the page's Retry asks. It keeps what
// the page shows meanwhile (server.Starter). The lock is ensure's default,
// by the daemon's name: the desktop and terminal UIs' too.
type starter struct {
	ctx     context.Context
	address string
	logs    string
	log     func(format string, v ...any)
	again   time.Duration

	mutex   sync.Mutex
	busy    bool
	last    time.Time
	text    string // "Starting media-player-musicd…", or why it could not be
	problem bool
}

// Start starts the daemon now, unless a start is under way.
func (thisRef *starter) Start() { thisRef.start(true) }

// Unreachable starts the daemon again, unless the last start was less than
// again ago.
func (thisRef *starter) Unreachable() { thisRef.start(false) }

func (thisRef *starter) start(forced bool) {
	thisRef.mutex.Lock()
	if thisRef.busy || !forced && time.Since(thisRef.last) < thisRef.again {
		thisRef.mutex.Unlock()
		return
	}
	thisRef.busy, thisRef.last = true, time.Now()
	thisRef.mutex.Unlock()
	go func() {
		_, err := ensureDaemon(thisRef.ctx, ensure.Options{
			Name: musicapi.DaemonName, Address: thisRef.address, Logs: thisRef.logs,
			Service: userservice.Starter(musicapi.DaemonName),
			Starting: func(ensure.How) {
				thisRef.mutex.Lock()
				thisRef.text, thisRef.problem = "Starting "+musicapi.DaemonName+"…", false
				thisRef.mutex.Unlock()
			},
		})
		thisRef.mutex.Lock()
		defer thisRef.mutex.Unlock()
		thisRef.busy, thisRef.text, thisRef.problem = false, "", false
		if err != nil && thisRef.ctx.Err() == nil {
			thisRef.text, thisRef.problem = err.Error(), true
			thisRef.log("starting %s: %v", musicapi.DaemonName, err)
		}
	}()
}

// Status is what the page shows while the daemon is away: "" when there is
// nothing to say.
func (thisRef *starter) Status() (string, bool) {
	thisRef.mutex.Lock()
	defer thisRef.mutex.Unlock()
	return thisRef.text, thisRef.problem
}
