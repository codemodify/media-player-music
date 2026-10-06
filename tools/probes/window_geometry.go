//go:build ignore

// Standalone public X11 window geometry and utility-owner audit.
// Run on an isolated native display with -tags theme_engine_all.
package main

import (
	"fmt"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
	"os/exec"
	"strings"
	"time"
)

func main() {
	look := style.DarkLook()
	if pack, ok := style.LoadTheme("breeze"); ok {
		look = pack.Look()
	}
	a := uitoolkit.New(uitoolkit.Options{Backend: "x11", Scale: 1.75, Look: look, DisableLookWatch: true})
	a.SetTitleBarPrefs(platform.DefaultTitleBarPrefs(""))
	w, e := a.NewWindow(platform.WindowOptions{Title: "geometry audit", Width: 520, Height: 340, Decorations: platform.DecorationsClient})
	if e != nil {
		panic(e)
	}
	defer w.Close()
	w.SetContent(widgets.NewLabel("geometry"))
	w.SetTitleBar(widgets.NewHeaderBar(nil, nil, []widget.Component{widgets.NewLabel("geometry")}))
	w.Hide()
	w.Move(120, 75)
	pump := func(d time.Duration) {
		until := time.Now().Add(d)
		for time.Now().Before(until) {
			a.PumpOnce()
			time.Sleep(5 * time.Millisecond)
		}
	}
	pump(200 * time.Millisecond)
	x, y, _ := w.Position()
	fmt.Printf("hidden before-frame move=(%d,%d) want=(120,75) margin=%+v\n", x, y, platform.FrameOf(w.Surface()).Frame().Margin)
	w.SetSize(700, 450)
	pump(200 * time.Millisecond)
	ww, hh := w.Size()
	fmt.Printf("resize without RequestLayout=%dx%d want=700x450 rect=%v\n", ww, hh, w.WindowRect())
	for _, at := range [][2]int{{420, 250}, {0, 0}, {-120, -75}} {
		count, mismatch := 0, 0
		w.OnMove(func(ex, ey int) {
			px, py, _ := w.Position()
			count++
			if ex != px || ey != py {
				mismatch++
			}
			fmt.Printf("move callback=(%d,%d) position=(%d,%d)\n", ex, ey, px, py)
		})
		w.Move(at[0], at[1])
		pump(150 * time.Millisecond)
		x, y, _ = w.Position()
		fmt.Printf("hidden move=%v position=(%d,%d) callbacks=%d mismatches=%d\n", at, x, y, count, mismatch)
	}
	w.Show()
	pump(300 * time.Millisecond)
	child, e := a.NewWindow(platform.WindowOptions{Title: "utility audit", Width: 200, Height: 100, Decorations: platform.DecorationsClient, Role: platform.RoleUtility, Owner: w.Surface()})
	if e != nil {
		panic(e)
	}
	defer child.Close()
	child.SetContent(widgets.NewLabel("utility"))
	pump(300 * time.Millisecond)
	props := func(stage string) {
		id := strings.TrimPrefix(child.PortalParent(), "x11:")
		b, e := exec.Command("xprop", "-id", "0x"+id, "_NET_WM_WINDOW_TYPE", "_NET_WM_STATE", "WM_TRANSIENT_FOR").CombinedOutput()
		fmt.Printf("%s owner=%s child=%s err=%v\n%s", stage, w.PortalParent(), child.PortalParent(), e, b)
	}
	props("initial")
	child.Hide()
	pump(100 * time.Millisecond)
	child.Show()
	pump(200 * time.Millisecond)
	props("hide/show")
	forceAlpha := func(w *app.Window) {
		s := w.Surface().(platform.WindowFrame)
		f := s.Frame()
		f.Alpha = !f.Alpha
		s.SetFrame(f)
	}
	forceAlpha(child)
	pump(250 * time.Millisecond)
	props("child visual change")
	forceAlpha(w)
	pump(250 * time.Millisecond)
	props("owner visual change")
}
