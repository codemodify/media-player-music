package plans

import (
	"strings"

	"github.com/codemodify/media-player-music/internal/skins/panel"
	"goframework.io/graphics-paint2d"
)

// Winamp is the compact player's base-skin look of the era: slate-blue
// bevelled chrome, title bands with a gold groove either side of the words,
// a black display with thin green segments, grey keys, an orange volume bar
// and a green balance bar.
//
// Its original 1× artwork is drawn in whole pixels. Chrome and transport
// outlines have edge-aware 2× variants; text and fine labels keep their pixel
// grid. It is a *panel*: most of what is on the sheet is
// not bound to a part
// at all. The three window faces are pictures with the wells, grooves and
// printed labels in them, and the keys, digits, lamps and letters are loose
// sprites the player paints into its own layout (style.DrawSkinSprite).
// Where each one goes is internal/skins/panel, which this file reads
// the same rects from, so the art and the app cannot disagree about a key.
//
// Every colour below was chosen by eye against the look of the era and then
// written down as a number; no pixel of anybody's skin is in here, and the
// display's digits and capitals are the application's own (pixfont.go).
//
// Nearly everything is a panel sprite (pixfont.go, panel): a piece the
// player draws at its own size. Pixel text stays on the device grid at
// fractional scales while chrome uses its higher-resolution sheet. The frame,
// playlist chrome and playlist wells
// use slices so their edges keep their proportions when the window grows.

// The classic palette.
const (
	clInk     = "#0c0c14" // the outline round every face
	clBody    = "#393959" // the chrome
	clBodyTop = "#3f3f63"
	clBodyBot = "#30304c"
	clLight   = "#6c6c8c" // a bevel's lit edge on the chrome
	clShade   = "#1c1c2c" // and its shadowed one

	clWellC = "#030305" // a display well
	clDot   = "#0d0d17" // the grid in the clock's well
	clLCD   = "#00e000" // lit segments and letters
	clLCDLo = "#006000"

	clKey   = "#bdced6" // a key's face
	clKeyHi = "#eaf6fd"
	clKeyLo = "#76818f"
	clKeyDn = "#a1b1bb"
	clGlyph = "#56627a" // the mark on a key
	clGlyHi = "#9eacbc"

	clCream  = "#ffefa6" // the groove, top to bottom
	clSilver = "#d0d3dc"
	clGoldDk = "#443e32"
	clGold   = "#d4bd7b"

	clOrange = "#de711d"
	clGreen  = "#1b9a0c"
	clYellow = "#e4d12f"
	clLabel  = "#d6dae4" // the printed labels
	clDim    = "#5c6072"
	clDB     = "#d9ac2a"
)

// Winamp builds the skin's plan.
func Winamp() *Plan {
	p := winamp()
	polish(p)
	return p
}

func winamp() *Plan {
	f := panel.Classic
	sh := &Sheet{Name: "chrome", Pixelated: true}
	l := &lay{sh: sh, gap: 2}
	p := &Plan{
		Name:    "slate",
		Label:   "Slate",
		Year:    2026,
		Lineage: "uitoolkit",
		Summary: "The compact player's base-skin look of the era: slate-blue chrome, gold grooves, a black display with green segments — a pixel panel the player lays its own keys on.",
		Family:  "dark",
		Base:    "win95",
		Sheets:  []*Sheet{sh},
	}

	// ---- the three faces ----------------------------------------------------
	//
	// The strip and equaliser contain their stationary wells and labels.
	// The playlist draws its wells separately so they follow its row viewport
	// and anchored footer when resized.
	cw := f.ContentW()
	l.row(f.ContentH(panel.MainH))
	l.panel("main.face", cw, f.ContentH(panel.MainH), false, func(ctx *paint2d.Context, w, h float32) {
		clMainFace(ctx, w, h, f)
	})
	l.row(f.EqContentH())
	l.panel("eq.face", cw, f.EqContentH(), false, func(ctx *paint2d.Context, w, h float32) {
		clEqFace(ctx, w, h, f)
	})
	l.row(f.ContentH(panel.ListH))
	l.cell("list.face", cw, f.ContentH(panel.ListH), [4]int{6, 8, 36, 8}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clListFace(ctx, w, h, f)
	})
	clListPieces(l, f)

	// ---- the keys ------------------------------------------------------------
	//
	// Every key in four states: at rest, held down, and the same two with
	// its lamp lit for the ones that are toggles. The mark or the word is
	// printed on the face, so a key is one sprite rather than a face and a
	// glyph that have to agree.
	m := f.Main
	type key struct {
		name   string
		r      panel.R
		print  func(ctx *paint2d.Context, w, h float32, down bool)
		toggle bool
	}
	glyph := func(g string) func(*paint2d.Context, float32, float32, bool) {
		return func(ctx *paint2d.Context, w, h float32, down bool) {
			clTransportGlyph(ctx, w, h, g, down)
		}
	}
	word := func(s string) func(*paint2d.Context, float32, float32, bool) {
		return func(ctx *paint2d.Context, w, h float32, down bool) {
			d := float32(0)
			if down {
				d = 1
			}
			tw := pixWidth(s)
			pixLabel(ctx, float32(int((w-float32(tw))/2))+d, float32(int((h-6)/2))+d, s, Hex(clGlyph))
		}
	}
	lampWord := func(s string) func(*paint2d.Context, float32, float32, bool) {
		return func(ctx *paint2d.Context, w, h float32, down bool) {
			d := float32(0)
			if down {
				d = 1
			}
			tw := pixWidth(s)
			x := float32(int((w-float32(tw)-4)/2)) + 4
			pixLabel(ctx, x+d, float32(int((h-6)/2))+d, s, Hex(clGlyph))
		}
	}
	keys := []key{
		{"prev", m.Prev, glyph("prev"), false},
		{"play", m.Play, glyph("play"), false},
		{"pause", m.Pause, glyph("pause"), false},
		{"stop", m.Stop, glyph("stop"), false},
		{"next", m.Next, glyph("next"), false},
		{"eject", m.Eject, glyph("eject"), false},
		{"shuffle", m.Shuffle, lampWord("SHUFFLE"), true},
		{"repeat", m.Repeat, func(ctx *paint2d.Context, w, h float32, down bool) {
			d := float32(0)
			if down {
				d = 1
			}
			clRepeatGlyph(ctx, 8+d, float32(int((h-6)/2))+d)
		}, true},
		{"eq", m.EQ, lampWord("EQ"), true},
		{"pl", m.PL, lampWord("PL"), true},
		{"on", f.Eq.On, lampWord("ON"), true},
		{"auto", f.Eq.Auto, lampWord("AUTO"), true},
		{"presets", f.Eq.Presets, word("PRESETS"), false},
		{"add", f.List.Add, word("ADD"), false},
		{"rem", f.List.Rem, word("REM"), false},
		{"sel", f.List.Sel, word("SEL"), false},
		{"misc", f.List.Misc, word("MISC"), false},
		{"opts", f.List.Opts, func(ctx *paint2d.Context, w, h float32, down bool) {
			d := float32(0)
			if down {
				d = 1
			}
			pixLabel(ctx, float32(int((w-float32(pixWidth("LIST")))/2))+d, 2+d, "LIST", Hex(clGlyph))
			pixLabel(ctx, float32(int((w-float32(pixWidth("OPTS")))/2))+d, 10+d, "OPTS", Hex(clGlyph))
		}, false},
	}
	for _, k := range keys {
		k := k
		l.row(k.r.H())
		states := []string{"", ".down"}
		if k.toggle {
			states = append(states, ".on", ".on.down")
		}
		for _, st := range states {
			down := strings.HasSuffix(st, "down")
			on := strings.HasPrefix(st, ".on")
			l.panel("key."+k.name+st, k.r.W(), k.r.H(), false, func(ctx *paint2d.Context, w, h float32) {
				clKeyFace(ctx, w, h, down)
				if k.toggle {
					clKeyLamp(ctx, 3, 3, on, down)
				}
				k.print(ctx, w, h, down)
			})
		}
	}

	// The option column the skin key stands in: the four letters of what
	// it does, stood one above the other, on the display's black.
	l.row(m.Skin.H())
	for _, st := range []string{"", ".down"} {
		down := st != ""
		l.panel("key.skin"+st, m.Skin.W(), m.Skin.H(), false, func(ctx *paint2d.Context, w, h float32) {
			clSkinColumn(ctx, w, h, down)
		})
	}

	// The small transport under the playlist: marks only, in gold, as the
	// era printed them straight on the chrome.
	l.row(8)
	for i, g := range []string{"prev", "play", "pause", "stop", "next", "eject"} {
		r := f.List.Mini[i]
		for _, st := range []string{"", ".down"} {
			col := Hex(clGold)
			if st != "" {
				col = Hex(clCream)
			}
			g := g
			l.panel("mini."+g+st, r.W(), r.H(), false, func(ctx *paint2d.Context, w, h float32) {
				clMiniGlyph(ctx, g, col)
			})
		}
	}

	// ---- the display ------------------------------------------------------
	l.row(13)
	for _, d := range "0123456789-" {
		d := d
		l.panel("led."+string(d), 9, 13, false, func(ctx *paint2d.Context, w, h float32) {
			segDigit(ctx, w, h, 1, segDigits[d], Hex(clLCD))
		})
	}
	l.panel("led.colon", 3, 13, false, func(ctx *paint2d.Context, w, h float32) {
		px(ctx, 1, 4, 1, 2, Hex(clLCD))
		px(ctx, 1, 8, 1, 2, Hex(clLCD))
	})
	l.row(9)
	l.panel("state.play", 9, 9, false, func(ctx *paint2d.Context, w, h float32) {
		px(ctx, 0, 1, 2, 2, Hex(clLCD)) // the working dot
		for i := float32(0); i < 4; i++ {
			px(ctx, 3+i, 1+i, 1, 7-2*i, Hex(clLCD))
		}
	})
	l.panel("state.pause", 9, 9, false, func(ctx *paint2d.Context, w, h float32) {
		px(ctx, 0, 1, 2, 2, Hex(clLCDLo))
		px(ctx, 3, 1, 2, 7, Hex(clLCD))
		px(ctx, 6, 1, 2, 7, Hex(clLCD))
	})
	l.panel("state.stop", 9, 9, false, func(ctx *paint2d.Context, w, h float32) {
		px(ctx, 0, 1, 2, 2, Hex("#a02010"))
		px(ctx, 3, 2, 5, 5, Hex(clLCD))
	})
	l.row(10)
	l.panel("lamp.mono", f.Main.Mono.W(), f.Main.Mono.H(), false, func(ctx *paint2d.Context, w, h float32) {
		clLamp(ctx, w, h, "MONO", true)
	})
	l.panel("lamp.stereo", f.Main.Stereo.W(), f.Main.Stereo.H(), false, func(ctx *paint2d.Context, w, h float32) {
		clLamp(ctx, w, h, "STEREO", true)
	})

	// The capitals, white, for the app to set in whatever ink it needs.
	pixText(l, func(ctx *paint2d.Context, x, y float32) {
		px(ctx, x, y, 1, 1, paint2d.RGB(1, 1, 1))
	})

	// ---- the sliders' thumbs ------------------------------------------------
	l.row(11)
	for _, st := range []string{"", ".down"} {
		down := st != ""
		l.panel("thumb"+st, 14, 11, false, func(ctx *paint2d.Context, w, h float32) {
			clThumb(ctx, w, h, down)
		})
		l.panel("seek.thumb"+st, 29, 10, false, func(ctx *paint2d.Context, w, h float32) {
			clSeekThumb(ctx, w, h, down)
		})
		l.panel("eq.thumb"+st, 11, 11, false, func(ctx *paint2d.Context, w, h float32) {
			clEqThumb(ctx, w, h, down)
		})
		l.panel("list.thumb"+st, 8, 18, false, func(ctx *paint2d.Context, w, h float32) {
			clListThumb(ctx, w, h, down)
		})
	}

	// ---- the frame ------------------------------------------------------------
	//
	// These are sliced for real: the engine stretches them to the window.
	l.row(14)
	l.cell("window.normal", 8, 8, [4]int{2, 2, 2, 2}, "", false, func(ctx *paint2d.Context, w, h float32) {
		px(ctx, 0, 0, w, h, Hex(clInk))
		px(ctx, 1, 1, w-2, h-2, Hex(clLight))
		px(ctx, 2, 2, w-3, h-3, Hex(clShade))
		px(ctx, 2, 2, w-4, h-4, Hex(clBody))
	})
	// The band: the mark at the left, then the groove, which runs on under
	// the title and is stopped either side of it by the plate.
	l.cell("caption.normal", 64, f.Caption, [4]int{0, 30, 0, 22}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clBand(ctx, w, h, true)
	})
	l.cell("caption.inactive", 64, f.Caption, [4]int{0, 30, 0, 22}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clBand(ctx, w, h, false)
	})
	l.cell("caption.plate", 12, f.Caption, [4]int{0, 4, 0, 4}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clPlate(ctx, w, h, true)
	})
	l.cell("caption.plate.inactive", 12, f.Caption, [4]int{0, 4, 0, 4}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clPlate(ctx, w, h, false)
	})
	l.cell("capbtn.normal", 9, 9, [4]int{2, 2, 2, 2}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clCapKey(ctx, w, h, Hex("#2c2c40"), false)
	})
	l.cell("capbtn.hover", 9, 9, [4]int{2, 2, 2, 2}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clCapKey(ctx, w, h, Hex("#3c3c56"), false)
	})
	l.cell("capbtn.pressed", 9, 9, [4]int{2, 2, 2, 2}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clCapKey(ctx, w, h, Hex("#1a1a28"), true)
	})
	// A skin may re-draw the focus ring and may never remove it: this one
	// is the dotted rectangle of the era, in the display's green.
	l.cell("focus.ring", 8, 8, [4]int{2, 2, 2, 2}, "none", false, func(ctx *paint2d.Context, w, h float32) {
		focusDots(ctx, 0, 0, w, h, Hex(clLCD))
	})
	l.close()

	// ---- bindings -------------------------------------------------------------
	//
	// Deliberately few. The panel is the app's; what is bound here is the
	// frame the engine draws, and the focus ring. Everything else — the
	// menus the player opens, a tooltip — is win95's, which is what the
	// desktop under a player of this era looked like anyway.
	p.Text = []TextRole{
		{Name: "control", Color: "#101018", Disabled: "#6c7082"},
		// Capitals, as the era's title bars printed them: the skin's choice
		// about how its art reads. The windows keep the player's titles.
		{Name: "caption", Color: "#f4f6fb", Disabled: "#9aa0b8", Size: 9, Bold: true, Upper: true},
		{Name: "capkey", Color: clGold, Hover: clCream, Pressed: clCream, Disabled: "#5c5a50"},
	}
	p.Parts = []PartBinding{
		{Part: "window", States: [][2]string{{"normal", "window.normal"}}},
		{Part: "caption", Text: "caption", States: [][2]string{
			{"normal", "caption.normal"}, {"inactive", "caption.inactive"},
		}},
		{Part: "caption.title", States: [][2]string{
			{"normal", "caption.plate"}, {"inactive", "caption.plate.inactive"},
		}},
		{Part: "caption.button", Text: "capkey", States: [][2]string{
			{"normal", "capbtn.normal"}, {"hover", "capbtn.hover"}, {"pressed", "capbtn.pressed"},
		}},
		{Part: "focus", States: [][2]string{{"normal", "focus.ring"}}},
	}
	p.Colors = map[string]string{
		"focus": clLCD,
	}
	// A menu opens inside the window it is asked from, and the strip is 116
	// design pixels tall: rows of 20 let the skin menu's four fit in it.
	p.Metrics = map[string]float32{"menuItemH": 20}
	p.Window = &WindowSpec{
		Border:  f.Border,
		Caption: f.Caption,
		Layout:  ":minimize,close",
	}
	p.Layouts = winampLayouts(f, sh)
	mainCaptionRole(p)
	return p
}

// ---- the faces ------------------------------------------------------------------

// clChrome fills a face with the chrome: a gentle fall from a lit top to a
// darker foot, one flat colour per row, which is how a gradient was drawn on
// a panel with a palette.
func clChrome(ctx *paint2d.Context, w, h float32) {
	top, bot := Hex(clBodyTop), Hex(clBodyBot)
	for y := float32(0); y < h; y++ {
		px(ctx, 0, y, w, 1, lerpColor(top, bot, y/max(h-1, 1)))
	}
}

// clInset is the panel's inner frame: a lit line and a shadowed line a few
// pixels in from the edge, the tell of a face pressed out of one sheet.
func clInset(ctx *paint2d.Context, x, y, w, h float32) {
	lit, dark := Hex(clLight), Hex(clShade)
	px(ctx, x, y, w, 1, dark)
	px(ctx, x, y, 1, h, dark)
	px(ctx, x+1, y+1, w-2, 1, lit)
	px(ctx, x+1, y+1, 1, h-2, lit)
	px(ctx, x+1, y+h-1, w-1, 1, lit)
	px(ctx, x+w-1, y+1, 1, h-1, lit)
	px(ctx, x+2, y+h-2, w-3, 1, dark)
	px(ctx, x+w-2, y+2, 1, h-3, dark)
}

// clWell is a black well sunk into the chrome: a shadowed top and left, a lit
// bottom and right. dots puts the display's faint grid in it.
func clWell(ctx *paint2d.Context, r panel.R, dots bool) {
	x, y, w, h := float32(r.X()), float32(r.Y()), float32(r.W()), float32(r.H())
	px(ctx, x, y, w, h, Hex(clWellC))
	if dots {
		for yy := y + 2; yy < y+h-1; yy += 2 {
			for xx := x + 2; xx < x+w-1; xx += 2 {
				px(ctx, xx, yy, 1, 1, Hex(clDot))
			}
		}
	}
	px(ctx, x, y, w, 1, Hex(clShade))
	px(ctx, x, y, 1, h, Hex(clShade))
	px(ctx, x, y+h-1, w, 1, Hex(clLight))
	px(ctx, x+w-1, y, 1, h, Hex(clLight))
}

// clGroove is a slot sunk into the chrome for a thumb to run in.
func clGroove(ctx *paint2d.Context, r panel.R) {
	x, y, w, h := float32(r.X()), float32(r.Y()), float32(r.W()), float32(r.H())
	px(ctx, x, y, w, h, Hex("#2a2a42"))
	px(ctx, x, y, w, 1, Hex(clShade))
	px(ctx, x, y, 1, h, Hex(clShade))
	px(ctx, x+1, y+1, w-2, 1, Hex("#222236"))
	px(ctx, x, y+h-1, w, 1, Hex(clLight))
	px(ctx, x+w-1, y, 1, h, Hex(clLight))
}

// clBar is a coloured bar a slider's thumb rides on, printed on the chrome:
// a lit top row, the colour, a dark foot, and an outline.
func clBar(ctx *paint2d.Context, x, y, w, h float32, col string) {
	c := Hex(col)
	px(ctx, x+1, y, w-2, h, Hex(clInk))
	px(ctx, x, y+1, w, h-2, Hex(clInk))
	px(ctx, x+1, y+1, w-2, h-2, c)
	px(ctx, x+1, y+1, w-2, 1, lerpColor(c, paint2d.RGB(1, 1, 1), 0.35))
	px(ctx, x+1, y+h-2, w-2, 1, lerpColor(c, paint2d.RGB(0, 0, 0), 0.35))
}

func clMainFace(ctx *paint2d.Context, w, h float32, f *panel.Face) {
	m := f.Main
	clChrome(ctx, w, h)
	clInset(ctx, 4, 0, w-8, h-2)

	clWell(ctx, m.Display, true)
	clWell(ctx, m.Title, false)
	clWell(ctx, m.Rate, false)
	clWell(ctx, m.Freq, false)
	pixLabel(ctx, float32(m.Rate.Right()+3), float32(m.RateText.Y()), "KBPS", Hex(clLabel))
	pixLabel(ctx, float32(m.Freq.Right()+3), float32(m.FreqText.Y()), "KHZ", Hex(clLabel))
	clLamp(ctx2(ctx, m.Mono), float32(m.Mono.W()), float32(m.Mono.H()), "MONO", false)
	restore(ctx)
	clLamp(ctx2(ctx, m.Stereo), float32(m.Stereo.W()), float32(m.Stereo.H()), "STEREO", false)
	restore(ctx)

	// The two short bars the volume and balance thumbs ride on.
	v, b := m.Volume, m.Balance
	clBar(ctx, float32(v.X()), float32(v.Y()+4), float32(v.W()), 5, clOrange)
	clBar(ctx, float32(b.X()), float32(b.Y()+4), float32(b.W()), 5, clGreen)

	clGroove(ctx, m.Seek)
	clMark(ctx, float32(m.Mark.X()), float32(m.Mark.Y()))
}

func clEqFace(ctx *paint2d.Context, w, h float32, f *panel.Face) {
	e := f.Eq
	clChrome(ctx, w, h)
	clInset(ctx, 4, 0, w-8, h-2)

	// The graph: a faint rule per band and a baseline, on the chrome. The
	// curve is the app's to draw.
	g := e.Graph
	gx, gy, gw, gh := float32(g.X()), float32(g.Y()), float32(g.W()), float32(g.H())
	// No well: the era's graph was ruled straight onto the chrome.
	for i := float32(0); i < 10; i++ {
		x := gx + 4 + i*float32(int((gw-8)/9))
		px(ctx, x, gy, 1, gh, Hex("#5e5e7c"))
		px(ctx, x+1, gy, 1, gh, Hex("#2a2a40"))
	}

	fader := func(r panel.R) {
		x, y, fw, fh := float32(r.X()), float32(r.Y()), float32(r.W()), float32(r.H())
		cx := x + float32(int(fw/2))
		// The yellow bar the thumb rides, and a tick either side of it at
		// the top, the middle and the foot of its travel.
		clBar(ctx, cx-3, y, 6, fh, clYellow)
		for _, ty := range []float32{y + 2, y + float32(int(fh/2)) - 1, y + fh - 4} {
			px(ctx, cx-9, ty, 4, 2, Hex("#9aa0b4"))
			px(ctx, cx+5, ty, 4, 2, Hex("#9aa0b4"))
		}
	}
	fader(e.Preamp)
	for i := 0; i < 10; i++ {
		fader(e.Band(i))
	}

	// The scale beside the preamp, in gold, and the frequencies under the
	// bands, in white.
	d := e.DB
	for i, s := range []string{"+12DB", "+0DB", "-12DB"} {
		y := float32(d.Y()) + []float32{0, float32(d.H()/2) - 3, float32(d.H()) - 6}[i]
		pixLabel(ctx, float32(d.X())+float32(d.W()-pixWidth(s))/2, y, s, Hex(clDB))
	}
	y := float32(e.Labels.Y())
	centre := func(r panel.R, s string) {
		pixLabel(ctx, float32(r.X())+float32(int(float32(r.W()-pixWidth(s))/2)), y, s, Hex(clLabel))
	}
	pre := e.Preamp
	centre(panel.R{pre.X() - 6, 0, pre.W() + 12, 0}, "PREAMP")
	for i, s := range []string{"31", "62", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		centre(e.Band(i), s)
	}
}

func clListFace(ctx *paint2d.Context, w, h float32, f *panel.Face) {
	clChrome(ctx, w, h)
	clInset(ctx, 4, 0, w-8, h-2)
	for i := float32(0); i < 4; i++ {
		px(ctx, w-8+2*i, h-4, 1, 1, Hex("#8c90a4"))
		px(ctx, w-6+2*i, h-6, 1, 1, Hex("#8c90a4"))
		px(ctx, w-4, h-8+2*i, 1, 1, Hex("#8c90a4"))
	}
}

// Playlist chrome, wells and footer are separate skin pieces: their corners
// keep their size while the viewport expands and the footer stays at the bottom.
func clListPieces(l *lay, f *panel.Face) {
	li := f.List
	well := func(name string, r panel.R, slice [4]int) {
		l.cell(name, r.W(), r.H(), slice, "", false, func(ctx *paint2d.Context, w, h float32) {
			clWell(ctx, panel.R{0, 0, int(w), int(h)}, false)
		})
	}
	well("list.rows", li.Rows.Grow(1), [4]int{2, 2, 2, 2})
	r := li.Scroll.Grow(1)
	l.cell("list.scroll", r.W(), r.H(), [4]int{2, 2, 2, 2}, "", false, func(ctx *paint2d.Context, w, h float32) {
		clGroove(ctx, panel.R{0, 0, int(w), int(h)})
	})
	well("list.info", li.Info, [4]int{})
	well("list.clock", li.Clock, [4]int{})
	l.cell("list.arrows", li.Scroll.W(), 20, [4]int{}, "", false, func(ctx *paint2d.Context, w, h float32) {
		for i := float32(0); i < 3; i++ {
			px(ctx, 4-i, 4+i, 1+2*i, 1, Hex("#8c90a4"))
			px(ctx, 4-i, 14-i, 1+2*i, 1, Hex("#8c90a4"))
		}
	})
}

// ---- the keys ------------------------------------------------------------------

// clKeyFace is a key of the era: a light face, a bright top and left, a
// grey foot and right, and a black outline — turned over while it is held.
func clKeyFace(ctx *paint2d.Context, w, h float32, down bool) {
	px(ctx, 0, 0, w, h, Hex(clInk))
	hi, lo, face := Hex(clKeyHi), Hex(clKeyLo), Hex(clKey)
	if down {
		hi, lo, face = Hex(clKeyLo), Hex(clKeyHi), Hex(clKeyDn)
	}
	px(ctx, 1, 1, w-2, h-2, face)
	px(ctx, 1, 1, w-2, 1, hi)
	px(ctx, 1, 1, 1, h-2, hi)
	px(ctx, 1, h-2, w-2, 1, lo)
	px(ctx, w-2, 1, 1, h-2, lo)
	if !down {
		px(ctx, 2, h-3, w-4, 1, lerpColor(face, lo, 0.5))
	}
}

// clKeyLamp is the little lamp in a toggle's corner: dark green off, lit
// green on.
func clKeyLamp(ctx *paint2d.Context, x, y float32, on, down bool) {
	if down {
		x++
		y++
	}
	px(ctx, x, y, 3, 3, Hex("#1a3a1a"))
	col := Hex("#2f6a2f")
	if on {
		col = Hex("#18e818")
	}
	px(ctx, x, y, 3, 3, col)
	px(ctx, x, y, 3, 1, lerpColor(col, paint2d.RGB(1, 1, 1), 0.3))
}

// clTransportGlyph is the mark on a transport key: a slate shape with a lit
// face, engraved rather than printed.
func clTransportGlyph(ctx *paint2d.Context, w, h float32, g string, down bool) {
	cx := float32(int(w/2)) - 4
	cy := float32(int(h/2)) - 4
	if down {
		cx++
		cy++
	}
	ink, lit := Hex(clGlyph), Hex(clGlyHi)
	tri := func(x, y float32, right bool) {
		for i := float32(0); i < 5; i++ {
			col := ink
			if i > 0 && i < 4 {
				col = lit
			}
			if right {
				px(ctx, x+i, y+i, 1, 9-2*i, col)
				px(ctx, x+i, y+i, 1, 1, ink)
				px(ctx, x+i, y+8-i, 1, 1, ink)
			} else {
				px(ctx, x+4-i, y+i, 1, 9-2*i, col)
				px(ctx, x+4-i, y+i, 1, 1, ink)
				px(ctx, x+4-i, y+8-i, 1, 1, ink)
			}
		}
	}
	bar := func(x, y, bw float32) {
		px(ctx, x, y, bw, 9, ink)
		px(ctx, x+1, y+1, bw-2, 7, lit)
	}
	switch g {
	case "prev":
		bar(cx, cy, 3)
		tri(cx+3, cy, false)
	case "play":
		tri(cx+2, cy, true)
	case "pause":
		bar(cx+1, cy, 3)
		bar(cx+5, cy, 3)
	case "stop":
		px(ctx, cx, cy, 9, 9, ink)
		px(ctx, cx+1, cy+1, 7, 7, lit)
	case "next":
		tri(cx, cy, true)
		bar(cx+6, cy, 3)
	case "eject":
		for i := float32(0); i < 5; i++ {
			px(ctx, cx+4-i, cy+i, 1+2*i, 1, ink)
			if i > 0 {
				px(ctx, cx+5-i, cy+i, 2*i-1, 1, lit)
			}
		}
		px(ctx, cx, cy+6, 9, 3, ink)
		px(ctx, cx+1, cy+7, 7, 1, lit)
	}
}

// clRepeatGlyph is the repeat key's mark: a loop with an arrowhead.
func clRepeatGlyph(ctx *paint2d.Context, x, y float32) {
	ink := Hex(clGlyph)
	px(ctx, x+1, y, 11, 1, ink)
	px(ctx, x, y+1, 1, 4, ink)
	px(ctx, x+12, y+1, 1, 4, ink)
	px(ctx, x+1, y+5, 11, 1, ink)
	px(ctx, x+3, y+4, 3, 3, ink) // the arrowhead where the loop closes
}

// clSkinColumn is the skin key: the four letters of the word it changes,
// stood in a column on the display's black, the way the era stood its
// option letters.
func clSkinColumn(ctx *paint2d.Context, w, h float32, down bool) {
	px(ctx, 0, 0, w, h, Hex(clWellC))
	col := Hex("#8a8ea4")
	if down {
		px(ctx, 0, 0, w, h, Hex("#16162a"))
		col = Hex(clLCD)
	}
	x := float32(int((w - 4) / 2))
	for i, r := range "SKIN" {
		pixLabel(ctx, x, 3+float32(i)*9, string(r), col)
	}
}

// clMiniGlyph is one of the playlist's small transport marks, five pixels
// high, in gold.
func clMiniGlyph(ctx *paint2d.Context, g string, col paint2d.Color) {
	switch g {
	case "prev":
		px(ctx, 1, 1, 1, 5, col)
		for i := float32(0); i < 3; i++ {
			px(ctx, 4-i, 1+i, 1, 5-2*i, col)
		}
	case "play":
		for i := float32(0); i < 3; i++ {
			px(ctx, 2+i, 1+i, 1, 5-2*i, col)
		}
	case "pause":
		px(ctx, 1, 1, 2, 5, col)
		px(ctx, 4, 1, 2, 5, col)
	case "stop":
		px(ctx, 1, 1, 5, 5, col)
	case "next":
		for i := float32(0); i < 3; i++ {
			px(ctx, 1+i, 1+i, 1, 5-2*i, col)
		}
		px(ctx, 5, 1, 1, 5, col)
	case "eject":
		for i := float32(0); i < 3; i++ {
			px(ctx, 3-i, 1+i, 1+2*i, 1, col)
		}
		px(ctx, 1, 5, 5, 1, col)
	}
}

// clLamp is a channel lamp: the word, green and lit, or grey and dark.
func clLamp(ctx *paint2d.Context, w, h float32, s string, on bool) {
	col := Hex(clDim)
	if on {
		col = Hex(clLCD)
	}
	x := float32(int((w - float32(pixWidth(s))) / 2))
	y := float32(int((h - 6) / 2))
	if on {
		// A glow a pixel wide, the way a lit lamp bled into its bezel.
		glow := Hex("#0c5a0c")
		for _, d := range [][2]float32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			pixLabel(ctx, x+d[0], y+d[1], s, glow)
		}
	}
	pixLabel(ctx, x, y, s, col)
}

// clThumb is the volume and balance thumb: a grey key with three grip lines.
func clThumb(ctx *paint2d.Context, w, h float32, down bool) {
	clKeyFace(ctx, w, h, down)
	for i := float32(0); i < 3; i++ {
		px(ctx, 5+2*i, 3, 1, 5, Hex(clGlyph))
	}
}

// clSeekThumb is the seek bar's thumb: a gold bar with the groove's lines on
// it, so it reads as a piece of the title bands.
func clSeekThumb(ctx *paint2d.Context, w, h float32, down bool) {
	px(ctx, 0, 0, w, h, Hex(clInk))
	rows := []string{clCream, clGold, clGoldDk, clCream, clSilver, clGoldDk, clGold, clGoldDk}
	if down {
		rows = []string{clGold, clGoldDk, clCream, clSilver, clGoldDk, clGold, clCream, clGoldDk}
	}
	for i, c := range rows {
		px(ctx, 1, 1+float32(i), w-2, 1, Hex(c))
	}
	px(ctx, 1, 1, 1, h-2, Hex(clCream))
	px(ctx, w-2, 1, 1, h-2, Hex(clGoldDk))
}

// clEqThumb is an equaliser thumb: a small grey key with a dark eye.
func clEqThumb(ctx *paint2d.Context, w, h float32, down bool) {
	clKeyFace(ctx, w, h, down)
	px(ctx, 4, 3, 3, 5, Hex(clGlyph))
}

// clListThumb is the playlist's scroll thumb, gold like the seek bar's.
func clListThumb(ctx *paint2d.Context, w, h float32, down bool) {
	px(ctx, 0, 0, w, h, Hex(clInk))
	face := Hex(clGold)
	if down {
		face = Hex(clCream)
	}
	px(ctx, 1, 1, w-2, h-2, face)
	px(ctx, 1, 1, w-2, 1, Hex(clCream))
	px(ctx, 1, 1, 1, h-2, Hex(clCream))
	px(ctx, w-2, 1, 1, h-2, Hex(clGoldDk))
	px(ctx, 1, h-2, w-2, 1, Hex(clGoldDk))
	for i := float32(0); i < 3; i++ {
		px(ctx, 2, h/2-3+2*i, w-4, 1, Hex(clGoldDk))
	}
}

// ---- the frame -------------------------------------------------------------------

// clGrooveRows are the groove's rows, top to bottom: the ribbed gold strip a
// title band of the era ran either side of its words.
var clGrooveRows = []string{"#403a2c", clCream, clSilver, clGoldDk, clGold, "#baa464"}

// clGrooveDim is the same groove on a window without the focus.
var clGrooveDim = []string{"#2a2a3c", "#8a8c9a", "#70727e", "#2a2a3c", "#5e606c", "#4c4e5a"}

// clBandBack is the title band's ground: darker than the chrome under it,
// with a grey rule along its top, so the band reads as a strip of its own.
func clBandBack(ctx *paint2d.Context, w, h float32) {
	px(ctx, 0, 0, w, h, Hex("#141421"))
	px(ctx, 0, 0, w, 1, Hex("#16161e"))
	px(ctx, 0, 1, w, 1, Hex("#55555f"))
	px(ctx, 0, 2, w, 2, Hex("#171724"))
	px(ctx, 0, h-2, w, 1, Hex("#0c0c16"))
	px(ctx, 0, h-1, w, 1, Hex("#05050d"))
}

// clBand is the caption: the chrome, the player logo at the left, and the
// groove from there to the caption keys. Out of focus, the groove greys.
func clBand(ctx *paint2d.Context, w, h float32, active bool) {
	clBandBack(ctx, w, h)
	rows := clGrooveRows
	if !active {
		rows = clGrooveDim
	}
	gy := float32(4)
	x0, x1 := float32(20), w-28
	for i, c := range rows {
		px(ctx, x0, gy+float32(i), x1-x0, 1, Hex(c))
	}
	// Rounded ends: the outermost rows stop a pixel short.
	px(ctx, x0, gy, 1, 1, Hex("#141421"))
	px(ctx, x0, gy+float32(len(rows))-1, 1, 1, Hex("#141421"))
	px(ctx, x1-1, gy, 1, 1, Hex("#141421"))
	px(ctx, x1-1, gy+float32(len(rows))-1, 1, 1, Hex("#141421"))
	paintPlayerLogo(ctx, paint2d.XYWH(5, float32(int((h-10)/2)), 10, 10))
}

// clPlate is the plate under the title: plain chrome that stops the groove
// with a rounded end either side of the words.
func clPlate(ctx *paint2d.Context, w, h float32, active bool) {
	clBandBack(ctx, w, h)
	rows := clGrooveRows
	if !active {
		rows = clGrooveDim
	}
	gy := float32(4)
	// The last two columns of the groove coming in from the left, rounded,
	// and the first two of the one going out on the right.
	for i, c := range rows {
		edge := i == 0 || i == len(rows)-1
		if !edge {
			px(ctx, 0, gy+float32(i), 1, 1, Hex(c))
			px(ctx, w-1, gy+float32(i), 1, 1, Hex(c))
		}
	}
}

// clCapKey is a caption key: a tiny dark bevelled square the frame prints
// its mark on in the groove's gold.
func clCapKey(ctx *paint2d.Context, w, h float32, face paint2d.Color, down bool) {
	px(ctx, 0, 0, w, h, Hex(clInk))
	hi, lo := lerpColor(face, paint2d.RGB(1, 1, 1), 0.45), lerpColor(face, paint2d.RGB(0, 0, 0), 0.45)
	if down {
		hi, lo = lo, hi
	}
	px(ctx, 1, 1, w-2, h-2, face)
	px(ctx, 1, 1, w-2, 1, hi)
	px(ctx, 1, 1, 1, h-2, hi)
	px(ctx, 1, h-2, w-2, 1, lo)
	px(ctx, w-2, 1, 1, h-2, lo)
}

// ---- the mark --------------------------------------------------------------------

// clMark is the mark printed at the strip's lower right: the original player logo.
func clMark(ctx *paint2d.Context, x, y float32) {
	paintPlayerLogo(ctx, paint2d.XYWH(x, y, 15, 15))
}

// ---- small helpers -------------------------------------------------------------

// pixLabel prints a line of the capital face straight onto a face, whole
// pixels, for the labels that never change (KBPS, PREAMP, +12DB).
func pixLabel(ctx *paint2d.Context, x, y float32, s string, col paint2d.Color) {
	for _, r := range strings.ToUpper(s) {
		rows, ok := pixGlyphs[r]
		if !ok {
			rows = pixGlyphs['?']
		}
		for yy, line := range rows {
			for xx, c := range line {
				if c == '#' {
					px(ctx, x+float32(xx), y+float32(yy), 1, 1, col)
				}
			}
		}
		x += float32(len(rows[0]) + 1)
	}
}

// pixWidth is a line's width in the capital face, less the trailing space.
func pixWidth(s string) int {
	w := 0
	for _, r := range strings.ToUpper(s) {
		rows, ok := pixGlyphs[r]
		if !ok {
			rows = pixGlyphs['?']
		}
		w += len(rows[0]) + 1
	}
	return max(w-1, 0)
}

// lerpColor mixes two colours, t of the way from a to b.
func lerpColor(a, b paint2d.Color, t float32) paint2d.Color {
	return paint2d.RGBA(a.R+(b.R-a.R)*t, a.G+(b.G-a.G)*t, a.B+(b.B-a.B)*t, a.A+(b.A-a.A)*t)
}

// ctx2 moves the origin to a rect's corner for a helper that draws from
// (0, 0); restore puts it back.
func ctx2(ctx *paint2d.Context, r panel.R) *paint2d.Context {
	ctx.Save()
	ctx.Translate(float32(r.X()), float32(r.Y()))
	return ctx
}

func restore(ctx *paint2d.Context) { ctx.Restore() }
