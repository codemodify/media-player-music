package fandom

import (
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// Keep toolkit slider input, keys and accessibility, while reserving other
// pointer buttons for the enclosing player's context menu.
type silverSlider struct{ *players.Fader }

func newSilverSlider(vertical bool, lo, hi, value float32, label string, on func(float32)) *silverSlider {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, label, on)
	} else {
		f = players.NewBar(lo, hi, value, label, on)
	}
	s := &silverSlider{f}
	s.Init(s)
	f.Travel = 4
	f.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(s.Look(), 1)
		track := paintengine2d.XYWH(r.Min.X+4*d, (r.Min.Y+r.Max.Y)/2-d, r.Dx()-8*d, 2*d)
		thumb := paintengine2d.XYWH(r.Min.X+t*(r.Dx()-8*d), r.Min.Y+2*d, 8*d, r.Dy()-4*d)
		if vertical {
			track = paintengine2d.XYWH((r.Min.X+r.Max.X)/2-d, r.Min.Y+4*d, 2*d, r.Dy()-8*d)
			thumb = paintengine2d.XYWH(r.Min.X+2*d, r.Min.Y+(1-t)*(r.Dy()-8*d), r.Dx()-4*d, 8*d)
		}
		ctx.DrawRoundRect(track, d, d, paintengine2d.Fill(paintengine2d.RGB(.16, .18, .19)))
		col := paintengine2d.RGB(.75, .77, .79)
		if st.Pressed() {
			col = paintengine2d.RGB(.55, .62, .60)
		}
		if st.Disabled() {
			col = paintengine2d.RGB(.38, .40, .42)
		}
		ctx.DrawRoundRect(thumb, 2*d, 2*d, paintengine2d.Fill(col))
		ctx.DrawRoundRect(thumb, 2*d, 2*d, paintengine2d.StrokePaint(paintengine2d.RGB(.16, .18, .20), d))
		if st.Focused() {
			s.Look().DrawFocusRing(ctx, r)
		}
		return true
	}
	return s
}
func (s *silverSlider) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MousePress(e)
}
func (s *silverSlider) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MouseRelease(e)
}
