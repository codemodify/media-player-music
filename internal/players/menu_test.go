package players

import (
	"fmt"
	"testing"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// An advertised menu action must execute the player's choice through the
// window, including when the toolkit paints that menu on another surface.
func TestSkinMenuAccessibleActionsWorkOnLayerAndNativeSurface(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "layer"
		if native {
			name = "native surface"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, "dark")
			a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
			w, err := a.NewWindow(platform.WindowOptions{Width: 400, Height: 300, Headless: true})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(w.Close)
			from := widgets.NewButton("Skin", nil)
			w.SetContent(from)
			a.PumpOnce()
			if native {
				w.Surface().(*platform.Offscreen).SimulatePopups(platform.FrameRect{W: 1000, H: 800})
			}
			h := NewHost(a)
			h.Packs = SkinnedIDs
			chosen, calls := "", 0
			h.ChooseSkin = func(id string) { chosen = id; calls++ }
			h.ContextMenu(from, paint2d.Pt(8, 8))
			a.PumpOnce()
			menu, ok := w.Popup().(*widgets.PopupMenu)
			if !ok {
				t.Fatalf("skin action opened %T", w.Popup())
			}
			items := menu.AccessibleItems()
			choices := AppearanceIDs()
			last := len(choices) - 1
			if len(items) != len(choices) || !items[last].Actions.Has(a11y.ActionDefault) {
				t.Fatal("skin menu did not advertise its complete choices")
			}
			opened := w.Surface().(*platform.Offscreen).PopupsOpened()
			if native != (opened > 0) {
				t.Fatalf("native=%t but toolkit opened %d popup surfaces", native, opened)
			}
			if !w.AccessibleAction(widget.ItemID(menu, last), a11y.ActionDefault) || chosen != choices[last] || calls != 1 {
				t.Fatalf("accessible choice: chosen=%q calls=%d", chosen, calls)
			}
			if w.Popup() != nil {
				t.Fatal("activated skin menu stayed open")
			}
		})
	}
}

func TestCompactSkinMenuScrollsAndPicksWithMouseAndKeyboard(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		for _, native := range []bool{false, true} {
			for _, input := range []string{"keyboard", "wheel and click"} {
				t.Run(fmt.Sprintf("%gx/native%t/%s", scale, native, input), func(t *testing.T) {
					t.Setenv("XDG_CONFIG_HOME", t.TempDir())
					t.Setenv(style.ThemeEnv, "dark")
					mode := "layer"
					if native {
						mode = "surface"
					}
					t.Setenv(platform.EnvPopups, mode)
					a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
					w, err := a.NewWindow(platform.WindowOptions{Width: 275, Height: 116, Headless: true})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(w.Close)
					from := widgets.NewButton("Skins", nil)
					w.SetContent(from)
					a.PumpOnce()
					if native {
						w.Surface().(*platform.Offscreen).SimulatePopups(platform.FrameRect{W: 1600, H: 1200})
					}
					h := NewHost(a)
					h.Packs = SkinnedIDs
					picked, calls := "", 0
					h.ChooseSkin = func(id string) { picked = id; calls++ }
					h.ContextMenu(from, paint2d.Pt(8, 8))
					a.PumpOnce()
					pop, ok := w.Popup().(*widgets.PopupMenu)
					if !ok {
						t.Fatalf("opened %T", w.Popup())
					}
					if !native && pop.MaxOffset() <= 0 {
						t.Fatal("compact menu did not scroll")
					}
					choices := AppearanceIDs()
					last := len(choices) - 1
					if input == "keyboard" {
						for i := 0; i <= last; i++ {
							w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyDown})
							a.PumpOnce()
							r := pop.ItemBounds(i)
							if r.Min.Y < 0 || r.Max.Y > pop.LocalBounds().Dy() {
								t.Fatalf("row %d clipped after Down: %v in %v", i, r, pop.LocalBounds())
							}
						}
						w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
					} else {
						pos := widget.DeviceOrigin(pop).Add(paint2d.Pt(pop.Bounds().Dx()/2, pop.Bounds().Dy()/2))
						for i := 0; i < 100 && pop.OffsetY < pop.MaxOffset(); i++ {
							w.Inject(platform.Event{Kind: platform.EventScroll, Pos: pos, Scroll: paint2d.Pt(0, 1)})
							a.PumpOnce()
						}
						row := pop.ItemBounds(last)
						if row.Min.Y < 0 || row.Max.Y > pop.LocalBounds().Dy() {
							t.Fatalf("last row clipped after wheel: %v", row)
						}
						pos = widget.DeviceOrigin(pop).Add(paint2d.Pt((row.Min.X+row.Max.X)/2, (row.Min.Y+row.Max.Y)/2))
						w.Inject(platform.Event{Kind: platform.EventMouseDown, Pos: pos, Button: platform.ButtonLeft})
						w.Inject(platform.Event{Kind: platform.EventMouseUp, Pos: pos, Button: platform.ButtonLeft})
					}
					a.PumpOnce()
					if picked != choices[last] || calls != 1 || w.Popup() != nil {
						t.Fatalf("picked %q calls %d popup %T", picked, calls, w.Popup())
					}
				})
			}
		}
	}
}
