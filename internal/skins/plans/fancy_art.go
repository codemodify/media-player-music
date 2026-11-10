package plans

import (
	"math"

	"goframework.io/graphics-paint2d"
)

var (
	fnCyan = Hex("#68d6e5")
)

// The main outline is the union of the two curved wings, their transport
// shelves, the display ring and the round play socket. Transparent pixels
// beyond these paths supply the native window's silhouette too.
func fancyMainFace(ctx *paint2d.Context, w, h float32) {
	for _, mirror := range []bool{false, true} {
		ctx.Save()
		if mirror {
			ctx.Translate(w, 0)
			ctx.Scale(-1, 1)
		}
		wing := paint2d.NewPath()
		wing.MoveTo(131, 46)
		wing.LineTo(48, 46)
		wing.CubicTo(17, 46, 2, 67, 2, 98)
		wing.LineTo(2, 124)
		wing.CubicTo(2, 153, 12, 169, 39, 173)
		wing.CubicTo(67, 176, 112, 177, 154, 195)
		wing.CubicTo(164, 152, 154, 91, 131, 46)
		wing.Close()
		ctx.DrawPath(wing, fancyVertical(46, 195, stop(0, Hex("#4b515e")), stop(.12, Hex("#303640")), stop(.42, Hex("#1e232d")), stop(.69, Hex("#151921")), stop(1, Hex("#080b10"))))
		ctx.DrawPath(wing, paint2d.StrokePaint(Hex("#07090d"), 2))
		ctx.Save()
		ctx.ClipPath(wing)
		fancyBrush(ctx, paint2d.XYWH(0, 45, 165, 155))
		// A thin brushed highlight follows the top instead of giving the
		// whole wing the flat border of a conventional rectangular window.
		shine := paint2d.NewPath()
		shine.MoveTo(5, 106)
		shine.CubicTo(5, 72, 17, 49, 49, 49)
		shine.LineTo(132, 49)
		ctx.DrawPath(shine, paint2d.StrokePaint(Hex("#a8b6c044"), 1))
		ctx.Restore()
		fancyWingGrille(ctx)
		fancyTransportShelf(ctx)
		ctx.Restore()
	}
	for _, r := range []paint2d.Rect{paint2d.XYWH(17, 43, 113, 19), paint2d.XYWH(367, 43, 48, 20)} {
		vgrad(ctx, r, 6, stop(0, Hex("#444c59")), stop(1, Hex("#232b35")))
		outline(ctx, r, 6, .8, Hex("#080d14"))
	}

	c := paint2d.Pt(210, 96)
	// Separate chrome rings give the centre display its deep machined rim.
	ctx.DrawCircle(c, 95, fancyVertical(1, 191, stop(0, Hex("#77818f")), stop(.06, Hex("#292f3a")), stop(.24, Hex("#10141c")), stop(.64, Hex("#232a35")), stop(1, Hex("#020408"))))
	ctx.DrawCircle(c, 93, paint2d.StrokePaint(Hex("#06080d"), 2.5))
	ctx.DrawCircle(c, 89.5, fancyVertical(7, 187, stop(0, Hex("#303746")), stop(.25, Hex("#151a23")), stop(.55, Hex("#080c13")), stop(.86, Hex("#272e38")), stop(1, Hex("#080b11"))))
	ctx.DrawCircle(c, 84.5, paint2d.StrokePaint(Hex("#090c13"), 4))
	ctx.DrawCircle(paint2d.Pt(210, 91), 75.5, paint2d.Radial(paint2d.RadialGradient{
		Center: paint2d.Pt(194, 49), Radius: 120,
		Stops: []paint2d.GradientStop{stop(0, Hex("#212935")), stop(.42, Hex("#121923")), stop(1, Hex("#04080d"))},
	}))
	ctx.DrawCircle(paint2d.Pt(210, 91), 75.5, paint2d.StrokePaint(Hex("#070a10"), 2))
	ctx.DrawArc(c, 95, 95, float32(math.Pi*1.03), float32(math.Pi*.91), paint2d.StrokePaint(Hex("#c1cad44b"), .9))
	ctx.DrawArc(c, 83.5, 83.5, .28, float32(math.Pi-.56), paint2d.StrokePaint(Hex("#060b10"), 4))
	ctx.DrawArc(c, 83.5, 83.5, .32, float32(math.Pi-.64), paint2d.StrokePaint(Hex("#135660"), 3.4))
	ctx.DrawArc(c, 83.5, 83.5, .37, float32(math.Pi-.74), paint2d.StrokePaint(fnCyan.WithAlpha(.92), 1.15))
	for _, a := range []float64{.65, math.Pi - .65} {
		p := paint2d.Pt(c.X+83.5*float32(math.Cos(a)), c.Y+83.5*float32(math.Sin(a)))
		ctx.DrawCircle(p, 4.5, paint2d.Fill(Hex("#111720")))
		ctx.DrawCircle(p, 3.3, paint2d.Fill(Hex("#647482")))
		ctx.DrawCircle(paint2d.Pt(p.X-1, p.Y-1), .8, paint2d.Fill(Hex("#bdc9d0")))
	}
	// Dark LED cells remain visible beneath the real animated analyser.
	for x := float32(166); x < 256; x += 8 {
		for y := float32(33); y < 75; y += 4 {
			ctx.DrawRect(paint2d.XYWH(x, y, 4.5, 1.2), paint2d.Fill(Hex("#263a44")))
		}
	}
	ctx.DrawLine(paint2d.Pt(162, 113), paint2d.Pt(258, 113), paint2d.StrokePaint(Hex("#64738024"), .7))
	fancyPlaySocket(ctx)
	for _, x := range []float32{43, 377} {
		fancyKnobSocket(ctx, paint2d.Pt(x, 83))
	}
	for _, r := range []paint2d.Rect{
		paint2d.XYWH(70, 68, 44, 17), paint2d.XYWH(70, 91, 44, 17),
		paint2d.XYWH(306, 68, 44, 17), paint2d.XYWH(306, 91, 44, 17),
	} {
		fancyInset(ctx, r.Inset(1), 7, Hex("#141a23"))
	}
}

func fancyVertical(top, bottom float32, stops ...paint2d.GradientStop) paint2d.Paint {
	return paint2d.Linear(paint2d.LinearGradient{Start: paint2d.Pt(0, top), End: paint2d.Pt(0, bottom), Stops: stops})
}

func fancyBrush(ctx *paint2d.Context, r paint2d.Rect) {
	// Very low-contrast grain; second rasterisation retains thin strokes at
	// 2x rather than interpolating a 1x photograph of the material.
	for y := r.Min.Y; y < r.Max.Y; y += 1.7 {
		alpha := float32(.015 + .009*math.Sin(float64(y)*1.31))
		ctx.DrawLine(paint2d.Pt(r.Min.X, y), paint2d.Pt(r.Max.X, y), paint2d.StrokePaint(paint2d.White.WithAlpha(alpha), .35))
	}
}

func fancyWingGrille(ctx *paint2d.Context) {
	p := paint2d.NewPath()
	p.MoveTo(10, 107)
	p.CubicTo(23, 110, 70, 130, 109, 151)
	p.CubicTo(96, 163, 61, 169, 32, 162)
	p.CubicTo(16, 157, 8, 139, 10, 107)
	p.Close()
	ctx.DrawPath(p, paint2d.Fill(Hex("#030509")))
	ctx.DrawPath(p, paint2d.StrokePaint(Hex("#4c56633a"), 1))
	ctx.Save()
	ctx.ClipPath(p)
	for d := float32(-150); d < 200; d += 6 {
		ctx.DrawLine(paint2d.Pt(d, 95), paint2d.Pt(d+120, 185), paint2d.StrokePaint(Hex("#35414d69"), .6))
		ctx.DrawLine(paint2d.Pt(d, 185), paint2d.Pt(d+120, 95), paint2d.StrokePaint(Hex("#35414d52"), .6))
	}
	ctx.DrawRect(paint2d.XYWH(0, 107, 120, 75), fancyVertical(107, 182, stop(0, Hex("#00000000")), stop(1, Hex("#000000bc"))))
	ctx.Restore()
}

func fancyTransportShelf(ctx *paint2d.Context) {
	p := paint2d.NewPath()
	p.MoveTo(20, 164)
	p.CubicTo(65, 169, 101, 168, 141, 177)
	p.CubicTo(150, 189, 162, 196, 179, 204)
	p.CubicTo(114, 207, 35, 189, 21, 177)
	p.CubicTo(17, 173, 17, 168, 20, 164)
	p.Close()
	ctx.DrawPath(p, fancyVertical(164, 206, stop(0, Hex("#363e48")), stop(.18, Hex("#121820")), stop(.5, Hex("#060a0f")), stop(1, Hex("#151b23"))))
	ctx.DrawPath(p, paint2d.StrokePaint(Hex("#090d13"), 1))
	line := paint2d.NewPath()
	line.MoveTo(23, 165)
	line.CubicTo(70, 170, 99, 171, 139, 179)
	ctx.DrawPath(line, paint2d.StrokePaint(Hex("#b6d5df43"), .7))
	ctx.DrawLine(paint2d.Pt(56, 181), paint2d.Pt(132, 198), paint2d.StrokePaint(fnCyan.WithAlpha(.08), .8))
}

func fancyKnobSocket(ctx *paint2d.Context, c paint2d.Point) {
	ctx.DrawCircle(c, 22.5, fancyVertical(c.Y-23, c.Y+23, stop(0, Hex("#03070b")), stop(.5, Hex("#25333c")), stop(1, Hex("#02060a"))))
	ctx.DrawCircle(c, 19, paint2d.StrokePaint(Hex("#0d3c47"), 1.4))
	ctx.DrawArc(c, 19, 19, float32(math.Pi*.8), float32(math.Pi*1.4), paint2d.StrokePaint(fnCyan.WithAlpha(.85), 1.2))
	ctx.DrawCircle(c, 15.5, paint2d.Radial(paint2d.RadialGradient{Center: paint2d.Pt(c.X-6, c.Y-8), Radius: 31, Stops: []paint2d.GradientStop{stop(0, Hex("#eef2ef")), stop(.42, Hex("#c9d0d1")), stop(.74, Hex("#a6b2b8")), stop(1, Hex("#364552"))}}))
	ctx.DrawCircle(c, 13, paint2d.StrokePaint(Hex("#ffffff8c"), .65))
	for i := range 8 {
		a := float64(i) * math.Pi / 4
		ctx.DrawLine(c, paint2d.Pt(c.X+12*float32(math.Cos(a)), c.Y+12*float32(math.Sin(a))), paint2d.StrokePaint(Hex("#72828c33"), .7))
	}
}

func fancyPlaySocket(ctx *paint2d.Context) {
	c := paint2d.Pt(210, 190)
	ctx.DrawCircle(c, 29.5, fancyVertical(160, 220, stop(0, Hex("#4c5b66")), stop(.22, Hex("#0d141c")), stop(.75, Hex("#050a11")), stop(1, Hex("#26313d"))))
	ctx.DrawCircle(c, 26, paint2d.StrokePaint(Hex("#08121b"), 2))
	ctx.DrawCircle(c, 23.5, paint2d.StrokePaint(fnCyan.WithAlpha(.85), 1.2))
	ctx.DrawCircle(c, 21.2, fancyVertical(168, 212, stop(0, Hex("#263640")), stop(.45, Hex("#101a23")), stop(1, Hex("#02070d"))))
	ctx.DrawArc(c, 21.2, 21.2, float32(math.Pi*1.17), float32(math.Pi*.68), paint2d.StrokePaint(Hex("#b0d2dc35"), 1))
}

func fancyPanel(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(1, 1, w-2, h-2)
	vgrad(ctx, r, 39, stop(0, Hex("#545b67")), stop(.06, Hex("#353b45")), stop(.2, Hex("#272d37")), stop(.56, Hex("#171c25")), stop(.78, Hex("#141920")), stop(.95, Hex("#363d47")), stop(1, Hex("#0a0e14")))
	outline(ctx, r, 39, 1.4, Hex("#080b10"))
	ctx.Save()
	ctx.ClipRoundRect(r.Inset(1), 38, 38)
	fancyBrush(ctx, r)
	ctx.DrawLine(paint2d.Pt(35, 3), paint2d.Pt(w-35, 3), paint2d.StrokePaint(Hex("#b3c1ca4b"), .7))
	ctx.DrawLine(paint2d.Pt(35, h-5), paint2d.Pt(w-35, h-5), paint2d.StrokePaint(Hex("#697e8b6b"), .6))
	ctx.Restore()
	fancyPanelGrille(ctx, paint2d.XYWH(8, 28, 25, h-49))
	fancyPanelGrille(ctx, paint2d.XYWH(w-33, 28, 25, h-49))
}

func fancyPanelGrille(ctx *paint2d.Context, r paint2d.Rect) {
	fancyInset(ctx, r, 13, Hex("#04080c"))
	ctx.Save()
	ctx.ClipRoundRect(r.Inset(2), 11, 11)
	for y := r.Min.Y + 3; y < r.Max.Y; y += 4.4 {
		for x := r.Min.X + 3; x < r.Max.X; x += 4.4 {
			ctx.DrawCircle(paint2d.Pt(x, y), .75, paint2d.Fill(Hex("#2b35404d")))
		}
	}
	ctx.Restore()
}

func fancyInset(ctx *paint2d.Context, r paint2d.Rect, rad float32, fill paint2d.Color) {
	vgrad(ctx, r, rad, stop(0, Hex("#080d13")), stop(.3, fill), stop(1, Hex("#0c1118")))
	outline(ctx, r, rad, .85, Hex("#05090e"))
	ctx.DrawLine(paint2d.Pt(r.Min.X+rad, r.Max.Y-.3), paint2d.Pt(r.Max.X-rad, r.Max.Y-.3), paint2d.StrokePaint(Hex("#9bb3bf2d"), .6))
}

func fancyEqFace(ctx *paint2d.Context, w, h float32) {
	fancyPanel(ctx, w, h)
	fancyInset(ctx, paint2d.XYWH(47, 28, 267, 77), 4, Hex("#151c23"))
	for x := float32(62); x < 295; x += 21 {
		ctx.DrawLine(paint2d.Pt(x, 32), paint2d.Pt(x, 97), paint2d.StrokePaint(Hex("#050a10"), 4))
		ctx.DrawLine(paint2d.Pt(x+2, 32), paint2d.Pt(x+2, 97), paint2d.StrokePaint(Hex("#9dbbc518"), .5))
	}
	fancyInset(ctx, paint2d.XYWH(326, 30, 67, 71), 9, Hex("#161d25"))
	// The reference's track-tag tile is decorative, not fake album metadata.
	note := paint2d.NewPath()
	note.MoveTo(363, 46)
	note.LineTo(363, 73)
	note.CubicTo(363, 81, 350, 84, 350, 76)
	note.CubicTo(350, 71, 355, 68, 360, 70)
	note.LineTo(360, 44)
	note.CubicTo(365, 50, 373, 52, 371, 60)
	note.CubicTo(370, 54, 366, 53, 363, 51)
	note.Close()
	ctx.DrawPath(note, paint2d.Fill(Hex("#566b7c3d")))
	for _, r := range []paint2d.Rect{paint2d.XYWH(103, 7, 42, 17), paint2d.XYWH(149, 7, 42, 17), paint2d.XYWH(195, 7, 42, 17), paint2d.XYWH(241, 7, 68, 17)} {
		vgrad(ctx, r, 3, stop(0, Hex("#454f5c")), stop(1, Hex("#232b36")))
		outline(ctx, r, 3, .6, Hex("#0e141b"))
	}
}

func fancyListFace(ctx *paint2d.Context, w, h float32) {
	fancyPanel(ctx, w, h)
	fancyInset(ctx, paint2d.XYWH(41, 28, 287, h-54), 6, Hex("#141e27"))
	// Fixed-size controls and branding have their own slot assets. They must
	// not be stretched along with this resizable frame's middle slices.
}

func fancyListButton(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	vgrad(ctx, r, 3, stop(0, Hex("#454f5c")), stop(1, Hex("#232b36")))
	outline(ctx, r, 3, .6, Hex("#0e141b"))
}

func fancyListToggle(ctx *paint2d.Context, w, h float32) {
	fancyInset(ctx, paint2d.XYWH(0, 1, w, h-2), 7, Hex("#17212b"))
}

func fancyListLogo(ctx *paint2d.Context, w, h float32) {
	ctx.DrawCircle(paint2d.Pt(w/2, h/2), 11.5, paint2d.StrokePaint(Hex("#55758169"), 1))
	paintPlayerLogo(ctx, paint2d.XYWH(3, 3, 18, 18))
}

func fancyCompactFace(ctx *paint2d.Context, w, h float32) {
	r := paint2d.XYWH(1, 1, w-2, h-2)
	vgrad(ctx, r, 19, stop(0, Hex("#59636e")), stop(.12, Hex("#2b323d")), stop(.45, Hex("#141a23")), stop(.8, Hex("#111720")), stop(1, Hex("#343d47")))
	outline(ctx, r, 19, 1.4, Hex("#05090e"))
	fancyInset(ctx, paint2d.XYWH(13, 5, 265, 29), 9, Hex("#0b151d"))
	ctx.DrawLine(paint2d.Pt(23, 32), paint2d.Pt(269, 32), paint2d.StrokePaint(fnCyan.WithAlpha(.17), .6))
	for x := float32(280); x < 355; x += 19 {
		ctx.DrawCircle(paint2d.Pt(x+9, 20), 8.7, paint2d.Fill(Hex("#080e14")))
		ctx.DrawCircle(paint2d.Pt(x+9, 20), 8.7, paint2d.StrokePaint(Hex("#54747e63"), .6))
	}
}
