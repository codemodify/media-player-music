package plans

import (
	"strings"

	"goframework.io/graphics-paint2d"
)

// Wingspan is a jade console: a rounded head with a brow of pearl keys over
// a dark glass display and a grilled chin, and two wings that fold out of
// its sides, the equaliser's to the left and the playlist's to the right,
// each with a speaker cluster and a grip at its tip. Closed, each wing
// leaves a short fin on the head that carries its grip.
func Wingspan() *Plan {
	p := wingspanPalette()
	return genericSkin("wingspan", "Wingspan", "A jade console with a dark glass display and speaker wings that fold out for the equaliser and the playlist.", &geomWingspan, p, func(u use) drawFunc { return wingspanArt(p, u) }, nil, nil)
}

func wingspanPalette() *palette {
	return &palette{body: rgb("#23806c"), bodyTop: rgb("#4cc0a3"), bodyBottom: rgb("#125244"), edge: rgb("#082520"), rim: rgb("#a6f2dc"),
		well: rgb("#06120f"), wellDark: rgb("#020706"), wellLight: rgb("#3c9a84"),
		key: rgb("#e6f6f1"), keyTop: rgb("#fbfffe"), keyBottom: rgb("#93cbbd"), keyEdge: rgb("#0e3a31"),
		ink: rgb("#0f4a3d"), inkHover: rgb("#047a5c"), accent: rgb("#ffc23a"), lcd: rgb("#78cf08"), lcdDim: rgb("#24502a"),
		radius: 18, keyRadius: 6, gloss: 0.55, round: true}
}

// The face's parts, in the layout's design pixels.
var (
	wsEqWing  = paint2d.XYWH(4, 88, 282, 164)
	wsPlWing  = paint2d.XYWH(474, 88, 282, 164)
	wsEqFin   = paint2d.XYWH(204, 138, 80, 94)
	wsPlFin   = paint2d.XYWH(476, 138, 80, 94)
	wsHeadBox = paint2d.XYWH(268, 0, 224, 262)
	wsBrow    = paint2d.XYWH(298, 26, 164, 36)
	wsDisplay = paint2d.XYWH(280, 62, 200, 154)
)

func wingspanArt(p *palette, u use) drawFunc {
	name := u.name
	switch {
	case strings.HasPrefix(name, "face."):
		state := strings.TrimPrefix(name, "face.")
		eq, pl := state == "eq" || state == "both", state == "pl" || state == "both"
		return func(ctx *paint2d.Context, w, h float32) { wsFace(ctx, p, eq, pl, true) }
	case name == "head.foreground":
		return func(ctx *paint2d.Context, w, h float32) {
			ctx.Translate(-261, 0)
			wsHead(ctx, p)
		}
	case name == "eq.drawer":
		return func(ctx *paint2d.Context, w, h float32) {
			ctx.Translate(0, -86)
			wsWing(ctx, p, true)
		}
	case name == "pl.drawer":
		return func(ctx *paint2d.Context, w, h float32) {
			ctx.Translate(-488, -86)
			wsWing(ctx, p, false)
		}
	case strings.Contains(name, ".handle."):
		return func(ctx *paint2d.Context, w, h float32) { wsHandle(ctx, p, u, w, h) }
	case name == "key.eq.reset" || name == "key.eq.reset.hover":
		return func(ctx *paint2d.Context, w, h float32) {
			top, bottom := p.keyTop, p.keyBottom
			if u.hover() {
				top, bottom = rgb("#ffffff"), lift(p.keyBottom, .3)
			}
			pillow(ctx, paint2d.XYWH(.5, .5, w-1, h-1), (h-1)/2, top, bottom, p.keyEdge, .5)
			pixLabel(ctx, float32(int((w-float32(pixWidth("RESET")))/2)), float32(int((h-6)/2)), "RESET", p.ink)
		}
	case name == "key.list.header":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(.5, .5, w-1, h-1)
			pillow(ctx, r, 5, rgb("#2f9a82"), rgb("#145446"), p.edge, .3)
			ctx.DrawCircle(paint2d.Pt(10, h/2), 3.5, paint2d.Fill(p.accent))
			pixLabel(ctx, 19, float32(int((h-6)/2)), "PLAYLIST", rgb("#e8fff8"))
			symbol(ctx, "down", paint2d.XYWH(w-18, 0, 14, h), 8, rgb("#e8fff8"))
		}
	case name == "horizontal.track":
		return func(ctx *paint2d.Context, w, h float32) {
			c := paint2d.XYWH(0, h/2-2.5, w, 5)
			fill(ctx, c, p.wellLight)
			fill(ctx, paint2d.XYWH(0, c.Min.Y, w, 4), p.wellDark)
			fill(ctx, paint2d.XYWH(0, c.Min.Y+1, w, 3), rgb("#0d2722"))
		}
	case name == "vertical.track":
		return func(ctx *paint2d.Context, w, h float32) {
			c := paint2d.XYWH(w/2-2.5, 0, 5, h)
			fill(ctx, c, p.wellLight)
			fill(ctx, paint2d.XYWH(c.Min.X, 0, 4, h), p.wellDark)
			fill(ctx, paint2d.XYWH(c.Min.X+1, 0, 3, h), rgb("#0d2722"))
		}
	case name == "horizontal.thumb" || name == "vertical.thumb":
		return func(ctx *paint2d.Context, w, h float32) {
			if w < h {
				pillow(ctx, paint2d.XYWH(.5, .5, w-1, h-1), (w-1)/2, p.keyTop, p.keyBottom, p.keyEdge, .5)
				return
			}
			orb(ctx, paint2d.Pt(w/2, h/2), min(w, h)/2-.25, p.keyTop, p.keyBottom, p.keyEdge, .55)
		}
	case name == "seek.track" || name == "seek.progress":
		lit := name == "seek.progress"
		return func(ctx *paint2d.Context, w, h float32) {
			c := paint2d.XYWH(.5, h/2-3, w-1, 6)
			round(ctx, c, 3, paint2d.Fill(p.wellLight))
			round(ctx, paint2d.XYWH(c.Min.X, c.Min.Y, c.Dx(), 5), 3, paint2d.Fill(p.wellDark))
			in := paint2d.XYWH(c.Min.X+1, c.Min.Y+1, c.Dx()-2, 3.5)
			col := rgb("#0d2722")
			if lit {
				round(ctx, in, 1.75, hpaint(in, stop(0, sink(p.accent, .35)), stop(1, p.accent)))
				return
			}
			round(ctx, in, 1.75, paint2d.Fill(col))
		}
	case strings.HasPrefix(name, "seek.thumb"):
		return func(ctx *paint2d.Context, w, h float32) {
			top, bottom := p.keyTop, p.keyBottom
			if u.down() {
				top, bottom = p.keyBottom, p.keyTop
			} else if u.hover() {
				top, bottom = rgb("#ffffff"), lift(p.keyBottom, .3)
			}
			pillow(ctx, paint2d.XYWH(.5, .5, w-1, h-1), (h-1)/2, top, bottom, p.keyEdge, .5)
			fill(ctx, paint2d.XYWH(w/2-.5, 2.5, 1, h-5), p.ink.WithAlpha(.6))
		}
	case name == "key.skin" || name == "key.skin.hover" || name == "key.skin.down":
		return func(ctx *paint2d.Context, w, h float32) {
			top, bottom := p.keyTop, p.keyBottom
			if u.down() {
				top, bottom = p.keyBottom, p.keyTop
			} else if u.hover() {
				top = rgb("#ffffff")
			}
			c := paint2d.Pt(w/2, h/2)
			orb(ctx, c, min(w, h)/2-.5, top, bottom, p.keyEdge, .5)
			emblem(ctx, paint2d.XYWH(c.X-8, c.Y-8, 16, 16), p.ink, lift(p.keyBottom, .4))
		}
	case strings.HasPrefix(name, "key."):
		return func(ctx *paint2d.Context, w, h float32) { wsKey(ctx, p, u, w, h) }
	}
	return nil
}

// wsKey is a pearl round key with its mark.
func wsKey(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	top, bottom, ink := p.keyTop, p.keyBottom, p.ink
	switch {
	case strings.HasSuffix(u.name, ".disabled"):
		top, bottom, ink = rgb("#c9d6d2"), rgb("#8fa39d"), rgb("#62746f")
	case u.down():
		top, bottom, ink = sink(p.keyBottom, .1), p.keyTop, p.inkHover
	case u.hover():
		top, bottom, ink = rgb("#ffffff"), lift(p.keyBottom, .35), p.inkHover
	}
	g := glyphFor(u.name)
	if strings.HasSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(u.name, ".hover"), ".disabled"), ".down"), ".close") && strings.Count(u.name, ".") >= 2 && (strings.HasPrefix(u.name, "key.eq.") || strings.HasPrefix(u.name, "key.pl.")) {
		// a wing's own close: a small amber bead
		top, bottom = lift(p.accent, .45), sink(p.accent, .2)
		if u.hover() {
			top = rgb("#fff3c8")
		}
		ink = rgb("#4a2a00")
		g = "close"
	}
	c := paint2d.Pt(w/2, h/2)
	r := min(w, h)/2 - .5
	orb(ctx, c, r, top, bottom, p.keyEdge, .55)
	d := float32(0)
	if u.down() {
		d = .5
	}
	if g != "" {
		symbol(ctx, g, paint2d.XYWH(d, d, w, h), r*.95, ink)
	}
}

// wsHandle is a wing's grip: a tall amber-edged pill with a chevron that
// points the way the wing will go.
func wsHandle(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	eq := strings.HasPrefix(u.name, "key.eq.")
	closing := strings.Contains(u.name, ".handle.close")
	top, bottom := rgb("#2fa88c"), rgb("#0f4a3d")
	if u.hover() {
		top, bottom = rgb("#46c7a8"), rgb("#16604f")
	}
	if u.down() {
		top, bottom = rgb("#0f4a3d"), rgb("#23806c")
	}
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	pillow(ctx, r, (w-1)/2, top, bottom, p.edge, .35)
	ring(ctx, r.Inset(1.5), (w-4)/2, 1, p.accent.WithAlpha(.8))
	for y := h/2 - 16; y <= h/2+16; y += 4 {
		if y > h/2-7 && y < h/2+7 {
			continue
		}
		fill(ctx, paint2d.XYWH(5, y, w-10, 1), p.edge.WithAlpha(.7))
		fill(ctx, paint2d.XYWH(5, y+1, w-10, 1), p.rim.WithAlpha(.35))
	}
	// left wing: closing points right (into the head); right wing: left
	right := eq == closing
	cx, cy := w/2, h/2
	v := paint2d.NewPath()
	if right {
		v.MoveTo(cx-2.5, cy-5)
		v.LineTo(cx+3, cy)
		v.LineTo(cx-2.5, cy+5)
	} else {
		v.MoveTo(cx+2.5, cy-5)
		v.LineTo(cx-3, cy)
		v.LineTo(cx+2.5, cy+5)
	}
	ctx.DrawPath(v, paint2d.StrokePaint(p.accent, 2))
}

// wsFace is the whole window: the open wings or the closed fins, then the
// head over them.
func wsFace(ctx *paint2d.Context, p *palette, eq, pl, head bool) {
	if eq {
		wsWing(ctx, p, true)
	} else {
		wsFin(ctx, p, wsEqFin)
	}
	if pl {
		wsWing(ctx, p, false)
	} else {
		wsFin(ctx, p, wsPlFin)
	}
	if head {
		wsHead(ctx, p)
	}
}

func wsShell(ctx *paint2d.Context, p *palette, r paint2d.Rect, radius float32) {
	round(ctx, r, radius, paint2d.Fill(p.edge))
	in := r.Inset(1.5)
	round(ctx, in, radius-1.5, vpaint(in, stop(0, p.bodyTop), stop(.45, p.body), stop(1, p.bodyBottom)))
	ring(ctx, r.Inset(2), radius-2, 1, p.rim.WithAlpha(.45))
}

func wsFin(ctx *paint2d.Context, p *palette, r paint2d.Rect) {
	wsShell(ctx, p, r, 20)
	slot := paint2d.XYWH(r.Min.X+8, r.Min.Y+8, r.Dx()-16, r.Dy()-16)
	if r.Min.X < 300 {
		slot = paint2d.XYWH(209, 146, 30, 78)
	} else {
		slot = paint2d.XYWH(517, 145, 30, 79)
	}
	round(ctx, slot, 15, paint2d.Fill(p.edge.WithAlpha(.85)))
}

// speaker is a round grille: a dark cone behind concentric rings of holes.
func speaker(ctx *paint2d.Context, p *palette, c paint2d.Point, r float32) {
	ctx.DrawCircle(c, r+1.5, paint2d.Fill(p.rim.WithAlpha(.5)))
	ctx.DrawCircle(c, r, paint2d.Fill(p.edge))
	ctx.DrawCircle(c, r-1.5, paint2d.Radial(paint2d.RadialGradient{Center: c, Focal: c, Radius: r - 1.5,
		Stops: []paint2d.GradientStop{stop(0, rgb("#2b5c52")), stop(.6, rgb("#0b2621")), stop(1, rgb("#03100d"))}}))
	for ring := float32(4); ring < r-2; ring += 4 {
		n := int(ring * 1.4)
		for i := 0; i < n; i++ {
			a := float32(i) / float32(n) * 6.2832
			x, y := c.X+ring*cos32(a), c.Y+ring*sin32(a)
			ctx.DrawCircle(paint2d.Pt(x, y), .8, paint2d.Fill(rgb("#000000").WithAlpha(.75)))
		}
	}
	ctx.DrawCircle(c, 2.5, paint2d.Fill(p.accent))
}

func wsWing(ctx *paint2d.Context, p *palette, left bool) {
	r := wsEqWing
	recess := paint2d.XYWH(86, 98, 170, 144)
	grip := paint2d.XYWH(5, 140, 24, 90)
	sx := float32(56)
	if !left {
		r = wsPlWing
		recess = paint2d.XYWH(502, 95, 174, 143)
		grip = paint2d.XYWH(731, 139, 24, 91)
		sx = 709
	}
	wsShell(ctx, p, r, 34)
	// the speaker cluster between the grip and the recess
	speaker(ctx, p, paint2d.Pt(sx, 170), 20)
	speaker(ctx, p, paint2d.Pt(sx, 117), 11)
	speaker(ctx, p, paint2d.Pt(sx, 223), 11)
	round(ctx, grip, 12, paint2d.Fill(p.edge.WithAlpha(.85)))
	well(ctx, recess, 10, p.well, p.wellDark, p.wellLight)
	in := recess.Inset(1)
	ctx.Save()
	ctx.ClipRoundRect(in, 9, 9)
	ctx.DrawRect(paint2d.XYWH(in.Min.X, in.Min.Y, in.Dx(), 30), vpaint(paint2d.XYWH(0, in.Min.Y, 1, 30), stop(0, rgb("#ffffff").WithAlpha(.07)), stop(1, rgb("#ffffff").WithAlpha(0))))
	ctx.Restore()
}

func wsHead(ctx *paint2d.Context, p *palette) {
	// the chin, under the head's lower edge
	chin := paint2d.NewPath()
	chin.MoveTo(286, 228)
	chin.CubicTo(294, 296, 336, 350, 380, 352)
	chin.CubicTo(424, 350, 466, 296, 474, 228)
	chin.Close()
	ctx.DrawPath(chin, paint2d.Fill(p.edge))
	inner := paint2d.NewPath()
	inner.MoveTo(289, 230)
	inner.CubicTo(297, 294, 338, 347, 380, 349)
	inner.CubicTo(422, 347, 463, 294, 471, 230)
	inner.Close()
	chinPaint := vpaint(paint2d.XYWH(0, 228, 1, 124), stop(0, p.bodyBottom), stop(1, sink(p.bodyBottom, .35)))
	ctx.DrawPath(inner, chinPaint)
	wsShell(ctx, p, wsHeadBox, 48)
	// the chin runs on out of the head without a seam
	ctx.Save()
	ctx.ClipRect(paint2d.XYWH(0, 252, 760, 120))
	ctx.DrawPath(inner, chinPaint)
	ctx.Restore()
	// a grille of holes down the chin
	for row := 0; row < 6; row++ {
		y := float32(276 + row*10)
		half := 52 - float32(row*row)*1.6
		for x := 380 - half; x <= 380+half; x += 9 {
			ctx.DrawCircle(paint2d.Pt(x, y), 2, paint2d.Fill(p.edge))
			ctx.DrawCircle(paint2d.Pt(x, y-.6), 1.2, paint2d.Fill(rgb("#000000").WithAlpha(.6)))
		}
	}
	// the brow that carries the transport
	well(ctx, wsBrow, 18, mix(p.body, p.edge, .55), p.edge, p.rim.WithAlpha(.6))
	// the display
	well(ctx, wsDisplay.Inset(-3), 20, mix(p.body, p.edge, .6), p.edge, p.rim.WithAlpha(.55))
	round(ctx, wsDisplay, 17, paint2d.Fill(p.well))
	in := wsDisplay.Inset(1)
	ctx.Save()
	ctx.ClipRoundRect(in, 16, 16)
	ctx.DrawRect(in, vpaint(in, stop(0, rgb("#0c231e")), stop(.55, p.well), stop(1, rgb("#081a16"))))
	for y := in.Min.Y + 2; y < in.Max.Y; y += 3 {
		fill(ctx, paint2d.XYWH(in.Min.X, y, in.Dx(), 1), rgb("#000000").WithAlpha(.22))
	}
	glare := paint2d.NewPath()
	glare.MoveTo(in.Min.X, in.Min.Y)
	glare.LineTo(in.Min.X+in.Dx()*.62, in.Min.Y)
	glare.LineTo(in.Min.X+in.Dx()*.38, in.Min.Y+in.Dy()*.45)
	glare.LineTo(in.Min.X, in.Min.Y+in.Dy()*.45)
	glare.Close()
	ctx.DrawPath(glare, paint2d.Fill(rgb("#ffffff").WithAlpha(.035)))
	ctx.Restore()
	// the band between the display and the seek bar
	fill(ctx, paint2d.XYWH(300, 220, 160, 1), p.rim.WithAlpha(.3))
	// small amber pilot lamps beside the window keys
	ctx.DrawCircle(paint2d.Pt(352, 12), 2, paint2d.Fill(p.accent))
	ctx.DrawCircle(paint2d.Pt(400, 12), 2, paint2d.Fill(p.accent))
}
