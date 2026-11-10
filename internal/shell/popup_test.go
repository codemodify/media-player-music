package shell

import (
	"fmt"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

// The toolkit scrolls a long choice list inside the compact panel when a
// native popup is unavailable or refused. Every row must become visible
// through normal keyboard input before it can be picked.
func TestSkinMenuOffersTheSameChoicesOnEveryPopupBackend(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		for _, mode := range []string{"surface", "layer", "headless", "refused"} {
			for _, initial := range []string{players.DefaultSkin, winamp.Skin} {
				for choice, id := range Appearances {
					t.Run(fmt.Sprintf("%s/%s@%gx/%s", initial, mode, scale, id), func(t *testing.T) {
						t.Setenv(platform.EnvPopups, mode)
						a, s := openAtScale(t, initial, scale)
						w := s.MainWindow()
						surface := w.Surface().(*platform.Offscreen)
						if mode != "headless" {
							surface.SimulatePopups(platform.FrameRect{W: 1600, H: 1200})
						}
						if mode == "refused" {
							surface.SimulatePopupsRefused(true)
						}
						w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyMenu})
						a.PumpOnce()
						pop, ok := w.Popup().(*widgets.PopupMenu)
						if !ok {
							t.Fatalf("menu key opened %T", w.Popup())
						}
						if len(pop.Items) != len(Appearances)+2 {
							t.Fatalf("context menu has %d rows, want skins, separator and Settings", len(pop.Items))
						}
						if pop.Items[len(pop.Items)-1].Text != "Settings…" {
							t.Fatal("context menu omitted Settings")
						}
						for i, skin := range Appearances {
							label := players.SkinLabel(skin)
							if pop.Items[i].Text != label {
								t.Fatalf("row %d: %q, want %q", i, pop.Items[i].Text, label)
							}
						}
						if mode != "surface" {
							if len(surface.Popups()) != 0 || pop.Bounds().Dy() > w.WindowRect().Dy() {
								t.Fatal("compact fallback clipped choices or opened a native surface")
							}
							if initial == winamp.Skin && pop.MaxOffset() <= 0 {
								t.Fatal("compact fallback did not provide a scrollable choice list")
							}
						} else if len(surface.Popups()) != 1 || surface.Popups()[0].Buffer() == nil {
							t.Fatal("native popup did not paint its surface")
						}
						menuChoice(t, a, w, pop, choice)
						model := s.Host.Model
						if !w.AccessibleAction(widget.ItemID(pop, choice), a11y.ActionDefault) {
							t.Fatalf("%s rejected accessible menu activation", id)
						}
						a.PumpOnce()
						a.PumpOnce()
						if s.Skin() != id || s.Host.Model != model || w.Closed() != (presentationFamily(initial) != presentationFamily(id)) || len(surface.Popups()) != 0 || w.Popup() != nil {
							t.Fatalf("%s did not switch safely around the same model", id)
						}
					})
				}
			}
		}
	}
}

func menuChoice(t *testing.T, a *ui.Application, w interface{ Inject(platform.Event) }, pop *widgets.PopupMenu, choice int) {
	t.Helper()
	// A mouse-opened PopupMenu's first Down highlights row0; subsequent
	// Down keys scroll the newly focused row into the constrained viewport.
	for i := 0; i <= choice; i++ {
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyDown})
		a.PumpOnce()
	}
	r := pop.ItemBounds(choice)
	b := pop.LocalBounds()
	if r.Min.Y < b.Min.Y || r.Max.Y > b.Max.Y || r.Min.X < b.Min.X || r.Max.X > b.Max.X {
		t.Fatalf("row %d is clipped after keyboard navigation: %v in %v", choice, r, b)
	}
}
