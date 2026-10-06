//go:build ignore

// Standalone public UIToolkit list probes; run with tools/testenv.sh.
package main

import (
	"fmt"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func main() {
	activationContract()
	rowHitTesting()
	for _, interruption := range []string{"pointer exit", "focus loss", "wheel", "context", "keyboard", "modifier", "disabled"} {
		interruptedClicks(interruption)
	}
}

func activationContract() {
	a := uitoolkit.New(uitoolkit.Options{Headless: true, DisableLookWatch: true, Scale: 1})
	w, err := a.NewWindow(platform.WindowOptions{Width: 320, Height: 180, Headless: true})
	if err != nil {
		panic(err)
	}
	defer w.Close()
	selects, activations, contexts := 0, 0, 0
	l := widgets.NewListView(6, func(i int) string { return fmt.Sprint(i) }, func(int) { selects++ })
	l.Selected = 0
	l.OnActivate = func(int) { activations++ }
	l.ContextKeepsSelection = true
	l.OnContext = func(int, paintengine2d.Point) { contexts++ }
	w.SetContent(l)
	a.PumpOnce()
	click := func(row int, button platform.MouseButton) {
		p := l.AccessibleItems()[row].Bounds.Center()
		w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: p})
		w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: p})
		w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: p})
		a.PumpOnce()
	}
	click(2, platform.ButtonLeft)
	fmt.Printf("single click: selected=%d selections=%d activations=%d; want 2/1/0\n", l.Selected, selects, activations)
	click(2, platform.ButtonLeft)
	fmt.Printf("double click: activations=%d; want 1\n", activations)
	click(3, platform.ButtonRight)
	fmt.Printf("context: selected=%d contexts=%d activations=%d; want 2/1/1\n", l.Selected, contexts, activations)
	l.KeyPress(widget.KeyEvent{Key: platform.KeyReturn})
	w.AccessibleAction(widget.ItemID(l, 2), a11y.ActionDefault)
	w.AccessibleAction(widget.ItemID(l, 2), a11y.ActionDefault)
	fmt.Printf("Return plus repeated default action: activations=%d; want 4\n", activations)
	l.SetEnabled(false)
	click(1, platform.ButtonLeft)
	click(4, platform.ButtonRight)
	fmt.Printf("disabled pointer: selected=%d selections=%d contexts=%d activations=%d; want 2/2/1/4\n", l.Selected, selects, contexts, activations)
}

func rowHitTesting() {
	l := widgets.NewListView(8, nil, nil)
	l.RowGeo = func(style.LookAndFeel) *widgets.RowGeometry {
		return &widgets.RowGeometry{
			Rows: paintengine2d.XYWH(10, 20, 120, 60), Bar: paintengine2d.XYWH(140, 20, 10, 60), RowHeight: 20,
		}
	}
	l.Arrange(paintengine2d.XYWH(0, 0, 200, 100))
	for _, p := range []paintengine2d.Point{
		paintengine2d.Pt(0, 30), paintengine2d.Pt(135, 30), paintengine2d.Pt(151, 30), paintengine2d.Pt(20, 19), paintengine2d.Pt(20, 80),
	} {
		fmt.Printf("outside rows: RowAt(%v)=%d; want -1\n", p, l.RowAt(p))
	}
}

func interruptedClicks(interruption string) {
	l := widgets.NewListView(4, nil, nil)
	l.Arrange(paintengine2d.XYWH(0, 0, 200, 100))
	l.ContextKeepsSelection = true
	activations := 0
	l.OnActivate = func(int) { activations++ }
	click := widget.MouseEvent{Pos: paintengine2d.Pt(20, 40), Button: platform.ButtonLeft}
	l.MousePress(click)
	switch interruption {
	case "pointer exit":
		l.MouseExit()
	case "focus loss":
		l.FocusLost()
	case "wheel":
		l.MouseWheel(widget.MouseEvent{Scroll: paintengine2d.Pt(0, -1)})
	case "context":
		l.MousePress(widget.MouseEvent{Pos: click.Pos, Button: platform.ButtonRight})
	case "keyboard":
		l.KeyPress(widget.KeyEvent{Key: platform.KeyDown})
	case "modifier":
		l.MousePress(widget.MouseEvent{Pos: click.Pos, Button: platform.ButtonLeft, Mods: platform.ModCtrl})
	case "disabled":
		l.SetEnabled(false)
		l.MousePress(click)
		l.SetEnabled(true)
	}
	l.MousePress(click)
	fmt.Printf("after %s: activations=%d; want 0\n", interruption, activations)
}
