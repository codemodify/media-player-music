package vlc

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

// Secondary presses belong to the player context menu rather than changing
// playback or an equalizer level. Everything else remains a stock slider.
type sourceSlider struct{ *players.Fader }

func newSlider(vertical bool, lo, hi, value float32, label string, on func(float32)) *sourceSlider {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, label, on)
	} else {
		f = players.NewBar(lo, hi, value, label, on)
	}
	s := &sourceSlider{Fader: f}
	s.Init(s)
	return s
}

func (s *sourceSlider) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MousePress(e)
}

func (s *sourceSlider) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MouseRelease(e)
}
