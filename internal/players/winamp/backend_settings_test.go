package winamp

import (
	"strings"
	"testing"

	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/widget"
)

func TestLimitedBackendDisablesSkinnedDSPWithoutLosingSavedAdjustments(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1.75)
			p.Transport.SetPan(-.4)
			p.Equaliser.SetPreamp(3)
			p.Equaliser.Set(4, -6)
			m := p.Host.Model
			m.Live = true
			p.Host.AudioBackend = "mpg123"
			m.BackendLimited = true
			p.Refresh()
			a.PumpOnce()
			controls := []widget.Component{p.strip.balance, p.eqPane.on, p.eqPane.auto, p.eqPane.presets, p.eqPane.preamp}
			for _, band := range p.eqPane.bands {
				controls = append(controls, band)
			}
			for _, c := range controls {
				if c.Enabled() {
					t.Errorf("unsupported %T enabled", c)
				}
				w := p.Eq
				if c == p.strip.balance {
					w = p.Main
				}
				if w.AccessibleAction(c.ID(), a11y.ActionDefault) || w.AccessibleAction(c.ID(), a11y.ActionIncrement) {
					t.Fatal("unsupported DSP accepted user input")
				}
			}
			if !strings.Contains(p.strip.balance.Tooltip(), "unavailable") || !strings.Contains(p.eqPane.preamp.Tooltip(), "mpg123") {
				t.Fatal("skinned DSP limitation not explained")
			}
			m.BackendEqualizer, m.BackendBalance, m.BackendSpectrum = true, true, true
			p.Host.AudioBackend = "mpv"
			p.Refresh()
			a.PumpOnce()
			for _, c := range controls {
				if !c.Enabled() {
					t.Fatal("skinned supported DSP did not re-enable")
				}
			}
			if p.Transport.Pan != -.4 || p.Equaliser.Preamp != 3 || p.Equaliser.Gains[4] != -6 {
				t.Fatal("capability switch lost saved DSP")
			}
		})
	}
}
