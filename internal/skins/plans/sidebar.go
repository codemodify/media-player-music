package plans

import (
	"math"
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// Sidebar is a light library window: an indigo caption, a toolbar of
// two-tone indigo icons drawn from paths (sharp at every scale), a tabbed
// sidebar, and an empty-cover card of a record half out of its sleeve.
// The face paints its own panels; the pack gives it the caption, the
// icons, the marks and the cover.
func Sidebar() *Plan {
	p := sidebarPalette()
	return genericSkin("sidebar", "Sidebar", "A light library window with an indigo caption, two-tone indigo toolbar icons, a tabbed sidebar and a multi-column playlist.", &geomSidebar, p, func(u use) drawFunc { return sidebarArt(p, u) },
		map[string]skingen.TextRole{
			"player.current":      {Color: "#2B3590"},
			"player.info":         {Color: "#33334A"},
			"player.row":          {Color: "#17171F"},
			"player.selected":     {Color: "#5560C4"},
			"player.selectedText": {Color: "#FFFFFF"},
			"player.text":         {Color: "#1A1A24"},
		}, map[string]string{"focus": "#5560C4"})
}

func sidebarPalette() *palette {
	return &palette{body: rgb("#e4e3e8"), bodyTop: rgb("#efeef2"), bodyBottom: rgb("#d7d6dd"), edge: rgb("#8c8a99"), rim: rgb("#fbfbfd"),
		well: rgb("#ffffff"), wellDark: rgb("#9a98a8"), wellLight: rgb("#ffffff"),
		key: rgb("#e6e5ea"), keyTop: rgb("#f6f6f8"), keyBottom: rgb("#d0cfd8"), keyEdge: rgb("#8c8a99"),
		ink: rgb("#3b4a8f"), inkHover: rgb("#2440b0"), accent: rgb("#5a63c9"), lcd: rgb("#222233"), lcdDim: rgb("#a3a2b3"),
		radius: 0, keyRadius: 3, gloss: .2, round: false}
}

// The icons' two tones and their outline.
var (
	sbLight   = rgb("#8a97f0")
	sbDeep    = rgb("#3340b0")
	sbOutline = rgb("#1f2770")
	sbPaper   = rgb("#fbfbff")
)

func sidebarArt(p *palette, u use) func(*paint2d.Context, float32, float32) {
	name := u.name
	switch {
	case name == "chrome.title":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			ctx.DrawRect(r, vpaint(r, stop(0, rgb("#7079cf")), stop(.5, rgb("#4d57b4")), stop(1, rgb("#3a4396"))))
			fill(ctx, paint2d.XYWH(0, 0, w, 1), rgb("#a6adec"))
			fill(ctx, paint2d.XYWH(0, h-1, w, 1), rgb("#262d70"))
		}
	case name == "app.icon" || name == "app.logo":
		return func(ctx *paint2d.Context, w, h float32) {
			emblem(ctx, paint2d.XYWH(0, 0, w, h), sbDeep, sbPaper)
		}
	case name == "cover.empty":
		return sidebarCover
	case strings.HasPrefix(name, "icon."):
		k := strings.TrimPrefix(name, "icon.")
		return func(ctx *paint2d.Context, w, h float32) { sidebarIcon(ctx, k, w, h) }
	}
	return nil
}

// sbFill paints a path in the icons' two tones, outlined.
func sbFill(ctx *paint2d.Context, path *paint2d.Path, r paint2d.Rect) {
	ctx.DrawPath(path, vpaint(r, stop(0, sbLight), stop(1, sbDeep)))
	ctx.DrawPath(path, paint2d.StrokePaint(sbOutline, 1))
}

func sbLine(ctx *paint2d.Context, path *paint2d.Path, width float32) {
	ctx.DrawPath(path, paint2d.StrokePaint(sbOutline, width+1.6))
	ctx.DrawPath(path, paint2d.StrokePaint(sbDeep, width))
}

func sidebarIcon(ctx *paint2d.Context, k string, w, h float32) {
	r := paint2d.XYWH(0, 0, w, h)
	cx, cy := w/2, h/2
	switch k {
	case "play", "pause", "stop", "prev", "next":
		// a round indigo key with a white mark
		orb(ctx, paint2d.Pt(cx, cy), 10, sbLight, sbDeep, sbOutline, .45)
		symbol(ctx, k, paint2d.XYWH(0, 0, w, h), 9, sbPaper)
	case "add", "open", "playlist", "remove":
		// a page, with its badge
		page := paint2d.NewPath()
		page.MoveTo(4.5, 2.5)
		page.LineTo(13.5, 2.5)
		page.LineTo(17.5, 6.5)
		page.LineTo(17.5, 19.5)
		page.LineTo(4.5, 19.5)
		page.Close()
		if k == "open" {
			consoleFolder(ctx, r, 18, sbOutline)
			f := paint2d.NewPath()
			f.MoveTo(5, 9)
			f.LineTo(19.5, 9)
			f.LineTo(17.5, 17.5)
			f.LineTo(3, 17.5)
			f.Close()
			sbFill(ctx, f, r)
			return
		}
		ctx.DrawPath(page, paint2d.Fill(sbPaper))
		ctx.DrawPath(page, paint2d.StrokePaint(sbOutline, 1))
		for y := float32(8); y <= 15; y += 3 {
			fill(ctx, paint2d.XYWH(7, y, 8, 1), sbLight)
		}
		badge := paint2d.Pt(15.5, 15.5)
		switch k {
		case "add":
			ctx.DrawCircle(badge, 5.5, paint2d.Fill(sbOutline))
			ctx.DrawCircle(badge, 4.5, paint2d.Fill(sbDeep))
			symbol(ctx, "add", paint2d.XYWH(badge.X-5, badge.Y-5, 10, 10), 7, sbPaper)
		case "remove":
			ctx.DrawCircle(badge, 5.5, paint2d.Fill(rgb("#6a1c2a")))
			ctx.DrawCircle(badge, 4.5, paint2d.Fill(rgb("#c8405a")))
			symbol(ctx, "close", paint2d.XYWH(badge.X-5, badge.Y-5, 10, 10), 6, sbPaper)
		case "playlist":
			ctx.DrawCircle(badge, 5.5, paint2d.Fill(sbOutline))
			ctx.DrawCircle(badge, 4.5, paint2d.Fill(sbDeep))
			symbol(ctx, "play", paint2d.XYWH(badge.X-4.5, badge.Y-5, 10, 10), 5, sbPaper)
		}
	case "context":
		// a window with its side panel
		box := paint2d.XYWH(2.5, 3.5, 17, 15)
		ctx.DrawRect(box, paint2d.Fill(sbPaper))
		side := paint2d.XYWH(2.5, 3.5, 6, 15)
		ctx.DrawRect(side, vpaint(side, stop(0, sbLight), stop(1, sbDeep)))
		ctx.DrawRect(box, paint2d.StrokePaint(sbOutline, 1))
		for y := float32(8); y <= 15; y += 3 {
			fill(ctx, paint2d.XYWH(10.5, y, 7, 1), sbLight)
		}
	case "eq":
		for i, at := range []float32{7, 13, 9} {
			x := 5 + float32(i)*6
			fill(ctx, paint2d.XYWH(x, 3, 1.5, 16), sbOutline)
			knob := paint2d.XYWH(x-2.5, at, 6.5, 4)
			round(ctx, knob, 1.5, vpaint(knob, stop(0, sbLight), stop(1, sbDeep)))
			ring(ctx, knob, 1.5, 1, sbOutline)
		}
	case "settings":
		gear := paint2d.NewPath()
		const teeth = 8
		for i := 0; i < teeth*2; i++ {
			a := float64(i) * math.Pi / teeth
			rad := float32(8.5)
			if i%2 == 1 {
				rad = 6.2
			}
			for _, da := range []float64{-.17, .17} {
				x := cx + rad*float32(math.Cos(a+da))
				y := cy + rad*float32(math.Sin(a+da))
				if i == 0 && da < 0 {
					gear.MoveTo(x, y)
				} else {
					gear.LineTo(x, y)
				}
			}
		}
		gear.Close()
		sbFill(ctx, gear, r)
		ctx.DrawCircle(paint2d.Pt(cx, cy), 2.6, paint2d.Fill(sbPaper))
		ctx.DrawCircle(paint2d.Pt(cx, cy), 2.6, paint2d.StrokePaint(sbOutline, 1))
	case "shuffle":
		path := paint2d.NewPath()
		path.MoveTo(3, 7)
		path.CubicTo(10, 7, 11, 15, 17, 15)
		path.MoveTo(3, 15)
		path.CubicTo(10, 15, 11, 7, 17, 7)
		sbLine(ctx, path, 2)
		for _, y := range []float32{7, 15} {
			a := paint2d.NewPath()
			a.MoveTo(16, y-4)
			a.LineTo(20.5, y)
			a.LineTo(16, y+4)
			a.Close()
			sbFill(ctx, a, r)
		}
	case "repeat", "repeat.off", "repeat.one":
		path := paint2d.NewPath()
		path.MoveTo(15, 6)
		path.LineTo(7, 6)
		path.CubicTo(3, 6, 3, 16, 7, 16)
		path.LineTo(15, 16)
		path.CubicTo(19, 16, 19, 9, 17.5, 8)
		a := paint2d.NewPath()
		a.MoveTo(13, 2)
		a.LineTo(17.5, 6)
		a.LineTo(13, 10)
		a.Close()
		if k == "repeat.off" {
			ctx.DrawPath(path, paint2d.StrokePaint(rgb("#9a98a8"), 2))
			ctx.DrawPath(a, paint2d.Fill(rgb("#9a98a8")))
			return
		}
		sbLine(ctx, path, 2)
		sbFill(ctx, a, r)
		if k == "repeat.one" {
			b := paint2d.Pt(11, 11)
			ctx.DrawCircle(b, 4.5, paint2d.Fill(sbOutline))
			fill(ctx, paint2d.XYWH(10.5, 8.5, 1.5, 5), sbPaper)
			fill(ctx, paint2d.XYWH(9.3, 9, 1.5, 1.2), sbPaper)
		}
	}
}

// sidebarCover is the empty cover: a record half out of its plain sleeve
// on a soft card.
func sidebarCover(ctx *paint2d.Context, w, h float32) {
	card := paint2d.XYWH(.5, .5, w-1, h-1)
	round(ctx, card, 4, vpaint(card, stop(0, rgb("#f7f7fb")), stop(1, rgb("#e6e5ef"))))
	ring(ctx, card, 4, 1, rgb("#b7b5c6"))
	// the record, peeking out to the right
	rad := h * .33
	c := paint2d.Pt(w-rad-8, h/2)
	ctx.DrawCircle(c, rad, paint2d.Fill(rgb("#1d1d26")))
	for i := float32(0); i < 6; i++ {
		ctx.DrawCircle(c, rad*(.92-i*.09), paint2d.StrokePaint(rgb("#34343f"), .8))
	}
	ctx.DrawCircle(c, rad*.34, paint2d.Fill(sbDeep))
	ctx.DrawCircle(c, rad*.34, paint2d.StrokePaint(sbLight, 1))
	ctx.DrawCircle(c, rad*.06, paint2d.Fill(rgb("#f7f7fb")))
	// a highlight across the grooves
	arc := paint2d.NewPath()
	arc.AddArc(c, rad*.75, rad*.75, -2.4, .9)
	ctx.DrawPath(arc, paint2d.StrokePaint(paint2d.RGBA(1, 1, 1, .18), 2))
	// the sleeve in front, on the left
	side := rad*1.75 + 8
	sleeve := paint2d.XYWH(12, h/2-side/2, side, side)
	ctx.DrawRect(paint2d.XYWH(sleeve.Min.X+2, sleeve.Min.Y+2, sleeve.Dx(), sleeve.Dy()), paint2d.Fill(paint2d.RGBA(0, 0, 0, .12)))
	ctx.DrawRect(sleeve, vpaint(sleeve, stop(0, rgb("#d9dcf6")), stop(1, rgb("#b8bdea"))))
	ctx.DrawRect(sleeve.Inset(.5), paint2d.StrokePaint(rgb("#8d93cc"), 1))
	ctx.DrawCircle(sleeve.Center(), rad*.55, paint2d.StrokePaint(paint2d.RGBA(1, 1, 1, .45), 1.2))
	emblem(ctx, paint2d.XYWH(sleeve.Center().X-9, sleeve.Center().Y-9, 18, 18), sbDeep, rgb("#d9dcf6"))
}
