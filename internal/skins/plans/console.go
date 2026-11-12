package plans

import (
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// Console is a warm grey desk console: a slate caption band with a teal
// rule, soft rounded keys in a recessed tray, engraved dividers between
// its rows, light panels for the now-playing canvas and equaliser, a dark
// playlist well and a dark status strip along its foot in green.
func Console() *Plan {
	p := consolePalette()
	return genericSkin("console", "RealPlayer - Classic", "A warm grey console with a slate caption band, soft keys in a recessed tray, light panels, a dark playlist and a green status strip.", &geomConsole, p, func(u use) drawFunc { return consoleArt(p, u) },
		map[string]skingen.TextRole{
			"player.current":      {Color: "#8FF0B4"},
			"player.info":         {Color: "#46D28A"},
			"player.row":          {Color: "#E4E6E0"},
			"player.selected":     {Color: "#2C6A72"},
			"player.selectedText": {Color: "#FFFFFF"},
			"player.text":         {Color: "#1C2229"},
		}, map[string]string{"focus": "#1F8C84"})
}

func consolePalette() *palette {
	return &palette{body: rgb("#c3c3c0"), bodyTop: rgb("#d5d5d2"), bodyBottom: rgb("#b7b7b3"), edge: rgb("#5f5a52"), rim: rgb("#f3f1ec"),
		well: rgb("#0f1714"), wellDark: rgb("#4a4640"), wellLight: rgb("#f3f1ec"),
		key: rgb("#d8d8d5"), keyTop: rgb("#f4f4f2"), keyBottom: rgb("#bdbdb8"), keyEdge: rgb("#62625c"),
		ink: rgb("#2b3a46"), inkHover: rgb("#1a6a6a"), accent: rgb("#1f8c84"), lcd: rgb("#46d28a"), lcdDim: rgb("#24423a"),
		radius: 0, keyRadius: 5, gloss: .35, round: false}
}

// The console's own colours beyond the palette.
var (
	consoleSlateTop    = rgb("#3b4e60")
	consoleSlateBottom = rgb("#1d2935")
	consoleTeal        = rgb("#3fb6a8")
	consoleGroove      = rgb("#86857f")
)

// consoleArt draws every sprite of the console.
func consoleArt(p *palette, u use) func(*paint2d.Context, float32, float32) {
	name := u.name
	switch {
	case name == "face.main":
		return func(ctx *paint2d.Context, w, h float32) { consoleMain(ctx, p, w, h) }
	case name == "face.playlist":
		return func(ctx *paint2d.Context, w, h float32) {
			consolePanel(ctx, p, paint2d.XYWH(0, 0, w, h))
			consoleEngrave(ctx, 4, 29, w-8)
			consoleEngrave(ctx, 4, h-30, w-8)
			r := paint2d.XYWH(4, 32, w-8, h-62)
			fill(ctx, r, p.wellDark)
			fill(ctx, r.Inset(1), p.well)
			fill(ctx, paint2d.XYWH(r.Min.X, r.Max.Y-1, r.Dx(), 1), p.rim)
		}
	case name == "face.equalizer":
		return func(ctx *paint2d.Context, w, h float32) {
			consolePanel(ctx, p, paint2d.XYWH(0, 0, w, h))
			consoleEngrave(ctx, 4, 30, w-8)
			consoleEngrave(ctx, 4, h-25, w-8)
		}
	case name == "face.canvas":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			fill(ctx, r, p.edge)
			in := r.Inset(1)
			fill(ctx, in, rgb("#f8f7f3"))
			fill(ctx, paint2d.XYWH(1, 1, w-2, 1), p.wellDark.WithAlpha(.35))
			fill(ctx, paint2d.XYWH(1, 1, 1, h-2), p.wellDark.WithAlpha(.25))
			// a faint teal edge along the top: the canvas reads as a screen
			fill(ctx, paint2d.XYWH(2, 2, w-4, 2), consoleTeal.WithAlpha(.35))
		}
	case name == "app.icon":
		return func(ctx *paint2d.Context, w, h float32) {
			emblem(ctx, paint2d.XYWH(0, 0, w, h), consoleTeal, consoleSlateBottom)
		}
	case strings.HasPrefix(name, "caption."):
		return func(ctx *paint2d.Context, w, h float32) { consoleCaptionKey(ctx, p, u, w, h) }
	case strings.HasPrefix(name, "key."):
		return func(ctx *paint2d.Context, w, h float32) { consoleKey(ctx, p, u, w, h) }
	}
	return nil
}

// consolePanel is a raised warm-grey panel with a one-pixel outline.
func consolePanel(ctx *paint2d.Context, p *palette, r paint2d.Rect) {
	fill(ctx, r, p.edge)
	in := r.Inset(1)
	fill(ctx, in, p.body)
	bevel(ctx, in, p.rim, consoleGroove, false)
}

// consoleEngrave is a horizontal engraved line: dark over light.
func consoleEngrave(ctx *paint2d.Context, x, y, w float32) {
	fill(ctx, paint2d.XYWH(x, y, w, 1), consoleGroove)
	fill(ctx, paint2d.XYWH(x, y+1, w, 1), rgb("#f3f1ec"))
}

func consoleMain(ctx *paint2d.Context, p *palette, w, h float32) {
	all := paint2d.XYWH(0, 0, w, h)
	fill(ctx, all, p.edge)
	in := all.Inset(1)
	fill(ctx, in, p.body)
	// the top band is lit from above; the middle and foot stay flat so the
	// stretched rows match
	top := paint2d.XYWH(1, 1, w-2, 99)
	ctx.DrawRect(top, vpaint(top, stop(0, p.bodyTop), stop(1, p.body)))
	bevel(ctx, in, p.rim, consoleGroove, false)

	// caption band
	cap := paint2d.XYWH(3, 3, w-6, 20)
	round(ctx, cap, 3, vpaint(cap, stop(0, consoleSlateTop), stop(1, consoleSlateBottom)))
	fill(ctx, paint2d.XYWH(6, 4, w-12, 1), paint2d.RGBA(1, 1, 1, .14))
	fill(ctx, paint2d.XYWH(3, 23, w-6, 1), consoleTeal)

	// the menu row and the transport row, with engraved dividers
	consoleEngrave(ctx, 4, 45, w-8)
	tray := paint2d.XYWH(13, 46, 162, 26)
	round(ctx, tray, 6, paint2d.Fill(p.rim))
	round(ctx, paint2d.XYWH(tray.Min.X, tray.Min.Y, tray.Dx(), tray.Dy()-1), 6, paint2d.Fill(consoleGroove))
	round(ctx, paint2d.XYWH(tray.Min.X+1, tray.Min.Y+1, tray.Dx()-2, tray.Dy()-2), 5, vpaint(tray, stop(0, rgb("#aaaaa5")), stop(1, rgb("#bebebb"))))
	consoleEngrave(ctx, 4, 73, w-8)
	consoleEngrave(ctx, 4, 99, w-8)

	// the foot: a divider over the view tabs, and the status strip
	consoleEngrave(ctx, 4, h-91, w-8)
	consoleEngrave(ctx, 4, h-59, w-8)
	strip := paint2d.XYWH(7, h-24, w-14, 20)
	fill(ctx, strip, consoleGroove)
	fill(ctx, paint2d.XYWH(strip.Min.X, strip.Max.Y-1, strip.Dx(), 1), p.rim)
	s := paint2d.XYWH(strip.Min.X+1, strip.Min.Y+1, strip.Dx()-2, strip.Dy()-2)
	ctx.DrawRect(s, vpaint(s, stop(0, rgb("#0a110e")), stop(1, p.well)))
	for y := s.Min.Y + 1; y < s.Max.Y; y += 2 {
		fill(ctx, paint2d.XYWH(s.Min.X, y, s.Dx(), 1), paint2d.RGBA(1, 1, 1, .025))
	}
	// the clock's part of the strip
	fill(ctx, paint2d.XYWH(w-210, s.Min.Y+3, 1, s.Dy()-6), p.lcdDim)
}

// consoleKey is a soft rounded key with its mark, in its four states.
func consoleKey(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	k := strings.TrimPrefix(u.base(), "key.")
	r := paint2d.XYWH(0, 0, w, h)
	top, bottom, edge, ink := p.keyTop, p.keyBottom, p.keyEdge, p.ink
	gloss := p.gloss
	disabled := strings.HasSuffix(u.name, ".disabled")
	switch {
	case disabled:
		top, bottom = mix(p.keyTop, p.body, .6), mix(p.keyBottom, p.body, .6)
		edge, ink, gloss = mix(p.keyEdge, p.body, .55), mix(p.ink, p.body, .6), 0
	case u.down():
		top, bottom = rgb("#a9a398"), rgb("#d2cdc4")
		ink, gloss = p.accent, 0
	case u.hover():
		top, bottom = rgb("#ffffff"), rgb("#d3e6e2")
		edge, ink = p.accent, p.inkHover
	}
	pillow(ctx, r, min(w, h)*.3, top, bottom, edge, gloss)
	if u.hover() && !disabled {
		ring(ctx, r.Inset(1), min(w, h)*.3-1, 1, consoleTeal.WithAlpha(.45))
	}
	d := float32(0)
	if u.down() {
		d = .7
	}
	box := paint2d.XYWH(d, d, w, h)
	s := min(w, h) * .5
	switch k {
	case "open":
		consoleFolder(ctx, box, s*1.1, ink)
	case "visual":
		symbol(ctx, "vis", box, s, ink)
	case "mute":
		symbol(ctx, "speaker", box, s, ink)
	case "prev", "next":
		symbol(ctx, k, box, h*.52, ink)
	default:
		symbol(ctx, k, box, s, ink)
	}
}

// consoleFolder is an open folder.
func consoleFolder(ctx *paint2d.Context, r paint2d.Rect, s float32, c paint2d.Color) {
	cx, cy := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
	h := s / 2
	back := paint2d.NewPath()
	back.MoveTo(cx-h, cy-h*.7)
	back.LineTo(cx-h*.3, cy-h*.7)
	back.LineTo(cx-h*.1, cy-h*.45)
	back.LineTo(cx+h*.8, cy-h*.45)
	back.LineTo(cx+h*.8, cy+h*.7)
	back.LineTo(cx-h, cy+h*.7)
	back.Close()
	ctx.DrawPath(back, paint2d.Fill(c.WithAlpha(.55)))
	front := paint2d.NewPath()
	front.MoveTo(cx-h*.75, cy-h*.15)
	front.LineTo(cx+h, cy-h*.15)
	front.LineTo(cx+h*.8, cy+h*.7)
	front.LineTo(cx-h, cy+h*.7)
	front.Close()
	ctx.DrawPath(front, paint2d.Fill(c))
}

// consoleCaptionKey is a small key on the slate caption band.
func consoleCaptionKey(ctx *paint2d.Context, p *palette, u use, w, h float32) {
	k := strings.TrimPrefix(u.base(), "caption.")
	r := paint2d.XYWH(0, 0, w, h)
	top, bottom, ink := rgb("#5a6e82"), rgb("#34465a"), rgb("#e8eef3")
	switch {
	case strings.HasSuffix(u.name, ".disabled"):
		top, bottom, ink = rgb("#465566"), rgb("#34404d"), rgb("#7f8c99")
	case u.down():
		top, bottom, ink = rgb("#1d2935"), rgb("#2f4152"), consoleTeal
	case u.hover():
		top, bottom = rgb("#6f8aa0"), rgb("#3f566c")
		if k == "close" {
			top, bottom = rgb("#c4604f"), rgb("#8f3a2e")
		}
	}
	pillow(ctx, r, 3, top, bottom, rgb("#141c24"), .2)
	d := float32(0)
	if u.down() {
		d = .6
	}
	symbol(ctx, k, paint2d.XYWH(d, d, w, h), 8, ink)
}
