package plans

import (
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// Cobalt is a deep cobalt-blue player with pale brushed-steel trim: a wide
// dark display under a title bar of glossy steel keys, a steel transport
// shelf of glossy cobalt keys, a pale steel equaliser drawer below, a
// playlist drawer to the left and a compact mode. Drawn from gradients and
// rounded shapes only.
func Cobalt() *Plan {
	return build(skinArt{id: "cobalt", label: "Cobalt", g: &geomCobalt,
		summary: "Deep cobalt and pale steel: a wide display under a bar of glossy keys, a steel transport shelf, drawers for the playlist and equaliser, and a compact mode.",
		paint:   cobaltArt,
		text: map[string]skingen.TextRole{
			"player.text": {Color: "#7CF09A", Bold: true}, "player.info": {Color: "#7CF09A", Bold: true},
			"player.selected": {Color: "#2B4F9E"},
		},
		colors: map[string]string{"focus": "#7CF09A"}})
}

var (
	cBlueHi  = rgb("#5f86d2")
	cBlue    = rgb("#2f55a4")
	cBlueLo  = rgb("#18336f")
	cEdge    = rgb("#0a1631")
	cSteelHi = rgb("#f4f7fb")
	cSteel   = rgb("#c9d3e3")
	cSteelLo = rgb("#93a3bf")
	cScreen  = rgb("#03060d")
	cInk     = rgb("#1b2d55")
	cGreen   = rgb("#5ee27e")
	cCyan    = rgb("#5fd0f0")
)

func cobaltArt(u use) drawFunc {
	name := u.name
	switch name {
	case "face.main":
		return cobaltMain
	case "face.transport":
		return cobaltTransport
	case "face.equalizer":
		return cobaltEqualizer
	case "face.playlist":
		return cobaltPlaylist
	case "face.compact":
		return cobaltCompact
	case "app.icon", "app.logo":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			if w > 20 {
				ctx.DrawCircle(r.Center(), w/2, paint2d.Fill(cBlueLo.WithAlpha(.6)))
				ctx.DrawCircle(r.Center(), w/2-1, paint2d.StrokePaint(cBlueHi, 1.5))
				r = r.Inset(w * .12)
			}
			emblem(ctx, r, cSteel, cBlueLo)
		}
	case "playlist.grip":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			pillow(ctx, r, 4, cSteelHi, cSteelLo, cEdge, .3)
			for y := h/2 - 6; y <= h/2+6; y += 4 {
				fill(ctx, paint2d.XYWH(3, y, w-6, 1), cBlueLo)
				fill(ctx, paint2d.XYWH(3, y+1, w-6, 1), cSteelHi)
			}
		}
	case "resize":
		return func(ctx *paint2d.Context, w, h float32) {
			for i := float32(0); i < 4; i++ {
				q := paint2d.NewPath()
				q.MoveTo(w-3-i*4, h-3)
				q.LineTo(w-3, h-3-i*4)
				ctx.DrawPath(q, paint2d.StrokePaint(cBlueLo, 1.4))
				q2 := paint2d.NewPath()
				q2.MoveTo(w-2-i*4, h-2)
				q2.LineTo(w-2, h-2-i*4)
				ctx.DrawPath(q2, paint2d.StrokePaint(cSteelHi, .8))
			}
		}
	}
	switch {
	case strings.HasSuffix(name, ".track") || strings.HasSuffix(name, ".track.disabled"):
		return func(ctx *paint2d.Context, w, h float32) {
			cobaltGroove(ctx, w, h, strings.HasPrefix(name, "eq."), strings.HasSuffix(name, "disabled"), strings.HasPrefix(name, "balance"))
		}
	case strings.HasSuffix(name, ".foreground"):
		return func(ctx *paint2d.Context, w, h float32) { cobaltFill(ctx, w, h) }
	case strings.Contains(name, ".thumb"):
		return func(ctx *paint2d.Context, w, h float32) { cobaltThumb(ctx, u, w, h) }
	case strings.HasPrefix(name, "key."):
		return func(ctx *paint2d.Context, w, h float32) { cobaltKey(ctx, u, w, h) }
	}
	return func(ctx *paint2d.Context, w, h float32) { round(ctx, paint2d.XYWH(0, 0, w, h), 3, paint2d.Fill(cBlue)) }
}

// cobaltBody is a cobalt panel with a dark outline and a lit inner rim.
func cobaltBody(ctx *paint2d.Context, r paint2d.Rect, radius float32) {
	round(ctx, r, radius, paint2d.Fill(cEdge))
	in := r.Inset(1)
	round(ctx, in, radius-1, vpaint(in, stop(0, cBlueHi), stop(.18, cBlue), stop(1, cBlueLo)))
	ring(ctx, in, radius-1, 1, cBlueHi.WithAlpha(.55))
}

// cobaltShelf is a pale brushed-steel panel.
func cobaltShelf(ctx *paint2d.Context, r paint2d.Rect, radius float32, seed uint32) {
	round(ctx, r, radius, paint2d.Fill(cEdge))
	in := r.Inset(1)
	round(ctx, in, radius-1, vpaint(in, stop(0, cSteelHi), stop(.5, cSteel), stop(1, cSteelLo)))
	ctx.Save()
	ctx.ClipRoundRect(in, radius-1, radius-1)
	streaks(ctx, in, .05, seed)
	ctx.Restore()
	ring(ctx, in, radius-1, 1, paint2d.RGBA(1, 1, 1, .7))
}

// cobaltScreen is the display's recess.
func cobaltScreen(ctx *paint2d.Context, r paint2d.Rect, radius float32) {
	round(ctx, r.Inset(-1), radius+1, paint2d.Fill(cBlueHi.WithAlpha(.8)))
	round(ctx, paint2d.XYWH(r.Min.X-1, r.Min.Y-1, r.Dx()+1, r.Dy()+1), radius+1, paint2d.Fill(cEdge))
	round(ctx, r, radius, vpaint(r, stop(0, rgb("#0b1426")), stop(.25, cScreen), stop(1, cScreen)))
}

// The main face is nine-sliced (top 33, right 59, bottom 23, left 286): the
// display and both bands run straight across the stretching column and row.
func cobaltMain(ctx *paint2d.Context, w, h float32) {
	cobaltBody(ctx, paint2d.XYWH(0, 0, w, h), 12)
	// the title bar's sheen and its rule
	bar := paint2d.XYWH(1, 1, w-2, 24)
	ctx.Save()
	ctx.ClipRoundRect(paint2d.XYWH(1, 1, w-2, h-2), 11, 11)
	ctx.DrawRect(bar, vpaint(bar, stop(0, paint2d.RGBA(1, 1, 1, .22)), stop(1, paint2d.RGBA(1, 1, 1, .04))))
	ctx.Restore()
	emblem(ctx, paint2d.XYWH(5, 6, 12, 12), cSteel, cBlueLo)
	// the steel bay the bar's keys sit in
	bay := paint2d.XYWH(111, 1, 101, 23)
	cobaltShelf(ctx, bay, 11, 0x3a1)
	// the display, and the readout strip under it
	cobaltScreen(ctx, paint2d.XYWH(19, 26, w-26, h-50), 4)
	strip := paint2d.XYWH(58, h-21, w-64, 18)
	round(ctx, strip, 5, paint2d.Fill(cEdge))
	round(ctx, strip.Inset(1), 4, paint2d.Fill(rgb("#050a16")))
	// the left rail's grooves
	for _, x := range []float32{6, 10} {
		fill(ctx, paint2d.XYWH(x, 40, 1, h-80), cEdge.WithAlpha(.7))
		fill(ctx, paint2d.XYWH(x+1, 40, 1, h-80), cBlueHi.WithAlpha(.5))
	}
}

// The transport shelf is sliced left 274, right 59.
func cobaltTransport(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	p := paint2d.NewPath()
	p.AddRoundRectCorners(r, 3, 3, 16, 16)
	ctx.DrawPath(p, paint2d.Fill(cEdge))
	in := r.Inset(1)
	q := paint2d.NewPath()
	q.AddRoundRectCorners(in, 2, 2, 15, 15)
	ctx.DrawPath(q, vpaint(in, stop(0, cSteelHi), stop(.55, cSteel), stop(1, cSteelLo)))
	ctx.Save()
	ctx.ClipPath(q)
	streaks(ctx, in, .05, 0x77e)
	// the seek row's darker lane
	lane := paint2d.XYWH(0, 0, w, 16)
	ctx.DrawRect(lane, vpaint(lane, stop(0, cBlueLo), stop(1, cBlue)))
	fill(ctx, paint2d.XYWH(0, 16, w, 1), cEdge)
	fill(ctx, paint2d.XYWH(0, 17, w, 1), paint2d.RGBA(1, 1, 1, .7))
	ctx.Restore()
	// the play key's socket
	ctx.DrawCircle(paint2d.Pt(23.5, 35), 18.5, paint2d.Fill(cSteelLo))
	ctx.DrawCircle(paint2d.Pt(23.5, 35), 18.5, paint2d.StrokePaint(cEdge.WithAlpha(.6), 1))
	// the small keys' tray
	tray := paint2d.XYWH(40, 23, 106, 25)
	round(ctx, tray, 12.5, paint2d.Fill(cSteelLo.WithAlpha(.7)))
	ring(ctx, tray, 12.5, 1, cEdge.WithAlpha(.35))
}

// The equaliser drawer is sliced only at its sides.
func cobaltEqualizer(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	p := paint2d.NewPath()
	p.AddRoundRectCorners(r, 2, 2, 10, 10)
	ctx.DrawPath(p, paint2d.Fill(cEdge))
	in := r.Inset(1)
	q := paint2d.NewPath()
	q.AddRoundRectCorners(in, 1, 1, 9, 9)
	ctx.DrawPath(q, vpaint(in, stop(0, cSteelHi), stop(1, cSteelLo)))
	ctx.Save()
	ctx.ClipPath(q)
	streaks(ctx, in, .045, 0x1e5)
	area := paint2d.XYWH(2, 31, w-5, 71)
	ctx.DrawRect(area, vpaint(area, stop(0, rgb("#aebbd1")), stop(1, rgb("#c7d1e1"))))
	fill(ctx, paint2d.XYWH(2, 31, w-5, 1), cSteelLo)
	fill(ctx, paint2d.XYWH(2, 102, w-5, 1), cSteelHi)
	fill(ctx, paint2d.XYWH(2, 66, w-5, 1), cBlue.WithAlpha(.25))
	ctx.Restore()
}

// The playlist drawer is sliced top 107, right 11, bottom 129, left 26.
func cobaltPlaylist(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	cobaltBody(ctx, r, 12)
	head := paint2d.XYWH(58, 6, w-89, 22)
	round(ctx, head, 6, paint2d.Fill(cEdge.WithAlpha(.5)))
	cobaltScreen(ctx, paint2d.XYWH(26, 34, w-37, h-63), 3)
	for _, x := range []float32{6, 10} {
		fill(ctx, paint2d.XYWH(x, 20, 1, 50), cEdge.WithAlpha(.7))
		fill(ctx, paint2d.XYWH(x+1, 20, 1, 50), cBlueHi.WithAlpha(.5))
		fill(ctx, paint2d.XYWH(x, h-100, 1, 80), cEdge.WithAlpha(.7))
		fill(ctx, paint2d.XYWH(x+1, h-100, 1, 80), cBlueHi.WithAlpha(.5))
	}
}

func cobaltCompact(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	cobaltBody(ctx, r, 14)
	ctx.Save()
	ctx.ClipRoundRect(r.Inset(1), 13, 13)
	bar := paint2d.XYWH(0, 0, w, 24)
	ctx.DrawRect(bar, vpaint(bar, stop(0, paint2d.RGBA(1, 1, 1, .22)), stop(1, paint2d.RGBA(1, 1, 1, .04))))
	shelf := paint2d.XYWH(0, 60, w, h-60)
	ctx.DrawRect(shelf, vpaint(shelf, stop(0, cSteelHi), stop(.5, cSteel), stop(1, cSteelLo)))
	streaks(ctx, shelf, .05, 0x5c3)
	fill(ctx, paint2d.XYWH(0, 60, w, 1), cEdge)
	ctx.Restore()
	ring(ctx, r.Inset(1), 13, 1, cBlueHi.WithAlpha(.4))
	emblem(ctx, paint2d.XYWH(5, 6, 12, 12), cSteel, cBlueLo)
	cobaltScreen(ctx, paint2d.XYWH(60, 26, 278, 18), 4)
	ctx.DrawCircle(paint2d.Pt(35.5, 78), 18.5, paint2d.Fill(cSteelLo))
	ctx.DrawCircle(paint2d.Pt(35.5, 78), 18.5, paint2d.StrokePaint(cEdge.WithAlpha(.6), 1))
	tray := paint2d.XYWH(52, 66, 106, 25)
	round(ctx, tray, 12.5, paint2d.Fill(cSteelLo.WithAlpha(.7)))
	ring(ctx, tray, 12.5, 1, cEdge.WithAlpha(.35))
}

// ---- sliders ------------------------------------------------------------------------

func cobaltGroove(ctx *paint2d.Context, w, h float32, vertical, disabled, centre bool) {
	var r paint2d.Rect
	if vertical {
		r = paint2d.XYWH(w/2-3, 1, 6, h-2)
	} else {
		th := min(h-2, float32(7))
		r = paint2d.XYWH(1, (h-th)/2, w-2, th)
	}
	rad := min(r.Dx(), r.Dy()) / 2
	round(ctx, r.Inset(-1), rad+1, paint2d.Fill(cSteelHi.WithAlpha(.8)))
	round(ctx, r, rad, paint2d.Fill(cEdge))
	in := r.Inset(1)
	c := rgb("#0b1730")
	if disabled {
		c = cSteelLo
	}
	round(ctx, in, rad-1, paint2d.Fill(c))
	if centre {
		fill(ctx, paint2d.XYWH(w/2-.5, 1, 1, h-2), cBlueLo)
	}
}

func cobaltFill(ctx *paint2d.Context, w, h float32) {
	th := min(h-2, float32(7))
	r := paint2d.XYWH(2, (h-th)/2+1, w-4, th-2)
	round(ctx, r, r.Dy()/2, hpaint(r, stop(0, cCyan), stop(1, cGreen)))
	fill(ctx, paint2d.XYWH(r.Min.X+2, r.Min.Y+.5, r.Dx()-4, 1), paint2d.RGBA(1, 1, 1, .45))
}

func cobaltThumb(ctx *paint2d.Context, u use, w, h float32) {
	top, bottom := cSteelHi, cSteelLo
	switch {
	case strings.HasSuffix(u.name, ".disabled"):
		top, bottom = cSteel, cSteel
	case u.down():
		top, bottom = cSteelLo, cSteelHi
	case u.hover():
		top, bottom = rgb("#ffffff"), cSteel
	}
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	pillow(ctx, r, min(w, h)/2-1, top, bottom, cEdge, .5)
	mark := cBlue
	if u.hover() {
		mark = cGreen
	}
	if w > h {
		fill(ctx, paint2d.XYWH(w/2-.5, 3, 1, h-6), mark)
		fill(ctx, paint2d.XYWH(w/2-3.5, 4, 1, h-8), mark.WithAlpha(.6))
		fill(ctx, paint2d.XYWH(w/2+2.5, 4, 1, h-8), mark.WithAlpha(.6))
	} else {
		fill(ctx, paint2d.XYWH(3, h/2-.5, w-6, 1), mark)
	}
}

// ---- keys -----------------------------------------------------------------------------

func cobaltKey(ctx *paint2d.Context, u use, w, h float32) {
	key := gaugeKeyName(u.name)
	disabled := strings.HasSuffix(u.name, ".disabled")
	lit := u.on() || key == "eq" && strings.Contains(u.name, "eq.on")
	glyph := map[string]string{
		"compact.open": "open", "eq.on": "eq", "eq.preset": "presets", "eq.reset": "repeat",
		"eq.close": "close", "pl.close": "close", "pl": "playlist", "skin": "skin", "compact": "compact",
	}[key]
	if glyph == "" {
		glyph = key
	}
	transport := false
	switch key {
	case "play", "pause", "stop", "prev", "next", "mute", "rew", "ffwd":
		transport = true
	}
	top, bottom, edge, ink := cSteelHi, cSteelLo, cEdge, cInk
	if transport {
		top, bottom, ink = cBlueHi, cBlueLo, rgb("#ffffff")
	}
	if u.hover() {
		top, bottom = lift(top, .2), lift(bottom, .15)
	}
	if u.down() {
		top, bottom = sink(bottom, .12), top
	}
	if lit {
		ink = rgb("#1f9a45")
		if transport {
			ink = cGreen
		}
	}
	if disabled {
		top, bottom, ink = mix(top, cSteel, .6), mix(bottom, cSteel, .6), mix(ink, cSteel, .55)
	}
	d := float32(0)
	if u.down() {
		d = .6
	}
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	switch {
	case key == "minimize" || key == "close" && w < 18 && h > w:
		pillow(ctx, r, 4, top, bottom, edge, .45)
		symbol(ctx, glyph, paint2d.XYWH(d, d, w, h), w*.5, ink)
		return
	case w > h*1.8: // rew, ffwd: lozenges on the seek lane
		pillow(ctx, r, h/2-.5, top, bottom, edge, .45)
		symbol(ctx, glyph, paint2d.XYWH(d, d, w, h), h*.55, ink)
		return
	}
	c := paint2d.Pt(w/2, h/2)
	rad := min(w, h)/2 - .5
	orb(ctx, c, rad, top, bottom, edge, .55)
	if lit {
		ctx.DrawCircle(c, rad-1.5, paint2d.StrokePaint(cGreen.WithAlpha(.7), 1.2))
	}
	s := rad * 1.05
	if key == "play" || key == "pause" {
		s = rad * .9
	}
	symbol(ctx, glyph, paint2d.XYWH(d, d, w, h), s, ink)
}
