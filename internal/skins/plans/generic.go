package plans

import (
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// palette is the handful of colours a redrawn skin is painted from.
type palette struct {
	body, bodyTop, bodyBottom, edge, rim paint2d.Color // the chrome
	well, wellDark, wellLight            paint2d.Color // recesses: displays, lists
	key, keyTop, keyBottom, keyEdge      paint2d.Color // keys at rest
	ink, inkHover, accent                paint2d.Color // the marks on keys, a lit key's mark
	lcd, lcdDim                          paint2d.Color // a display's lit and unlit ink
	radius, keyRadius                    float32
	gloss                                float32
	round                                bool // keys are discs where they are about square
}

// genericArt paints a sprite by what its name says it is. A skin's own
// painter handles what it draws differently and hands the rest here.
func genericArt(p *palette, g *geometry, u use) drawFunc {
	name := u.name
	switch {
	case strings.HasSuffix(name, ".map"):
		return mapRamp
	case strings.HasSuffix(name, ".shape"), strings.HasPrefix(name, "shape."):
		return func(ctx *paint2d.Context, w, h float32) {
			round(ctx, paint2d.XYWH(0, 0, w, h), p.radius, paint2d.Fill(p.body))
		}
	case strings.HasPrefix(name, "face.") || strings.HasSuffix(name, ".face") || name == "eq.panel" || strings.HasSuffix(name, ".background"):
		return func(ctx *paint2d.Context, w, h float32) { genericFace(ctx, p, u, w, h) }
	case strings.HasPrefix(name, "font.") || strings.HasPrefix(name, "info.font."):
		return pixGlyphArt(name, p.lcd)
	case strings.HasPrefix(name, "led."):
		return func(ctx *paint2d.Context, w, h float32) { ledArt(ctx, name, w, h, p.lcd) }
	case strings.Contains(name, "thumb"):
		return func(ctx *paint2d.Context, w, h float32) {
			top, bottom := p.keyTop, p.keyBottom
			if u.down() {
				top, bottom = p.keyBottom, p.keyTop
			}
			if w > h*1.6 || h > w*1.6 {
				pillow(ctx, paint2d.XYWH(0, 0, w, h), min(w, h)/2, top, bottom, p.keyEdge, p.gloss)
				return
			}
			orb(ctx, paint2d.Pt(w/2, h/2), min(w, h)/2, top, bottom, p.keyEdge, p.gloss)
		}
	case isFrameName(name):
		return func(ctx *paint2d.Context, w, h float32) {
			i, n := u.frame(g)
			t := float32(0)
			if n > 1 {
				t = float32(i) / float32(n-1)
			}
			groove(ctx, p, w, h, t, strings.Contains(name, "eq") && h > w)
		}
	case strings.HasSuffix(name, ".track") || strings.HasPrefix(name, "seek.") || strings.HasPrefix(name, "volume.") || strings.HasPrefix(name, "balance."):
		return func(ctx *paint2d.Context, w, h float32) { groove(ctx, p, w, h, 0, h > w*2) }
	case strings.HasPrefix(name, "key.") || strings.HasPrefix(name, "mini.") || strings.HasPrefix(name, "btn.") || strings.HasPrefix(name, "button."):
		return func(ctx *paint2d.Context, w, h float32) { genericKey(ctx, p, u, w, h) }
	case name == "app.icon" || strings.HasSuffix(name, "logo"):
		return func(ctx *paint2d.Context, w, h float32) { emblem(ctx, paint2d.XYWH(0, 0, w, h), p.accent, p.ink) }
	}
	return func(ctx *paint2d.Context, w, h float32) {
		r := paint2d.XYWH(0, 0, w, h)
		round(ctx, r, min(p.radius, 3), vpaint(r, stop(0, p.bodyTop), stop(1, p.bodyBottom)))
		ring(ctx, r, min(p.radius, 3), 1, p.edge)
	}
}

func isFrameName(name string) bool {
	i := strings.LastIndexByte(name, '.')
	if i < 0 || i == len(name)-1 || strings.Trim(name[i+1:], "0123456789") != "" {
		return false
	}
	return strings.HasPrefix(name, "seek.") || strings.HasPrefix(name, "volume.") || strings.HasPrefix(name, "balance.") || strings.Contains(name, ".frame.") || strings.HasPrefix(name, "eq.")
}

// mapRamp is a curved slider's value map: red is the value along the strip,
// left to right, opaque everywhere the strip is.
func mapRamp(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	ctx.DrawRect(r, hpaint(r, stop(0, paint2d.RGBA(0, 0, 0, 1)), stop(1, paint2d.RGBA(1, 1, 1, 1))))
}

// groove is a slider's channel, lit from 0 to t.
func groove(ctx *paint2d.Context, p *palette, w, h, t float32, vertical bool) {
	if vertical {
		c := paint2d.XYWH(w/2-2, 1, 4, h-2)
		well(ctx, c, 2, p.well, p.wellDark, p.wellLight)
		if t > 0 {
			f := paint2d.XYWH(c.Min.X+1, c.Max.Y-1-(c.Dy()-2)*t, 2, (c.Dy()-2)*t)
			fill(ctx, f, p.accent)
		}
		return
	}
	th := min(h, max(4, h*.45))
	c := paint2d.XYWH(1, (h-th)/2, w-2, th)
	well(ctx, c, th/2, p.well, p.wellDark, p.wellLight)
	if t > 0 {
		f := paint2d.XYWH(c.Min.X+1, c.Min.Y+1, (c.Dx()-2)*t, c.Dy()-2)
		round(ctx, f, (th-2)/2, hpaint(f, stop(0, sink(p.accent, .3)), stop(1, p.accent)))
	}
}

// genericKey is a key with its mark.
func genericKey(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	top, bottom, ink := p.keyTop, p.keyBottom, p.ink
	switch {
	case u.down():
		top, bottom = sink(p.keyBottom, .1), p.keyTop
		ink = p.accent
	case u.hover():
		top, bottom = lift(p.keyTop, .25), lift(p.keyBottom, .2)
		ink = p.inkHover
	}
	if u.on() {
		ink = p.accent
	}
	if p.round && w/h < 1.35 && h/w < 1.35 {
		orb(ctx, paint2d.Pt(w/2, h/2), min(w, h)/2-.5, top, bottom, p.keyEdge, p.gloss)
	} else {
		pillow(ctx, r, p.keyRadius, top, bottom, p.keyEdge, p.gloss)
	}
	g := glyphFor(u.name)
	if g == "" {
		return
	}
	d := float32(0)
	if u.down() {
		d = .6
	}
	s := min(w, h) * .46
	symbol(ctx, g, paint2d.XYWH(d, d, w, h), s, ink)
}

// genericFace is a window's face: the chrome, and a well under every slot
// that shows text or pictures.
func genericFace(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	all := paint2d.XYWH(0, 0, w, h)
	round(ctx, all, p.radius, paint2d.Fill(p.edge))
	in := all.Inset(1)
	round(ctx, in, p.radius-1, vpaint(in, stop(0, p.bodyTop), stop(.5, p.body), stop(1, p.bodyBottom)))
	ring(ctx, in, p.radius-1, 1, p.rim)
	if u.layout == nil || u.layout.Art != u.name {
		return
	}
	for _, s := range u.layout.Slots {
		if !wellSlot(s.Name) {
			continue
		}
		r := slotRect(s, w, h)
		if r.Dx() < 2 || r.Dy() < 2 {
			continue
		}
		well(ctx, r.Inset(-1), min(p.radius/2, 4), p.well, p.wellDark, p.wellLight)
	}
}

// wellSlot reports whether a slot of that name sits in a recess.
func wellSlot(name string) bool {
	switch name {
	case "title", "time", "display", "analyser", "vis", "list", "rows", "status", "metadata", "playlist", "info", "video", "header", "search", "location", "lcd":
		return true
	}
	return false
}

// pixGlyphArt draws a "font.<hex>" sprite from the app's capital face.
func pixGlyphArt(name string, c paint2d.Color) drawFunc {
	hexs := name[strings.LastIndexByte(name, '.')+1:]
	r := rune(0)
	for _, ch := range hexs {
		r <<= 4
		switch {
		case ch >= '0' && ch <= '9':
			r |= ch - '0'
		case ch >= 'a' && ch <= 'f':
			r |= ch - 'a' + 10
		}
	}
	rows, ok := pixGlyphs[r]
	if !ok && r >= 'a' && r <= 'z' {
		rows, ok = pixGlyphs[r-32]
	}
	return func(ctx *paint2d.Context, w, h float32) {
		if !ok {
			return
		}
		gw := float32(len(rows[0]))
		ox := float32(int((w - gw) / 2))
		oy := float32(int((h - 6) / 2))
		for y, line := range rows {
			for x, ch := range line {
				if ch == '#' {
					px(ctx, ox+float32(x), oy+float32(y), 1, 1, c)
				}
			}
		}
	}
}

// ledArt is a "led.<digit>" sprite: a seven-segment digit, or the colon.
func ledArt(ctx *paint2d.Context, name string, w, h float32, c paint2d.Color) {
	d := strings.TrimPrefix(name, "led.")
	if d == "colon" {
		px(ctx, float32(int(w/2-1)), float32(int(h*.3)), 2, 2, c)
		px(ctx, float32(int(w/2-1)), float32(int(h*.65)), 2, 2, c)
		return
	}
	segs, ok := segDigits[rune(d[0])]
	if !ok {
		return
	}
	segDigit(ctx, w, h, max(1, float32(int(w/4))), segs, c)
}

// emblem is the redrawn skins' mark: a ring round a note-like dot.
func emblem(ctx *paint2d.Context, r paint2d.Rect, ringc, dot paint2d.Color) {
	c := r.Center()
	s := min(r.Dx(), r.Dy()) / 2
	ctx.DrawCircle(c, s-.5, paint2d.Fill(ringc))
	ctx.DrawCircle(c, s*.72, paint2d.Fill(dot))
	ctx.DrawCircle(paint2d.Pt(c.X-s*.12, c.Y+s*.16), s*.26, paint2d.Fill(ringc))
	fill(ctx, paint2d.XYWH(c.X+s*.06, c.Y-s*.45, max(1, s*.12), s*.62), ringc)
}

// genericSkin builds a skin on the generic painter alone.
func genericSkin(id, label, summary string, g *geometry, p *palette, own func(u use) drawFunc, text map[string]skingen.TextRole, colors map[string]string) *skingen.Plan {
	return build(skinArt{id: id, label: label, summary: summary, g: g, text: text, colors: colors,
		paint: func(u use) drawFunc {
			if own != nil {
				if f := own(u); f != nil {
					return f
				}
			}
			return genericArt(p, g, u)
		}})
}
