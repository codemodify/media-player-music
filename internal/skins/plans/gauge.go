package plans

import (
	"math"
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// Gauge is a round instrument housing in gunmetal with a copper bezel: a
// glass dial whose left and right arcs are the seek and volume scales, a
// cluster of round transport keys on a pod at its shoulder, an iris shutter
// that closes over the dial in two steps, a light steel equaliser bay and a
// framed playlist. Drawn from circles, arcs and gradients only.
func Gauge() *Plan {
	return build(skinArt{id: "gauge", label: "Gauge", g: &geomGauge,
		summary: "A round gunmetal instrument with a copper bezel: arc scales for seek and volume, a key cluster, an iris shutter, a steel equaliser bay and a framed playlist.",
		paint:   gaugeArt,
		text: map[string]skingen.TextRole{
			"player.text": {Color: "#FFB35C", Bold: true}, "player.info": {Color: "#FFB35C", Bold: true},
			"player.row": {Color: "#F0A35A"}, "player.selected": {Color: "#C4702C"}, "player.selectedText": {Color: "#FFFFFF"},
		},
		colors: map[string]string{"focus": "#FFB35C"}})
}

var (
	gSteelHi  = rgb("#a3aab2")
	gSteel    = rgb("#626970")
	gSteelLo  = rgb("#2c3136")
	gEdge     = rgb("#0e1012")
	gCopperHi = rgb("#f2a55c")
	gCopper   = rgb("#c4702c")
	gCopperLo = rgb("#6a3410")
	gDial     = rgb("#0f0b08")
	gAmber    = rgb("#ffb35c")
	gPanelHi  = rgb("#e4e7ea")
	gPanel    = rgb("#c3c8cd")
	gPanelLo  = rgb("#9aa1a8")
)

// The dial's centre and the two arc scales, in the main layout's pixels.
var gaugeCentre = paint2d.Pt(151, 148)

type gaugeArc struct {
	origin    paint2d.Point // the slot's top left
	r, a0, a1 float32       // radius; the angles (degrees, clockwise) of 0 and 1
	frames    int
}

var gaugeArcs = map[string]gaugeArc{
	"seek":   {paint2d.Pt(67, 127), 77, 105, 175, 33},
	"volume": {paint2d.Pt(175, 96), 77, 60, -35, 27},
}

func (a gaugeArc) centre() paint2d.Point {
	return paint2d.Pt(gaugeCentre.X-a.origin.X, gaugeCentre.Y-a.origin.Y)
}

func (a gaugeArc) angle(t float32) float32 { return (a.a0 + (a.a1-a.a0)*t) * math.Pi / 180 }

func (a gaugeArc) at(t, r float32) paint2d.Point {
	th := float64(a.angle(t))
	c := a.centre()
	return paint2d.Pt(c.X+r*float32(math.Cos(th)), c.Y+r*float32(math.Sin(th)))
}

// band is the annular sector of the arc from t0 to t1, half w either side
// of its radius.
func (a gaugeArc) band(t0, t1, w float32) *paint2d.Path {
	return annulus(a.centre(), a.r-w, a.r+w, a.angle(t0), a.angle(t1))
}

func annulus(c paint2d.Point, r0, r1, a0, a1 float32) *paint2d.Path {
	p := paint2d.NewPath()
	p.AddArc(c, r1, r1, a0, a1-a0)
	p.AddArc(c, r0, r0, a1, a0-a1)
	p.Close()
	return p
}

func gaugeArt(u use) drawFunc {
	name := u.name
	switch {
	case name == "face.main.open.base" || name == "face.main.open":
		return func(ctx *paint2d.Context, w, h float32) { gaugeMain(ctx, 0) }
	case name == "face.main.inner":
		return func(ctx *paint2d.Context, w, h float32) { gaugeMain(ctx, 1) }
	case name == "face.main.closed":
		return func(ctx *paint2d.Context, w, h float32) { gaugeMain(ctx, 2) }
	case name == "shape.main":
		return func(ctx *paint2d.Context, w, h float32) { gaugeSilhouette(ctx, true, paint2d.Fill(gSteel)) }
	case name == "glass.foreground":
		return gaugeGlass
	case name == "face.equalizer":
		return gaugeEqualizer
	case name == "face.playlist":
		return func(ctx *paint2d.Context, w, h float32) { gaugePlaylist(ctx, u, w, h) }
	case strings.HasPrefix(name, "key.logo"):
		return func(ctx *paint2d.Context, w, h float32) { gaugeLogo(ctx, w, h, u.hover(), u.down()) }
	}
	for slot, a := range gaugeArcs {
		rest, ok := strings.CutPrefix(name, slot+".")
		if !ok {
			continue
		}
		a := a
		switch rest {
		case "map":
			return func(ctx *paint2d.Context, w, h float32) { gaugeMap(ctx, a) }
		case "shape":
			return func(ctx *paint2d.Context, w, h float32) {
				ctx.DrawPath(a.band(-.04, 1.04, 9), paint2d.Fill(gSteel))
			}
		case "track":
			return func(ctx *paint2d.Context, w, h float32) { gaugeScale(ctx, a, -1) }
		default:
			i, n := u.frame(&geomGauge)
			if i < 0 {
				break
			}
			return func(ctx *paint2d.Context, w, h float32) { gaugeScale(ctx, a, float32(i)/float32(max(n-1, 1))) }
		}
	}
	switch {
	case name == "band.track":
		return func(ctx *paint2d.Context, w, h float32) { gaugeChannel(ctx, w, h, true, false) }
	case name == "band.foreground":
		return func(ctx *paint2d.Context, w, h float32) { gaugeChannel(ctx, w, h, true, true) }
	case strings.HasSuffix(name, ".track"):
		return func(ctx *paint2d.Context, w, h float32) { gaugeChannel(ctx, w, h, false, false) }
	case strings.HasSuffix(name, ".foreground"):
		return func(ctx *paint2d.Context, w, h float32) { gaugeChannel(ctx, w, h, false, true) }
	case name == "eq.thumb" || name == "horizontal.thumb":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			pillow(ctx, r, 3, gSteelHi, gSteel, gEdge, .4)
			fill(ctx, paint2d.XYWH(2, h/2-.5, w-4, 1), gCopper)
			if w > h {
				fill(ctx, paint2d.XYWH(w/2-.5, 2, 1, h-4), gCopper)
			}
		}
	case strings.HasPrefix(name, "key."):
		return func(ctx *paint2d.Context, w, h float32) { gaugeKey(ctx, u, w, h) }
	}
	return func(ctx *paint2d.Context, w, h float32) {
		round(ctx, paint2d.XYWH(0, 0, w, h), 3, paint2d.Fill(gSteel))
	}
}

// ---- the main housing -----------------------------------------------------------

// gaugeSilhouette fills the housing: the dial, the key pod and its neck,
// the right arm, the bottom stem and the mute boss; with lid, the shutter
// visor that rises over the top when the iris closes.
func gaugeSilhouette(ctx *paint2d.Context, lid bool, p paint2d.Paint) {
	c := gaugeCentre
	ctx.DrawCircle(c, 113, p)
	ctx.DrawCircle(paint2d.Pt(272, 64), 43, p)
	neck := paint2d.NewPath()
	neck.MoveTo(196, 70)
	neck.LineTo(244, 28)
	neck.LineTo(300, 96)
	neck.LineTo(236, 140)
	neck.Close()
	ctx.DrawPath(neck, p)
	round(ctx, paint2d.XYWH(238, 115, 69, 38), 12, p)
	round(ctx, paint2d.XYWH(113, 228, 38, 71), 9, p)
	ctx.DrawCircle(paint2d.Pt(50, 199), 16, p)
	if lid {
		round(ctx, paint2d.XYWH(146, 0, 112, 60), 16, p)
	}
}

func gaugeMain(ctx *paint2d.Context, shutter int) {
	c := gaugeCentre
	all := paint2d.XYWH(0, 0, 339, 300)
	// the outline, then the metal a pixel inside it
	gaugeSilhouette(ctx, shutter != 0, paint2d.Fill(gEdge))
	ctx.Save()
	ctx.Translate(c.X, c.Y)
	ctx.Scale(.993, .993)
	ctx.Translate(-c.X, -c.Y)
	gaugeSilhouette(ctx, shutter != 0, vpaint(all, stop(0, gSteelHi), stop(.45, gSteel), stop(1, gSteelLo)))
	ctx.Restore()
	streaksIn(ctx, all)
	if shutter != 0 {
		// the visor: a copper-trimmed plate with louvres
		v := paint2d.XYWH(150, 4, 104, 50)
		round(ctx, v, 13, vpaint(v, stop(0, gSteel), stop(1, gSteelLo)))
		ring(ctx, v, 13, 2, gCopper)
		for y := float32(14); y < 46; y += 6 {
			round(ctx, paint2d.XYWH(166, y, 72, 3), 1.5, paint2d.Fill(gEdge.WithAlpha(.75)))
			fill(ctx, paint2d.XYWH(167, y+3, 70, 1), gSteelHi.WithAlpha(.5))
		}
	}
	// the pod's collar and the arm, stem and boss rims
	ctx.DrawCircle(paint2d.Pt(272, 64), 39, paint2d.StrokePaint(gCopperLo, 2))
	ring(ctx, paint2d.XYWH(240, 117, 65, 34), 10, 1.5, gCopperLo)
	ring(ctx, paint2d.XYWH(115, 230, 34, 67), 8, 1.5, gCopperLo)
	ctx.DrawCircle(paint2d.Pt(50, 199), 13, paint2d.StrokePaint(gCopperLo, 1.5))
	// the bezel: a copper ring with ticks, then the dial's well
	ctx.DrawCircle(c, 108, paint2d.Fill(gEdge))
	bez := paint2d.XYWH(c.X-107, c.Y-107, 214, 214)
	ctx.DrawOval(bez, vpaint(bez, stop(0, gCopperHi), stop(.5, gCopper), stop(1, gCopperLo)))
	ctx.DrawCircle(c, 97, paint2d.Fill(gEdge))
	for i := 0; i < 60; i++ {
		th := float64(i) * math.Pi / 30
		r0, r1 := float32(99), float32(102)
		if i%5 == 0 {
			r0 = 98.5
			r1 = 105
		}
		q := paint2d.NewPath()
		q.MoveTo(c.X+r0*float32(math.Cos(th)), c.Y+r0*float32(math.Sin(th)))
		q.LineTo(c.X+r1*float32(math.Cos(th)), c.Y+r1*float32(math.Sin(th)))
		ctx.DrawPath(q, paint2d.StrokePaint(gCopperLo, 1))
	}
	dial := paint2d.XYWH(c.X-96, c.Y-96, 192, 192)
	switch shutter {
	case 0:
		ctx.DrawOval(dial, paint2d.Radial(paint2d.RadialGradient{Center: paint2d.Pt(c.X, c.Y-20), Radius: 110,
			Stops: []paint2d.GradientStop{stop(0, rgb("#2a1e14")), stop(.7, gDial), stop(1, rgb("#050403"))}}))
		// the scales' printed ticks
		for _, a := range gaugeArcs {
			for k := 0; k <= 10; k++ {
				t := float32(k) / 10
				th := float64(a.angle(t))
				r0, r1 := a.r+9, a.r+12
				if k%5 == 0 {
					r1 = a.r + 14
				}
				q := paint2d.NewPath()
				q.MoveTo(c.X+r0*float32(math.Cos(th)), c.Y+r0*float32(math.Sin(th)))
				q.LineTo(c.X+r1*float32(math.Cos(th)), c.Y+r1*float32(math.Sin(th)))
				ctx.DrawPath(q, paint2d.StrokePaint(gAmber.WithAlpha(.55), 1))
			}
		}
		// the readout's plate under the title and times
		plate := paint2d.XYWH(78, 78, 150, 50)
		round(ctx, plate, 8, paint2d.Fill(rgb("#000000").WithAlpha(.45)))
		ring(ctx, plate, 8, 1, gCopperLo.WithAlpha(.8))
		// the open key's socket
		ctx.DrawCircle(paint2d.Pt(151, 214), 20, paint2d.Fill(gEdge))
	default:
		gaugeIris(ctx, c, 96, map[int]float32{1: 30, 2: 0}[shutter])
	}
	// the pod's face
	pod := paint2d.XYWH(272-37, 64-37, 74, 74)
	ctx.DrawOval(pod, vpaint(pod, stop(0, gSteelLo), stop(1, rgb("#1a1d20"))))
}

// streaksIn brushes the housing's metal with fine horizontal lines.
func streaksIn(ctx *paint2d.Context, r paint2d.Rect) {
	ctx.Save()
	ctx.ClipPath(gaugeOutline())
	streaks(ctx, r, .07, 0x9e37)
	ctx.Restore()
}

// gaugeOutline is the housing as a path, for clipping (without the visor).
func gaugeOutline() *paint2d.Path {
	p := paint2d.NewPath()
	p.AddCircle(gaugeCentre, 112)
	p.AddCircle(paint2d.Pt(272, 64), 42)
	p.AddRoundRect(paint2d.XYWH(239, 116, 67, 36), 11, 11)
	p.AddRoundRect(paint2d.XYWH(114, 229, 36, 69), 8, 8)
	p.AddCircle(paint2d.Pt(50, 199), 15)
	return p
}

// gaugeIris is the shutter over the dial: six steel blades leaving an
// aperture of radius a (none when shut).
func gaugeIris(ctx *paint2d.Context, c paint2d.Point, r, a float32) {
	disc := paint2d.XYWH(c.X-r, c.Y-r, 2*r, 2*r)
	ctx.DrawOval(disc, vpaint(disc, stop(0, rgb("#7d848b")), stop(1, rgb("#3a4046"))))
	for i := 0; i < 6; i++ {
		th0 := float64(i) * math.Pi / 3
		q := paint2d.NewPath()
		q.MoveTo(c.X+a*float32(math.Cos(th0)), c.Y+a*float32(math.Sin(th0)))
		for k := 1; k <= 8; k++ {
			f := float64(k) / 8
			rr := a + (r-a)*float32(f)
			th := th0 + f*1.1
			q.LineTo(c.X+rr*float32(math.Cos(th)), c.Y+rr*float32(math.Sin(th)))
		}
		ctx.DrawPath(q, paint2d.StrokePaint(gEdge.WithAlpha(.8), 1.5))
		// the lit edge of the next blade
		q2 := paint2d.NewPath()
		q2.MoveTo(c.X+a*float32(math.Cos(th0))+1, c.Y+a*float32(math.Sin(th0))+1)
		for k := 1; k <= 8; k++ {
			f := float64(k) / 8
			rr := a + (r-a)*float32(f)
			th := th0 + f*1.1
			q2.LineTo(c.X+rr*float32(math.Cos(th))+1, c.Y+rr*float32(math.Sin(th))+1)
		}
		ctx.DrawPath(q2, paint2d.StrokePaint(gSteelHi.WithAlpha(.5), 1))
	}
	if a > 0 {
		ctx.DrawCircle(c, a, paint2d.Fill(gDial))
		ctx.DrawCircle(c, a, paint2d.StrokePaint(gCopperLo, 1.5))
	} else {
		ctx.DrawCircle(c, 9, paint2d.Fill(gSteelLo))
		ctx.DrawCircle(c, 9, paint2d.StrokePaint(gCopper, 1.5))
	}
}

func gaugeGlass(ctx *paint2d.Context, w, h float32) {
	c := paint2d.Pt(w/2, h/2)
	ctx.DrawCircle(c, w/2-1, paint2d.StrokePaint(paint2d.RGBA(1, 1, 1, .18), 1.5))
	// a crescent of reflected light at the upper left
	q := paint2d.NewPath()
	q.AddArc(c, w/2-5, w/2-5, math.Pi*1.05, math.Pi*.55)
	q.AddArc(paint2d.Pt(c.X+10, c.Y+12), w/2-8, w/2-8, math.Pi*1.6, -math.Pi*.55)
	q.Close()
	ctx.DrawPath(q, paint2d.Fill(paint2d.RGBA(1, 1, 1, .1)))
	ctx.DrawCircle(paint2d.Pt(c.X-w*.28, c.Y-h*.3), 3, paint2d.Fill(paint2d.RGBA(1, 1, 1, .22)))
}

func gaugeLogo(ctx *paint2d.Context, w, h float32, hover, down bool) {
	r := paint2d.XYWH(0, 0, w, h)
	round(ctx, r, 14, paint2d.Fill(rgb("#000000").WithAlpha(.55)))
	in := r.Inset(4)
	round(ctx, in, 11, vpaint(in, stop(0, rgb("#1e1610")), stop(1, rgb("#0b0806"))))
	ring(ctx, in, 11, 1, gCopperLo)
	c := r.Center()
	if down {
		c = paint2d.Pt(c.X+1, c.Y+1)
	}
	ink := gCopper
	if hover {
		ink = gCopperHi
	}
	// concentric scale marks round the mark
	for i := 0; i < 24; i++ {
		th := float64(i) * math.Pi / 12
		q := paint2d.NewPath()
		q.MoveTo(c.X+30*float32(math.Cos(th)), c.Y+30*float32(math.Sin(th)))
		q.LineTo(c.X+35*float32(math.Cos(th)), c.Y+35*float32(math.Sin(th)))
		ctx.DrawPath(q, paint2d.StrokePaint(ink.WithAlpha(.6), 1))
	}
	emblem(ctx, paint2d.XYWH(c.X-24, c.Y-24, 48, 48), ink, rgb("#140e09"))
}

func gaugeMap(ctx *paint2d.Context, a gaugeArc) {
	const n = 256
	for i := -4; i < n+4; i++ {
		t0, t1 := float32(i)/n, float32(i+1)/n
		v := min(max((t0+t1)/2, 0), 1)
		ctx.DrawPath(a.band(t0-.002, t1+.002, 7), paint2d.Fill(paint2d.RGBA(v, v, v, 1)))
	}
}

// gaugeScale is an arc scale: the groove, lit in amber up to t, and its
// pointer at t (none for t < 0).
func gaugeScale(ctx *paint2d.Context, a gaugeArc, t float32) {
	ctx.DrawPath(a.band(0, 1, 6.5), paint2d.Fill(gEdge))
	ctx.DrawPath(a.band(0, 1, 5), paint2d.Fill(rgb("#24190f")))
	ctx.DrawPath(a.band(0, 1, 1.2), paint2d.Fill(gCopperLo.WithAlpha(.8)))
	if t < 0 {
		return
	}
	if t > 0 {
		ctx.DrawPath(a.band(0, t, 3.8), paint2d.Fill(gCopper))
		ctx.DrawPath(a.band(0, t, 1.6), paint2d.Fill(gAmber))
	}
	p := a.at(t, a.r)
	ctx.DrawCircle(p, 6.5, paint2d.Fill(gEdge))
	orb(ctx, p, 5.5, gCopperHi, gCopperLo, gEdge, .5)
}

// gaugeChannel is a straight slider's channel, or its amber fill.
func gaugeChannel(ctx *paint2d.Context, w, h float32, vertical, lit bool) {
	var r paint2d.Rect
	if vertical {
		r = paint2d.XYWH(w/2-2.5, 0, 5, h)
	} else {
		r = paint2d.XYWH(0, h/2-2.5, w, 5)
	}
	if lit {
		in := r.Inset(1)
		p := vpaint(in, stop(0, gCopperHi), stop(1, gCopper))
		if vertical {
			p = hpaint(in, stop(0, gCopperHi), stop(1, gCopper))
		}
		round(ctx, in, 1.5, p)
		return
	}
	round(ctx, r, 2.5, paint2d.Fill(gSteelHi))
	round(ctx, paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx()-.5, r.Dy()-.5), 2.5, paint2d.Fill(gEdge))
	round(ctx, r.Inset(1), 1.5, paint2d.Fill(rgb("#1c1712")))
}

// ---- keys -------------------------------------------------------------------------

func gaugeKey(ctx *paint2d.Context, u use, w, h float32) {
	g := glyphFor(u.name)
	base := gaugeKeyName(u.name)
	lit := u.on()
	top, bottom, ink := gSteelHi, gSteel, rgb("#1a1d20")
	big := false
	switch base {
	case "play", "pause", "stop", "prev", "next", "open":
		top, bottom, ink = gCopperHi, gCopper, rgb("#2a1405")
		big = true
	}
	if u.hover() {
		top, bottom = lift(top, .18), lift(bottom, .12)
	}
	if u.down() {
		top, bottom = sink(bottom, .1), top
	}
	if lit {
		ink = gCopperHi
		if big {
			ink = rgb("#fff2d8")
		}
	}
	d := float32(0)
	if u.down() {
		d = .7
	}
	cx, cy := w/2, h/2
	switch base {
	case "play", "pause":
		cx, cy = 27.5, 27
	}
	var r float32
	switch base {
	case "play", "pause":
		r = 17
	case "next", "stop", "prev":
		r = 12.5
	case "open":
		r = 16.5
	}
	if r > 0 {
		ctx.DrawCircle(paint2d.Pt(cx, cy), r+1.5, paint2d.Fill(gEdge))
		orb(ctx, paint2d.Pt(cx, cy), r, top, bottom, gCopperLo, .45)
		symbol(ctx, g, paint2d.XYWH(cx-r+d, cy-r+d, 2*r, 2*r), r*.85, ink)
		return
	}
	box := paint2d.XYWH(.5, .5, w-1, h-1)
	switch base {
	case "minimize", "close", "skin":
		// the arm's keys: tall rounded tabs
	case "mute":
		ctx.DrawCircle(paint2d.Pt(cx, cy), w/2-.5, paint2d.Fill(gEdge))
		orb(ctx, paint2d.Pt(cx, cy), w/2-1.5, top, bottom, gEdge, .4)
		if lit {
			ink = gCopper
		}
		symbol(ctx, "mute", paint2d.XYWH(d, d, w, h), 9, ink)
		return
	}
	pillow(ctx, box, 4, top, bottom, gEdge, .35)
	if lit {
		fill(ctx, paint2d.XYWH(3, h-4, w-6, 1.5), gCopperHi)
	}
	s := min(w, h) * .5
	r2 := paint2d.XYWH(d, d, w, h)
	switch base {
	case "on":
		gaugePower(ctx, r2, s, ink)
	case "reset":
		symbol(ctx, "repeat", r2, s, ink)
	case "crossfade":
		q := paint2d.NewPath()
		q.MoveTo(w/2-s*.8+d, h/2+s*.45+d)
		q.LineTo(w/2+s*.8+d, h/2-s*.45+d)
		q.MoveTo(w/2-s*.8+d, h/2-s*.45+d)
		q.LineTo(w/2+s*.8+d, h/2+s*.45+d)
		ctx.DrawPath(q, paint2d.StrokePaint(ink, 1.6))
	case "srs":
		for _, k := range []float32{.3, .6, .9} {
			q := paint2d.NewPath()
			q.AddArc(paint2d.Pt(w/2-s*.6+d, h/2+d), s*k, s*k, -.8, 1.6)
			ctx.DrawPath(q, paint2d.StrokePaint(ink, 1.2))
		}
	case "speaker":
		symbol(ctx, "speaker", r2, s, ink)
	case "shutter":
		ctx.DrawCircle(paint2d.Pt(w/2+d, h/2+d), s*.6, paint2d.StrokePaint(ink, 1.3))
		for i := 0; i < 3; i++ {
			th := float64(i) * 2 * math.Pi / 3
			q := paint2d.NewPath()
			q.MoveTo(w/2+d, h/2+d)
			q.LineTo(w/2+d+s*.6*float32(math.Cos(th)), h/2+d+s*.6*float32(math.Sin(th)))
			ctx.DrawPath(q, paint2d.StrokePaint(ink, 1))
		}
	case "vis":
		symbol(ctx, "vis", r2, s, ink)
	case "skin":
		symbol(ctx, "menu", r2, s*.9, ink)
	default:
		if g == "" {
			g = base
		}
		symbol(ctx, g, r2, s, ink)
	}
}

func gaugePower(ctx *paint2d.Context, r paint2d.Rect, s float32, ink paint2d.Color) {
	c := r.Center()
	q := paint2d.NewPath()
	q.AddArc(c, s*.55, s*.55, -math.Pi/2+.6, 2*math.Pi-1.2)
	ctx.DrawPath(q, paint2d.StrokePaint(ink, 1.5))
	fill(ctx, paint2d.XYWH(c.X-.75, c.Y-s*.7, 1.5, s*.6), ink)
}

// ---- the equaliser and the playlist ---------------------------------------------

func gaugeEqualizer(ctx *paint2d.Context, w, h float32) {
	all := paint2d.XYWH(0, 0, w, h)
	round(ctx, all, 18, paint2d.Fill(gEdge))
	in := all.Inset(1.5)
	round(ctx, in, 16.5, vpaint(in, stop(0, gPanelHi), stop(1, gPanelLo)))
	ctx.Save()
	ctx.ClipRoundRect(in, 16.5, 16.5)
	streaks(ctx, in, .05, 0x4411)
	// the copper band along the top, with the keys' bay at its right
	band := paint2d.XYWH(0, 0, w, 34)
	ctx.DrawRect(band, vpaint(band, stop(0, gCopperHi), stop(.6, gCopper), stop(1, gCopperLo)))
	fill(ctx, paint2d.XYWH(0, 34, w, 1.5), gEdge)
	ctx.Restore()
	bay := paint2d.XYWH(w-84, 2, 80, 29)
	round(ctx, bay, 8, paint2d.Fill(gSteelLo))
	ring(ctx, bay, 8, 1, gEdge)
	for x := float32(30); x < w-100; x += 7 {
		round(ctx, paint2d.XYWH(x, 13, 3, 10), 1.5, paint2d.Fill(gCopperLo.WithAlpha(.7)))
	}
	// the band bay: a dark well with a centre line
	well(ctx, paint2d.XYWH(42, 44, 188, 132), 8, rgb("#2a2622"), gSteelLo, gPanelHi)
	fill(ctx, paint2d.XYWH(48, 117, 176, 1), gCopper.WithAlpha(.45))
	for y := float32(70); y < 170; y += 10 {
		fill(ctx, paint2d.XYWH(46, y, 4, 1), gPanelLo)
		fill(ctx, paint2d.XYWH(222, y, 4, 1), gPanelLo)
	}
	// the preset's window
	well(ctx, paint2d.XYWH(79, 47, 113, 17), 4, rgb("#f4f1ea"), gSteelLo, gPanelHi)
	// the right bay's two framed groups and the effects' rail
	for _, r := range []paint2d.Rect{paint2d.XYWH(244, 44, 158, 62), paint2d.XYWH(244, 116, 158, 62)} {
		ring(ctx, r, 7, 1, gPanelLo)
		ring(ctx, r.Inset(1), 6, 1, gPanelHi)
	}
	rail := paint2d.XYWH(38, 178, 360, 22)
	ring(ctx, rail, 7, 1, gPanelLo)
	ring(ctx, rail.Inset(1), 6, 1, gPanelHi)
	for _, p := range []paint2d.Point{{X: 16, Y: 50}, {X: 16, Y: h - 18}, {X: w - 16, Y: 50}, {X: w - 16, Y: h - 18}} {
		ctx.DrawCircle(p, 4, paint2d.Fill(gSteel))
		ctx.DrawCircle(p, 4, paint2d.StrokePaint(gEdge, 1))
		fill(ctx, paint2d.XYWH(p.X-2.5, p.Y-.5, 5, 1), gEdge)
	}
	ring(ctx, in, 16.5, 1, paint2d.RGBA(1, 1, 1, .6))
}

// gaugePlaylist is drawn at its design size and nine-sliced: its corners
// and the two bands hold all detail; the middle row and column repeat.
func gaugePlaylist(ctx *paint2d.Context, u use, w, h float32) {
	all := paint2d.XYWH(0, 0, w, h)
	round(ctx, all, 18, paint2d.Fill(gEdge))
	in := all.Inset(1.5)
	round(ctx, in, 16.5, vpaint(in, stop(0, gSteel), stop(1, gSteelLo)))
	ctx.Save()
	ctx.ClipRoundRect(in, 16.5, 16.5)
	band := paint2d.XYWH(0, 0, w, 34)
	ctx.DrawRect(band, vpaint(band, stop(0, gCopperHi), stop(.6, gCopper), stop(1, gCopperLo)))
	fill(ctx, paint2d.XYWH(0, 34, w, 1.5), gEdge)
	ctx.Restore()
	bay := paint2d.XYWH(w-84, 2, 80, 29)
	round(ctx, bay, 8, paint2d.Fill(gSteelLo))
	ring(ctx, bay, 8, 1, gEdge)
	for x := float32(24); x < 76; x += 7 {
		round(ctx, paint2d.XYWH(x, 13, 3, 10), 1.5, paint2d.Fill(gCopperLo.WithAlpha(.7)))
	}
	// the content's well
	well(ctx, paint2d.XYWH(29, 42, w-58, h-76), 4, rgb("#0b0907"), gEdge, gSteelHi)
	// the grip's ridges at the lower right
	for i := float32(0); i < 4; i++ {
		q := paint2d.NewPath()
		q.MoveTo(w-8-i*5, h-6)
		q.LineTo(w-6, h-8-i*5)
		ctx.DrawPath(q, paint2d.StrokePaint(gSteelHi.WithAlpha(.7), 1.2))
	}
	ring(ctx, in, 16.5, 1, gSteelHi.WithAlpha(.5))
}

// gaugeKeyName is a key sprite's key: "key.eq.on.down" is "eq", "key.on.on" is
// "on", "key.pl.close.hover" is "pl.close".
func gaugeKeyName(name string) string {
	parts := strings.Split(strings.TrimPrefix(name, "key."), ".")
	for len(parts) > 1 {
		switch parts[len(parts)-1] {
		case "down", "hover", "pressed", "disabled", "on", "checked":
			parts = parts[:len(parts)-1]
			continue
		}
		break
	}
	return strings.Join(parts, ".")
}
