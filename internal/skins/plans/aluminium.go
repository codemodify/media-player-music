package plans

import (
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// Aluminium is a pale brushed-aluminium window with softly rounded corners:
// a pill-shaped recess for the transport's round glassy keys, a pill-shaped
// pale-olive display, graphite window gems instead of coloured lights, and a
// floating equaliser of grooved faders with blue-steel caps.
func Aluminium() *Plan {
	p := aluminiumPalette()
	text := map[string]skingen.TextRole{
		"player.current":      {Color: "#14181C"},
		"player.info":         {Color: "#23282E"},
		"player.row":          {Color: "#14181C"},
		"player.selected":     {Color: "#3A6FB8"},
		"player.selectedText": {Color: "#FFFFFF"},
		"player.text":         {Color: "#283020"},
	}
	return genericSkin("aluminium", "Mac OS X - iTunes", "Pale brushed aluminium with rounded corners, a pill-shaped display, round glassy keys, graphite window gems and a floating equaliser.", &geomAluminium, p, func(u use) drawFunc { return aluminiumArt(p, u) }, text, map[string]string{"focus": "#3A6FB8"})
}

func aluminiumPalette() *palette {
	return &palette{body: rgb("#c9ccd0"), bodyTop: rgb("#e2e4e7"), bodyBottom: rgb("#aeb2b7"), edge: rgb("#6c7076"), rim: rgb("#f6f7f8"),
		well: rgb("#e8ecd9"), wellDark: rgb("#8b917d"), wellLight: rgb("#ffffff"),
		key: rgb("#e9ecef"), keyTop: rgb("#ffffff"), keyBottom: rgb("#c3c8ce"), keyEdge: rgb("#7c838b"),
		ink: rgb("#2e3238"), inkHover: rgb("#1660c9"), accent: rgb("#2a6fd6"), lcd: rgb("#2a2f22"), lcdDim: rgb("#a7ad94"),
		radius: 8, keyRadius: 9, gloss: .6, round: true}
}

// alState is a key's look in one state.
type alState struct{ down, hover, disabled, on bool }

func alStateOf(name string) alState {
	return alState{
		down:     strings.HasSuffix(name, ".down"),
		hover:    strings.HasSuffix(name, ".hover"),
		disabled: strings.HasSuffix(name, ".disabled"),
		on:       strings.HasSuffix(name, ".on"),
	}
}

// alMetal paints brushed aluminium over r, its fill lit from the top.
func alMetal(ctx *paint2d.Context, r paint2d.Rect, radius float32, seed uint32) {
	round(ctx, r, radius, paint2d.Fill(rgb("#6a6f75")))
	in := r.Inset(1)
	round(ctx, in, radius-1, vpaint(in, stop(0, rgb("#e6e8ea")), stop(.5, rgb("#d2d5d8")), stop(1, rgb("#c0c4c8"))))
	ctx.Save()
	if radius > 1 {
		ctx.ClipRoundRect(in, radius-1, radius-1)
	} else {
		ctx.ClipRect(in)
	}
	streaks(ctx, in, .07, seed)
	ctx.Restore()
	ring(ctx, in, radius-1, 1, rgb("#fbfcfc").WithAlpha(.8))
}

// alDisc is a round glassy key.
func alDisc(ctx *paint2d.Context, w, h float32, s alState, glyph string) {
	c := paint2d.Pt(w/2, h/2)
	r := min(w, h)/2 - .5
	top, bottom, edge := rgb("#ffffff"), rgb("#c2c8cf"), rgb("#6e757d")
	ink := rgb("#262b31")
	switch {
	case s.disabled:
		top, bottom, ink = rgb("#f1f2f3"), rgb("#d9dcdf"), rgb("#9ba1a7")
		edge = rgb("#a4aab0")
	case s.down:
		top, bottom = rgb("#a9b1ba"), rgb("#e4e7ea")
		ink = rgb("#0f1317")
	case s.hover:
		top, bottom = rgb("#ffffff"), rgb("#d3e0ef")
		ink = rgb("#1b4f99")
	}
	// a soft shadow under the key
	ctx.DrawCircle(paint2d.Pt(c.X, c.Y+.8), r, paint2d.Fill(paint2d.RGBA(0, 0, 0, .18)))
	orb(ctx, c, r, top, bottom, edge, .55)
	symbol(ctx, glyph, paint2d.XYWH(0, 0, w, h), r*.8, ink)
}

// alSmallKey is a rounded rectangular key with a mark: the toolbar keys.
func alSmallKey(ctx *paint2d.Context, w, h float32, s alState, glyph string) {
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	top, bottom, ink := rgb("#fdfdfe"), rgb("#cdd2d7"), rgb("#2e3238")
	switch {
	case s.disabled:
		top, bottom, ink = rgb("#eff0f1"), rgb("#dfe1e4"), rgb("#a2a8ae")
	case s.down:
		top, bottom = rgb("#aeb5bd"), rgb("#dfe3e7")
	case s.hover:
		bottom = rgb("#dbe5f0")
	}
	if s.on {
		top, bottom, ink = rgb("#cfe0f6"), rgb("#93b5e2"), rgb("#103e85")
	}
	pillow(ctx, r, 5, top, bottom, rgb("#737a82"), .45)
	symbol(ctx, glyph, paint2d.XYWH(0, 0, w, h), min(w, h)*.48, ink)
}

// alGem is a window key: a small graphite gem with its mark etched in.
func alGem(ctx *paint2d.Context, w, h float32, s alState, glyph string) {
	c := paint2d.Pt(w/2, h/2)
	r := min(w, h)/2 - .5
	top, bottom := rgb("#8d949c"), rgb("#4b5158")
	ink := rgb("#e9edf1")
	switch {
	case s.disabled:
		top, bottom, ink = rgb("#c7ccd1"), rgb("#aab0b6"), rgb("#e6e8ea")
	case s.down:
		top, bottom = rgb("#3b4046"), rgb("#6b737b")
	case s.hover:
		top, bottom = rgb("#9fb3c9"), rgb("#4e6278")
	}
	orb(ctx, c, r, top, bottom, rgb("#30353a"), .5)
	if !s.hover && !s.down {
		return
	}
	symbol(ctx, glyph, paint2d.XYWH(0, 0, w, h), r*1.05, ink)
}

func aluminiumArt(p *palette, u use) func(*paint2d.Context, float32, float32) {
	name := u.name
	s := alStateOf(name)
	g := glyphFor(name)
	switch {
	case name == "face.main" || name == "face.eq" || name == "face.mini":
		radius := float32(9)
		if name == "face.mini" {
			radius = 8
		}
		return func(ctx *paint2d.Context, w, h float32) {
			all := paint2d.XYWH(0, 0, w, h)
			alMetal(ctx, all, radius, 0x1f2e3d)
			if name == "face.eq" {
				aluminiumEqMarks(ctx, u, w, h)
			}
		}
	case name == "controlwell.main" || name == "controlwell.mini":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(.5, .5, w-1, h-1)
			round(ctx, r, h/2, vpaint(r, stop(0, rgb("#9ea4aa")), stop(1, rgb("#f2f3f4"))))
			in := r.Inset(1)
			round(ctx, in, in.Dy()/2, vpaint(in, stop(0, rgb("#b4b9bf")), stop(1, rgb("#dadde0"))))
			innerShadow(ctx, in, in.Dy()/2, 4, paint2d.RGBA(0, 0, 0, .18))
		}
	case name == "lcd.main" || name == "lcd.mini":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(.5, .5, w-1, h-1)
			round(ctx, r, h/2, paint2d.Fill(rgb("#7f8572")))
			in := r.Inset(1)
			round(ctx, in, in.Dy()/2, vpaint(in, stop(0, rgb("#e9edd6")), stop(.55, rgb("#dde3c4")), stop(1, rgb("#cfd6b3"))))
			innerShadow(ctx, in, in.Dy()/2, 5, paint2d.RGBA(.2, .25, .1, .22))
			hl := paint2d.XYWH(in.Min.X+in.Dy()/3, in.Min.Y+1, in.Dx()-in.Dy()*2/3, in.Dy()*.35)
			round(ctx, hl, hl.Dy()/2, vpaint(hl, stop(0, paint2d.RGBA(1, 1, 1, .45)), stop(1, paint2d.RGBA(1, 1, 1, 0))))
		}
	case strings.HasPrefix(name, "key.play") || strings.HasPrefix(name, "key.pause") || strings.HasPrefix(name, "key.stop") ||
		strings.HasPrefix(name, "key.prev") || strings.HasPrefix(name, "key.next") ||
		strings.HasPrefix(name, "mini.play") || strings.HasPrefix(name, "mini.pause") || strings.HasPrefix(name, "mini.stop") ||
		strings.HasPrefix(name, "mini.prev") || strings.HasPrefix(name, "mini.next"):
		return func(ctx *paint2d.Context, w, h float32) { alDisc(ctx, w, h, s, g) }
	case strings.HasPrefix(name, "key.close") || strings.HasPrefix(name, "key.minimize") || strings.HasPrefix(name, "key.mini") ||
		strings.HasPrefix(name, "mini.close") || strings.HasPrefix(name, "mini.minimize") || strings.HasPrefix(name, "mini.mini"):
		return func(ctx *paint2d.Context, w, h float32) { alGem(ctx, w, h, s, g) }
	case strings.HasPrefix(name, "key.open") || strings.HasPrefix(name, "key.shuffle") || strings.HasPrefix(name, "key.repeat") ||
		strings.HasPrefix(name, "key.visual") || strings.HasPrefix(name, "key.eq"):
		glyph := g
		if strings.HasPrefix(name, "key.visual") {
			glyph = "vis"
		} else if strings.HasPrefix(name, "key.open") {
			glyph = "add"
		}
		return func(ctx *paint2d.Context, w, h float32) { alSmallKey(ctx, w, h, s, glyph) }
	case strings.HasPrefix(name, "key.on"):
		on := name == "key.on.on"
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(1.5, 2, w-3, w-3)
			top, bottom := rgb("#ffffff"), rgb("#d6dade")
			if s.down {
				top, bottom = rgb("#c3c9cf"), rgb("#e7eaed")
			}
			pillow(ctx, r, 3, top, bottom, rgb("#6c737a"), .4)
			if on {
				q := paint2d.NewPath()
				q.MoveTo(r.Min.X+2.5, r.Min.Y+r.Dy()*.52)
				q.LineTo(r.Min.X+r.Dx()*.42, r.Max.Y-3)
				q.LineTo(r.Max.X-2, r.Min.Y+2.5)
				ctx.DrawPath(q, paint2d.StrokePaint(rgb("#1d58b0"), 2))
			}
		}
	case strings.HasPrefix(name, "key.preset"):
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(.5, .5, w-1, h-1)
			top, bottom := rgb("#ffffff"), rgb("#d5d9de")
			if s.down {
				top, bottom = rgb("#b4bbc3"), rgb("#e3e6e9")
			} else if s.hover {
				bottom = rgb("#dbe5f0")
			}
			pillow(ctx, r, h/2-.5, top, bottom, rgb("#737a82"), .4)
			// the chooser's arrows in the right cap
			cap := paint2d.XYWH(w-16, 0, 16, h)
			ink := rgb("#2e3238")
			if s.disabled {
				ink = rgb("#a2a8ae")
			}
			symbol(ctx, "up", paint2d.XYWH(cap.Min.X-1, h/2-6, 14, 6), 6, ink)
			symbol(ctx, "down", paint2d.XYWH(cap.Min.X-1, h/2, 14, 6), 6, ink)
		}
	case name == "key.mute":
		return func(ctx *paint2d.Context, w, h float32) {
			symbol(ctx, "speaker", paint2d.XYWH(0, 0, w, h), min(w, h)*.85, rgb("#3b4047"))
		}
	case name == "eq.track":
		return func(ctx *paint2d.Context, w, h float32) {
			c := paint2d.XYWH(w/2-2.5, 2, 5, h-4)
			round(ctx, c, 2.5, paint2d.Fill(rgb("#f4f5f6")))
			round(ctx, paint2d.XYWH(c.Min.X, c.Min.Y, c.Dx(), c.Dy()-1), 2.5, paint2d.Fill(rgb("#6c7279")))
			round(ctx, paint2d.XYWH(c.Min.X+1, c.Min.Y+1, c.Dx()-2, c.Dy()-2), 1.5, paint2d.Fill(rgb("#3c4147")))
			fill(ctx, paint2d.XYWH(w/2-5, h/2, 10, 1), rgb("#7b8189").WithAlpha(.6))
		}
	case strings.HasPrefix(name, "eq.thumb"):
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(1, h/2-5, w-2, 10)
			top, bottom := rgb("#e8f0fa"), rgb("#7f9cc2")
			switch {
			case s.disabled:
				top, bottom = rgb("#eceef0"), rgb("#c3c8cd")
			case s.down:
				top, bottom = rgb("#7d9bc4"), rgb("#c9daef")
			case s.hover:
				top, bottom = rgb("#f4f8fd"), rgb("#94b2d9")
			}
			ctx.DrawRoundRect(paint2d.XYWH(r.Min.X, r.Min.Y+1, r.Dx(), r.Dy()), 5, 5, paint2d.Fill(paint2d.RGBA(0, 0, 0, .2)))
			pillow(ctx, r, 5, top, bottom, rgb("#3f5675"), .5)
			fill(ctx, paint2d.XYWH(r.Min.X+5, h/2, r.Dx()-10, 1), rgb("#2f4766").WithAlpha(.7))
		}
	case name == "seek.track" || name == "volume.track":
		return func(ctx *paint2d.Context, w, h float32) {
			th := float32(5)
			if name == "seek.track" {
				th = 6
			}
			c := paint2d.XYWH(0, (h-th)/2, w, th)
			round(ctx, c, th/2, paint2d.Fill(rgb("#f6f7f8")))
			round(ctx, paint2d.XYWH(0, c.Min.Y, w, th-1), th/2, paint2d.Fill(rgb("#5f656c")))
			in := paint2d.XYWH(1, c.Min.Y+1, w-2, th-2)
			round(ctx, in, (th-2)/2, vpaint(in, stop(0, rgb("#9ca3aa")), stop(1, rgb("#c8cdd2"))))
		}
	case strings.HasPrefix(name, "seek.thumb"):
		return func(ctx *paint2d.Context, w, h float32) {
			q := paint2d.NewPath()
			q.MoveTo(w/2, .5)
			q.LineTo(w-.5, h/2)
			q.LineTo(w/2, h-.5)
			q.LineTo(.5, h/2)
			q.Close()
			col := rgb("#22272c")
			switch {
			case s.disabled:
				col = rgb("#a3a9af")
			case s.down:
				col = rgb("#1d58b0")
			case s.hover:
				col = rgb("#2f4766")
			}
			ctx.DrawPath(q, paint2d.Fill(col))
		}
	case strings.HasPrefix(name, "volume.thumb"):
		return func(ctx *paint2d.Context, w, h float32) {
			alDisc(ctx, w, h, s, "")
		}
	case name == "search":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(.5, 3.5, w-1, h-7)
			round(ctx, r, r.Dy()/2, paint2d.Fill(rgb("#7a8087")))
			in := r.Inset(1)
			round(ctx, in, in.Dy()/2, paint2d.Fill(rgb("#ffffff")))
			innerShadow(ctx, in, in.Dy()/2, 3, paint2d.RGBA(0, 0, 0, .2))
		}
	case name == "resize" || name == "mini.resize":
		return func(ctx *paint2d.Context, w, h float32) {
			for i := float32(0); i < 3; i++ {
				o := 3 + i*3.5
				q := paint2d.NewPath()
				q.MoveTo(w-1, o)
				q.LineTo(o, h-1)
				ctx.DrawPath(q, paint2d.StrokePaint(rgb("#6c7279"), 1))
				q = paint2d.NewPath()
				q.MoveTo(w-1, o+1)
				q.LineTo(o+1, h-1)
				ctx.DrawPath(q, paint2d.StrokePaint(rgb("#ffffff").WithAlpha(.8), 1))
			}
		}
	case name == "app.icon":
		return func(ctx *paint2d.Context, w, h float32) {
			emblem(ctx, paint2d.XYWH(0, 0, w, h), rgb("#3a6fb8"), rgb("#eef3f9"))
		}
	case name == "library.icon" || name == "source.library":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(.5, .5, w-1, h-1)
			pillow(ctx, r, 3, rgb("#7fa6dc"), rgb("#2f62ad"), rgb("#1f4277"), .3)
			c := paint2d.Pt(w*.42, h*.68)
			ctx.DrawCircle(c, w*.16, paint2d.Fill(rgb("#ffffff")))
			fill(ctx, paint2d.XYWH(c.X+w*.1, h*.2, max(1, w*.1), h*.5), rgb("#ffffff"))
			fill(ctx, paint2d.XYWH(c.X+w*.1, h*.2, w*.24, h*.1), rgb("#ffffff"))
		}
	}
	return nil
}

// aluminiumEqMarks etches the faders' scale beside the preamp: a long tick
// at the top, the middle and the foot of the travel, and the dB values.
func aluminiumEqMarks(ctx *paint2d.Context, u use, w, h float32) {
	ink := rgb("#3f444b")
	for _, slot := range []struct{ name, text string }{{"db.max", "+12 DB"}, {"db.zero", "0 DB"}, {"db.min", "-12 DB"}} {
		r, ok := slotAt(u.layout, slot.name)
		if !ok {
			continue
		}
		tw := float32(pixWidth(slot.text))
		pixLabel(ctx, float32(int(r.Max.X-tw-2)), float32(int(r.Min.Y+(r.Dy()-6)/2)), slot.text, ink)
	}
	// a hairline under the title bar
	fill(ctx, paint2d.XYWH(10, 22, w-20, 1), rgb("#a3a8ae"))
	fill(ctx, paint2d.XYWH(10, 23, w-20, 1), rgb("#f6f7f8"))
}
