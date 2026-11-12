package sidebar

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

var (
	chrome = paint2d.RGB(.93, .93, .95)
	ink    = paint2d.RGB(.08, .08, .1)
	muted  = paint2d.RGB(.3, .3, .36)
	blue   = paint2d.RGB(.37, .37, .66)
	border = paint2d.RGB(.48, .48, .56)
)

func font(lk style.LookAndFeel, size float32) *style.Font {
	return players.ScaledFace(lk.Font(), style.Dip(lk, size))
}
func bevel(ctx *paint2d.Context, r paint2d.Rect, down bool, scale float32) {
	ctx.DrawRect(r, paint2d.Fill(chrome))
	light, dark := paint2d.White, border
	if down {
		light, dark = dark, light
	}
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), scale), paint2d.Fill(light))
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Min.Y, scale, r.Dy()), paint2d.Fill(light))
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Max.Y-scale, r.Dx(), scale), paint2d.Fill(dark))
	ctx.DrawRect(paint2d.XYWH(r.Max.X-scale, r.Min.Y, scale, r.Dy()), paint2d.Fill(dark))
}
func iconButton(p *Player, name, label string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, label, action)
	c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
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
		box := paint2d.XYWH(r.Min.X+(r.Dx()-side)/2, r.Min.Y+(r.Dy()-side)/2, side, side)
		if !skins.DrawSkinSprite(lk, ctx, box, "icon."+art, paint2d.Color{}) {
			players.DrawGlyph(ctx, box, glyph, ink, d)
		}
		if st.Disabled() {
			ctx.DrawRect(r, paint2d.Fill(chrome.WithAlpha(.65)))
		}
		return true
	}
	return c
}
func textButton(p *Player, text string, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(players.GlyphNone, text, action)
	c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
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
type fieldLook struct{ style.LookAndFeel }

func (l *fieldLook) Font() *style.Font { return font(l.LookAndFeel, 11) }
func (l *fieldLook) Palette() style.Palette {
	p := l.LookAndFeel.Palette()
	p.Text = ink
	p.TextMuted = muted
	p.Field = paint2d.White
	p.FieldBorder = border
	p.Selection = blue
	p.TextOnAccent = paint2d.White
	return p
}
func (l *fieldLook) DrawTextField(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState, text, placeholder string, caret, a, b int, blink bool, scroll float32, face *style.Font) {
	ctx.DrawRect(r, paint2d.Fill(paint2d.White))
	ctx.DrawRect(r.Inset(style.Dip(l, .5)), paint2d.StrokePaint(border, style.Dip(l, 1)))
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
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(f.Look(), 1)
		track := r
		if vertical {
			track.Min.X = r.Min.X + r.Dx()/2 - d
			track.Max.X = track.Min.X + 2*d
		} else {
			track.Min.Y = r.Min.Y + r.Dy()/2 - d
			track.Max.Y = track.Min.Y + 2*d
		}
		ctx.DrawRect(track, paint2d.Fill(border))
		thumb := paint2d.XYWH(r.Min.X+t*(r.Dx()-12*d), r.Min.Y+(r.Dy()-14*d)/2, 12*d, 14*d)
		if vertical {
			thumb = paint2d.XYWH(r.Min.X+(r.Dx()-18*d)/2, r.Min.Y+(1-t)*(r.Dy()-12*d), 18*d, 12*d)
		}
		bevel(ctx, thumb, st.Pressed(), d)
		if st.Disabled() {
			ctx.DrawRect(thumb, paint2d.Fill(chrome.WithAlpha(.7)))
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
func slot(lk style.LookAndFeel, name string, box paint2d.Rect) paint2d.Rect {
	r, _ := skins.SkinSlotRect(lk, LayoutMain, name, box)
	return r
}
func menuAt(c widget.Component) paint2d.Point {
	o := widget.DeviceOrigin(c)
	return paint2d.Pt(o.X, o.Y+c.Bounds().Dy())
}
