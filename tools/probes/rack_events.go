//go:build ignore

// Public-API rack event-batch/deadline probe. It creates offscreen windows only.
// Run with tools/testenv.sh go run ./tools/probes/rack_events.go.
package main

import (
	"fmt"
	"time"

	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

type moveDriver struct {
	widget.Base
	step func()
}

func (c *moveDriver) KeyPress(widget.KeyEvent) bool { c.step(); return true }

type rig struct {
	a         *app.Application
	d         *rack.Desk
	main, sat *app.Window
	mo, so    *platform.Offscreen
	si        int
	now       time.Time
}

func newRig() *rig {
	r := &rig{a: app.New(app.Options{Headless: true, Look: style.DarkLook(), Scale: 1}), now: time.Unix(1000, 0)}
	open := func(y int) *app.Window {
		w, err := r.a.NewWindow(platform.WindowOptions{X: 0, Y: y, Width: 200, Height: 100, Headless: true, Decorations: platform.DecorationsNone})
		if err != nil {
			panic(err)
		}
		return w
	}
	r.main, r.sat = open(0), open(100)
	r.mo, r.so = r.main.Surface().(*platform.Offscreen), r.sat.Surface().(*platform.Offscreen)
	r.a.PumpOnce()
	r.d = rack.NewDesk(10)
	r.d.SetClock(func() time.Time { return r.now })
	mi := r.d.Add("Main", r.main)
	r.si = r.d.Add("Satellite", r.sat)
	r.d.Attach(r.si, mi, rack.SideBottom)
	return r
}

func (r *rig) close() { r.main.Close(); r.sat.Close() }

func position(w *app.Window) [2]int {
	x, y, ok := w.Position()
	if !ok {
		panic("fixture position unavailable")
	}
	return [2]int{x, y}
}

func batch() {
	r := newRig()
	defer r.close()
	driver := &moveDriver{}
	driver.Init(driver)
	driver.SetWantsFocus(true)
	step := 0
	driver.step = func() { step++; r.mo.Move(10*step, 0) }
	r.main.SetContent(driver)
	r.main.RequestFocus(driver)
	r.a.PumpOnce()
	var placements [][2]int
	r.d.Moved = func() { placements = append(placements, position(r.sat)) }
	// SimulateDesktopMove updates its position immediately. Queuing several
	// of those before Pump would make every callback observe the FINAL cached
	// position, accidentally appearing coalesced. Interleave a public key
	// handler that updates the position with each EventMove, so positions
	// become observable in the same order a backend processes configures.
	for i := 1; i <= 4; i++ {
		r.main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyRight})
		r.main.Inject(platform.Event{Kind: platform.EventMove, Width: 10 * i, Height: 0})
	}
	r.a.PumpOnce()
	fmt.Printf("single Pump: initiator events=4 changed follow passes=%d follower placements=%v final=%v\n", len(placements), placements, position(r.sat))
	if position(r.sat) != [2]int{40, 100} {
		panic("final event-batch position was lost")
	}
}

func precachedBatch() {
	r := newRig()
	defer r.close()
	passes := 0
	r.d.Moved = func() { passes++ }
	for i := 1; i <= 4; i++ {
		r.mo.SimulateDesktopMove(10*i, 0)
	}
	r.a.PumpOnce()
	fmt.Printf("pre-cached offscreen control: initiator events=4 changed follow passes=%d final=%v (position was already final before dispatch)\n", passes, position(r.sat))
}

func reentrant() {
	r := newRig()
	defer r.close()
	var placements [][2]int
	r.d.Moved = func() {
		placements = append(placements, position(r.sat))
		if len(placements) == 1 {
			// Deliver Main's final move while the first Follow is still
			// notifying Moved. The stock Desk owns both OnMove callbacks.
			r.mo.SimulateDesktopMove(350, 350)
			r.a.PumpOnce()
		}
	}
	r.mo.SimulateDesktopMove(300, 300)
	r.a.PumpOnce()
	fmt.Printf("reentrant final move: follower placements=%v final=%v\n", placements, position(r.sat))
	if position(r.sat) != [2]int{350, 450} {
		panic("reentrant final move was lost")
	}
}

func deadline() {
	r := newRig()
	defer r.close()
	// The public offscreen backend acknowledges Move immediately. Briefly
	// hide Position while Show issues a move, making stock Desk record its
	// real 250ms deadline, then expose a stale/refused position without an
	// acknowledgement event. This is a controlled deadline fixture, not a
	// claim that normal offscreen Move is asynchronous or refuses placement.
	r.so.SimulateGeometry(platform.GeometryPosition)
	r.d.Show(r.si, true)
	r.so.SimulateGeometry(0)
	r.so.Move(700, 700)
	r.sat.Inject(platform.Event{Kind: platform.EventMove, Width: 700, Height: 700})
	r.a.PumpOnce()
	for i := 0; i < 100; i++ {
		r.d.Follow()
	}
	before := r.d.Rack.Pane(r.si).Box
	fmt.Printf("held clock after100 Follow calls: rack=%d,%d reported=%v bondedTo=%d\n", before.X, before.Y, position(r.sat), r.d.Rack.Pane(r.si).To)
	if before.X != 0 || before.Y != 100 {
		panic("deadline was consumed by calls rather than time")
	}
	r.now = r.now.Add(249 * time.Millisecond)
	r.d.Follow()
	if p := r.d.Rack.Pane(r.si); p.Box.X != 0 || p.Box.Y != 100 {
		panic("deadline expired before 250ms")
	}
	r.now = r.now.Add(2 * time.Millisecond)
	for i := 0; i < 3; i++ {
		r.a.PumpOnce()
	}
	stale := r.d.Rack.Pane(r.si).Box
	fmt.Printf("after251ms and3 idle Pumps: rack=%d,%d reported=%v bondedTo=%d\n", stale.X, stale.Y, position(r.sat), r.d.Rack.Pane(r.si).To)
	changed := r.d.Follow()
	after := r.d.Rack.Pane(r.si).Box
	fmt.Printf("explicit Follow after deadline: changed=%t rack=%d,%d reported=%v bondedTo=%d\n", changed, after.X, after.Y, position(r.sat), r.d.Rack.Pane(r.si).To)
	if after.X != 700 || after.Y != 700 || r.d.Rack.Pane(r.si).To != -1 {
		panic("expired deadline did not accept refused position")
	}
}

func acknowledgement() {
	r := newRig()
	defer r.close()
	r.so.SimulateGeometry(platform.GeometryPosition)
	r.d.Show(r.si, true)
	r.so.SimulateGeometry(0)
	r.so.Move(700, 700)
	r.sat.Inject(platform.Event{Kind: platform.EventMove, Width: 700, Height: 700})
	r.a.PumpOnce()
	// The requested position arrives without advancing the clock. Its
	// acknowledgement must end the wait immediately, so a subsequent real
	// drag cannot be hidden by the remainder of that old 250ms deadline.
	r.so.SimulateDesktopMove(0, 100)
	r.a.PumpOnce()
	r.so.SimulateDesktopMove(800, 800)
	r.a.PumpOnce()
	p := r.d.Rack.Pane(r.si)
	fmt.Printf("acknowledged then dragged at frozen clock: rack=%d,%d reported=%v bondedTo=%d\n", p.Box.X, p.Box.Y, position(r.sat), p.To)
	if p.Box.X != 800 || p.Box.Y != 800 || p.To != -1 {
		panic("acknowledgement failed to clear the pending deadline")
	}
}

func main() {
	// Keep the import list restricted to toolkit/public types. No
	// player guards, polling pulse, or native display participates.
	precachedBatch()
	batch()
	reentrant()
	deadline()
	acknowledgement()
}
