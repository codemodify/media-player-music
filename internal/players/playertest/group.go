package playertest

import (
	"testing"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
)

// SettleGroup pumps a's loop until g, a racked face's panels, has settled
// (rack.Group.Settled): with a window each, its declared bonds, places and
// sizes in place, and the windows where the desktop takes them from then on.
// A group settles in two seconds at most, a hundred looks on the loop's
// clock; one that has not in ten fails the test.
func SettleGroup(t *testing.T, a *ui.Application, g *rack.Group) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !g.Settled() && time.Now().Before(deadline); {
		a.PumpOnce()
		time.Sleep(2 * time.Millisecond)
	}
	if !g.Settled() {
		t.Fatal("the group did not settle")
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
