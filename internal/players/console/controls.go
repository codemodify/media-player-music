package console

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func bindButton(parent widget.Component, slots *skins.Slots, role, slot, label string, glyph players.Glyph, run func()) *players.GlyphButton {
	b := players.NewGlyphButton(glyph, label, run)
	b.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		if skins.DrawSkinSlot(b.Look(), ctx, skins.SlotArtRect(b, role, slot), role, slot, st) {
			return true
		}
		ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.75, .75, .75)))
		if st.Pressed() || st.Checked() {
			drawBevel(ctx, r, false)
		} else if st.Hovered() {
			drawBevel(ctx, r, true)
		}
		players.DrawGlyph(ctx, r, glyph, players.GlyphInk(b.Look(), st), style.Dip(b.Look(), 1))
		return true
	}
	slots.Bind(slot, b)
	parent.Add(b)
	return b
}
func textButton(parent widget.Component, slots *skins.Slots, role, slot, label, text string, run func()) *players.GlyphButton {
	b := bindButton(parent, slots, role, slot, label, players.GlyphNone, run)
	b.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.75, .75, .75)))
		if st.Pressed() || st.Checked() {
			drawBevel(ctx, r, false)
		} else if st.Hovered() || slot != "file" && slot != "view" && slot != "play.menu" && slot != "tools" && slot != "help" {
			drawBevel(ctx, r, true)
		}
		ink := paint2d.RGB(.06, .06, .06)
		if st.Disabled() {
			ink = paint2d.RGB(.43, .43, .43)
		}
		players.DrawTextIn(ctx, players.ScaledFace(b.Look().Font(), style.Dip(b.Look(), 11)), r.Inset(style.Dip(b.Look(), 2)), text, ink, style.AlignCenter)
		return true
	}
	return b
}
func drawBevel(ctx *paint2d.Context, r paint2d.Rect, raised bool) {
	light, dark := paint2d.White, paint2d.RGB(.4, .4, .4)
	if !raised {
		light, dark = dark, light
	}
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), 1), paint2d.Fill(light))
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Min.Y, 1, r.Dy()), paint2d.Fill(light))
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Max.Y-1, r.Dx(), 1), paint2d.Fill(dark))
	ctx.DrawRect(paint2d.XYWH(r.Max.X-1, r.Min.Y, 1, r.Dy()), paint2d.Fill(dark))
}

type classicFader struct{ *players.Fader }

func (f *classicFader) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return f.Fader.MousePress(e)
}
func (f *classicFader) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return f.Fader.MouseRelease(e)
}
func classicSlider(vertical bool, lo, hi, value float32, name string, run func(float32)) *classicFader {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, name, run)
	} else {
		f = players.NewBar(lo, hi, value, name, run)
	}
	f.Travel = 4
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(f.Look(), 1)
		ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.75, .75, .75)))
		groove := paint2d.XYWH(r.Min.X+3*d, r.Min.Y+r.Dy()/2-2*d, r.Dx()-6*d, 4*d)
		thumb := paint2d.XYWH(r.Min.X+t*(r.Dx()-9*d), r.Min.Y, 9*d, r.Dy())
		if vertical {
			groove = paint2d.XYWH(r.Min.X+r.Dx()/2-2*d, r.Min.Y+3*d, 4*d, r.Dy()-6*d)
			thumb = paint2d.XYWH(r.Min.X, r.Min.Y+(1-t)*(r.Dy()-9*d), r.Dx(), 9*d)
		}
		ctx.DrawRect(groove, paint2d.Fill(paint2d.RGB(.47, .47, .47)))
		drawBevel(ctx, groove, false)
		ctx.DrawRect(thumb, paint2d.Fill(paint2d.RGB(.8, .8, .8)))
		drawBevel(ctx, thumb, true)
		if st.Focused() {
			f.Look().DrawFocusRing(ctx, r)
		}
		return true
	}
	wrapped := &classicFader{Fader: f}
	wrapped.Init(wrapped)
	return wrapped
}

type resizeGrip struct {
	widget.Base
	p *Player
}

func newResizeGrip(p *Player) *resizeGrip {
	g := &resizeGrip{p: p}
	g.Init(g)
	g.SetAccessibleName("Resize Music Player")
	return g
}
func (g *resizeGrip) Paint(ctx *paint2d.Context) {
	r := g.LocalBounds()
	d := style.Dip(g.Look(), 1)
	for i := float32(3); i < 12; i += 3 {
		var p paint2d.Path
		p.MoveTo(r.Max.X-i*d, r.Max.Y-d)
		p.LineTo(r.Max.X-d, r.Max.Y-i*d)
		ctx.DrawPath(&p, paint2d.StrokePaint(paint2d.RGB(.4, .4, .4), d))
	}
}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.restore = nil
	return g.p.Main.StartResize(platform.EdgeBottom | platform.EdgeRight)
}
func ink(lk style.LookAndFeel, role string, fallback paint2d.Color) paint2d.Color {
	if sk, ok := skins.LoadSkin(style.LookAppearance(lk).Name); ok {
		if t := sk.Text["player."+role]; t != nil {
			return t.Color
		}
	}
	return fallback
}
