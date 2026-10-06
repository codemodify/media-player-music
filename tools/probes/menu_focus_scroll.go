//go:build ignore

// Standalone UIToolkit popup focus/scroll reproduction; run with tools/testenv.sh.
package main

import (
	"fmt"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func main() {
	for _, scale := range []float32{1, 1.75} {
		a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
		w, err := a.NewWindow(platform.WindowOptions{Width: 275, Height: 116, Headless: true})
		if err != nil {
			panic(err)
		}
		button := widgets.NewButton("Menu", nil)
		w.SetContent(button)
		a.PumpOnce()
		for _, scenario := range []string{"End", "initial Up", "Home after scrolling", "arrows"} {
			picked := -1
			var items []*widgets.MenuItem
			for i := 0; i < 12; i++ {
				i := i
				items = append(items, widgets.Item(fmt.Sprintf("Choice%d", i), func() { picked = i }))
			}
			pop := widgets.ShowContextMenu(button, paintengine2d.Pt(8, 8), items...)
			a.PumpOnce()
			key := func(k platform.Key) { w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k}); a.PumpOnce() }
			target := 11
			switch scenario {
			case "End":
				key(platform.KeyEnd)
			case "initial Up":
				key(platform.KeyUp)
			case "Home after scrolling":
				for i := 0; i < 12; i++ {
					key(platform.KeyDown)
				}
				key(platform.KeyHome)
				target = 0
			case "arrows":
				for i := 0; i < 12; i++ {
					key(platform.KeyDown)
				}
			}
			r := pop.ItemBounds(target)
			fullyVisible := r.Min.Y >= 0 && r.Max.Y <= pop.Bounds().Dy()
			offset := pop.OffsetY
			key(platform.KeyReturn)
			fmt.Printf("scale=%g scenario=%q target=%d picked=%d offset=%g rowY=%g..%g viewportH=%g fullyVisible=%t popupClosed=%t\n", scale, scenario, target, picked, offset, r.Min.Y, r.Max.Y, pop.Bounds().Dy(), fullyVisible, w.Popup() == nil)
			widget.DismissPopup(button)
		}
		w.Close()
	}
}
