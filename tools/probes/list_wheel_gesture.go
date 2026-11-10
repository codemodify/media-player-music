//go:build ignore

// Standalone stock ListView wheel/activation probe. Run with tools/testenv.sh.
// Positive wheel Y scrolls down; negative Y scrolls up. Calls are immediate so
// elapsed double-click time cannot hide the gesture-history result.
package main

import (
	"fmt"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func main() {
	for _, name := range []string{"ordinary double click", "fitting list: up", "fitting list: down", "top boundary: up", "bottom boundary: down", "scroll down then back up"} {
		count := 10
		if name == "fitting list: up" || name == "fitting list: down" {
			count = 2
		}
		l := widgets.NewListView(count, nil, nil)
		l.Arrange(paint2d.XYWH(0, 0, 200, 100))
		if name == "bottom boundary: down" {
			l.ScrollTo(1e6)
		}
		activations := 0
		l.OnActivate = func(int) { activations++ }
		click := widget.MouseEvent{Button: platform.ButtonLeft, Pos: paint2d.Pt(20, 30)}
		l.MousePress(click)
		before := l.OffsetY
		handled, moved := false, false
		scroll := func(y float32) {
			old := l.OffsetY
			handled = l.MouseWheel(widget.MouseEvent{Scroll: paint2d.Pt(0, y)}) || handled
			moved = l.OffsetY != old || moved
		}
		switch name {
		case "fitting list: up", "top boundary: up":
			scroll(-1)
		case "fitting list: down", "bottom boundary: down":
			scroll(1)
		case "scroll down then back up":
			scroll(1)
			scroll(-1)
		}
		l.MousePress(click)
		want := 0
		if name == "ordinary double click" {
			want = 1
		}
		fmt.Printf("%s: count=%d offset=%g->%g wheelHandled=%t moved=%t activations=%d; want %d\n", name, count, before, l.OffsetY, handled, moved, activations, want)
	}
}
