package itunes

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// fieldLook keeps the stock editor's selection, caret and clear control while
// drawing the original Aqua search well at the inherited display scale.
type fieldLook struct{ style.LookAndFeel }

func (l *fieldLook) Font() *style.Font {
	return players.ScaledFace(l.LookAndFeel.Font(), style.Dip(l.LookAndFeel, 11))
}
func (l *fieldLook) DrawTextField(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState, text, placeholder string, caret, selA, selB int, blink bool, scrollX float32, face *style.Font) {
	if !skins.DrawSkinSprite(l.LookAndFeel, ctx, r, "search", paint2d.Color{}) {
		l.LookAndFeel.DrawTextField(ctx, r, st, text, placeholder, caret, selA, selB, blink, scrollX, face)
		return
	}
	l.LookAndFeel.DrawTextField(ctx, r, st|style.StateFrameless, text, placeholder, caret, selA, selB, blink, scrollX, face)
	if st.Focused() {
		ctx.DrawRoundRect(r.Inset(style.Dip(l, 1)), r.Dy()/2, r.Dy()/2, paint2d.StrokePaint(paint2d.RGBA(.24, .48, .83, .8), style.Dip(l, 1)))
	}
}

func skinButton(role, slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, name, action)
	artSlot := func() string {
		if slot == "play" && c.Glyph == players.GlyphPause {
			return "pause"
		}
		return slot
	}
	c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		if skins.DrawSkinSlot(c.Look(), ctx, skins.SlotArtRect(c, role, artSlot()), role, artSlot(), st) {
			return true
		}
		ink := paint2d.RGB(.15, .16, .17)
		if st.Checked() {
			ink = paint2d.RGB(.12, .32, .62)
		}
		if st.Disabled() {
			ink = paint2d.RGB(.5, .5, .5)
		}
		if st.Hovered() || st.Pressed() {
			ctx.DrawRoundRect(r, style.Dip(c.Look(), 3), style.Dip(c.Look(), 3), paint2d.Fill(paint2d.RGBA(.2, .35, .6, .2)))
		}
		players.DrawGlyph(ctx, r, c.Glyph, ink, style.Dip(c.Look(), 1.3))
		return true
	}
	c.Shaper = func(lk style.LookAndFeel, size paint2d.Point) *style.Silhouette {
		return skins.SkinSlotShape(lk, size, skins.SlotArtRect(c, role, artSlot()), role, artSlot())
	}
	return c
}

type sourceSlider struct{ *players.Fader }

func newSourceSlider(vertical bool, lo, hi, value float32, label string, on func(float32), prefix string) *sourceSlider {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, label, on)
	} else {
		f = players.NewBar(lo, hi, value, label, on)
	}
	s := &sourceSlider{f}
	s.Init(s)
	f.Travel = 6
	if vertical {
		f.Travel = 10
	}
	if prefix == "seek" {
		f.Travel = 4
	}
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, st style.ControlState) bool {
		lk := s.Look()
		skins.DrawSkinSprite(lk, ctx, r, prefix+".track", paint2d.Color{})
		d := style.Dip(lk, 1)
		thumb := paint2d.XYWH(r.Min.X+t*(r.Dx()-12*d), r.Min.Y, 12*d, r.Dy())
		if vertical {
			thumb = paint2d.XYWH(r.Min.X, r.Min.Y+(1-t)*(r.Dy()-20*d), r.Dx(), 20*d)
		}
		if prefix == "seek" {
			thumb = paint2d.XYWH(r.Min.X+t*(r.Dx()-8*d), r.Min.Y, 8*d, r.Dy())
		}
		name := prefix + ".thumb"
		if st.Pressed() {
			name += ".down"
		} else if st.Disabled() {
			name += ".disabled"
		}
		if !skins.DrawSkinSprite(lk, ctx, thumb, name, paint2d.Color{}) {
			skins.DrawSkinSprite(lk, ctx, thumb, prefix+".thumb", paint2d.Color{})
		}
		return true
	}
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
