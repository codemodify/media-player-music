package fancy

import (
	"math"
	"strings"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func cyan() paint2d.Color { return paint2d.RGB(.388, .843, .89) }
func ink() paint2d.Color  { return paint2d.RGB(.537, .588, .647) }
func face(lk style.LookAndFeel, size float32) *style.Font {
	return players.ScaledFace(lk.Font(), style.Dip(lk, size))
}
func paintLayout(ctx *paint2d.Context, lk style.LookAndFeel, box paint2d.Rect, role string, w *ui.Window) {
	if w.ShapeActive() {
		ctx.DrawRect(box, paint2d.Paint{Color: paint2d.White, Blend: paint2d.BlendDestOut})
	} else {
		ctx.DrawRect(box, paint2d.Fill(paint2d.RGB(.067, .082, .106)))
	}
	slot := "shape." + strings.TrimPrefix(role, "fancy.")
	if r, ok := skins.SkinSlotRect(lk, role, slot, box); ok {
		skins.DrawSkinSlot(lk, ctx, r, role, slot, 0)
	}
}
func textSlot(ctx *paint2d.Context, lk style.LookAndFeel, box paint2d.Rect, role, slot, text string, size float32, col paint2d.Color, align style.Align) {
	if r, ok := skins.SkinSlotRect(lk, role, slot, box); ok {
		players.DrawTextIn(ctx, face(lk, size), r, text, col, align)
	}
}
func button(role, slot, label, text string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, label, action)
	c.Shaper = func(lk style.LookAndFeel, size paint2d.Point) *style.Silhouette {
		r := paint2d.XYWH(0, 0, size.X, size.Y)
		if role == LayoutMain && slot == "play" {
			return &style.Silhouette{Path: paint2d.EllipsePath(r.Center(), r.Dx()/2, r.Dy()/2)}
		}
		radius := min(style.Dip(lk, 3), min(size.X, size.Y)/2)
		return style.SilhouetteOfRects(style.SilhouetteRect{Rect: r, Radius: [4]float32{radius, radius, radius, radius}})
	}
	c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		lk := c.Look()
		skins.DrawSkinSlot(lk, ctx, r, role, slot, 0)
		d := style.Dip(lk, 1)
		col := ink()
		if st.Hovered() || st.Pressed() || st.Checked() {
			col = cyan()
			a := float32(.15)
			if st.Pressed() {
				a = .3
			}
			if role == LayoutMain && slot == "play" {
				ctx.DrawCircle(r.Center(), min(r.Dx(), r.Dy())/2-d, paint2d.Fill(cyan().WithAlpha(a)))
			} else {
				ctx.DrawRoundRect(r.Inset(d), 3*d, 3*d, paint2d.Fill(cyan().WithAlpha(a)))
			}
		}
		if st.Disabled() {
			col = ink().WithAlpha(.35)
		}
		if text != "" {
			players.DrawTextIn(ctx, face(lk, 8), r, text, col, style.AlignCenter)
			return true
		}
		if slot == "close" || slot == "minimize" || slot == "compact" || slot == "restore" {
			q := r.Inset(4 * d)
			path := paint2d.NewPath()
			switch slot {
			case "close":
				path.MoveTo(q.Min.X, q.Min.Y)
				path.LineTo(q.Max.X, q.Max.Y)
				path.MoveTo(q.Min.X, q.Max.Y)
				path.LineTo(q.Max.X, q.Min.Y)
			case "minimize":
				path.MoveTo(q.Min.X, q.Max.Y)
				path.LineTo(q.Max.X, q.Max.Y)
			case "compact":
				path.AddRect(paint2d.XYWH(q.Min.X, q.Center().Y, q.Dx(), max(d, q.Dy()/3)))
			case "restore":
				path.AddRect(q)
			}
			ctx.DrawPath(path, paint2d.StrokePaint(col, d))
			return true
		}
		padding := min(style.Dip(lk, 4), r.Dx()/5)
		if role == LayoutMain && slot == "play" {
			padding = style.Dip(lk, 17)
		}
		players.DrawGlyph(ctx, r.Inset(padding), c.Glyph, col, 1.4*d)
		return true
	}
	return c
}

type fancySlider struct {
	*players.Fader
	dial bool
}

func slider(vertical, dial bool, lo, hi, value float32, label string, on func(float32)) *fancySlider {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, label, on)
	} else {
		f = players.NewBar(lo, hi, value, label, on)
	}
	s := &fancySlider{Fader: f, dial: dial}
	s.Init(s)
	f.Travel = 3
	if dial {
		f.Travel = .001
	}
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(s.Look(), 1)
		col := cyan()
		if st.Disabled() {
			col = ink().WithAlpha(.4)
		}
		if dial {
			c := r.Center()
			radius := min(r.Dx(), r.Dy()) * .36
			angle := float64(135+270*t) * math.Pi / 180
			at := c.Add(paint2d.Pt(float32(math.Cos(angle))*radius*.75, float32(math.Sin(angle))*radius*.75))
			ctx.DrawLine(c, at, paint2d.StrokePaint(col, d))
			ctx.DrawCircle(at, 1.7*d, paint2d.Fill(col))
			return true
		}
		track := paint2d.XYWH(r.Min.X+3*d, r.Center().Y-d, r.Dx()-6*d, 2*d)
		thumb := paint2d.XYWH(r.Min.X+t*(r.Dx()-6*d), r.Min.Y, 6*d, r.Dy())
		if vertical {
			track = paint2d.XYWH(r.Center().X-d, r.Min.Y+3*d, 2*d, r.Dy()-6*d)
			thumb = paint2d.XYWH(r.Min.X, r.Min.Y+(1-t)*(r.Dy()-7*d), r.Dx(), 7*d)
		}
		ctx.DrawRoundRect(track, d, d, paint2d.Fill(paint2d.RGB(.035, .045, .06)))
		if vertical {
			level := track
			level.Min.Y = thumb.Center().Y
			ctx.DrawRoundRect(level, d, d, paint2d.Fill(col))
			col = paint2d.RGB(.65, .7, .75)
			if st.Disabled() {
				col = col.WithAlpha(.4)
			}
		}
		ctx.DrawRoundRect(thumb.Inset(.5*d), d, d, paint2d.Fill(col))
		return true
	}
	return s
}
func (s *fancySlider) ArtShape(_ style.LookAndFeel, size paint2d.Point) *style.Silhouette {
	if !s.dial {
		return nil
	}
	return &style.Silhouette{Path: paint2d.EllipsePath(paint2d.Pt(size.X/2, size.Y/2), size.X/2, size.Y/2)}
}
func (s *fancySlider) pointer(e widget.MouseEvent) widget.MouseEvent {
	if !s.dial {
		return e
	}
	r := s.LocalBounds()
	v := e.Pos.Sub(r.Center())
	angle := math.Atan2(float64(v.Y), float64(v.X)) * 180 / math.Pi
	if angle < 135 {
		angle += 360
	}
	if angle > 405 {
		if angle > 450 {
			angle = 135
		} else {
			angle = 405
		}
	}
	t := float32((angle - 135) / 270)
	travel := style.Dip(s.Look(), s.Travel)
	e.Pos.X = travel + t*(r.Dx()-2*travel)
	e.Pos.Y = r.Center().Y
	return e
}
func (s *fancySlider) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MousePress(s.pointer(e))
}
func (s *fancySlider) MouseMove(e widget.MouseEvent) bool { return s.Fader.MouseMove(s.pointer(e)) }
func (s *fancySlider) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MouseRelease(s.pointer(e))
}
