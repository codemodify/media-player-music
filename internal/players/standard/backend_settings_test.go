package standard

import (
	"fmt"
	"strings"
	"testing"

	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
)

func TestSettingsButtonWorksWithoutAMenuAndFitsMinimumLayout(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			hits := 0
			p.Host.ShowSettings = func() { hits++ }
			p.Main.SetSize(MinW, MinH)
			a.PumpOnce()
			settings := p.body.main.settings
			b, parent := settings.Bounds(), settings.Parent().LocalBounds()
			if b.Empty() || b.Min.X < 0 || b.Min.Y < 0 || b.Max.X > parent.Max.X || b.Max.Y > parent.Max.Y || b.Overlaps(p.body.main.balance.Bounds()) {
				t.Fatal("Settings button clipped or overlaps balance at minimum size")
			}
			if n := p.Main.AccessibleTree().Find(settings.ID()); n == nil || n.Name != "Settings" {
				t.Fatal("direct Settings action missing from accessibility tree")
			}
			p.Main.RequestFocus(settings)
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
			a.PumpOnce()
			if hits != 1 || !p.Main.AccessibleAction(settings.ID(), a11y.ActionDefault) || hits != 2 {
				t.Fatal("Settings button did not invoke the host through keyboard and accessibility")
			}
		})
	}
}

func TestLimitedBackendDisablesDSPWithoutLosingSavedAdjustments(t *testing.T) {
	a, p := open(t, 1.75)
	p.Transport.SetPan(.6)
	p.Equaliser.SetPreamp(5)
	p.Equaliser.Set(0, 7)
	m := p.Host.Model
	m.Live = true
	p.Host.AudioBackend = "VLC"
	m.BackendLimited = true
	p.Refresh()
	a.PumpOnce()
	eq := p.body.eq
	controls := []widget.Component{p.body.main.balance, eq.on, eq.auto, eq.presets, eq.preamp}
	for _, band := range eq.bands {
		controls = append(controls, band)
	}
	for _, c := range controls {
		if c.Enabled() {
			t.Errorf("unsupported control remained enabled: %T", c)
		}
		if p.Main.AccessibleAction(c.ID(), a11y.ActionDefault) || p.Main.AccessibleAction(c.ID(), a11y.ActionIncrement) {
			t.Fatal("disabled DSP control accepted user input")
		}
	}
	if !strings.Contains(p.body.main.balance.Tooltip(), "unavailable") || !strings.Contains(eq.preamp.Tooltip(), "VLC") || !strings.Contains(p.Host.StatusText(), "visualizer") {
		t.Fatal("feature limitations were not explained")
	}
	if p.Transport.Pan != .6 || p.Equaliser.Preamp != 5 || p.Equaliser.Gains[0] != 7 {
		t.Fatal("switching capabilities erased saved adjustments")
	}
	m.BackendEqualizer, m.BackendBalance, m.BackendSpectrum = true, true, true
	p.Host.AudioBackend = "mpv"
	p.Refresh()
	a.PumpOnce()
	for _, c := range controls {
		if !c.Enabled() {
			t.Fatal("supported DSP control did not re-enable")
		}
	}
	if p.Transport.Pan != .6 || p.Equaliser.Preamp != 5 || p.Equaliser.Gains[0] != 7 {
		t.Fatal("returning to mpv erased saved adjustments")
	}
}
