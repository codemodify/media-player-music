package plans

import (
	"sort"
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// The redrawn skins. Each is a geometry table (geometry_<name>.go: the
// sizes, slices, slots, window frames and text roles its layout needs,
// numbers and names only) and a painter in Go that draws every sprite of
// it from paths, gradients and the app's own pixel faces. Nothing here
// opens a file: the generator reads no picture of anybody's.

// sprite is one sprite's place in the geometry: its sheet (an index into
// geometry.sheets), its design size, its nine-slice and how its middle
// fills, and whether it is a glyph tinted by its label's colour.
type sprite struct {
	name   string
	sheet  int
	w, h   int
	slice  [4]int
	middle string
	tint   bool
}

type sheetSpec struct {
	name      string
	pixelated bool
}

// geometry is a skin's layout without its art.
type geometry struct {
	family, base string
	sheets       []sheetSpec
	sprites      []sprite
	layouts      []skingen.LayoutSpec
	window       *skingen.WindowSpec
	parts        []skingen.PartBinding
	text         []skingen.TextRole
	colors       map[string]string
	metrics      map[string]float32
	params       map[string]float32
	fonts        *style.FontPrefs
}

// drawFunc paints one sprite in design pixels from (0, 0).
type drawFunc = func(ctx *paint2d.Context, w, h float32)

// skinArt is a redrawn skin: who it is, and how its sprites are drawn.
type skinArt struct {
	id, label, summary string
	g                  *geometry
	// paint returns the drawing of one sprite; the geometry and the layouts
	// it is used in are at hand through the use.
	paint func(u use) drawFunc
	// text and colors replace the geometry's text roles' and colours'
	// values where the new art wants others (nil keeps them).
	text   map[string]skingen.TextRole
	colors map[string]string
}

// use is a sprite as the painter sees it: its geometry, and where it is
// used (the first layout slot that names it, and the state it is bound
// to there).
type use struct {
	sprite
	layout *skingen.LayoutSpec // the layout whose art it is, or whose slot shows it
	slot   string
	state  string
}

func (u use) w32() float32 { return float32(u.w) }
func (u use) h32() float32 { return float32(u.h) }

// is reports whether the sprite's name starts with any of the prefixes.
func (u use) is(prefixes ...string) bool {
	for _, p := range prefixes {
		if u.name == p || strings.HasPrefix(u.name, p+".") {
			return true
		}
	}
	return false
}

// down, hover, on: the sprite's state, by the names' conventions.
func (u use) down() bool {
	return strings.HasSuffix(u.name, ".down") || strings.HasSuffix(u.name, ".pressed") || u.state == "pressed" || u.state == "checkedPressed"
}
func (u use) hover() bool { return strings.HasSuffix(u.name, ".hover") || u.state == "hover" }
func (u use) on() bool {
	n := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(u.name, ".down"), ".hover"), ".pressed")
	if strings.Count(n, ".") >= 2 && (strings.HasSuffix(n, ".on") || strings.HasSuffix(n, ".checked")) {
		return true
	}
	return strings.HasPrefix(u.state, "checked") && !strings.HasPrefix(u.name, "key.")
}

// base is the name without its state suffix.
func (u use) base() string {
	n := u.name
	for _, s := range []string{".down", ".hover", ".pressed", ".inactive", ".checked", ".disabled", ".on"} {
		n = strings.TrimSuffix(n, s)
	}
	return n
}

// frame is the number at the end of a frame sprite's name ("volume.frame.12"
// is 12), and how many frames of that name the geometry has.
func (u use) frame(g *geometry) (int, int) {
	i := strings.LastIndexByte(u.name, '.')
	n := 0
	for _, c := range u.name[i+1:] {
		if c < '0' || c > '9' {
			return -1, 0
		}
		n = n*10 + int(c-'0')
	}
	prefix := u.name[:i+1]
	count := 0
	for _, s := range g.sprites {
		rest, ok := strings.CutPrefix(s.name, prefix)
		if ok && rest != "" && strings.Trim(rest, "0123456789") == "" {
			count++
		}
	}
	return n, count
}

// build makes the plan: one sheet per geometry sheet, cells packed in
// shelves, and the manifest's every other block from the geometry.
func build(a skinArt) *skingen.Plan {
	g := a.g
	uses := map[string]use{}
	for i := range g.layouts {
		l := &g.layouts[i]
		if l.Art != "" {
			uses[l.Art] = use{layout: l, slot: "", state: "normal"}
		}
	}
	for i := range g.layouts {
		l := &g.layouts[i]
		names := make([]string, 0, len(l.Slots))
		for _, s := range l.Slots {
			names = append(names, s.Name)
		}
		for _, s := range l.Slots {
			for _, kv := range s.Art {
				if _, ok := uses[kv[1]]; !ok {
					uses[kv[1]] = use{layout: l, slot: s.Name, state: kv[0]}
				}
			}
		}
	}
	sheets := make([]*skingen.Sheet, len(g.sheets))
	byHeight := make([][]sprite, len(g.sheets))
	for i, s := range g.sheets {
		sheets[i] = &skingen.Sheet{Name: s.name, Pixelated: s.pixelated}
	}
	for _, sp := range g.sprites {
		byHeight[sp.sheet] = append(byHeight[sp.sheet], sp)
	}
	for i, sh := range sheets {
		list := byHeight[i]
		sort.SliceStable(list, func(a, b int) bool { return list[a].h > list[b].h })
		width := 512
		for _, sp := range list {
			width = max(width, sp.w)
		}
		x, y, row := 0, 0, 0
		for _, sp := range list {
			if x+sp.w > width {
				x, y, row = 0, y+row+2, 0
			}
			u := uses[sp.name]
			u.sprite = sp
			sh.Cells = append(sh.Cells, skingen.Cell{
				Name: sp.name, X: x, Y: y, W: sp.w, H: sp.h,
				Slice: sp.slice, Middle: sp.middle, Tint: sp.tint,
				Draw: a.paint(u),
			})
			x += sp.w + 2
			row = max(row, sp.h)
		}
		sh.W, sh.H = width, max(y+row, 1)
	}
	text := make([]skingen.TextRole, len(g.text))
	for i, t := range g.text {
		if r, ok := a.text[t.Name]; ok {
			r.Name = t.Name
			if r.Size == 0 {
				r.Size = t.Size
			}
			t = r
		}
		text[i] = t
	}
	colors := map[string]string{}
	for k, v := range g.colors {
		colors[k] = v
	}
	for k, v := range a.colors {
		colors[k] = v
	}
	if len(colors) == 0 {
		colors = nil
	}
	return &skingen.Plan{
		Name: a.id, Label: a.label, Year: 2026,
		Lineage: "media-player-music", Summary: a.summary,
		Family: g.family, Base: g.base,
		Sheets: sheets, Parts: g.parts, Text: text,
		Colors: colors, Metrics: g.metrics, Params: g.params,
		Fonts: g.fonts, Window: g.window, Layouts: g.layouts,
	}
}

// ---- a layout's slots, resolved ---------------------------------------------

// slotRect is a slot's rect in a box of w × h, by the format's anchors.
func slotRect(s skingen.SlotSpec, w, h float32) paint2d.Rect {
	x, y := float32(s.At[0]), float32(s.At[1])
	sw, sh := float32(s.At[2]), float32(s.At[3])
	switch {
	case s.StretchX:
		sw = w - x - sw
	case s.FromRight:
		x = w - x - sw
	}
	switch {
	case s.StretchY:
		sh = h - y - sh
	case s.FromBottom:
		y = h - y - sh
	}
	return paint2d.XYWH(x, y, sw, sh)
}

// slotAt is the rect of a slot of the layout at its design size.
func slotAt(l *skingen.LayoutSpec, name string) (paint2d.Rect, bool) {
	if l == nil {
		return paint2d.Rect{}, false
	}
	for _, s := range l.Slots {
		if s.Name == name {
			return slotRect(s, float32(l.W), float32(l.H)), true
		}
	}
	return paint2d.Rect{}, false
}

// layoutNamed is the geometry's layout of that name.
func (g *geometry) layout(name string) *skingen.LayoutSpec {
	for i := range g.layouts {
		if g.layouts[i].Name == name {
			return &g.layouts[i]
		}
	}
	return nil
}

// ---- drawing helpers shared by the redrawn skins ------------------------------

func rgb(s string) paint2d.Color { return skingen.Hex(s) }

func mix(a, b paint2d.Color, t float32) paint2d.Color { return a.Lerp(b, t) }

func lift(c paint2d.Color, t float32) paint2d.Color {
	return c.Lerp(paint2d.Color{R: 1, G: 1, B: 1, A: c.A}, t)
}

func sink(c paint2d.Color, t float32) paint2d.Color { return c.Lerp(paint2d.Color{A: c.A}, t) }

func fill(ctx *paint2d.Context, r paint2d.Rect, c paint2d.Color) { ctx.DrawRect(r, paint2d.Fill(c)) }

func vpaint(r paint2d.Rect, stops ...paint2d.GradientStop) paint2d.Paint {
	return paint2d.Linear(paint2d.LinearGradient{Start: paint2d.Pt(r.Min.X, r.Min.Y), End: paint2d.Pt(r.Min.X, r.Max.Y), Stops: stops})
}

func hpaint(r paint2d.Rect, stops ...paint2d.GradientStop) paint2d.Paint {
	return paint2d.Linear(paint2d.LinearGradient{Start: paint2d.Pt(r.Min.X, r.Min.Y), End: paint2d.Pt(r.Max.X, r.Min.Y), Stops: stops})
}

// round fills a rounded rect (a plain one at radius 0).
func round(ctx *paint2d.Context, r paint2d.Rect, radius float32, p paint2d.Paint) {
	if radius <= 0 {
		ctx.DrawRect(r, p)
		return
	}
	radius = min(radius, r.Dx()/2, r.Dy()/2)
	ctx.DrawRoundRect(r, radius, radius, p)
}

// ring strokes a rounded rect inside r.
func ring(ctx *paint2d.Context, r paint2d.Rect, radius, width float32, c paint2d.Color) {
	r = r.Inset(width / 2)
	if radius <= 0 {
		ctx.DrawRect(r, paint2d.StrokePaint(c, width))
		return
	}
	radius = min(max(radius-width/2, 0), r.Dx()/2, r.Dy()/2)
	ctx.DrawRoundRect(r, radius, radius, paint2d.StrokePaint(c, width))
}

// bevel is a square-edged raised (or, down, sunken) frame of one pixel.
func bevel(ctx *paint2d.Context, r paint2d.Rect, light, dark paint2d.Color, down bool) {
	if down {
		light, dark = dark, light
	}
	fill(ctx, paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), 1), light)
	fill(ctx, paint2d.XYWH(r.Min.X, r.Min.Y, 1, r.Dy()), light)
	fill(ctx, paint2d.XYWH(r.Min.X, r.Max.Y-1, r.Dx(), 1), dark)
	fill(ctx, paint2d.XYWH(r.Max.X-1, r.Min.Y, 1, r.Dy()), dark)
}

// pillow is a raised, rounded, lit key: a gradient body, an outline and a
// gloss along its top half.
func pillow(ctx *paint2d.Context, r paint2d.Rect, radius float32, top, bottom, edge paint2d.Color, gloss float32) {
	round(ctx, r, radius, paint2d.Fill(edge))
	in := r.Inset(1)
	round(ctx, in, radius-1, vpaint(in, stop(0, top), stop(1, bottom)))
	if gloss > 0 {
		g := paint2d.XYWH(in.Min.X+1, in.Min.Y+1, in.Dx()-2, in.Dy()*.45)
		round(ctx, g, radius-2, vpaint(g, stop(0, paint2d.RGBA(1, 1, 1, gloss)), stop(1, paint2d.RGBA(1, 1, 1, gloss*.15))))
	}
}

// orb is a lit round key.
func orb(ctx *paint2d.Context, c paint2d.Point, radius float32, top, bottom, edge paint2d.Color, gloss float32) {
	ctx.DrawCircle(c, radius, paint2d.Fill(edge))
	in := paint2d.XYWH(c.X-radius+1, c.Y-radius+1, 2*radius-2, 2*radius-2)
	ctx.DrawOval(in, vpaint(in, stop(0, top), stop(1, bottom)))
	if gloss > 0 {
		g := paint2d.XYWH(c.X-radius*.7, c.Y-radius*.88, radius*1.4, radius*.9)
		ctx.DrawOval(g, vpaint(g, stop(0, paint2d.RGBA(1, 1, 1, gloss)), stop(1, paint2d.RGBA(1, 1, 1, 0))))
	}
}

// well is a recess: the fill, a dark rim along the top and left and a light
// one along the bottom and right.
func well(ctx *paint2d.Context, r paint2d.Rect, radius float32, fillc, dark, light paint2d.Color) {
	round(ctx, r, radius, paint2d.Fill(light))
	round(ctx, paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx()-1, r.Dy()-1), radius, paint2d.Fill(dark))
	round(ctx, paint2d.XYWH(r.Min.X+1, r.Min.Y+1, r.Dx()-2, r.Dy()-2), radius-1, paint2d.Fill(fillc))
}

// streaks lays fine horizontal lines over r from a fixed seed: brushed
// metal, the same on every run.
func streaks(ctx *paint2d.Context, r paint2d.Rect, strength float32, seed uint32) {
	s := seed | 1
	for y := r.Min.Y; y < r.Max.Y; y++ {
		s ^= s << 13
		s ^= s >> 17
		s ^= s << 5
		v := float32(s%1000) / 1000
		c := paint2d.RGBA(1, 1, 1, strength*(.5-v)*2)
		if v >= .5 {
			c = paint2d.RGBA(0, 0, 0, strength*(v-.5)*1.4)
		}
		fill(ctx, paint2d.XYWH(r.Min.X, y, r.Dx(), 1), c)
	}
}

// scanlines darkens every other row of r.
func scanlines(ctx *paint2d.Context, r paint2d.Rect, alpha float32) {
	for y := r.Min.Y + 1; y < r.Max.Y; y += 2 {
		fill(ctx, paint2d.XYWH(r.Min.X, y, r.Dx(), 1), paint2d.RGBA(0, 0, 0, alpha))
	}
}

// grid draws a lattice of lines every step pixels over r.
func grid(ctx *paint2d.Context, r paint2d.Rect, step float32, c paint2d.Color) {
	for x := r.Min.X + step; x < r.Max.X; x += step {
		fill(ctx, paint2d.XYWH(x, r.Min.Y, 1, r.Dy()), c)
	}
	for y := r.Min.Y + step; y < r.Max.Y; y += step {
		fill(ctx, paint2d.XYWH(r.Min.X, y, r.Dx(), 1), c)
	}
}

// ---- the symbols on keys ----------------------------------------------------

// glyphFor is the symbol a key of that name carries ("" for none): the
// name's last word, matched against the words the faces use.
func glyphFor(name string) string {
	n := strings.TrimPrefix(name, "key.")
	n = strings.TrimPrefix(n, "mini.")
	for _, s := range []string{".down", ".hover", ".pressed", ".inactive", ".checked", ".disabled", ".on"} {
		n = strings.TrimSuffix(n, s)
		n = strings.TrimSuffix(n, s)
	}
	switch n {
	case "play", "pause", "stop", "prev", "next", "eject", "open", "shuffle", "repeat", "close", "minimize", "maximize", "restore", "eq", "pl", "playlist", "vis", "mute", "speaker", "speaker_mute", "menu", "skin", "add", "rem", "remove", "rew", "ffwd", "back", "forward", "compact", "mini", "full", "settings", "search", "list", "library", "trash", "logo", "up", "down", "info", "reset", "auto", "on", "presets", "sel", "misc", "opts":
		return n
	}
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		switch n[i+1:] {
		case "next", "prev", "close":
			return n[i+1:]
		}
	}
	return ""
}

// symbol draws a named symbol centred in r, s pixels across, in c.
func symbol(ctx *paint2d.Context, name string, r paint2d.Rect, s float32, c paint2d.Color) {
	cx, cy := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
	h := s / 2
	p := paint2d.NewPath()
	tri := func(x0, x1 float32, right bool) {
		if right {
			p.MoveTo(x0, cy-h)
			p.LineTo(x1, cy)
			p.LineTo(x0, cy+h)
		} else {
			p.MoveTo(x1, cy-h)
			p.LineTo(x0, cy)
			p.LineTo(x1, cy+h)
		}
		p.Close()
	}
	bar := func(x, w float32) { p.AddRect(paint2d.XYWH(x, cy-h, w, s)) }
	stroke := max(1, s/7)
	switch name {
	case "play":
		tri(cx-h*.8, cx+h, true)
	case "pause":
		bar(cx-h*.8, s*.3)
		bar(cx+h*.2, s*.3)
	case "stop":
		p.AddRect(paint2d.XYWH(cx-h*.85, cy-h*.85, s*.85, s*.85))
	case "next", "ffwd", "forward":
		tri(cx-h, cx, true)
		tri(cx, cx+h*.8, true)
		if name == "next" {
			bar(cx+h*.8, s*.18)
		}
	case "prev", "rew", "back":
		tri(cx, cx+h, false)
		tri(cx-h*.8, cx, false)
		if name == "prev" {
			bar(cx-h*.8-s*.18, s*.18)
		}
	case "eject", "open":
		p.MoveTo(cx-h, cy+h*.15)
		p.LineTo(cx, cy-h)
		p.LineTo(cx+h, cy+h*.15)
		p.Close()
		p.AddRect(paint2d.XYWH(cx-h, cy+h*.45, s, s*.22))
	case "up":
		p.MoveTo(cx-h, cy+h*.5)
		p.LineTo(cx, cy-h*.5)
		p.LineTo(cx+h, cy+h*.5)
		p.Close()
	case "down":
		p.MoveTo(cx-h, cy-h*.5)
		p.LineTo(cx, cy+h*.5)
		p.LineTo(cx+h, cy-h*.5)
		p.Close()
	case "close", "trash", "rem", "remove":
		q := paint2d.NewPath()
		q.MoveTo(cx-h*.75, cy-h*.75)
		q.LineTo(cx+h*.75, cy+h*.75)
		q.MoveTo(cx+h*.75, cy-h*.75)
		q.LineTo(cx-h*.75, cy+h*.75)
		ctx.DrawPath(q, paint2d.StrokePaint(c, stroke*1.2))
		return
	case "minimize":
		p.AddRect(paint2d.XYWH(cx-h*.75, cy+h*.35, s*.75, stroke*1.3))
	case "maximize", "full", "restore":
		ctx.DrawRect(paint2d.XYWH(cx-h*.75, cy-h*.75, s*.75, s*.75).Inset(stroke/2), paint2d.StrokePaint(c, stroke))
		fill(ctx, paint2d.XYWH(cx-h*.75, cy-h*.75, s*.75, stroke*1.4), c)
		return
	case "add":
		p.AddRect(paint2d.XYWH(cx-h*.8, cy-stroke/2, s*.8, stroke))
		p.AddRect(paint2d.XYWH(cx-stroke/2, cy-h*.8, stroke, s*.8))
	case "shuffle":
		q := paint2d.NewPath()
		q.MoveTo(cx-h, cy-h*.6)
		q.CubicTo(cx, cy-h*.6, cx, cy+h*.6, cx+h*.7, cy+h*.6)
		q.MoveTo(cx-h, cy+h*.6)
		q.CubicTo(cx, cy+h*.6, cx, cy-h*.6, cx+h*.7, cy-h*.6)
		ctx.DrawPath(q, paint2d.StrokePaint(c, stroke))
		p.MoveTo(cx+h*.55, cy-h)
		p.LineTo(cx+h, cy-h*.6)
		p.LineTo(cx+h*.55, cy-h*.2)
		p.Close()
		p.MoveTo(cx+h*.55, cy+h*.2)
		p.LineTo(cx+h, cy+h*.6)
		p.LineTo(cx+h*.55, cy+h)
		p.Close()
	case "repeat":
		q := paint2d.NewPath()
		q.MoveTo(cx+h*.6, cy-h*.55)
		q.LineTo(cx-h*.55, cy-h*.55)
		q.CubicTo(cx-h, cy-h*.55, cx-h, cy+h*.55, cx-h*.55, cy+h*.55)
		q.LineTo(cx+h*.55, cy+h*.55)
		q.CubicTo(cx+h, cy+h*.55, cx+h, cy-h*.1, cx+h*.85, cy-h*.2)
		ctx.DrawPath(q, paint2d.StrokePaint(c, stroke))
		p.MoveTo(cx+h*.35, cy-h)
		p.LineTo(cx+h*.85, cy-h*.55)
		p.LineTo(cx+h*.35, cy-h*.1)
		p.Close()
	case "eq", "settings", "presets":
		for i := 0; i < 3; i++ {
			x := cx - h*.7 + float32(i)*h*.7
			p.AddRect(paint2d.XYWH(x-stroke/2, cy-h, stroke, s))
			k := []float32{-.3, .35, -.05}[i]
			p.AddRect(paint2d.XYWH(x-stroke*1.4, cy+k*s-stroke, stroke*2.8, stroke*2))
		}
	case "pl", "playlist", "list", "library", "menu":
		for i := 0; i < 3; i++ {
			y := cy - h*.7 + float32(i)*h*.7
			p.AddRect(paint2d.XYWH(cx-h*.85, y-stroke/2, stroke*1.2, stroke))
			p.AddRect(paint2d.XYWH(cx-h*.85+stroke*2, y-stroke/2, s*.85-stroke*2, stroke))
		}
	case "vis":
		for i, k := range []float32{.45, .9, .65, 1, .35} {
			bh := s * k
			p.AddRect(paint2d.XYWH(cx-h+float32(i)*s/5+stroke*.2, cy+h-bh, s/5-stroke*.4, bh))
		}
	case "mute", "speaker", "speaker_mute":
		p.MoveTo(cx-h, cy-h*.35)
		p.LineTo(cx-h*.45, cy-h*.35)
		p.LineTo(cx+h*.1, cy-h*.9)
		p.LineTo(cx+h*.1, cy+h*.9)
		p.LineTo(cx-h*.45, cy+h*.35)
		p.LineTo(cx-h, cy+h*.35)
		p.Close()
		q := paint2d.NewPath()
		if name == "speaker" {
			q.AddArc(paint2d.Pt(cx+h*.1, cy), h*.55, h*.55, -0.9, 1.8)
			q.MoveTo(cx+h*.1+h*.9*0.6216, cy-h*.9*0.7833)
			q.AddArc(paint2d.Pt(cx+h*.1, cy), h*.9, h*.9, -0.9, 1.8)
		} else {
			q.MoveTo(cx+h*.35, cy-h*.4)
			q.LineTo(cx+h, cy+h*.4)
			q.MoveTo(cx+h, cy-h*.4)
			q.LineTo(cx+h*.35, cy+h*.4)
		}
		ctx.DrawPath(q, paint2d.StrokePaint(c, stroke))
	case "skin", "logo":
		ctx.DrawCircle(paint2d.Pt(cx, cy), h*.85, paint2d.StrokePaint(c, stroke))
		ctx.DrawCircle(paint2d.Pt(cx, cy), h*.3, paint2d.Fill(c))
		return
	case "compact", "mini":
		ctx.DrawRect(paint2d.XYWH(cx-h, cy-h*.6, s, s*.6*2).Inset(stroke/2), paint2d.StrokePaint(c, stroke))
		fill(ctx, paint2d.XYWH(cx-h, cy+h*.1, s, stroke*1.4), c)
		return
	case "search":
		ctx.DrawCircle(paint2d.Pt(cx-h*.2, cy-h*.2), h*.55, paint2d.StrokePaint(c, stroke))
		q := paint2d.NewPath()
		q.MoveTo(cx+h*.2, cy+h*.2)
		q.LineTo(cx+h*.9, cy+h*.9)
		ctx.DrawPath(q, paint2d.StrokePaint(c, stroke*1.3))
		return
	case "info":
		ctx.DrawCircle(paint2d.Pt(cx, cy), h*.9, paint2d.StrokePaint(c, stroke))
		fill(ctx, paint2d.XYWH(cx-stroke/2, cy-h*.1, stroke, h*.6), c)
		fill(ctx, paint2d.XYWH(cx-stroke/2, cy-h*.5, stroke, stroke), c)
		return
	default:
		ctx.DrawCircle(paint2d.Pt(cx, cy), h*.35, paint2d.Fill(c))
		return
	}
	ctx.DrawPath(p, paint2d.Fill(c))
}
