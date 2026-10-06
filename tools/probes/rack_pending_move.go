//go:build ignore

// Public native Desk pending-acknowledgement/final-event reproduction.
package main

import (
	"fmt"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"time"
)

func main() {
	a := app.New(app.Options{Backend: "x11", Scale: 1.75, Look: style.DarkLook(), DisableLookWatch: true})
	open := func(title string) *app.Window {
		w, e := a.NewWindow(platform.WindowOptions{Title: title, Width: 200, Height: 100, Decorations: platform.DecorationsNone})
		if e != nil {
			panic(e)
		}
		return w
	}
	main, child := open("desk-main"), open("desk-child")
	defer main.Close()
	defer child.Close()
	pump := func(d time.Duration) {
		until := time.Now().Add(d)
		for time.Now().Before(until) {
			a.PumpOnce()
			time.Sleep(time.Millisecond)
		}
	}
	pump(200 * time.Millisecond)
	main.Move(100, 100)
	child.Move(500, 100)
	pump(200 * time.Millisecond)
	d := rack.NewDesk(10)
	m, c := d.Add("main", main), d.Add("child", child)
	d.Adopt()
	now := time.Now()
	clockCalls := 0
	d.SetClock(func() time.Time { clockCalls++; return now })
	d.Attach(c, m, rack.SideBottom)
	// Both the requested placement and later final move are queued before
	// pumping. The Desk must preserve the later native position.
	for i := 0; i < 20; i++ {
		child.Move(650+i, 100)
	}
	pump(350 * time.Millisecond)
	x, y, _ := child.Position()
	frozen := d.Rack.Pane(c).Box
	frozenTo := d.Rack.Pane(c).To
	frozenCalls := clockCalls
	// More Follow calls at the same clock must not age the timeout.
	for i := 0; i < 80; i++ {
		d.Follow()
	}
	held := d.Rack.Pane(c).Box == frozen
	now = now.Add(251 * time.Millisecond)
	// Merely advancing time does not schedule another pass.
	pump(50 * time.Millisecond)
	before := d.Rack.Pane(c).Box
	changed := d.Follow()
	fmt.Printf("nativePosition=%d,%d pendingBox=%+v pendingParent=%d callbackClockCalls=%d heldAfter80Calls=%t afterClockAdvanceWithoutFollow=%+v explicitFollowChanged=%t finalBox=%+v finalParent=%d\n", x, y, frozen, frozenTo, frozenCalls, held, before, changed, d.Rack.Pane(c).Box, d.Rack.Pane(c).To)
	d.SetClock(nil)
	d.Attach(c, m, rack.SideBottom)
	child.Move(700, 150)
	pump(400 * time.Millisecond)
	x, y, _ = child.Position()
	before, parent := d.Rack.Pane(c).Box, d.Rack.Pane(c).To
	changed = d.Follow()
	fmt.Printf("realClock400ms native=%d,%d rack=%+v parent=%d explicitFollowChanged=%t\n", x, y, before, parent, changed)
}
