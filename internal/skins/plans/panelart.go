package plans

import (
	"fmt"
	"strings"

	"github.com/codemodify/media-player-music/internal/skins/panel"
	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// The nine redrawn compact-panel skins share the panel's layout (the strip,
// equaliser and playlist the winamp face draws) and one painter here; each
// is a theme: its colours, its keys' shape and the texture on its chrome.

type panelTheme struct {
	chrome, chromeTop, chromeBottom, light, shade, outline paint2d.Color
	well, wellRim                                          paint2d.Color
	lcd, lcdDim                                            paint2d.Color
	keyTop, keyBottom, keyEdge, keyInk, keyLit             paint2d.Color
	fillLow, fillHigh                                      paint2d.Color
	caption, captionInk, groove, label                     paint2d.Color
	rows, rowText, rowCurrent, rowSelected, rowSelText     paint2d.Color
	ramp                                                   []paint2d.Color
	texture                                                string // "", "scan", "grid", "brushed", "dots", "rivets", "dither", "lines"
	keys                                                   string // "bevel", "pill", "flat", "round", "outline"
	displayTexture                                         string // "", "scan", "grid", "lcd"
	fonts                                                  []string
}

func panelSkin(id, label, summary string, g *geometry, t *panelTheme) *skingen.Plan {
	text := map[string]skingen.TextRole{
		"capkey":              {Color: hexOf(t.keyInk), Hover: hexOf(t.keyInk), Pressed: hexOf(t.keyLit), Disabled: hexOf(mix(t.keyInk, t.keyBottom, .6))},
		"caption":             {Color: hexOf(t.captionInk), Disabled: hexOf(mix(t.captionInk, t.caption, .55)), Bold: true, Upper: true},
		"caption.main":        {Color: hexOf(t.captionInk), Disabled: hexOf(mix(t.captionInk, t.caption, .55)), Bold: true},
		"control":             {Color: hexOf(t.label), Disabled: hexOf(mix(t.label, t.chrome, .5))},
		"player.current":      {Color: hexOf(t.rowCurrent)},
		"player.curve":        {Color: hexOf(t.lcdDim)},
		"player.info":         {Color: hexOf(t.lcd)},
		"player.peak":         {Color: hexOf(lift(t.lcd, .4))},
		"player.row":          {Color: hexOf(t.rowText)},
		"player.selected":     {Color: hexOf(t.rowSelected)},
		"player.selectedText": {Color: hexOf(t.rowSelText)},
		"player.text":         {Color: hexOf(t.lcd)},
	}
	for i := 0; i < 16; i++ {
		c := rampAt(t.ramp, float32(i)/15)
		text[fmt.Sprintf("player.ramp.%d", i)] = skingen.TextRole{Color: hexOf(c)}
	}
	gg := *g
	gg.fonts = &style.FontPrefs{UI: t.fonts}
	if len(t.fonts) == 0 {
		gg.fonts = nil
	}
	strip := gg.layout(panel.LayoutStrip)
	eq := gg.layout(panel.LayoutEqualiser)
	list := gg.layout(panel.LayoutPlaylist)
	return build(skinArt{id: id, label: label, summary: summary, g: &gg, text: text,
		colors: map[string]string{"focus": hexOf(t.lcd)},
		paint: func(u use) drawFunc {
			return panelArt(t, &gg, strip, eq, list, u)
		}})
}

func hexOf(c paint2d.Color) string {
	n := c.Clamp().NRGBA()
	return fmt.Sprintf("#%02X%02X%02X", n.R, n.G, n.B)
}

func rampAt(r []paint2d.Color, t float32) paint2d.Color {
	if len(r) == 0 {
		return paint2d.White
	}
	if len(r) == 1 {
		return r[0]
	}
	f := t * float32(len(r)-1)
	i := min(int(f), len(r)-2)
	return mix(r[i], r[i+1], f-float32(i))
}

func panelArt(t *panelTheme, g *geometry, strip, eq, list *skingen.LayoutSpec, u use) drawFunc {
	name := u.name
	switch {
	case name == "main.face":
		return func(ctx *paint2d.Context, w, h float32) { panelMainFace(ctx, t, strip, w, h) }
	case strings.HasPrefix(name, "backdrop."):
		return func(ctx *paint2d.Context, w, h float32) {
			r, ok := slotAt(strip, name)
			if !ok {
				return
			}
			ctx.Translate(-r.Min.X, -r.Min.Y)
			panelMainFace(ctx, t, strip, float32(strip.W), float32(strip.H))
		}
	case name == "eq.face":
		return func(ctx *paint2d.Context, w, h float32) { panelEqFace(ctx, t, eq, w, h) }
	case name == "list.face":
		return func(ctx *paint2d.Context, w, h float32) { panelListFace(ctx, t, u, w, h) }
	case name == "list.rows":
		return func(ctx *paint2d.Context, w, h float32) {
			fill(ctx, paint2d.XYWH(0, 0, w, h), t.wellRim)
			fill(ctx, paint2d.XYWH(1, 1, w-2, h-2), t.rows)
		}
	case name == "list.scroll":
		return func(ctx *paint2d.Context, w, h float32) {
			fill(ctx, paint2d.XYWH(0, 0, w, h), t.wellRim)
			fill(ctx, paint2d.XYWH(1, 1, w-2, h-2), mix(t.rows, t.chrome, .35))
		}
	case name == "list.clock" || name == "list.info":
		return func(ctx *paint2d.Context, w, h float32) {
			fill(ctx, paint2d.XYWH(0, 0, w, h), t.well)
		}
	case name == "list.arrows":
		return func(ctx *paint2d.Context, w, h float32) {
			panelKeyBody(ctx, t, paint2d.XYWH(0, 0, w, h/2), false, false)
			panelKeyBody(ctx, t, paint2d.XYWH(0, h/2, w, h/2), false, false)
			symbol(ctx, "up", paint2d.XYWH(0, 0, w, h/2), 4, t.keyInk)
			symbol(ctx, "down", paint2d.XYWH(0, h/2, w, h/2), 4, t.keyInk)
		}
	case strings.HasPrefix(name, "info.font."), strings.HasPrefix(name, "font."):
		ink := t.lcd
		if u.tint {
			ink = paint2d.White
		}
		return pixGlyphArt(name, ink)
	case strings.HasPrefix(name, "led."):
		return func(ctx *paint2d.Context, w, h float32) { ledArt(ctx, name, w, h, t.lcd) }
	case strings.HasPrefix(name, "lamp."):
		word := strings.ToUpper(strings.TrimPrefix(name, "lamp."))
		return func(ctx *paint2d.Context, w, h float32) {
			tw := float32(pixWidth(word))
			pixLabel(ctx, float32(int((w-tw)/2)), float32(int((h-6)/2)), word, t.lcd)
		}
	case strings.HasPrefix(name, "state."):
		g := strings.TrimPrefix(name, "state.")
		return func(ctx *paint2d.Context, w, h float32) { pixSymbol(ctx, g, w, h, t.lcd) }
	case strings.HasPrefix(name, "volume.frame.") || strings.HasPrefix(name, "balance.frame.") || strings.HasPrefix(name, "eq.frame."):
		return func(ctx *paint2d.Context, w, h float32) {
			i, n := u.frame(g)
			f := float32(i) / float32(max(n-1, 1))
			switch {
			case strings.HasPrefix(name, "eq."):
				panelSliderBack(ctx, t, w, h, f, "eq")
			case strings.HasPrefix(name, "balance."):
				panelSliderBack(ctx, t, w, h, f, "balance")
			default:
				panelSliderBack(ctx, t, w, h, f, "volume")
			}
		}
	case name == "thumb" || name == "thumb.down" || name == "seek.thumb" || name == "seek.thumb.down" || name == "eq.thumb" || name == "eq.thumb.down" || name == "list.thumb" || name == "list.thumb.down":
		return func(ctx *paint2d.Context, w, h float32) { panelThumb(ctx, t, w, h, u.down()) }
	case strings.HasPrefix(name, "capbtn."):
		return func(ctx *paint2d.Context, w, h float32) {
			panelKeyBody(ctx, t, paint2d.XYWH(0, 0, w, h), name == "capbtn.pressed", name == "capbtn.hover")
		}
	case strings.HasPrefix(name, "caption.plate") || strings.HasPrefix(name, "eq.plate") || strings.HasPrefix(name, "list.plate"):
		active := !strings.HasSuffix(name, ".inactive")
		return func(ctx *paint2d.Context, w, h float32) { panelPlate(ctx, t, w, h, active) }
	case strings.HasPrefix(name, "caption.") || strings.HasPrefix(name, "eq.caption") || strings.HasPrefix(name, "list.caption"):
		active := !strings.HasSuffix(name, ".inactive")
		logo := strings.HasPrefix(name, "caption.")
		return func(ctx *paint2d.Context, w, h float32) { panelCaption(ctx, t, u.slice, w, h, active, logo) }
	case name == "window.normal":
		return func(ctx *paint2d.Context, w, h float32) {
			fill(ctx, paint2d.XYWH(0, 0, w, h), t.outline)
			fill(ctx, paint2d.XYWH(1, 1, w-2, h-2), t.chrome)
		}
	case name == "focus.ring":
		return func(ctx *paint2d.Context, w, h float32) { focusDots(ctx, 0, 0, w, h, t.lcd) }
	case strings.HasPrefix(name, "key.") || strings.HasPrefix(name, "mini."):
		return func(ctx *paint2d.Context, w, h float32) { panelKey(ctx, t, name, w, h) }
	}
	return func(ctx *paint2d.Context, w, h float32) { fill(ctx, paint2d.XYWH(0, 0, w, h), t.chrome) }
}

// ---- the chrome ---------------------------------------------------------------

func panelChrome(ctx *paint2d.Context, t *panelTheme, r paint2d.Rect) {
	ctx.DrawRect(r, vpaint(r, stop(0, t.chromeTop), stop(.5, t.chrome), stop(1, t.chromeBottom)))
	switch t.texture {
	case "scan":
		scanlines(ctx, r, .18)
	case "grid":
		grid(ctx, r, 8, t.light.WithAlpha(.14))
	case "brushed":
		streaks(ctx, r, .09, 0x51ed)
	case "dots":
		for y := r.Min.Y + 2; y < r.Max.Y; y += 4 {
			for x := r.Min.X + 2 + float32(int(y)%8)/2; x < r.Max.X; x += 4 {
				px(ctx, x, y, 1, 1, t.shade.WithAlpha(.25))
			}
		}
	case "dither":
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X + float32(int(y)%2); x < r.Max.X; x += 2 {
				px(ctx, x, y, 1, 1, t.shade.WithAlpha(.35))
			}
		}
	case "lines":
		for y := r.Min.Y + 3; y < r.Max.Y; y += 3 {
			fill(ctx, paint2d.XYWH(r.Min.X, y, r.Dx(), 1), t.light.WithAlpha(.07))
		}
	case "rivets":
		streaks(ctx, r, .06, 0x2b7)
	}
}

func panelFrame(ctx *paint2d.Context, t *panelTheme, w, h float32) {
	panelChrome(ctx, t, paint2d.XYWH(0, 0, w, h))
	bevel(ctx, paint2d.XYWH(0, 0, w, h), t.light, t.shade, false)
	if t.texture == "rivets" {
		for _, p := range [][2]float32{{3, 3}, {w - 5, 3}, {3, h - 5}, {w - 5, h - 5}} {
			ctx.DrawCircle(paint2d.Pt(p[0]+1, p[1]+1), 1.5, paint2d.Fill(t.shade))
			px(ctx, p[0], p[1], 1, 1, t.light)
		}
	}
}

// panelWell is a recess round r, with the display's own texture.
func panelWell(ctx *paint2d.Context, t *panelTheme, r paint2d.Rect, display bool) {
	fill(ctx, r, t.wellRim)
	in := r.Inset(1)
	fill(ctx, in, t.well)
	bevel(ctx, r, t.shade, t.light, false)
	if !display {
		return
	}
	switch t.displayTexture {
	case "scan":
		scanlines(ctx, in, .3)
	case "grid":
		grid(ctx, in, 4, t.lcdDim.WithAlpha(.25))
	case "lcd":
		ctx.DrawRect(in, vpaint(in, stop(0, paint2d.RGBA(1, 1, 1, .12)), stop(1, paint2d.RGBA(0, 0, 0, .08))))
	}
}

func panelMainFace(ctx *paint2d.Context, t *panelTheme, l *skingen.LayoutSpec, w, h float32) {
	panelFrame(ctx, t, w, h)
	at := func(n string) paint2d.Rect { r, _ := slotAt(l, n); return r }
	d := at("display")
	panelWell(ctx, t, d.Inset(-1), true)
	if r := at("skin"); r.Dx() > 0 {
		// a narrow ribbed column at the display's left
		fill(ctx, r, mix(t.well, t.chrome, .5))
		for y := r.Min.Y + 2; y < r.Max.Y-1; y += 4 {
			fill(ctx, paint2d.XYWH(r.Min.X+2, y, r.Dx()-4, 1), t.lcdDim)
		}
	}
	ti := at("title")
	panelWell(ctx, t, paint2d.XYWH(ti.Min.X-3, ti.Min.Y-3, ti.Dx()+6, ti.Dy()+6), true)
	rate, freq := at("rate"), at("freq")
	panelWell(ctx, t, paint2d.XYWH(rate.Min.X-2, rate.Min.Y-2, rate.Dx()+4, rate.Dy()+4), true)
	panelWell(ctx, t, paint2d.XYWH(freq.Min.X-2, freq.Min.Y-2, freq.Dx()+4, freq.Dy()+4), true)
	pixLabel(ctx, rate.Max.X+4, rate.Min.Y, "KBPS", t.label)
	pixLabel(ctx, freq.Max.X+4, freq.Min.Y, "KHZ", t.label)
	for _, n := range []string{"mono", "stereo"} {
		r := at(n)
		word := strings.ToUpper(n)
		tw := float32(pixWidth(word))
		pixLabel(ctx, r.Min.X+float32(int((r.Dx()-tw)/2)), r.Min.Y+float32(int((r.Dy()-6)/2)), word, mix(t.label, t.chrome, .55))
	}
	for _, n := range []string{"volume", "balance"} {
		r := at(n)
		fill(ctx, paint2d.XYWH(r.Min.X-1, r.Min.Y-1, r.Dx()+2, r.Dy()+2), t.shade.WithAlpha(.35))
	}
	s := at("seek")
	panelWell(ctx, t, paint2d.XYWH(s.Min.X, s.Min.Y+2, s.Dx(), s.Dy()-4), false)
	// the shelf under the transport keys
	p, e := at("prev"), at("eject")
	shelf := paint2d.XYWH(p.Min.X-3, p.Min.Y-3, e.Max.X-p.Min.X+6, p.Dy()+6)
	fill(ctx, shelf, t.shade.WithAlpha(.45))
	bevel(ctx, shelf, t.shade, t.light, false)
	// the mark at the foot's right
	logo := paint2d.XYWH(w-27, h-25, 18, 18)
	emblem(ctx, logo, t.label, t.chromeBottom)
}

func panelEqFace(ctx *paint2d.Context, t *panelTheme, l *skingen.LayoutSpec, w, h float32) {
	panelFrame(ctx, t, w, h)
	at := func(n string) paint2d.Rect { r, _ := slotAt(l, n); return r }
	gr := at("graph")
	panelWell(ctx, t, gr.Inset(-1), true)
	fill(ctx, paint2d.XYWH(gr.Min.X, gr.Min.Y+gr.Dy()/2, gr.Dx(), 1), t.lcdDim)
	pre := at("preamp")
	labels := []string{"60", "170", "310", "600", "1K", "3K", "6K", "12K", "14K", "16K"}
	for i := 0; i < 10; i++ {
		r := at(fmt.Sprintf("band.%d", i))
		lw := float32(pixWidth(labels[i]))
		pixLabel(ctx, r.Min.X+float32(int((r.Dx()-lw)/2)), r.Max.Y+3, labels[i], t.label)
	}
	pixLabel(ctx, pre.Min.X-6, pre.Max.Y+3, "PRE", t.label)
	b0 := at("band.0")
	x := pre.Max.X + 4
	pixLabel(ctx, x, b0.Min.Y, "+12", t.label)
	pixLabel(ctx, x+4, b0.Min.Y+b0.Dy()/2-3, "0", t.label)
	pixLabel(ctx, x, b0.Max.Y-6, "-12", t.label)
}

func panelListFace(ctx *paint2d.Context, t *panelTheme, u use, w, h float32) {
	panelFrame(ctx, t, w, h)
	b := float32(u.slice[2])
	foot := paint2d.XYWH(2, h-b+2, w-4, b-4)
	fill(ctx, foot, t.shade.WithAlpha(.25))
	bevel(ctx, foot, t.shade, t.light, false)
}

func panelCaption(ctx *paint2d.Context, t *panelTheme, slice [4]int, w, h float32, active, logo bool) {
	r := paint2d.XYWH(0, 0, w, h)
	c := t.caption
	if !active {
		c = mix(t.caption, t.chrome, .6)
	}
	ctx.DrawRect(r, vpaint(r, stop(0, lift(c, .18)), stop(1, sink(c, .18))))
	bevel(ctx, r, t.light, t.shade, false)
	left := float32(slice[3])
	right := w - float32(slice[1])
	x0 := float32(4)
	if logo {
		emblem(ctx, paint2d.XYWH(3, (h-10)/2, 10, 10), t.captionInk, c)
		x0 = 16
	}
	ink := t.groove
	if !active {
		ink = mix(t.groove, c, .6)
	}
	// grooves run from the left band through the middle to the buttons
	for y := float32(int(h/2)) - 3; y <= float32(int(h/2))+3; y += 3 {
		fill(ctx, paint2d.XYWH(x0, y, right-x0-2, 1), ink)
		fill(ctx, paint2d.XYWH(x0, y+1, right-x0-2, 1), t.shade.WithAlpha(.5))
	}
	_ = left
}

func panelPlate(ctx *paint2d.Context, t *panelTheme, w, h float32, active bool) {
	c := t.caption
	if !active {
		c = mix(t.caption, t.chrome, .6)
	}
	r := paint2d.XYWH(0, 0, w, h)
	ctx.DrawRect(r, vpaint(r, stop(0, lift(c, .18)), stop(1, sink(c, .18))))
	fill(ctx, paint2d.XYWH(0, 1, w, 1), t.light.WithAlpha(.4))
	fill(ctx, paint2d.XYWH(0, h-1, w, 1), t.shade)
}

// ---- keys, sliders, thumbs ------------------------------------------------------

func panelKeyBody(ctx *paint2d.Context, t *panelTheme, r paint2d.Rect, down, hover bool) {
	top, bottom := t.keyTop, t.keyBottom
	if hover {
		top, bottom = lift(top, .15), lift(bottom, .1)
	}
	if down {
		top, bottom = sink(t.keyBottom, .15), t.keyTop
	}
	switch t.keys {
	case "pill":
		pillow(ctx, r, min(r.Dy()/2, 6), top, bottom, t.keyEdge, .35)
	case "round":
		pillow(ctx, r, min(r.Dy(), r.Dx())/3, top, bottom, t.keyEdge, .25)
	case "flat":
		fill(ctx, r, t.keyEdge)
		fill(ctx, r.Inset(1), mix(top, bottom, .5))
		if down {
			fill(ctx, r.Inset(1), t.keyBottom)
		}
	case "outline":
		fill(ctx, r, t.keyEdge)
		fill(ctx, r.Inset(1), top)
		if down {
			fill(ctx, r.Inset(1), t.keyLit.WithAlpha(.35))
		}
	default: // "bevel"
		ctx.DrawRect(r, vpaint(r, stop(0, top), stop(1, bottom)))
		fill(ctx, paint2d.XYWH(r.Min.X, r.Max.Y-1, r.Dx(), 1), t.keyEdge)
		fill(ctx, paint2d.XYWH(r.Max.X-1, r.Min.Y, 1, r.Dy()), t.keyEdge)
		bevel(ctx, paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx()-1, r.Dy()-1), lift(top, .5), sink(bottom, .25), down)
	}
}

var panelWords = map[string]string{
	"eq": "EQ", "pl": "PL", "shuffle": "SHUFFLE", "on": "ON", "auto": "AUTO", "presets": "PRESETS",
	"add": "ADD", "rem": "REM", "sel": "SEL", "misc": "MISC", "opts": "LIST",
}

func panelKey(ctx *paint2d.Context, t *panelTheme, name string, w, h float32) {
	parts := strings.Split(name, ".")
	key := parts[1]
	down := parts[len(parts)-1] == "down"
	lit := len(parts) > 2 && parts[2] == "on"
	r := paint2d.XYWH(0, 0, w, h)
	panelKeyBody(ctx, t, r, down, false)
	ink := t.keyInk
	if lit {
		ink = t.keyLit
	}
	d := float32(0)
	if down {
		d = 1
	}
	if key == "skin" {
		for y := float32(4); y < h-3; y += 4 {
			fill(ctx, paint2d.XYWH(2+d, y+d, w-4, 1), ink)
		}
		return
	}
	if word, ok := panelWords[key]; ok && parts[0] == "key" {
		tw := float32(pixWidth(word))
		x := float32(int((w-tw)/2)) + d
		if lit || isToggleKey(key) {
			lamp := t.keyEdge
			if lit {
				lamp = t.keyLit
			}
			px(ctx, 3+d, float32(int(h/2))-1+d, 2, 2, lamp)
			x = max(x, 7+d)
		}
		pixLabel(ctx, x, float32(int((h-6)/2))+d, word, ink)
		return
	}
	if parts[0] == "mini" {
		pixSymbol(ctx, key, w, h, ink)
		return
	}
	s := float32(int(min(w, h) * .5))
	if key == "repeat" && (lit || len(parts) > 2) {
		px(ctx, 3+d, float32(int(h/2))-1+d, 2, 2, map[bool]paint2d.Color{true: t.keyLit, false: t.keyEdge}[lit])
	}
	symbol(ctx, key, paint2d.XYWH(d, d, w, h), s, ink)
}

func isToggleKey(k string) bool {
	return k == "eq" || k == "pl" || k == "shuffle" || k == "on" || k == "auto"
}

// pixSymbol is a tiny transport symbol in whole pixels.
func pixSymbol(ctx *paint2d.Context, g string, w, h float32, c paint2d.Color) {
	pics := map[string][]string{
		"play":  {"#....", "##...", "###..", "####.", "###..", "##...", "#...."},
		"pause": {"##.##", "##.##", "##.##", "##.##", "##.##", "##.##", "##.##"},
		"stop":  {".....", "#####", "#####", "#####", "#####", "#####", "....."},
		"prev":  {"#...#", "#..##", "#.###", "#####", "#.###", "#..##", "#...#"},
		"next":  {"#...#", "##..#", "###.#", "#####", "###.#", "##..#", "#...#"},
		"eject": {"..#..", ".###.", "#####", ".....", "#####", ".....", "....."},
	}
	rows, ok := pics[g]
	if !ok {
		return
	}
	ox, oy := float32(int((w-5)/2)), float32(int((h-7)/2))
	for y, line := range rows {
		for x, ch := range line {
			if ch == '#' {
				px(ctx, ox+float32(x), oy+float32(y), 1, 1, c)
			}
		}
	}
}

func panelSliderBack(ctx *paint2d.Context, t *panelTheme, w, h, f float32, kind string) {
	if kind == "eq" {
		c := paint2d.XYWH(w/2-2, 0, 4, h)
		fill(ctx, c, t.wellRim)
		fill(ctx, c.Inset(1), t.well)
		lit := (h - 2) * f
		fill(ctx, paint2d.XYWH(c.Min.X+1, c.Max.Y-1-lit, 2, lit), mix(t.fillLow, t.fillHigh, f))
		for y := float32(3); y < h; y += 6 {
			px(ctx, 1, y, 2, 1, t.label.WithAlpha(.5))
			px(ctx, w-3, y, 2, 1, t.label.WithAlpha(.5))
		}
		return
	}
	c := paint2d.XYWH(0, h/2-3, w, 6)
	fill(ctx, c, t.wellRim)
	fill(ctx, c.Inset(1), t.well)
	in := c.Inset(1)
	col := mix(t.fillLow, t.fillHigh, f)
	if kind == "balance" {
		half := in.Dx() / 2 * f
		fill(ctx, paint2d.XYWH(in.Min.X+in.Dx()/2-half, in.Min.Y, max(half*2, 1), in.Dy()), col)
		return
	}
	fill(ctx, paint2d.XYWH(in.Min.X, in.Min.Y, in.Dx()*f, in.Dy()), col)
}

func panelThumb(ctx *paint2d.Context, t *panelTheme, w, h float32, down bool) {
	r := paint2d.XYWH(0, 0, w, h)
	panelKeyBody(ctx, t, r, down, false)
	ink := t.keyInk
	if down {
		ink = t.keyLit
	}
	if w > h {
		for x := float32(int(w/2)) - 2; x <= float32(int(w/2))+2; x += 2 {
			fill(ctx, paint2d.XYWH(x, 3, 1, h-6), ink.WithAlpha(.7))
		}
	} else {
		for y := float32(int(h/2)) - 2; y <= float32(int(h/2))+2; y += 2 {
			fill(ctx, paint2d.XYWH(3, y, w-6, 1), ink.WithAlpha(.7))
		}
	}
}

// ---- the nine themes --------------------------------------------------------------

func ramp(stops ...string) []paint2d.Color {
	out := make([]paint2d.Color, len(stops))
	for i, s := range stops {
		out[i] = rgb(s)
	}
	return out
}

var monoFonts = []string{"Liberation Mono", "DejaVu Sans Mono", "Nimbus Mono PS"}
var sansFonts = []string{"Liberation Sans", "DejaVu Sans", "Nimbus Sans"}

// Dossier: olive field-case chrome, a black display in pale green, a red
// stamp for what is lit.
func Dossier() *Plan {
	return panelSkin("dossier", "Winamp Classified", "Olive field-case chrome with riveted corners, khaki keys and a black display in pale green.", &geomDossier, &panelTheme{
		chrome: rgb("#4b5136"), chromeTop: rgb("#5d6444"), chromeBottom: rgb("#3a3f2a"), light: rgb("#7f8862"), shade: rgb("#23261a"), outline: rgb("#14160e"),
		well: rgb("#0b0d08"), wellRim: rgb("#1a1d12"), lcd: rgb("#c2f09a"), lcdDim: rgb("#3e5530"),
		keyTop: rgb("#a4a882"), keyBottom: rgb("#7b7f5d"), keyEdge: rgb("#2d3021"), keyInk: rgb("#23261a"), keyLit: rgb("#d0342a"),
		fillLow: rgb("#6f8f3a"), fillHigh: rgb("#d0342a"), caption: rgb("#3f4430"), captionInk: rgb("#e4e8cf"), groove: rgb("#8e976a"), label: rgb("#d6dcb8"),
		rows: rgb("#0b0d08"), rowText: rgb("#a9d68a"), rowCurrent: rgb("#ffffff"), rowSelected: rgb("#3e4a2c"), rowSelText: rgb("#e8ffd0"),
		ramp: ramp("#5f8f2a", "#a6c43c", "#e0c040", "#d0342a"), texture: "rivets", keys: "bevel", displayTexture: "", fonts: sansFonts,
	})
}

// Textmode: a character-cell screen: deep blue panels, double rules, grey
// text and cyan highlights.
func Textmode() *Plan {
	return panelSkin("textmode", "DOSamp", "A character-cell screen: deep blue panels, flat bracketed keys, grey text and cyan highlights.", &geomTextmode, &panelTheme{
		chrome: rgb("#10208a"), chromeTop: rgb("#10208a"), chromeBottom: rgb("#10208a"), light: rgb("#5a6ad8"), shade: rgb("#060c40"), outline: rgb("#000000"),
		well: rgb("#000000"), wellRim: rgb("#6a7ad0"), lcd: rgb("#c8c8c8"), lcdDim: rgb("#4a4a4a"),
		keyTop: rgb("#b8b8b8"), keyBottom: rgb("#b8b8b8"), keyEdge: rgb("#000000"), keyInk: rgb("#10208a"), keyLit: rgb("#007f7f"),
		fillLow: rgb("#00a8a8"), fillHigh: rgb("#55ffff"), caption: rgb("#00a8a8"), captionInk: rgb("#000000"), groove: rgb("#007878"), label: rgb("#ffff55"),
		rows: rgb("#000000"), rowText: rgb("#b8b8b8"), rowCurrent: rgb("#ffffff"), rowSelected: rgb("#00a8a8"), rowSelText: rgb("#000000"),
		ramp: ramp("#00a8a8", "#55ffff", "#ffffff"), texture: "dither", keys: "flat", displayTexture: "", fonts: monoFonts,
	})
}

// Bevel: the light grey bevelled desktop of the late nineties, a teal
// caption and square keys.
func Bevel() *Plan {
	return panelSkin("bevel", "Windows 98", "Light grey bevelled chrome, a teal caption, square raised keys and a black display.", &geomBevel, &panelTheme{
		chrome: rgb("#c3c3c3"), chromeTop: rgb("#c3c3c3"), chromeBottom: rgb("#c3c3c3"), light: rgb("#ffffff"), shade: rgb("#7b7b7b"), outline: rgb("#000000"),
		well: rgb("#000000"), wellRim: rgb("#404040"), lcd: rgb("#3cf06a"), lcdDim: rgb("#1c5a2c"),
		keyTop: rgb("#cfcfcf"), keyBottom: rgb("#bdbdbd"), keyEdge: rgb("#000000"), keyInk: rgb("#000000"), keyLit: rgb("#006c6c"),
		fillLow: rgb("#006c6c"), fillHigh: rgb("#00b0b0"), caption: rgb("#106c74"), captionInk: rgb("#ffffff"), groove: rgb("#5aa8ae"), label: rgb("#000000"),
		rows: rgb("#ffffff"), rowText: rgb("#000000"), rowCurrent: rgb("#106c74"), rowSelected: rgb("#106c74"), rowSelText: rgb("#ffffff"),
		ramp: ramp("#1c8a3c", "#3cf06a", "#e8f040"), texture: "", keys: "bevel", displayTexture: "", fonts: sansFonts,
	})
}

// Amber: a dark terminal in amber phosphor, scanlines on every surface.
func Amber() *Plan {
	return panelSkin("amber", "Winamp Classic Amber", "Near-black chrome in amber phosphor: scanlined displays, outlined amber keys and warm rules.", &geomAmber, &panelTheme{
		chrome: rgb("#1d150c"), chromeTop: rgb("#271c10"), chromeBottom: rgb("#140e07"), light: rgb("#5a3e1a"), shade: rgb("#080502"), outline: rgb("#000000"),
		well: rgb("#0a0603"), wellRim: rgb("#3a2810"), lcd: rgb("#ffb030"), lcdDim: rgb("#5a3a10"),
		keyTop: rgb("#2c1f10"), keyBottom: rgb("#20160b"), keyEdge: rgb("#c88420"), keyInk: rgb("#ffb030"), keyLit: rgb("#fff0b0"),
		fillLow: rgb("#8a5810"), fillHigh: rgb("#ffc040"), caption: rgb("#2c1f10"), captionInk: rgb("#ffb030"), groove: rgb("#7a5218"), label: rgb("#e09a30"),
		rows: rgb("#0a0603"), rowText: rgb("#e09a30"), rowCurrent: rgb("#ffe0a0"), rowSelected: rgb("#4a3010"), rowSelText: rgb("#ffd080"),
		ramp: ramp("#7a4a10", "#ffb030", "#fff0b0"), texture: "scan", keys: "outline", displayTexture: "scan", fonts: monoFonts,
	})
}

// Tally: a desk adding machine: beige case, cream keys with an orange one,
// a grey-green liquid-crystal window.
func Tally() *Plan {
	return panelSkin("tally", "Calculator", "A desk adding machine: speckled beige case, cream rounded keys and a grey-green liquid-crystal window.", &geomTally, &panelTheme{
		chrome: rgb("#d8cbaa"), chromeTop: rgb("#e6dbbf"), chromeBottom: rgb("#c6b791"), light: rgb("#f6efdc"), shade: rgb("#8f8366"), outline: rgb("#5a5140"),
		well: rgb("#a9b496"), wellRim: rgb("#5f6652"), lcd: rgb("#1f261a"), lcdDim: rgb("#8a9678"),
		keyTop: rgb("#fbf6e8"), keyBottom: rgb("#ddd3ba"), keyEdge: rgb("#7a6f56"), keyInk: rgb("#3a3326"), keyLit: rgb("#e2661c"),
		fillLow: rgb("#9a8a5c"), fillHigh: rgb("#e2661c"), caption: rgb("#b8a67a"), captionInk: rgb("#2c2619"), groove: rgb("#8f8366"), label: rgb("#4a4231"),
		rows: rgb("#efe8d4"), rowText: rgb("#2c2619"), rowCurrent: rgb("#c24e10"), rowSelected: rgb("#c9b98e"), rowSelText: rgb("#1a160e"),
		ramp: ramp("#3a4a2a", "#1f261a"), texture: "dots", keys: "round", displayTexture: "lcd", fonts: sansFonts,
	})
}

// Headunit: a dashboard stereo: black brushed fascia, rubber keys and an
// orange segment display.
func Headunit() *Plan {
	return panelSkin("headunit", "Sony CDX-MP3", "A dashboard stereo: black brushed fascia, rubber keys and an orange segment display.", &geomHeadunit, &panelTheme{
		chrome: rgb("#1b1c1e"), chromeTop: rgb("#2a2b2e"), chromeBottom: rgb("#0f1011"), light: rgb("#4a4c50"), shade: rgb("#050505"), outline: rgb("#000000"),
		well: rgb("#090604"), wellRim: rgb("#2a2420"), lcd: rgb("#ff8a24"), lcdDim: rgb("#4a2a10"),
		keyTop: rgb("#36383c"), keyBottom: rgb("#1c1d20"), keyEdge: rgb("#000000"), keyInk: rgb("#d8dadc"), keyLit: rgb("#ff8a24"),
		fillLow: rgb("#a04a10"), fillHigh: rgb("#ffa040"), caption: rgb("#202124"), captionInk: rgb("#d8dadc"), groove: rgb("#3c3e42"), label: rgb("#a8abb0"),
		rows: rgb("#0b0b0c"), rowText: rgb("#e89a5a"), rowCurrent: rgb("#ffffff"), rowSelected: rgb("#4a2a10"), rowSelText: rgb("#ffd0a0"),
		ramp: ramp("#c04a10", "#ff8a24", "#ffd060"), texture: "brushed", keys: "pill", displayTexture: "", fonts: sansFonts,
	})
}

// Graphing: a school calculator: slate case, dark rounded keys and a
// gridded olive screen.
func Graphing() *Plan {
	return panelSkin("graphing", "TI-82", "A school calculator: slate case, dark rounded keys and a gridded olive screen in dark ink.", &geomGraphing, &panelTheme{
		chrome: rgb("#4e5762"), chromeTop: rgb("#5d6772"), chromeBottom: rgb("#3e4650"), light: rgb("#7d8894"), shade: rgb("#262c33"), outline: rgb("#16191d"),
		well: rgb("#b9c39d"), wellRim: rgb("#3a3f33"), lcd: rgb("#272c22"), lcdDim: rgb("#8f9a78"),
		keyTop: rgb("#3a4049"), keyBottom: rgb("#262b31"), keyEdge: rgb("#101215"), keyInk: rgb("#e8ecf0"), keyLit: rgb("#7cc4f0"),
		fillLow: rgb("#3a6e98"), fillHigh: rgb("#7cc4f0"), caption: rgb("#3e4650"), captionInk: rgb("#e8ecf0"), groove: rgb("#6d7782"), label: rgb("#d8dee6"),
		rows: rgb("#c5ceab"), rowText: rgb("#272c22"), rowCurrent: rgb("#000000"), rowSelected: rgb("#8f9a78"), rowSelText: rgb("#101210"),
		ramp: ramp("#4a5240", "#272c22"), texture: "lines", keys: "round", displayTexture: "grid", fonts: sansFonts,
	})
}

// Radar: a phosphor scope: black glass, green wire rules and outlined keys.
func Radar() *Plan {
	return panelSkin("radar", "Radar Amp", "A phosphor scope: black glass with a green grid, wire-outlined keys and bright green readouts.", &geomRadar, &panelTheme{
		chrome: rgb("#020a03"), chromeTop: rgb("#031005"), chromeBottom: rgb("#010502"), light: rgb("#1f8a2c"), shade: rgb("#000000"), outline: rgb("#2ed84a"),
		well: rgb("#000000"), wellRim: rgb("#2ed84a"), lcd: rgb("#4cff6a"), lcdDim: rgb("#126a22"),
		keyTop: rgb("#031206"), keyBottom: rgb("#020a03"), keyEdge: rgb("#2ed84a"), keyInk: rgb("#4cff6a"), keyLit: rgb("#d8ffd8"),
		fillLow: rgb("#127a22"), fillHigh: rgb("#6cff8a"), caption: rgb("#041a08"), captionInk: rgb("#4cff6a"), groove: rgb("#1f8a2c"), label: rgb("#3cd858"),
		rows: rgb("#000000"), rowText: rgb("#3cd858"), rowCurrent: rgb("#d8ffd8"), rowSelected: rgb("#0e4a18"), rowSelText: rgb("#a8ffb8"),
		ramp: ramp("#126a22", "#4cff6a", "#d8ffd8"), texture: "grid", keys: "outline", displayTexture: "grid", fonts: sansFonts,
	})
}

// Forge: dark iron with rust and a red glow.
func Forge() *Plan {
	return panelSkin("forge", "Quake 3 Arena", "Dark iron plate with rivets and rust, red-lit keys and an ember display.", &geomForge, &panelTheme{
		chrome: rgb("#2e2420"), chromeTop: rgb("#3c2f29"), chromeBottom: rgb("#1e1714"), light: rgb("#6a5246"), shade: rgb("#0e0a08"), outline: rgb("#000000"),
		well: rgb("#0c0605"), wellRim: rgb("#3a1c14"), lcd: rgb("#ff5a3a"), lcdDim: rgb("#5a1e14"),
		keyTop: rgb("#4a3a33"), keyBottom: rgb("#2a201c"), keyEdge: rgb("#0e0a08"), keyInk: rgb("#e04a2a"), keyLit: rgb("#ffc070"),
		fillLow: rgb("#7a1e10"), fillHigh: rgb("#ff6a3a"), caption: rgb("#3a1a14"), captionInk: rgb("#f0d8c8"), groove: rgb("#7a3a28"), label: rgb("#c89a86"),
		rows: rgb("#0c0605"), rowText: rgb("#e07a5a"), rowCurrent: rgb("#ffe0c0"), rowSelected: rgb("#4a1a10"), rowSelText: rgb("#ffd0b0"),
		ramp: ramp("#7a1e10", "#ff5a3a", "#ffc070"), texture: "rivets", keys: "bevel", displayTexture: "scan", fonts: monoFonts,
	})
}
