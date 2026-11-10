package playertest

import (
	"reflect"
	"testing"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
)

// DeskFixture exercises a presentation's actual desk without invoking its
// periodic Advance methods. Its windows must be offscreen; no desktop or
// playback process participates.
type DeskFixture struct {
	App             *ui.Application
	Desk            *rack.Desk
	Main, Satellite int
	Show            func(bool)
}

// AuditDeskRuntime checks deferred native-event integration and toolkit-owned
// reconciliation deadlines, including independently staggered requests.
func AuditDeskRuntime(t *testing.T, open func(*testing.T) DeskFixture) {
	t.Helper()
	drain := func(f DeskFixture) {
		for i := 0; i < 4; i++ {
			f.App.PumpOnce()
		}
	}
	t.Run("deferred-burst", func(t *testing.T) {
		f := open(t)
		drain(f)
		before := f.Desk.Rack.Boxes()
		passes := 0
		observe := f.Desk.Moved
		f.Desk.Moved = func() {
			passes++
			if observe != nil {
				observe()
			}
		}
		main := f.Desk.Window(f.Main).Surface().(*platform.Offscreen)
		for i := 0; i < 80; i++ {
			main.SimulateDesktopMove(600+i, 500+i)
		}
		f.App.PumpOnce()
		if !reflect.DeepEqual(f.Desk.Rack.Boxes(), before) || passes != 0 {
			t.Fatal("presentation still follows eagerly inside the configure batch")
		}
		f.App.PumpOnce()
		dx, dy := 679-before[f.Main].X, 579-before[f.Main].Y
		for i, old := range before {
			if got := f.Desk.Rack.Pane(i).Box; got != old.Moved(dx, dy) {
				t.Fatalf("pane%d missed the final native position: %+v", i, got)
			}
		}
		drain(f)
		if passes != 1 {
			t.Fatalf("acknowledged batch did not settle once: passes%d", passes)
		}
	})
	t.Run("refused-reshow", func(t *testing.T) {
		f := open(t)
		drain(f)
		now := time.Now()
		f.Desk.SetClock(func() time.Time { return now })
		f.Show(false)
		drain(f)
		satellite := f.Desk.Window(f.Satellite).Surface().(*platform.Offscreen)
		satellite.Move(1700, 1200) // A hidden desktop placement emits no event.
		satellite.SetMoveHonoured(false)
		f.Show(true)
		drain(f)
		if f.Desk.Rack.Pane(f.Satellite).To < 0 {
			t.Fatal("refused Show did not retain a pending requested bond")
		}
		now = now.Add(time.Second)
		pumpDeskUntil(f, time.Now().Add(350*time.Millisecond))
		assertRefusalSettled(t, f, 1700, 1200)
	})
	t.Run("closed-wake-owner", func(t *testing.T) {
		f := open(t)
		drain(f)
		main := f.Desk.Window(f.Main)
		owner, err := f.App.NewWindow(platform.WindowOptions{Width: 80, Height: 40, Headless: true, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: main.Surface()})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(owner.Close)
		last := f.Desk.Add("deadline owner", owner)
		f.Desk.Attach(last, f.Main, rack.SideLeft)
		drain(f)
		now := time.Now()
		f.Desk.SetClock(func() time.Time { return now })
		satellite := f.Desk.Window(f.Satellite).Surface().(*platform.Offscreen)
		x, y, _ := satellite.Position()
		satellite.SetMoveHonoured(false)
		owner.Surface().(*platform.Offscreen).SetMoveHonoured(false)
		main.Surface().(*platform.Offscreen).SimulateDesktopMove(900, 800)
		f.App.PumpOnce()
		f.App.PumpOnce()
		if p := f.Desk.Rack.Pane(f.Satellite); p.Box.X == x && p.Box.Y == y {
			t.Fatal("native Apply refusal did not retain the requested geometry")
		}
		owner.Close() // The surviving pending request must outlive this satellite.
		now = now.Add(time.Second)
		pumpDeskUntil(f, time.Now().Add(350*time.Millisecond))
		assertRefusalSettled(t, f, x, y)
	})
	t.Run("staggered-show-deadlines", func(t *testing.T) {
		f := open(t)
		drain(f)
		main := f.Desk.Window(f.Main)
		later, err := f.App.NewWindow(platform.WindowOptions{Width: 80, Height: 40, Headless: true, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: main.Surface()})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(later.Close)
		last := f.Desk.Add("later request", later)
		f.Desk.Attach(last, f.Main, rack.SideLeft)
		drain(f)
		f.Show(false)
		f.Desk.Show(last, false)
		drain(f)
		first := f.Desk.Window(f.Satellite).Surface().(*platform.Offscreen)
		second := later.Surface().(*platform.Offscreen)
		first.Move(1700, 1200)
		second.Move(1800, 1300)
		first.SetMoveHonoured(false)
		second.SetMoveHonoured(false)
		now := time.Now()
		f.Desk.SetClock(func() time.Time { return now })
		start := time.Now()
		f.Show(true)
		// Issue the independent second Show 160 real milliseconds later.
		// Do not dispatch a timer with the logical clock held at t0: an
		// expensive race-instrumented frame could otherwise consume its
		// entire real delay and incorrectly rearm that unexpired clock.
		time.Sleep(time.Until(start.Add(160 * time.Millisecond)))
		now = now.Add(160 * time.Millisecond)
		f.Desk.Show(last, true)
		now = now.Add(150 * time.Millisecond) // First is expired; second is not.
		pumpDeskUntil(f, start.Add(310*time.Millisecond))
		p, q := f.Desk.Rack.Pane(f.Satellite), f.Desk.Rack.Pane(last)
		if p.Box.X != 1700 || p.Box.Y != 1200 || p.To >= 0 {
			t.Fatalf("later Show postponed the toolkit's first deadline: %+v", p)
		}
		if q.Box.X == 1800 && q.Box.Y == 1300 || q.To < 0 {
			t.Fatalf("toolkit reconciled the later request before its own deadline: %+v", q)
		}
		now = now.Add(200 * time.Millisecond)
		// Allow the remaining UI timer its own dispatch interval even if
		// the first race-instrumented frame outlasted the wall-clock sample.
		// Its logical deadline is still 410ms; the clock now reads 510ms.
		pumpDeskUntil(f, time.Now().Add(200*time.Millisecond))
		if q.Box.X != 1800 || q.Box.Y != 1300 || q.To >= 0 {
			t.Fatalf("later request did not settle after its own deadline: %+v", q)
		}
	})
}

func assertRefusalSettled(t *testing.T, f DeskFixture, x, y int) {
	t.Helper()
	p := f.Desk.Rack.Pane(f.Satellite)
	if p.Box.X != x || p.Box.Y != y || p.To >= 0 {
		t.Fatalf("toolkit timer did not reconcile refused geometry: pane%+v", p)
	}
	if !f.Desk.Window(f.Satellite).Visible() || f.Desk.Window(f.Main).Closed() {
		t.Fatal("geometry reconciliation hid or closed the surviving player")
	}
}

// Pump the normal application loop so its public UI timers can fire. This
// deliberately calls neither a presentation pulse nor Desk.Follow.
func pumpDeskUntil(f DeskFixture, until time.Time) {
	for time.Now().Before(until) {
		f.App.PumpOnce()
		time.Sleep(time.Millisecond)
	}
	// Dispatch a timer callback that was posted by the last loop turn.
	f.App.PumpOnce()
	f.App.PumpOnce()
}
