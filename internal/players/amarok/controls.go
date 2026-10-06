package amarok

import (
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

var (
	chrome = paintengine2d.RGB(.93, .93, .95)
	ink    = paintengine2d.RGB(.08, .08, .1)
	muted  = paintengine2d.RGB(.3, .3, .36)
	blue   = paintengine2d.RGB(.37, .37, .66)
	border = paintengine2d.RGB(.48, .48, .56)
)

func font(lk style.LookAndFeel, size float32) *style.Font {
	return players.ScaledFace(lk.Font(), style.Dip(lk, size))
}
func bevel(ctx *paintengine2d.Context, r paintengine2d.Rect, down bool, scale float32) {
	ctx.DrawRect(r, paintengine2d.Fill(chrome))
	light, dark := paintengine2d.White, border
	if down {
		light, dark = dark, light
	}
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), scale), paintengine2d.Fill(light))
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Min.Y, scale, r.Dy()), paintengine2d.Fill(light))
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Max.Y-scale, r.Dx(), scale), paintengine2d.Fill(dark))
	ctx.DrawRect(paintengine2d.XYWH(r.Max.X-scale, r.Min.Y, scale, r.Dy()), paintengine2d.Fill(dark))
}
func iconButton(p *Player, name, label string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, label, action)
	c.Art = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		lk := p.Main.Look()
		d := style.Dip(lk, 1)
		if st.Hovered() || st.Pressed() || st.Checked() {
			bevel(ctx, r, st.Pressed() || st.Checked(), d)
		}
		art := name
		if name == "play" && p.Transport.State == players.Playing {
			art = "pause"
		}
		if name == "repeat" {
			if p.Transport.Repeat == players.RepeatOne {
				art = "repeat.one"
			} else if p.Transport.Repeat == players.RepeatOff {
				art = "repeat.off"
			}
		}
		side := min(style.Dip(lk, 22), min(r.Dx(), r.Dy()))
		box := paintengine2d.XYWH(r.Min.X+(r.Dx()-side)/2, r.Min.Y+(r.Dy()-side)/2, side, side)
		if !style.DrawSkinSprite(lk, ctx, box, "icon."+art, paintengine2d.Color{}) {
			players.DrawGlyph(ctx, box, glyph, ink, d)
		}
		if st.Disabled() {
			ctx.DrawRect(r, paintengine2d.Fill(chrome.WithAlpha(.65)))
		}
		return true
	}
	return c
}
func textButton(p *Player, text string, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(players.GlyphNone, text, action)
	c.Art = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		bevel(ctx, r, st.Pressed() || st.Checked(), style.Dip(c.Look(), 1))
		col := ink
		if st.Disabled() {
			col = muted
		}
		players.DrawTextIn(ctx, font(c.Look(), 11), r.Inset(style.Dip(c.Look(), 3)), text, col, style.AlignCenter)
		return true
	}
	return c
}

// fieldLook uses the stock text editor with a KDE3 well and its scaled font.
// The explicit placeholder avoids the toolkit's documented font-size gap32.
type fieldLook struct{ style.LookAndFeel }

func (l *fieldLook) Font() *style.Font { return font(l.LookAndFeel, 11) }
func (l *fieldLook) Palette() style.Palette {
	p := l.LookAndFeel.Palette()
	p.Text = ink
	p.TextMuted = muted
	p.Field = paintengine2d.White
	p.FieldBorder = border
	p.Selection = blue
	p.TextOnAccent = paintengine2d.White
	return p
}
func (l *fieldLook) DrawTextField(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState, text, placeholder string, caret, a, b int, blink bool, scroll float32, face *style.Font) {
	ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.White))
	ctx.DrawRect(r.Inset(style.Dip(l, .5)), paintengine2d.StrokePaint(border, style.Dip(l, 1)))
	if text == "" && placeholder != "" && !st.Focused() {
		players.DrawTextIn(ctx, l.Font(), r.Inset(l.Metrics().FieldPad), placeholder, muted, style.AlignStart)
		placeholder = ""
	}
	l.LookAndFeel.DrawTextField(ctx, r, st|style.StateFrameless, text, placeholder, caret, a, b, blink, scroll, face)
}

type sourceSlider struct{ *players.Fader }

func slider(vertical bool, lo, hi, value float32, label string, on func(float32)) *sourceSlider {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, label, on)
	} else {
		f = players.NewBar(lo, hi, value, label, on)
	}
	f.Travel = 6
	f.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(f.Look(), 1)
		track := r
		if vertical {
			track.Min.X = r.Min.X + r.Dx()/2 - d
			track.Max.X = track.Min.X + 2*d
		} else {
			track.Min.Y = r.Min.Y + r.Dy()/2 - d
			track.Max.Y = track.Min.Y + 2*d
		}
		ctx.DrawRect(track, paintengine2d.Fill(border))
		thumb := paintengine2d.XYWH(r.Min.X+t*(r.Dx()-12*d), r.Min.Y+(r.Dy()-14*d)/2, 12*d, 14*d)
		if vertical {
			thumb = paintengine2d.XYWH(r.Min.X+(r.Dx()-18*d)/2, r.Min.Y+(1-t)*(r.Dy()-12*d), 18*d, 12*d)
		}
		bevel(ctx, thumb, st.Pressed(), d)
		if st.Disabled() {
			ctx.DrawRect(thumb, paintengine2d.Fill(chrome.WithAlpha(.7)))
		}
		return true
	}
	s := &sourceSlider{f}
	s.Init(s)
	return s
}

// Secondary clicks belong to the player's context menu and must not edit a
// level or begin a slider drag. Keyboard and accessibility remain stock.
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
func slot(lk style.LookAndFeel, name string, box paintengine2d.Rect) paintengine2d.Rect {
	r, _ := style.SkinSlotRect(lk, LayoutMain, name, box)
	return r
}
func menuAt(c widget.Component) paintengine2d.Point {
	o := widget.DeviceOrigin(c)
	return paintengine2d.Pt(o.X, o.Y+c.Bounds().Dy())
}
