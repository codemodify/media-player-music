//go:build ignore

// Public-API Desk reconciliation probe for Apply and Show move requests.
// Run: tools/testenv.sh go run tools/probes/rack_refused_move.go
// Only offscreen windows and the toolkit's own real timer participate.
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
	for _, kind := range []string{"Apply", "Show", "Apply then latest timer window closes"} {
		a := ui.New(ui.Options{Headless: true, Look: style.DarkLook(), Scale: 1, DisableLookWatch: true})
		open := func(y int) *ui.Window {
			w, err := a.NewWindow(platform.WindowOptions{X: 0, Y: y, Width: 200, Height: 100, Headless: true, Decorations: platform.DecorationsNone})
			if err != nil {
				panic(err)
			}
			return w
		}
		main, satellite := open(0), open(100)
		var last *ui.Window
		if kind == "Apply then latest timer window closes" {
			last = open(200)
		}
		a.PumpOnce()
		d := rack.NewDesk(10)
		mi, si := d.Add("main", main), d.Add("satellite", satellite)
		d.Attach(si, mi, rack.SideBottom)
		if last != nil {
			li := d.Add("last satellite", last)
			d.Attach(li, si, rack.SideBottom)
			last.Surface().(*platform.Offscreen).SetMoveHonoured(false)
		}
		so := satellite.Surface().(*platform.Offscreen)
		if kind != "Show" {
			so.SetMoveHonoured(false)
			main.Surface().(*platform.Offscreen).SimulateDesktopMove(600, 600)
			a.PumpOnce()
			a.PumpOnce()
			if last != nil {
				last.Close()
			}
		} else {
			// While hidden the simulated desktop puts the window elsewhere,
			// without a movement event. Show must request its retained bond.
			d.Show(si, false)
			so.Move(700, 700)
			so.SetMoveHonoured(false)
			d.Show(si, true)
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
		fmt.Printf("%s refused request: requested=%+v actual=%d,%d after600ms rack=%+v parent=%d automaticallyReconciled=%t\n", kind, requested, x, y, p.Box, p.To, reconciled)
		changed := d.Follow()
		fmt.Printf("%s explicit Follow: changed=%t rack=%+v parent=%d\n", kind, changed, p.Box, p.To)
		main.Close()
		satellite.Close()
		if kind == "Apply" && !reconciled {
			panic("Apply did not automatically reconcile the refused move")
		}
	}
}
