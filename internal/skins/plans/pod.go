package plans

import (
	"strings"

	"goframework.io/graphics-paint2d"
)

// Pod is a gunmetal capsule: a raised jewel disc at its left end carrying
// the round mark, a smoked-glass screen at its right with arcs of lime light
// for the seek and volume, a tab of window keys on its crown and a shelf of
// rubber keys along its foot. Its playlist is a separate gunmetal frame
// round a deep well.
func Pod() *Plan {
	p := podPalette()
	return genericSkin("pod", "Win Media Player - Xbox", "A gunmetal capsule with a jewel disc, a smoked-glass screen with arcs of lime light, and a framed playlist.", &geomPod, p, func(u use) drawFunc { return podArt(p, u) }, nil, nil)
}

func podPalette() *palette {
	return &palette{body: rgb("#3a4248"), bodyTop: rgb("#636e76"), bodyBottom: rgb("#1a1e21"), edge: rgb("#040506"), rim: rgb("#a4b0b8"),
		well: rgb("#050a06"), wellDark: rgb("#000000"), wellLight: rgb("#4d5a50"),
		key: rgb("#30373c"), keyTop: rgb("#4a535a"), keyBottom: rgb("#1f2427"), keyEdge: rgb("#050607"),
		ink: rgb("#a4ec5c"), inkHover: rgb("#d8ffb0"), accent: rgb("#8fe04a"), lcd: rgb("#9ef261"), lcdDim: rgb("#2d4a1d"),
		radius: 22, keyRadius: 8, gloss: 0.3, round: true}
}

// The main face's parts, in its design pixels.
var (
	podBody   = paint2d.XYWH(8, 40, 334, 170)
	podTab    = paint2d.XYWH(156, 2, 102, 48)
	podScreen = paint2d.XYWH(120, 42, 186, 112)
	podDisc   = paint2d.Pt(71, 120)
)

const podDiscR = 52

func podArt(p *palette, u use) drawFunc {
	name := u.name
	switch {
	case name == "face.main":
		return func(ctx *paint2d.Context, w, h float32) { podMain(ctx, p) }
	case name == "face.playlist":
		return func(ctx *paint2d.Context, w, h float32) { podPlaylist(ctx, p, w, h) }
	case name == "eq.panel":
		return func(ctx *paint2d.Context, w, h float32) { podEqPanel(ctx, p, w, h) }
	case name == "eq.thumb" || name == "eq.thumb.down" || name == "vertical.thumb" || name == "vertical.thumb.down":
		down := u.down()
		return func(ctx *paint2d.Context, w, h float32) {
			c := p.accent
			if down {
				c = p.inkHover
			}
			round(ctx, paint2d.XYWH(0, 0, w, h), 1.5, paint2d.Fill(sink(c, .45)))
			round(ctx, paint2d.XYWH(.5, .5, w-1, h-1.5), 1.5, paint2d.Fill(c))
		}
	case name == "seek.map" || name == "volume.map":
		vol := name == "volume.map"
		return func(ctx *paint2d.Context, w, h float32) {
			path := podArc(w, h, vol)
			r := paint2d.XYWH(0, 0, w, h)
			ctx.DrawPath(path, paint2d.Paint{Shader: paint2d.LinearGradient{Start: paint2d.Pt(0, 0), End: paint2d.Pt(w, 0),
				Stops: []paint2d.GradientStop{stop(0, paint2d.RGBA(0, 0, 0, 1)), stop(1, paint2d.RGBA(1, 1, 1, 1))}},
				Style: paint2d.StyleStroke, AntiAlias: true, Stroke: podStroke(podMapWidth(vol))})
			_ = r
		}
	case name == "seek.shape" || name == "volume.shape":
		vol := name == "volume.shape"
		return func(ctx *paint2d.Context, w, h float32) {
			ctx.DrawPath(podArc(w, h, vol), podStrokePaint(p.edge, podMapWidth(vol)+4))
		}
	case name == "seek.track" || name == "volume.track":
		vol := name == "volume.track"
		return func(ctx *paint2d.Context, w, h float32) { podSlider(ctx, p, w, h, vol, 0) }
	case strings.HasPrefix(name, "seek.") || strings.HasPrefix(name, "volume."):
		vol := strings.HasPrefix(name, "volume.")
		return func(ctx *paint2d.Context, w, h float32) {
			i, n := u.frame(&geomPod)
			podSlider(ctx, p, w, h, vol, float32(i)/float32(max(n-1, 1)))
		}
	case strings.HasPrefix(name, "key.logo"):
		return func(ctx *paint2d.Context, w, h float32) { podJewel(ctx, p, u, w, h) }
	case strings.HasPrefix(name, "key."):
		return func(ctx *paint2d.Context, w, h float32) { podKey(ctx, p, u, w, h) }
	}
	return nil
}

// ---- the curved sliders ---------------------------------------------------------

func podMapWidth(vol bool) float32 {
	if vol {
		return 9
	}
	return 7
}

// podArc is a slider's centre line: a shallow smile for the seek bar, a
// deeper one for the volume under the screen. It runs left to right, so a
// value read off x along it rises the whole way.
func podArc(w, h float32, vol bool) *paint2d.Path {
	path := paint2d.NewPath()
	if vol {
		path.MoveTo(6, 6)
		path.CubicTo(w*.3, h-4, w*.7, h-4, w-6, 6)
		return path
	}
	path.MoveTo(4, 4.5)
	path.CubicTo(w*.3, h-3.5, w*.7, h-3.5, w-4, 4.5)
	return path
}

func podStroke(width float32) paint2d.Stroke {
	s := paint2d.DefaultStroke()
	s.Width = width
	s.Cap = paint2d.CapRound
	return s
}

func podStrokePaint(c paint2d.Color, width float32) paint2d.Paint {
	return paint2d.Paint{Color: c, Style: paint2d.StyleStroke, AntiAlias: true, Stroke: podStroke(width)}
}

// podSlider is a curved channel lit from its left end to t.
func podSlider(ctx *paint2d.Context, p *palette, w, h float32, vol bool, t float32) {
	arc := podArc(w, h, vol)
	width := podMapWidth(vol) - 2
	ctx.DrawPath(arc, podStrokePaint(rgb("#2a332c").WithAlpha(.9), width+2))
	ctx.DrawPath(arc, podStrokePaint(rgb("#020403"), width))
	if t <= 0 {
		return
	}
	ctx.Save()
	ctx.ClipRect(paint2d.XYWH(0, 0, 2+(w-4)*t, h))
	ctx.DrawPath(arc, podStrokePaint(sink(p.accent, .35), width-1))
	ctx.DrawPath(arc, podStrokePaint(p.accent, width-3))
	ctx.Restore()
}

// ---- the keys -----------------------------------------------------------------

func podKey(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	top, bottom, ink, rim := p.keyTop, p.keyBottom, p.ink, p.accent.WithAlpha(.35)
	switch {
	case u.down():
		top, bottom, ink, rim = rgb("#15191b"), rgb("#2c3337"), p.inkHover, p.accent
	case u.hover():
		top, bottom, ink, rim = lift(p.keyTop, .12), lift(p.keyBottom, .08), p.inkHover, p.accent.WithAlpha(.8)
	}
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	radius := min(w, h) / 2
	base := u.base()
	if base == "key.play" || base == "key.pause" {
		radius = h / 2
	}
	pillow(ctx, r, radius, top, bottom, p.keyEdge, .22)
	ring(ctx, r.Inset(1), radius-1, 1, rim)
	g := glyphFor(u.name)
	switch base {
	case "key.eq.prev", "key.vis.prev":
		g = "back"
	case "key.eq.next", "key.vis.next":
		g = "forward"
	case "key.pl.close":
		g = "close"
	}
	d := float32(0)
	if u.down() {
		d = .5
	}
	s := min(w, h) * .5
	if base == "key.play" || base == "key.pause" {
		s = h * .46
	}
	if g == "back" || g == "forward" {
		// one chevron, not the double skip mark
		cx, cy := w/2+d, h/2+d
		k := min(w, h) * .2
		v := paint2d.NewPath()
		if g == "forward" {
			v.MoveTo(cx-k*.7, cy-k*1.4)
			v.LineTo(cx+k, cy)
			v.LineTo(cx-k*.7, cy+k*1.4)
		} else {
			v.MoveTo(cx+k*.7, cy-k*1.4)
			v.LineTo(cx-k, cy)
			v.LineTo(cx+k*.7, cy+k*1.4)
		}
		v.Close()
		ctx.DrawPath(v, paint2d.Fill(ink))
		return
	}
	symbol(ctx, g, paint2d.XYWH(d, d, w, h), s, ink)
}

// podJewel is the round mark's key: an oval of smoked glass in a lime ring.
func podJewel(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	r := paint2d.XYWH(1, 1, w-2, h-2)
	ctx.DrawOval(r, paint2d.Fill(p.edge))
	in := r.Inset(1.5)
	glow := .55
	if u.hover() {
		glow = .85
	}
	if u.down() {
		glow = 1
	}
	ctx.DrawOval(in, paint2d.Fill(p.accent.WithAlpha(float32(glow))))
	glass := in.Inset(2.5)
	c := glass.Center()
	ctx.DrawOval(glass, paint2d.Radial(paint2d.RadialGradient{Center: c, Focal: c, Radius: glass.Dx() / 2,
		Stops: []paint2d.GradientStop{stop(0, rgb("#2c3a30")), stop(1, rgb("#050806"))}}))
	emblem(ctx, paint2d.XYWH(c.X-15, c.Y-15, 30, 30), p.accent, rgb("#0b120d"))
	g := paint2d.XYWH(glass.Min.X+glass.Dx()*.15, glass.Min.Y+1, glass.Dx()*.7, glass.Dy()*.42)
	ctx.DrawOval(g, vpaint(g, stop(0, paint2d.RGBA(1, 1, 1, .22)), stop(1, paint2d.RGBA(1, 1, 1, 0))))
}

// ---- the faces ----------------------------------------------------------------

func podShell(ctx *paint2d.Context, p *palette, r paint2d.Rect, radius float32) {
	round(ctx, r, radius, paint2d.Fill(p.edge))
	in := r.Inset(1.5)
	round(ctx, in, radius-1.5, vpaint(in, stop(0, p.bodyTop), stop(.35, p.body), stop(1, p.bodyBottom)))
	ring(ctx, r.Inset(2), radius-2, 1, p.rim.WithAlpha(.35))
}

func podMain(ctx *paint2d.Context, p *palette) {
	podShell(ctx, p, podTab, 16)
	podShell(ctx, p, podBody, 62)
	// a lime seam along the capsule's waist
	seam := paint2d.NewPath()
	seam.MoveTo(40, 64)
	seam.CubicTo(120, 52, 230, 52, 330, 70)
	ctx.DrawPath(seam, podStrokePaint(p.accent.WithAlpha(.35), 1))
	// the tab's well for the window keys
	well(ctx, paint2d.XYWH(165, 6, 84, 30), 11, rgb("#14181a"), p.edge, p.rim.WithAlpha(.4))
	// the jewel disc
	ctx.DrawCircle(podDisc, podDiscR+2, paint2d.Fill(p.edge))
	ctx.DrawCircle(podDisc, podDiscR, paint2d.Linear(paint2d.LinearGradient{Start: paint2d.Pt(podDisc.X, podDisc.Y-podDiscR), End: paint2d.Pt(podDisc.X, podDisc.Y+podDiscR),
		Stops: []paint2d.GradientStop{stop(0, rgb("#7b868e")), stop(.5, rgb("#3c454b")), stop(1, rgb("#171b1e"))}}))
	for i := 0; i < 24; i++ {
		a := float32(i) / 24 * 6.2832
		x0, y0 := podDisc.X+(podDiscR-7)*cos32(a), podDisc.Y+(podDiscR-7)*sin32(a)
		x1, y1 := podDisc.X+(podDiscR-2)*cos32(a), podDisc.Y+(podDiscR-2)*sin32(a)
		tick := paint2d.NewPath()
		tick.MoveTo(x0, y0)
		tick.LineTo(x1, y1)
		ctx.DrawPath(tick, podStrokePaint(p.edge.WithAlpha(.55), 1))
	}
	ctx.DrawCircle(podDisc, podDiscR-9, paint2d.StrokePaint(p.accent.WithAlpha(.45), 1))
	// the shelf the transport keys sit in
	shelf := paint2d.NewPath()
	shelf.MoveTo(40, 172)
	shelf.LineTo(268, 172)
	shelf.CubicTo(276, 172, 276, 204, 262, 205)
	shelf.LineTo(60, 206)
	shelf.CubicTo(44, 206, 36, 186, 40, 172)
	shelf.Close()
	ctx.DrawPath(shelf, paint2d.Fill(rgb("#101315")))
	ctx.DrawPath(shelf, paint2d.StrokePaint(p.rim.WithAlpha(.25), 1))
	// the screen
	round(ctx, podScreen.Inset(-3), 25, paint2d.Fill(p.edge))
	round(ctx, podScreen.Inset(-1.5), 23.5, paint2d.Fill(rgb("#5c6a62")))
	round(ctx, podScreen, 22, paint2d.Fill(p.well))
	in := podScreen.Inset(1)
	ctx.Save()
	ctx.ClipRoundRect(in, 21, 21)
	ctx.DrawRect(in, vpaint(in, stop(0, rgb("#0d1a10")), stop(.6, p.well), stop(1, rgb("#0a140c"))))
	grid(ctx, in, 8, p.accent.WithAlpha(.05))
	g := paint2d.NewPath()
	g.MoveTo(in.Min.X, in.Min.Y)
	g.LineTo(in.Min.X+in.Dx()*.55, in.Min.Y)
	g.LineTo(in.Min.X+in.Dx()*.3, in.Min.Y+in.Dy()*.4)
	g.LineTo(in.Min.X, in.Min.Y+in.Dy()*.4)
	g.Close()
	ctx.DrawPath(g, paint2d.Fill(paint2d.RGBA(1, 1, 1, .04)))
	ctx.Restore()
	// screws at the capsule's right end
	for _, c := range []paint2d.Point{{X: 326, Y: 104}, {X: 326, Y: 150}} {
		ctx.DrawCircle(c, 3.5, paint2d.Fill(p.edge))
		ctx.DrawCircle(c, 2.5, paint2d.Fill(rgb("#6a757c")))
		fill(ctx, paint2d.XYWH(c.X-2, c.Y-.5, 4, 1), p.edge)
	}
}

func podEqPanel(ctx *paint2d.Context, p *palette, w, h float32) {
	r := paint2d.XYWH(.5, .5, w-1, h-1)
	round(ctx, r, 10, paint2d.Fill(rgb("#070c08").WithAlpha(.96)))
	ring(ctx, r, 10, 1, p.accent.WithAlpha(.5))
	for i := 0; i < 10; i++ {
		x := []float32{26, 39, 51, 63, 76, 88, 101, 113, 125, 138}[i]
		c := paint2d.XYWH(x+3.5, 10, 3, 42)
		fill(ctx, c, rgb("#000000"))
		fill(ctx, paint2d.XYWH(c.Min.X, c.Min.Y, 1, c.Dy()), p.lcdDim)
		fill(ctx, paint2d.XYWH(x+1, 31, 8, 1), p.lcdDim.WithAlpha(.7))
	}
	fill(ctx, paint2d.XYWH(57, 54, 59, 12), rgb("#0d160f"))
}

func podPlaylist(ctx *paint2d.Context, p *palette, w, h float32) {
	all := paint2d.XYWH(0, 0, w, h)
	round(ctx, all, 26, paint2d.Fill(p.edge))
	in := all.Inset(1.5)
	// flat through the tiled middle rows, shaded only in the end bands
	round(ctx, in, 24.5, paint2d.Fill(p.body))
	ctx.Save()
	ctx.ClipRoundRect(in, 24.5, 24.5)
	top := paint2d.XYWH(0, 0, w, 40)
	ctx.DrawRect(top, vpaint(top, stop(0, p.bodyTop), stop(1, p.body)))
	bot := paint2d.XYWH(0, h-36, w, 36)
	ctx.DrawRect(bot, vpaint(bot, stop(0, p.body), stop(1, p.bodyBottom)))
	ctx.Restore()
	ring(ctx, all.Inset(2), 24, 1, p.rim.WithAlpha(.3))
	// the list's well
	list := paint2d.XYWH(40, 41, w-79, h-84)
	round(ctx, list.Inset(-3), 12, paint2d.Fill(p.edge))
	round(ctx, list.Inset(-1.5), 10.5, paint2d.Fill(p.accent.WithAlpha(.55)))
	round(ctx, list, 9, paint2d.Fill(rgb("#030604")))
	// the crown's key well at the left and the close well at the right
	well(ctx, paint2d.XYWH(12, 1, 50, 21), 9, rgb("#14181a"), p.edge, p.rim.WithAlpha(.35))
	well(ctx, paint2d.XYWH(w-73, 0, 29, 22), 9, rgb("#14181a"), p.edge, p.rim.WithAlpha(.35))
	// rows of vents along the crown and the foot, in the end slices only
	for _, y := range []float32{12, h - 22} {
		for x := float32(76); x < 200; x += 6 {
			round(ctx, paint2d.XYWH(x, y, 3, 8), 1.5, paint2d.Fill(p.edge.WithAlpha(.75)))
		}
		for x := w - 150; x < w-60; x += 6 {
			round(ctx, paint2d.XYWH(x, y, 3, 8), 1.5, paint2d.Fill(p.edge.WithAlpha(.75)))
		}
	}
	// the grip at the resize corner
	for i := float32(0); i < 4; i++ {
		l := paint2d.NewPath()
		l.MoveTo(w-10-i*5, h-6)
		l.LineTo(w-6, h-10-i*5)
		ctx.DrawPath(l, podStrokePaint(p.accent.WithAlpha(.6), 1.2))
	}
}
