//go:build ignore

// Public diagnostic for independent refused requests issued at different times.
// tools/testenv.sh go run -p 2 tools/probes/rack_staggered_deadline.go
// Prints deadline observations; zero exit is not proof of the early-wake policy.
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
	a := ui.New(ui.Options{Headless: true, Look: style.DarkLook(), Scale: 1, DisableLookWatch: true})
	open := func(x, y int) *ui.Window {
		w, err := a.NewWindow(platform.WindowOptions{X: x, Y: y, Width: 200, Height: 100, Headless: true, Decorations: platform.DecorationsNone})
		if err != nil {
			panic(err)
		}
		return w
	}
	main, first, second := open(0, 0), open(0, 100), open(200, 0)
	defer main.Close()
	defer first.Close()
	defer second.Close()
	d := rack.NewDesk(10)
	mi, fi, si := d.Add("main", main), d.Add("first", first), d.Add("second", second)
	d.Attach(fi, mi, rack.SideBottom)
	d.Attach(si, mi, rack.SideRight)
	for i := 0; i < 4; i++ {
		a.PumpOnce()
	}
	first.Surface().(*platform.Offscreen).SetMoveHonoured(false)
	second.Surface().(*platform.Offscreen).SetMoveHonoured(false)
	pumpUntil := func(until time.Time) {
		for time.Now().Before(until) {
			a.PumpOnce()
			time.Sleep(time.Millisecond)
		}
		// Dispatch a timer posted on the last turn before reading the rack.
		a.PumpOnce()
		a.PumpOnce()
	}
	start := time.Now()
	d.Attach(fi, mi, rack.SideLeft)
	pumpUntil(start.Add(160 * time.Millisecond))
	secondAt := time.Now()
	if secondAt.Sub(start) >= 240*time.Millisecond {
		fmt.Printf("SKIP early deadline observation: scheduling consumed %s before second request\n", secondAt.Sub(start))
		return
	}
	d.Attach(si, mi, rack.SideTop)
	pumpUntil(start.Add(310 * time.Millisecond))
	elapsed := time.Since(start)
	if time.Since(secondAt) >= 240*time.Millisecond {
		fmt.Printf("SKIP early deadline observation: sampling occurred after the second deadline (%s total)\n", elapsed)
		return
	}
	fx, fy, _ := first.Position()
	sx, sy, _ := second.Position()
	fp, sp := d.Rack.Pane(fi), d.Rack.Pane(si)
	// Their actual original positions are adjacent to Main, so a correct
	// reconciliation may retain a newly chosen bond. Check geometry rather
	// than incorrectly demanding detachment from those valid neighbours.
	firstSettled := fp.Box.X == fx && fp.Box.Y == fy
	fmt.Printf("staggered request at %dms, sample at %dms: firstDeadline=250ms first actual=%d,%d rack=%+v parent=%d reconciled=%t; second actual=%d,%d rack=%+v parent=%d\n", secondAt.Sub(start).Milliseconds(), elapsed.Milliseconds(), fx, fy, fp.Box, fp.To, firstSettled, sx, sy, sp.Box, sp.To)
	pumpUntil(secondAt.Add(350 * time.Millisecond))
	finalFirst := fp.Box.X == fx && fp.Box.Y == fy
	finalSecond := sp.Box.X == sx && sp.Box.Y == sy
	fmt.Printf("after latest deadline at %dms: firstReconciled=%t secondReconciled=%t firstRack=%+v secondRack=%+v\n", time.Since(start).Milliseconds(), finalFirst, finalSecond, fp.Box, sp.Box)
	if !finalFirst || !finalSecond {
		panic("staggered requests did not eventually settle")
	}
}
