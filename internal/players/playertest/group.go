package playertest

import (
	"testing"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
)

// SettleGroup pumps a's loop until g, a racked face's panels with a window
// each, has put its declared bonds and places in place and lets the windows
// go where the desktop takes them: the group says so by moving (Moved) as
// it stops settling, with nothing else asked of it meanwhile. A group that
// says nothing for three seconds is taken as settled already.
func SettleGroup(t *testing.T, a *ui.Application, g *rack.Group) {
	t.Helper()
	moved := false
	inner := g.Moved
	g.Moved = func() {
		moved = true
		if inner != nil {
			inner()
		}
	}
	defer func() { g.Moved = inner }()
	for deadline := time.Now().Add(3 * time.Second); !moved && time.Now().Before(deadline); {
		a.PumpOnce()
		time.Sleep(2 * time.Millisecond)
	}
	a.PumpOnce()
	a.PumpOnce()
}

// Follow pumps a's loop for the two turns the group's desk takes to follow
// what the desktop said: the batch of events, and the pass it posts.
func Follow(a *ui.Application) {
	a.PumpOnce()
	a.PumpOnce()
}
