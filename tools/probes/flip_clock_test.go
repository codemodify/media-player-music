//go:build ignore

package flipclock_test

import (
	"testing"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

// An application need not replace a process-global clock to step one panel.
// tools/testenv.sh go test -p 2 -race tools/probes/flip_clock_test.go
func TestPublicFlipClocksAreIndependent(t *testing.T) {
	t.Setenv(style.AnimationsEnv, "1")
	a := ui.New(ui.Options{Headless: true, Scale: 1, Look: style.DarkLook(), DisableLookWatch: true})
	open := func() (*ui.Window, *widgets.FlipPanel) {
		w, err := a.NewWindow(platform.WindowOptions{Width: 120, Height: 80, Headless: true, Decorations: platform.DecorationsNone})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(w.Close)
		p := w.SetFlipFaces(widgets.NewButton("Front", nil), widgets.NewButton("Back", nil))
		p.Strips = 12
		a.PumpOnce()
		w.Capture()
		return w, p
	}
	w1, p1 := open()
	w2, p2 := open()
	now1, now2 := time.Unix(1000, 0), time.Unix(2000, 0)
	p1.SetClock(func() time.Time { return now1 })
	p2.SetClock(func() time.Time { return now2 })
	flips1, flips2 := 0, 0
	p1.OnFlip = func(bool) { flips1++ }
	p2.OnFlip = func(bool) { flips2++ }
	p1.ShowBack()
	p2.ShowBack()
	for i := 0; i < 3; i++ {
		w1.Capture()
		w2.Capture()
	}
	if !p1.Turning() || !p2.Turning() || flips1 != 0 || flips2 != 0 {
		t.Fatalf("frozen clocks completed a turn: turning=%t,%t flips=%d,%d", p1.Turning(), p2.Turning(), flips1, flips2)
	}
	now1 = now1.Add(time.Second)
	w1.Capture()
	w2.Capture()
	if p1.Turning() || !p2.Turning() || flips1 != 1 || flips2 != 0 || !p1.Back().Visible() {
		t.Fatalf("stepping panel1 changed the wrong turn: turning=%t,%t flips=%d,%d", p1.Turning(), p2.Turning(), flips1, flips2)
	}
	now2 = now2.Add(time.Second)
	w2.Capture()
	if p2.Turning() || flips2 != 1 || !p2.Back().Visible() {
		t.Fatalf("panel2 did not finish after its clock advanced: turning=%t flips=%d", p2.Turning(), flips2)
	}
	p1.SetClock(nil)
	p1.ShowFront()
	w1.Capture()
	if !p1.Turning() {
		t.Fatal("restoring the real clock did not start the next animation")
	}
	p1.ShowNow(false)
	if p1.Turning() || p1.Showing() || flips1 != 2 || !p1.Front().Visible() {
		t.Fatalf("immediate completion lost its public semantics: turning=%t showing=%t flips=%d", p1.Turning(), p1.Showing(), flips1)
	}
}

// Window.Flip builds a separate picture panel when the toolkit paints the
// caption. Holding only the public content panel must hold that turn too.
// This test deliberately uses no .15-only method, so the identical source
// can test the actual .14 baseline without reaching into app internals.
func TestFramedWindowUsesPublicFlipClock(t *testing.T) {
	t.Setenv(style.AnimationsEnv, "1")
	t.Setenv(platform.EnvDecorations, "")
	a := ui.New(ui.Options{Headless: true, Scale: 1, Look: style.DarkLook(), DisableLookWatch: true})
	w, err := a.NewWindow(platform.WindowOptions{Title: "Framed clock", Width: 160, Height: 110, Headless: true, Decorations: platform.DecorationsClient})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	p := w.SetFlipFaces(widgets.NewButton("Front", nil), widgets.NewButton("Back", nil))
	p.Strips = 16
	a.PumpOnce()
	w.Capture()
	if w.Decorations() != platform.DecorationsClient || w.Caption() == nil || !w.Caption().Framed() {
		t.Fatal("positive control has no toolkit-painted frame")
	}
	now := time.Unix(1000, 0)
	p.SetClock(func() time.Time { return now })
	w.Flip()
	started := time.Now()
	for i := 0; i < 3; i++ {
		w.Capture()
	}
	if !w.Turning() || p.Turning() {
		t.Fatal("the framed window did not start its distinct whole-window turn")
	}
	// Keep the old real-clock baseline distinguishable without asserting
	// a frame rate on slow instrumentation: that run must not have consumed
	// the full 420ms animation before the custom clock is stepped.
	if elapsed := time.Since(started); elapsed > 300*time.Millisecond {
		t.Skipf("no valid clock discrimination window: captures used %s", elapsed)
	}
	now = now.Add(time.Second)
	w.Capture()
	if w.Turning() {
		t.Fatal("advancing the reachable panel clock did not finish the framed window's turn")
	}
	if !w.ShowingBack() || !p.Back().Visible() || !w.Caption().Framed() {
		t.Fatal("finished turn did not preserve the back face and toolkit frame")
	}
}

func TestPublicFlipClockGetter(t *testing.T) {
	p := widgets.NewFlipPanel(widgets.NewLabel("front"), widgets.NewLabel("back"))
	getter, ok := any(p).(interface{ Clock() func() time.Time })
	if !ok {
		t.Fatal("FlipPanel has no public Clock accessor")
	}
	if getter.Clock() != nil {
		t.Fatal("the default clock should be nil for real time")
	}
	now := time.Unix(3000, 0)
	p.SetClock(func() time.Time { return now })
	if getter.Clock() == nil || !getter.Clock()().Equal(now) {
		t.Fatal("the accessor did not preserve the caller's clock")
	}
	p.SetClock(nil)
	if getter.Clock() != nil {
		t.Fatal("the accessor did not restore the nil real-clock policy")
	}
}
