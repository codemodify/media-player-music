package itunes

import (
	"fmt"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// Replacing the editor's painter must keep the toolkit's real editing and
// clear affordances, including their routed accessibility actions and focus.
func TestSourceSearchWellKeepsEditingAndClearActionsAcrossLookRefresh(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			libraryFixture(p)
			a.PumpOnce()
			field := p.body.search
			calls := 0
			input := field.OnInput
			field.OnInput = func(text string) { calls++; input(text) }
			p.Main.RequestFocus(field)
			if !p.Main.AccessibleSetText(field.ID(), "Copper") {
				t.Fatal("source search rejected editable-text action")
			}
			field.SetSelection(1, 4)
			if calls != 1 || p.body.list.Count != 2 {
				t.Fatal("editable-text did not filter exactly once")
			}
			look := field.Look()
			for i := 0; i < 5; i++ {
				p.Refresh()
				a.PumpOnce()
			}
			if field.Look() != look {
				t.Fatal("idle refresh replaced source look and editor state")
			}
			a.SetLook(style.WithDensity(a.Look(), style.DensityCompact))
			a.PumpOnce()
			start, end := field.Selection()
			if field.Look() == look || field.Text != "Copper" || start != 1 || end != 4 || field.Caret() != 4 || p.Main.Focus() != field || calls != 1 {
				t.Fatal("live look refresh lost source editor text, selection or focus")
			}
			clearID := widget.ItemID(field, 0)
			clear := p.Main.AccessibleTree().Find(clearID)
			if clear == nil || clear.Name != "Clear Search music library" || !clear.Actions.Has(a11y.ActionDefault) || !widget.DeviceBounds(field).Contains(clear.Bounds.Center()) {
				t.Fatal("styled search lost its named clear action or hit bounds")
			}
			click(p.Main, platform.ButtonLeft, clear.Bounds.Center())
			a.PumpOnce()
			if calls != 2 || field.Text != "" || p.body.list.Count != 5 || p.Main.Focus() != field || field.Caret() != 0 || p.Main.AccessibleTree().Find(clearID) != nil {
				t.Fatal("pointer clear did not restore library once while retaining editing focus")
			}
			if !p.Main.AccessibleSetText(field.ID(), "Copper") {
				t.Fatal("second source editable-text action failed")
			}
			p.Main.RequestFocus(p.body.play)
			if !p.Main.AccessibleAction(clearID, a11y.ActionDefault) || calls != 4 || field.Text != "" || p.body.list.Count != 5 || p.Main.Focus() != p.body.play {
				t.Fatal("accessible clear did not restore library once without stealing focus")
			}
			if p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
				t.Fatal("source search/clear changed playback")
			}
		})
	}
}
