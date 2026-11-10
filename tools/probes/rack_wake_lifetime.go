//go:build ignore

// Public Desk deadline checks for API-driven requests without move events.
// tools/testenv.sh go run -p 2 -race tools/probes/rack_wake_lifetime.go
// Read automaticallyReconciled for each case: a zero exit only certifies
// the positive control, not the known failing closure variants.
package main

import (
	"fmt"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

func main() {
	for _, kind := range []string{"direct Apply control", "direct Apply then last closes", "two Show requests then last closes"} {
		a := ui.New(ui.Options{Headless: true, Look: style.DarkLook(), Scale: 1, DisableLookWatch: true})
		open := func(y int) *ui.Window {
			w, err := a.NewWindow(platform.WindowOptions{X: 0, Y: y, Width: 200, Height: 100, Headless: true, Decorations: platform.DecorationsNone})
			if err != nil {
				panic(err)
			}
			return w
		}
		main, satellite, last := open(0), open(100), open(200)
		d := rack.NewDesk(10)
		mi, si, li := d.Add("main", main), d.Add("surviving satellite", satellite), d.Add("later satellite", last)
		d.Attach(si, mi, rack.SideBottom)
		d.Attach(li, si, rack.SideBottom)
		// Clear startup layout/paint work; all later geometry requests use
		// public Move/Apply/Show, with no SimulateDesktopMove event to run
		// the Desk's private followNow/rearm path first.
		for i := 0; i < 4; i++ {
			a.PumpOnce()
		}
		so, lo := satellite.Surface().(*platform.Offscreen), last.Surface().(*platform.Offscreen)
		if kind == "two Show requests then last closes" {
			d.Show(si, false)
			d.Show(li, false)
			so.Move(700, 700)
			lo.Move(900, 900)
			so.SetMoveHonoured(false)
			lo.SetMoveHonoured(false)
			d.Show(si, true)
			d.Show(li, true)
		} else {
			so.SetMoveHonoured(false)
			lo.SetMoveHonoured(false)
			d.Rack.MoveTo(mi, 600, 600)
			d.Apply()
		}
		if kind != "direct Apply control" {
			last.Close()
		}
		requested := d.Rack.Pane(si).Box
		until := time.Now().Add(600 * time.Millisecond)
		for time.Now().Before(until) {
			a.PumpOnce()
			time.Sleep(2 * time.Millisecond)
		}
		p := d.Rack.Pane(si)
		x, y, _ := satellite.Position()
		reconciled := p.Box.X == x && p.Box.Y == y && p.To == -1
		fmt.Printf("%s: requested=%+v actual=%d,%d after600ms rack=%+v parent=%d automaticallyReconciled=%t\n", kind, requested, x, y, p.Box, p.To, reconciled)
		changed := d.Follow()
		fmt.Printf("%s explicit Follow: changed=%t rack=%+v parent=%d\n", kind, changed, p.Box, p.To)
		main.Close()
		satellite.Close()
		last.Close()
		if kind == "direct Apply control" && !reconciled {
			panic("direct Apply positive control failed")
		}
	}
}
