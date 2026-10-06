package realplayer

import (
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func bindButton(parent widget.Component, slots *widget.Slots, role, slot, label string, glyph players.Glyph, run func()) *players.GlyphButton {
	b := players.NewGlyphButton(glyph, label, run)
	b.Art = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		if style.DrawSkinSlot(b.Look(), ctx, widget.SlotArtRect(b, role, slot), role, slot, st) {
			return true
		}
		ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.RGB(.75, .75, .75)))
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
func textButton(parent widget.Component, slots *widget.Slots, role, slot, label, text string, run func()) *players.GlyphButton {
	b := bindButton(parent, slots, role, slot, label, players.GlyphNone, run)
	b.Art = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.RGB(.75, .75, .75)))
		if st.Pressed() || st.Checked() {
			drawBevel(ctx, r, false)
		} else if st.Hovered() || slot != "file" && slot != "view" && slot != "play.menu" && slot != "tools" && slot != "help" {
			drawBevel(ctx, r, true)
		}
		ink := paintengine2d.RGB(.06, .06, .06)
		if st.Disabled() {
			ink = paintengine2d.RGB(.43, .43, .43)
		}
		players.DrawTextIn(ctx, players.ScaledFace(b.Look().Font(), style.Dip(b.Look(), 11)), r.Inset(style.Dip(b.Look(), 2)), text, ink, style.AlignCenter)
		return true
	}
	return b
}
func drawBevel(ctx *paintengine2d.Context, r paintengine2d.Rect, raised bool) {
	light, dark := paintengine2d.White, paintengine2d.RGB(.4, .4, .4)
	if !raised {
		light, dark = dark, light
	}
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), 1), paintengine2d.Fill(light))
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Min.Y, 1, r.Dy()), paintengine2d.Fill(light))
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Max.Y-1, r.Dx(), 1), paintengine2d.Fill(dark))
	ctx.DrawRect(paintengine2d.XYWH(r.Max.X-1, r.Min.Y, 1, r.Dy()), paintengine2d.Fill(dark))
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
	f.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(f.Look(), 1)
		ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.RGB(.75, .75, .75)))
		groove := paintengine2d.XYWH(r.Min.X+3*d, r.Min.Y+r.Dy()/2-2*d, r.Dx()-6*d, 4*d)
		thumb := paintengine2d.XYWH(r.Min.X+t*(r.Dx()-9*d), r.Min.Y, 9*d, r.Dy())
		if vertical {
			groove = paintengine2d.XYWH(r.Min.X+r.Dx()/2-2*d, r.Min.Y+3*d, 4*d, r.Dy()-6*d)
			thumb = paintengine2d.XYWH(r.Min.X, r.Min.Y+(1-t)*(r.Dy()-9*d), r.Dx(), 9*d)
		}
		ctx.DrawRect(groove, paintengine2d.Fill(paintengine2d.RGB(.47, .47, .47)))
		drawBevel(ctx, groove, false)
		ctx.DrawRect(thumb, paintengine2d.Fill(paintengine2d.RGB(.8, .8, .8)))
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
func (g *resizeGrip) Paint(ctx *paintengine2d.Context) {
	r := g.LocalBounds()
	d := style.Dip(g.Look(), 1)
	for i := float32(3); i < 12; i += 3 {
		var p paintengine2d.Path
		p.MoveTo(r.Max.X-i*d, r.Max.Y-d)
		p.LineTo(r.Max.X-d, r.Max.Y-i*d)
		ctx.DrawPath(&p, paintengine2d.StrokePaint(paintengine2d.RGB(.4, .4, .4), d))
	}
}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.restore = nil
	return g.p.Main.StartResize(platform.EdgeBottom | platform.EdgeRight)
}
func ink(lk style.LookAndFeel, role string, fallback paintengine2d.Color) paintengine2d.Color {
	if sk, ok := style.LoadSkin(style.LookAppearance(lk).Name); ok {
		if t := sk.Text["player."+role]; t != nil {
			return t.Color
		}
	}
	return fallback
}
