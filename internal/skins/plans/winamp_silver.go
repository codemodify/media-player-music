package plans

import (
	"math"
	"strings"

	"github.com/codemodify/media-player-music/internal/skins/panel"
	"github.com/codemodify/paintengine2d"
)

// WinampSilver is the compact player's rounded look of the later era: silver
// chrome under a navy title band, a blue dot-matrix display, glossy round
// keys, capsule toggles with blue lamps beside them, and a window whose four
// corners are round — the desktop shows through beyond them.
//
// It is drawn from paths and gradients, so its 2×
// sheet is a second rasterisation rather than a doubling and it is exact at
// 1.25, 1.5 and 1.75 as well as 1 and 2. Like Winamp it is a panel:
// the faces are pictures with the display, the grooves and the printed
// labels in them, and the keys are loose sprites the player lays out on
// internal/skins/panel's rects.
//
// The rounded window is the skin's own: window.shape is one rect the size of
// the window with a radius on every corner, resolved at the display scale,
// so the corners are round at every size rather than a picture of round
// corners. The equaliser has no title band: its header is a tab with its
// name on it, which is a caption of its own — a window variant for the
// role the player gives that window (docs/skins.md, "Windows that are not
// alike") — so the tab is the band's title plate, set from the start.
//
// Its round keys and capsules brighten under the pointer, which the later
// look did and the classic one did not; the layouts bind the hover faces,
// and a key the pointer is not on falls back to its resting face.

// The silver palette.
const (
	svNavy    = "#1b2645" // the title band and the rims
	svNavyHi  = "#34466f"
	svRim     = "#2a3148" // a dark outline round the chrome's shapes
	svSilver  = "#dfe2e9" // the chrome, lit
	svSilverM = "#bfc4ce"
	svSilverD = "#8d93a0"
	svSilverX = "#5d6474" // the foot of the transport's well

	svLCD    = "#23417f" // the display
	svLCDTop = "#2b4d92"
	svLCDDot = "#2e5299"
	svLCDRim = "#5f7cb4"
	svInk    = "#e6eeff" // lit dots
	svInkDim = "#6d88c2"

	svKey   = "#f4f5f8" // a round key's face
	svKeyLo = "#b9bdc7"
	svGlyph = "#1d212a"
	svBlue  = "#6f9cf0" // a lamp
	svCap   = "#34466f" // the equaliser's navy capsules
	svCapOn = "#4f73c0"
)

// WinampSilver builds the skin's plan.
func WinampSilver() *Plan {
	f := panel.Silver
	sh := &Sheet{Name: "chrome"}
	l := &lay{sh: sh, gap: 2}
	p := &Plan{
		Name:    "winamp-silver",
		Label:   "Winamp Silver",
		Year:    2026,
		Lineage: "uitoolkit",
		Summary: "The compact player's rounded look of the later era: silver chrome, a blue dot-matrix display, glossy round keys — a shaped panel with the desktop beyond its corners.",
		Family:  "light",
		Base:    "luna",
		Sheets:  []*Sheet{sh},
	}

	// ---- the three faces -------------------------------------------------
	cw := f.ContentW()
	l.row(f.ContentH(panel.MainH))
	l.panel("main.face", cw, f.ContentH(panel.MainH), false, func(ctx *paintengine2d.Context, w, h float32) {
		svMainFace(ctx, w, h, f)
	})
	l.row(f.EqContentH())
	l.panel("eq.face", cw, f.EqContentH(), false, func(ctx *paintengine2d.Context, w, h float32) {
		svEqFace(ctx, w, h, f)
	})
	l.row(f.ContentH(panel.ListH))
	l.cell("list.face", cw, f.ContentH(panel.ListH), [4]int{6, 8, 36, 8}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		svListFace(ctx, w, h, f)
	})
	svListPieces(l, f)

	// ---- the keys ----------------------------------------------------------
	m, q, li := f.Main, f.Eq, f.List
	type key struct {
		name   string
		r      panel.R
		draw   func(ctx *paintengine2d.Context, w, h float32, down, on, hover bool)
		toggle bool
	}
	round := func(g string) func(*paintengine2d.Context, float32, float32, bool, bool, bool) {
		return func(ctx *paintengine2d.Context, w, h float32, down, on, hover bool) {
			svRoundKey(ctx, w, h, down)
			if hover {
				d := min(w, h)
				svHoverRing(ctx, paintengine2d.XYWH((w-d)/2, (h-d)/2, d, d), d/2)
			}
			svGlyphMark(ctx, w, h, g, down)
		}
	}
	capsule := func(s string, navy bool) func(*paintengine2d.Context, float32, float32, bool, bool, bool) {
		return func(ctx *paintengine2d.Context, w, h float32, down, on, hover bool) {
			svCapsule(ctx, w, h, s, navy, down, on, hover)
		}
	}
	lamp := func(g string) func(*paintengine2d.Context, float32, float32, bool, bool, bool) {
		return func(ctx *paintengine2d.Context, w, h float32, down, on, hover bool) {
			cw := h + 5
			svCapsuleShape(ctx, paintengine2d.XYWH(0, 0, cw, h), down, false)
			if hover {
				svHoverRing(ctx, paintengine2d.XYWH(0, 0, cw, h), h/2)
			}
			svGlyphMark(ctx, cw, h, g, down)
			svLamp(ctx, cw+2+(w-cw-2)/2, h/2, on)
		}
	}
	keys := []key{
		{"prev", m.Prev, round("prev"), false},
		{"play", m.Play, round("play"), false},
		{"pause", m.Pause, round("pause"), false},
		{"stop", m.Stop, round("stop"), false},
		{"next", m.Next, round("next"), false},
		{"eject", m.Eject, round("eject"), false},
		{"shuffle", m.Shuffle, lamp("shuffle"), true},
		{"repeat", m.Repeat, lamp("repeat"), true},
		{"eq", m.EQ, capsule("EQ", false), true},
		{"pl", m.PL, capsule("PL", false), true},
		{"on", q.On, capsule("ON", true), true},
		{"auto", q.Auto, capsule("AUTO", true), true},
		{"presets", q.Presets, capsule("PRESETS", false), false},
		{"add", li.Add, round("add"), false},
		{"rem", li.Rem, round("rem"), false},
		{"sel", li.Sel, round("sel"), false},
		{"misc", li.Misc, round("misc"), false},
		{"opts", li.Opts, round("opts"), false},
	}
	for _, k := range keys {
		k := k
		l.row(k.r.H())
		states := []string{"", ".hover", ".down"}
		if k.toggle {
			states = append(states, ".on", ".on.hover", ".on.down")
		}
		for _, st := range states {
			down := strings.HasSuffix(st, "down")
			on := strings.HasPrefix(st, ".on")
			hover := strings.HasSuffix(st, "hover")
			l.panel("key."+k.name+st, k.r.W(), k.r.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
				k.draw(ctx, w, h, down, on, hover)
			})
		}
	}

	// The skin key: its word in dim dots, stood in a column at the display's
	// left edge where the later look kept a column of little marks.
	l.row(m.Skin.H())
	for _, st := range []string{"", ".down"} {
		col := Hex(svInkDim)
		if st != "" {
			col = Hex(svInk)
		}
		l.panel("key.skin"+st, m.Skin.W(), m.Skin.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
			x := float32(int((w - 4) / 2))
			for i, r := range "SKIN" {
				svDots(ctx, x, 3+float32(i)*9, string(r), col)
			}
		})
	}

	// The small transport on the playlist's display: white marks on blue.
	l.row(8)
	for i, g := range []string{"prev", "play", "pause", "stop", "next", "eject"} {
		r := li.Mini[i]
		for _, st := range []string{"", ".down"} {
			col := Hex(svInk)
			if st != "" {
				col = Hex(svBlue)
			}
			g := g
			l.panel("mini."+g+st, r.W(), r.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
				clMiniGlyph(ctx, g, col)
			})
		}
	}

	// ---- the display ---------------------------------------------------------
	l.row(14)
	for _, d := range "0123456789-" {
		d := d
		l.panel("led."+string(d), 10, 14, false, func(ctx *paintengine2d.Context, w, h float32) {
			svDotDigit(ctx, dotDigits[d])
		})
	}
	l.panel("led.colon", 4, 14, false, func(ctx *paintengine2d.Context, w, h float32) {
		svDot(ctx, 1, 4, Hex(svInk))
		svDot(ctx, 1, 8, Hex(svInk))
	})
	l.row(9)
	l.panel("state.play", m.State.W(), m.State.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
		svDot(ctx, 0, 0, Hex(svInk))
		for i := float32(0); i < 3; i++ {
			for j := float32(0); j < 5-2*i; j++ {
				svPix(ctx, 4+i, 1+i+j, Hex(svInk))
			}
		}
	})
	l.panel("state.pause", m.State.W(), m.State.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
		for j := float32(0); j < 5; j++ {
			svPix(ctx, 3, 1+j, Hex(svInk))
			svPix(ctx, 4, 1+j, Hex(svInk))
			svPix(ctx, 6, 1+j, Hex(svInk))
			svPix(ctx, 7, 1+j, Hex(svInk))
		}
	})
	l.panel("state.stop", m.State.W(), m.State.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
		for j := float32(0); j < 4; j++ {
			for i := float32(0); i < 4; i++ {
				svPix(ctx, 3+i, 1+j, Hex(svInk))
			}
		}
	})
	l.panel("lamp.mono", m.Mono.W(), m.Mono.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
		svDots(ctx, w-float32(pixWidth("MONO")), float32(int((h-6)/2)), "MONO", Hex(svInk))
	})
	l.panel("lamp.stereo", m.Stereo.W(), m.Stereo.H(), false, func(ctx *paintengine2d.Context, w, h float32) {
		svDots(ctx, w-float32(pixWidth("STEREO")), float32(int((h-6)/2)), "STEREO", Hex(svInk))
	})

	pixText(l, func(ctx *paintengine2d.Context, x, y float32) {
		px(ctx, x, y, 1, 1, paintengine2d.RGB(1, 1, 1))
	})

	// ---- the thumbs -----------------------------------------------------------
	l.row(18)
	for _, st := range []string{"", ".down"} {
		down := st != ""
		l.panel("thumb"+st, 16, 9, false, func(ctx *paintengine2d.Context, w, h float32) {
			svCapsuleShape(ctx, paintengine2d.XYWH(0, 0, w, h), down, true)
		})
		l.panel("seek.thumb"+st, 27, 9, false, func(ctx *paintengine2d.Context, w, h float32) {
			svCapsuleShape(ctx, paintengine2d.XYWH(0, 0, w, h), down, true)
		})
		l.panel("eq.thumb"+st, 12, 12, false, func(ctx *paintengine2d.Context, w, h float32) {
			svKnob(ctx, w, h, down)
		})
		l.panel("list.thumb"+st, 7, 18, false, func(ctx *paintengine2d.Context, w, h float32) {
			svListThumb(ctx, w, h, down)
		})
	}

	// ---- the frame --------------------------------------------------------------
	//
	// Sliced for real. The corners are round in the art as well as in the
	// silhouette, so the antialiased cut and the drawn rim agree.
	l.row(20)
	l.cell("window.normal", 20, 20, [4]int{8, 8, 8, 8}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		r := paintengine2d.XYWH(0, 0, w, h)
		ctx.DrawRoundRect(r, 7, 7, paintengine2d.Fill(Hex(svRim)))
		vgrad(ctx, r.Inset(1), 6, stop(0, Hex(svSilver)), stop(1, Hex(svSilverD)))
	})
	l.cell("caption.normal", 64, f.Caption, [4]int{0, 30, 0, 20}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		svBand(ctx, w, h, true)
	})
	l.cell("caption.inactive", 64, f.Caption, [4]int{0, 30, 0, 20}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		svBand(ctx, w, h, false)
	})
	l.cell("caption.plate", 12, f.Caption, [4]int{0, 4, 0, 4}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		svPlate(ctx, w, h, true)
	})
	l.cell("caption.plate.inactive", 12, f.Caption, [4]int{0, 4, 0, 4}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		svPlate(ctx, w, h, false)
	})
	for _, c := range []struct {
		name string
		face string
		down bool
	}{{"capbtn.normal", "#c9cdd6", false}, {"capbtn.hover", "#e6e8ee", false}, {"capbtn.pressed", "#9aa0ac", true}} {
		c := c
		l.cell(c.name, 9, 9, [4]int{3, 3, 3, 3}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
			r := paintengine2d.XYWH(0, 0, w, h)
			ctx.DrawRoundRect(r, 1.5, 1.5, paintengine2d.Fill(Hex("#0e1426")))
			vgrad(ctx, r.Inset(1), 1, stop(0, lerpColor(Hex(c.face), Hex("#ffffff"), 0.4)), stop(1, Hex(c.face)))
		})
	}
	l.cell("focus.ring", 8, 8, [4]int{2, 2, 2, 2}, "none", false, func(ctx *paintengine2d.Context, w, h float32) {
		outline(ctx, paintengine2d.XYWH(0, 0, w, h), 2, 1, Hex("#5f8fe8"))
	})

	// The equaliser's caption: pale chrome with a rule along its foot, and
	// the tab its name is on as the title's plate — as wide as the name,
	// set from the band's start.
	eqCap := f.EqCaption
	l.row(eqCap)
	for _, st := range []struct {
		suffix string
		active bool
	}{{"", true}, {".inactive", false}} {
		st := st
		l.cell("eq.caption"+st.suffix, 64, eqCap, [4]int{0, 30, 0, 20}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
			svEqBand(ctx, w, h, st.active)
		})
		l.cell("eq.tab"+st.suffix, 26, eqCap, [4]int{2, 14, 0, 6}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
			svTab(ctx, w, h, st.active)
		})
	}
	l.close()

	// The titles are set in capitals, as the look printed them: a choice
	// the skin makes about how its art reads, and only about that — the
	// windows keep the titles the player gave them.
	p.Text = []TextRole{
		{Name: "control", Color: "#1d212a", Disabled: "#7c818c"},
		{Name: "caption", Color: "#c9cfdb", Disabled: "#7f889c", Size: 10, Bold: true, Upper: true},
		{Name: "tab", Color: "#2d3240", Disabled: "#7c818c", Size: 8, Bold: true, Upper: true},
		{Name: "capkey", Color: "#141a2c", Hover: "#141a2c", Pressed: "#ffffff", Disabled: "#50586a"},
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
	p.Colors = map[string]string{"focus": "#5f8fe8"}
	// A menu opens inside the window it is asked from, and the strip is 116
	// design pixels tall: rows of 20 let the skin menu's four fit in it.
	p.Metrics = map[string]float32{"menuItemH": 20}
	// Four round corners, and a window that is the shape they make: one
	// rect the size of the window, rounder than the frame (which is square),
	// so it is a silhouette and the desktop shows beyond every corner.
	p.Window = &WindowSpec{
		Border:  f.Border,
		Caption: f.Caption,
		Layout:  ":minimize,close",
		Shape: []ShapeRect{
			{At: [4]int{0, 0, 0, 0}, Radius: [4]int{svCorner, svCorner, svCorner, svCorner}, StretchX: true, StretchY: true},
		},
		// The equaliser's own caption: its tab.
		Variants: map[string]*WindowSpec{
			panel.LayoutEqualiser: {
				Caption:    eqCap,
				TitleStart: true,
				TitleInset: svTabInset,
				Parts: []PartBinding{
					{Part: "caption", Text: "tab", States: [][2]string{
						{"normal", "eq.caption"}, {"inactive", "eq.caption.inactive"},
					}},
					{Part: "caption.title", States: [][2]string{
						{"normal", "eq.tab"}, {"inactive", "eq.tab.inactive"},
					}},
				},
			},
		},
	}
	p.Layouts = winampLayouts(f, sh)
	mainCaptionRole(p)
	return p
}

// svTabInset is how far in from the equaliser's left edge its tab starts,
// in design pixels.
const svTabInset = 18

// svCorner is the radius of the window's four corners, in design pixels.
const svCorner = 7

// ---- the faces --------------------------------------------------------------------

// svMainFace is the strip: the display across the top with the silver shelf
// rising into its lower right, the seek groove, and the transport's well
// with the lighter cove the two toggles sit in.
func svMainFace(ctx *paintengine2d.Context, w, h float32, f *panel.Face) {
	m := f.Main
	r := paintengine2d.XYWH(0, 0, w, h)
	ctx.DrawRect(r, paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, 0), End: paintengine2d.Pt(0, h),
		Stops: []paintengine2d.GradientStop{
			stop(0, Hex("#e4e7ee")), stop(0.55, Hex(svSilverM)),
			stop(0.66, Hex("#a3a9b6")), stop(1, Hex(svSilverX)),
		},
	}))
	// The display, and the shelf that rises into it.
	d := rectOf(m.Display)
	svLCDWell(ctx, d, 6)
	shelf := paintengine2d.XYWH(float32(m.Volume.X()-8), float32(m.Volume.Y()-3), d.Max.X-float32(m.Volume.X()-8)+1, d.Max.Y-float32(m.Volume.Y()-3)+1)
	svShelf(ctx, shelf, d)
	svDots(ctx, float32(m.Rate.Right()+3), float32(m.RateText.Y()), "KBPS", Hex(svInkDim))
	svDots(ctx, float32(m.Freq.Right()+3), float32(m.FreqText.Y()), "KHZ", Hex(svInkDim))
	svDots(ctx, float32(m.Mono.Right()-pixWidth("MONO")), float32(m.Mono.Y()+1), "MONO", Hex(svInkDim))
	svDots(ctx, float32(m.Stereo.Right()-pixWidth("STEREO")), float32(m.Stereo.Y()+1), "STEREO", Hex(svInkDim))

	// The two short grooves on the shelf, and the long one under it.
	svGroove(ctx, rectOf(m.Volume), true)
	svGroove(ctx, rectOf(m.Balance), false)
	svGroove(ctx, rectOf(m.Seek), false)

	// The cove at the lower right the toggles sit in: lighter, rounded at
	// its left end.
	cove := paintengine2d.XYWH(float32(m.Shuffle.X()-8), float32(m.Shuffle.Y()-4), w, float32(m.Shuffle.H()+12))
	ctx.Save()
	ctx.ClipRect(paintengine2d.XYWH(0, 0, w, h))
	ctx.DrawRoundRect(cove, 12, 12, paintengine2d.Fill(Hex("#7e8595")))
	vgrad(ctx, cove.Inset(1), 11, stop(0, Hex("#eceef3")), stop(1, Hex("#c3c8d2")))
	ctx.Restore()
	svMark(ctx, rectOf(m.Mark))
	svGrip(ctx, w, h)
}

// svEqFace is the equaliser under its tab: pale silver, the graph, the
// scale, eleven grooves, and the dark strip the frequencies are on.
func svEqFace(ctx *paintengine2d.Context, w, h float32, f *panel.Face) {
	q := f.Eq
	r := paintengine2d.XYWH(0, 0, w, h)
	vgrad(ctx, r, 0, stop(0, Hex("#e9ebf0")), stop(1, Hex("#c6cad4")))

	// The graph: a pale well with a rule at the middle and a faint grid.
	g := rectOf(q.Graph)
	ctx.DrawRoundRect(g, 2, 2, paintengine2d.Fill(Hex("#f2f4f8")))
	outline(ctx, g, 2, 1, Hex("#a7aebd"))
	for i := float32(1); i < 10; i++ {
		x := g.Min.X + float32(int(i*g.Dx()/10))
		ctx.DrawRect(paintengine2d.XYWH(x, g.Min.Y+2, 1, g.Dy()-4), paintengine2d.Fill(Hex("#dde2ec")))
	}
	ctx.DrawRect(paintengine2d.XYWH(g.Min.X+2, g.Min.Y+float32(int(g.Dy()/2)), g.Dx()-4, 1), paintengine2d.Fill(Hex("#9fb0d4")))

	// A pale band across the lower half of the faders, which is how the
	// look drew the zero line: everything under it is a cut.
	pre, last := rectOf(q.Preamp), rectOf(q.Band(9))
	mid := pre.Min.Y + float32(int(pre.Dy()/2))
	ctx.DrawRect(paintengine2d.XYWH(pre.Min.X-3, mid, last.Max.X-pre.Min.X+6, pre.Max.Y-mid), paintengine2d.Fill(Hex("#c2c6d0")))
	// A dotted rule between each pair of bands.
	for i := 0; i < 9; i++ {
		x := float32(int(rectOf(q.Band(i)).Max.X + (rectOf(q.Band(i+1)).Min.X-rectOf(q.Band(i)).Max.X)/2))
		for y := pre.Min.Y + 1; y < pre.Max.Y-1; y += 2 {
			ctx.DrawRect(paintengine2d.XYWH(x, y, 1, 1), paintengine2d.Fill(Hex("#8c92a0")))
		}
	}

	fader := func(fr paintengine2d.Rect) {
		cx := fr.Min.X + fr.Dx()/2
		slot := paintengine2d.XYWH(cx-3, fr.Min.Y+1, 6, fr.Dy()-2)
		ctx.DrawRoundRect(slot, 3, 3, paintengine2d.Linear(paintengine2d.LinearGradient{
			Start: paintengine2d.Pt(slot.Min.X, 0), End: paintengine2d.Pt(slot.Max.X, 0),
			Stops: []paintengine2d.GradientStop{stop(0, Hex("#2e323d")), stop(0.5, Hex("#5b6070")), stop(1, Hex("#3a3e4a"))},
		}))
		outline(ctx, slot, 3, 1, Hex("#262a33"))
	}
	fader(pre)
	for i := 0; i < 10; i++ {
		fader(rectOf(q.Band(i)))
	}

	// The scale: a bracket with its marks, and the three readings.
	s := rectOf(q.DB)
	ink := Hex("#3a3f4c")
	for _, x := range []float32{s.Min.X + 3, s.Max.X - 4} {
		for y := s.Min.Y + 2; y < s.Max.Y-2; y += 2 {
			ctx.DrawRect(paintengine2d.XYWH(x, y, 1, 1), paintengine2d.Fill(ink))
		}
	}
	for i, lbl := range []string{"+12 DB", "+0 DB", "-12 DB"} {
		y := []float32{s.Min.Y + 1, s.Min.Y + float32(int(s.Dy()/2)) - 3, s.Max.Y - 7}[i]
		tw := float32(pixWidth(lbl))
		ctx.DrawRect(paintengine2d.XYWH(s.Min.X+3, y+3, 4, 1), paintengine2d.Fill(ink))
		ctx.DrawRect(paintengine2d.XYWH(s.Max.X-7, y+3, 4, 1), paintengine2d.Fill(ink))
		pixLabel(ctx, s.Min.X+float32(int((s.Dx()-tw)/2)), y, lbl, ink)
	}

	// The strip the frequencies are printed on.
	lb := rectOf(q.Labels)
	ctx.DrawRoundRect(lb, 2, 2, paintengine2d.Fill(Hex("#5c6170")))
	ctx.DrawRect(paintengine2d.XYWH(lb.Min.X+1, lb.Min.Y+1, lb.Dx()-2, 1), paintengine2d.Fill(Hex("#7c8192")))
	centre := func(r paintengine2d.Rect, s string) {
		pixLabel(ctx, r.Min.X+float32(int((r.Dx()-float32(pixWidth(s)))/2)), lb.Min.Y+1, s, Hex("#eef1f7"))
	}
	centre(paintengine2d.XYWH(lb.Min.X, 0, pre.Max.X-lb.Min.X+14, 0), "PREAMP")
	for i, s := range []string{"31", "62", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		b := rectOf(q.Band(i))
		centre(b, s)
		ctx.DrawRect(paintengine2d.XYWH(b.Min.X-2, lb.Min.Y+1, 1, lb.Dy()-2), paintengine2d.Fill(Hex("#c9cdd6")))
	}
	outline(ctx, r, 0, 1, Hex("#9aa0ae"))
}

// svListFace is the playlist: the blue list, its scroll groove, and the
// silver foot with the small display in it.
func svListFace(ctx *paintengine2d.Context, w, h float32, f *panel.Face) {
	r := paintengine2d.XYWH(0, 0, w, h)
	vgrad(ctx, r, 0, stop(0, Hex("#d9dce4")), stop(0.8, Hex(svSilverM)), stop(1, Hex("#9aa0ad")))
	svGrip(ctx, w, h)
}

func svListPieces(l *lay, f *panel.Face) {
	li := f.List
	r := li.Rows.Grow(1)
	l.cell("list.rows", r.W(), r.H(), [4]int{4, 4, 4, 4}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		rows := paintengine2d.XYWH(0, 0, w, h)
		ctx.DrawRoundRect(rows, 3, 3, paintengine2d.Fill(Hex(svNavy)))
		ctx.DrawRect(rows.Inset(1), paintengine2d.Fill(Hex("#1c3d7c")))
	})
	r = li.Scroll.Grow(1)
	l.cell("list.scroll", r.W(), r.H(), [4]int{3, 3, 3, 3}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		ctx.DrawRoundRect(paintengine2d.XYWH(1, 1, w-2, h-2), 2, 2, paintengine2d.Fill(Hex("#10182c")))
	})
	l.cell("list.info", li.Info.W(), li.Info.H(), [4]int{}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		svLCDWell(ctx, paintengine2d.XYWH(0, 0, w, h), 7)
	})
	l.cell("list.arrows", li.Scroll.W(), 20, [4]int{}, "", false, func(ctx *paintengine2d.Context, w, h float32) {
		ax, ay := w/2, float32(6)
		for i, dir := range []float32{-1, 1} {
			p := paintengine2d.NewPath()
			y := ay + float32(i)*7
			p.MoveTo(ax-3, y-1.5*dir+1)
			p.LineTo(ax+3, y-1.5*dir+1)
			p.LineTo(ax, y+1.5*dir+1)
			p.Close()
			ctx.DrawPath(p, paintengine2d.Fill(Hex("#3d4352")))
		}
	})
}

// ---- pieces ------------------------------------------------------------------------

func rectOf(r panel.R) paintengine2d.Rect {
	return paintengine2d.XYWH(float32(r.X()), float32(r.Y()), float32(r.W()), float32(r.H()))
}

// svLCDWell is a blue dot-matrix display sunk into the chrome: a navy rim, a
// lit inner edge, a blue that deepens downwards, and the grid of dark dots
// every lit dot sits on.
func svLCDWell(ctx *paintengine2d.Context, r paintengine2d.Rect, radius float32) {
	ctx.DrawRoundRect(r, radius, radius, paintengine2d.Fill(Hex(svNavy)))
	in := r.Inset(1)
	ctx.DrawRoundRect(in, radius-1, radius-1, paintengine2d.Fill(Hex(svLCDRim)))
	vgrad(ctx, in.Inset(1), radius-2, stop(0, Hex(svLCDTop)), stop(1, Hex("#1d3668")))
	ctx.Save()
	ctx.ClipRoundRect(in.Inset(1), radius-2, radius-2)
	for y := in.Min.Y + 2; y < in.Max.Y-1; y += 2 {
		for x := in.Min.X + 2; x < in.Max.X-1; x += 2 {
			ctx.DrawRect(paintengine2d.XYWH(x, y, 1, 1), paintengine2d.Fill(Hex(svLCDDot)))
		}
	}
	innerShadow(ctx, in.Inset(1), radius-2, 3, Hex("#0c1630aa"))
	ctx.Restore()
}

// svShelf is the silver shelf that rises into the display's lower right, its
// top-left corner round, with the display's rim carried round it.
func svShelf(ctx *paintengine2d.Context, s, d paintengine2d.Rect) {
	ctx.Save()
	ctx.ClipRoundRect(d.Inset(1), 5, 5)
	ctx.DrawRoundRectCorners(s.Inset(-1), 8, 0, 0, 0, paintengine2d.Fill(Hex(svNavy)))
	ctx.DrawRoundRectCorners(s, 7, 0, 0, 0, paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, s.Min.Y), End: paintengine2d.Pt(0, s.Max.Y),
		Stops: []paintengine2d.GradientStop{stop(0, Hex("#f4f5f8")), stop(1, Hex("#bcc1cc"))},
	}))
	ctx.Restore()
}

// svGroove is a thin dark groove a thumb rides in, with a lit lip under it.
// fill runs the blue of a level along its left end.
func svGroove(ctx *paintengine2d.Context, r paintengine2d.Rect, fill bool) {
	cy := r.Min.Y + float32(int(r.Dy()/2))
	g := paintengine2d.XYWH(r.Min.X, cy-1.5, r.Dx(), 3)
	ctx.DrawRoundRect(g.Translate(paintengine2d.Pt(0, 1)), 1.5, 1.5, paintengine2d.Fill(Hex("#ffffffc0")))
	ctx.DrawRoundRect(g, 1.5, 1.5, paintengine2d.Fill(Hex("#3b4150")))
	if fill {
		ctx.DrawRoundRect(paintengine2d.XYWH(g.Min.X, g.Min.Y, g.Dx()*0.6, 3), 1.5, 1.5, paintengine2d.Fill(Hex("#4f74c4")))
	}
}

// svRoundKey is a glossy round key: a dark ring, a silver face lit from
// above, and a crescent of shine across its top.
func svRoundKey(ctx *paintengine2d.Context, w, h float32, down bool) {
	d := min(w, h)
	c := paintengine2d.Pt(w/2, h/2)
	ctx.DrawCircle(c, d/2, paintengine2d.Fill(Hex("#2b2f39")))
	face := paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, c.Y-d/2), End: paintengine2d.Pt(0, c.Y+d/2),
		Stops: []paintengine2d.GradientStop{stop(0, Hex("#ffffff")), stop(0.5, Hex(svKey)), stop(1, Hex(svKeyLo))},
	})
	if down {
		face = paintengine2d.Linear(paintengine2d.LinearGradient{
			Start: paintengine2d.Pt(0, c.Y-d/2), End: paintengine2d.Pt(0, c.Y+d/2),
			Stops: []paintengine2d.GradientStop{stop(0, Hex("#9aa0ad")), stop(1, Hex("#e2e5ec"))},
		})
	}
	ctx.DrawCircle(c, d/2-1.5, face)
	if !down {
		ctx.DrawOval(paintengine2d.XYWH(c.X-d*0.3, c.Y-d*0.4, d*0.6, d*0.32), paintengine2d.Fill(Hex("#ffffffa0")))
	}
}

// svCapsuleShape is a capsule: a dark rim and a silver face. knob makes it a
// slider's thumb, a shade darker at its foot.
func svCapsuleShape(ctx *paintengine2d.Context, r paintengine2d.Rect, down, knob bool) {
	rad := r.Dy() / 2
	ctx.DrawRoundRect(r, rad, rad, paintengine2d.Fill(Hex("#2e3340")))
	top, bot := Hex("#ffffff"), Hex("#bfc4ce")
	if down {
		top, bot = Hex("#a9aeb9"), Hex("#eceef2")
	}
	vgrad(ctx, r.Inset(1), rad-1, stop(0, top), stop(1, bot))
	if !down {
		ctx.DrawRoundRect(paintengine2d.XYWH(r.Min.X+rad*0.6, r.Min.Y+1.5, r.Dx()-rad*1.2, r.Dy()*0.3), r.Dy()*0.15, r.Dy()*0.15, paintengine2d.Fill(Hex("#ffffffb0")))
	}
	_ = knob
}

// svCapsule is a capsule key with a word on it: white with dark letters, or
// navy with white ones (the equaliser's ON and AUTO), lit blue when on.
func svCapsule(ctx *paintengine2d.Context, w, h float32, s string, navy, down, on, hover bool) {
	r := paintengine2d.XYWH(0, 0, w, h)
	col := Hex(svGlyph)
	if navy {
		rad := h / 2
		ctx.DrawRoundRect(r, rad, rad, paintengine2d.Fill(Hex("#10182e")))
		face := Hex(svCap)
		if on {
			face = Hex(svCapOn)
		}
		if down {
			face = lerpColor(face, Hex("#000000"), 0.25)
		}
		vgrad(ctx, r.Inset(1), rad-1, stop(0, lerpColor(face, Hex("#ffffff"), 0.25)), stop(1, face))
		col = Hex("#eef2fb")
	} else {
		svCapsuleShape(ctx, r, down, false)
		if on {
			rad := h / 2
			ctx.DrawRoundRect(r.Inset(1.5), rad-1.5, rad-1.5, paintengine2d.Fill(Hex("#8fb2f040")))
		}
	}
	if hover {
		svHoverRing(ctx, r, h/2)
	}
	d := float32(0)
	if down {
		d = 0.5
	}
	pixLabel(ctx, float32(int((w-float32(pixWidth(s)))/2))+d, float32(int((h-6)/2))+d, s, col)
}

// svHoverRing is what a key does under the pointer: a blue light round the
// inside of its rim, the colour of the lamps, and a breath of shine over the
// face.
func svHoverRing(ctx *paintengine2d.Context, r paintengine2d.Rect, rad float32) {
	ctx.DrawRoundRect(r.Inset(1), rad-1, rad-1, paintengine2d.Fill(Hex("#ffffff28")))
	ctx.DrawRoundRect(r.Inset(1.6), rad-1.6, rad-1.6, paintengine2d.StrokePaint(Hex("#5f8fe8d8"), 1.2))
}

// svEqBand is the equaliser's caption: the chrome of its face carried up to
// a dark rim along the top, round at the window's corners, with the rule the
// tab's foot runs into along the bottom.
func svEqBand(ctx *paintengine2d.Context, w, h float32, active bool) {
	r := paintengine2d.XYWH(0, 0, w, h)
	ctx.DrawRoundRectCorners(r, svCorner, svCorner, 0, 0, paintengine2d.Fill(Hex(svRim)))
	top, bot := Hex("#f1f2f6"), Hex("#e9ebf0")
	if !active {
		top, bot = Hex("#e0e2e8"), Hex("#d8dbe2")
	}
	ctx.DrawRoundRectCorners(paintengine2d.XYWH(0, 1, w, h-1), svCorner-1, svCorner-1, 0, 0, paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, 1), End: paintengine2d.Pt(0, h),
		Stops: []paintengine2d.GradientStop{stop(0, top), stop(1, bot)},
	}))
	ctx.DrawRect(paintengine2d.XYWH(0, h-1, w, 1), paintengine2d.Fill(Hex("#8a90a0")))
}

// svTab is the tab the equaliser's name is on: white, outlined, square at
// its foot where it joins the face, a small round corner at its top left and
// a long curve down to the rule at its right.
func svTab(ctx *paintengine2d.Context, w, h float32, active bool) {
	ink := Hex("#6c7282")
	fill := Hex("#f6f7fa")
	if !active {
		ink, fill = Hex("#8d93a0"), Hex("#eceef2")
	}
	// The tab stands on the band's rim, one pixel down.
	top := float32(2)
	tab := paintengine2d.NewPath()
	tab.MoveTo(0.5, h)
	tab.LineTo(0.5, top+4)
	tab.QuadTo(0.5, top+0.5, 4.5, top+0.5)
	tab.LineTo(w-13, top+0.5)
	tab.CubicTo(w-6, top+0.5, w-7, h-0.5, w, h-0.5)
	tab.LineTo(w, h)
	tab.Close()
	ctx.DrawPath(tab, paintengine2d.Fill(fill))
	edge := paintengine2d.NewPath()
	edge.MoveTo(0.5, h)
	edge.LineTo(0.5, top+4)
	edge.QuadTo(0.5, top+0.5, 4.5, top+0.5)
	edge.LineTo(w-13, top+0.5)
	edge.CubicTo(w-6, top+0.5, w-7, h-0.5, w, h-0.5)
	ctx.DrawPath(edge, paintengine2d.StrokePaint(ink, 1))
}

// svLamp is the small glossy lamp beside a toggle: a quiet blue off, and
// lit — brighter, with a halo bleeding onto the chrome — on.
func svLamp(ctx *paintengine2d.Context, x, y float32, on bool) {
	c := paintengine2d.Pt(x, y)
	col := Hex("#4b68a8")
	if on {
		col = Hex("#8fb6ff")
		ctx.DrawCircle(c, 4.8, paintengine2d.Radial(paintengine2d.RadialGradient{
			Center: c, Radius: 4.8,
			Stops: []paintengine2d.GradientStop{stop(0, Hex("#8fb6ffa0")), stop(1, Hex("#8fb6ff00"))},
		}))
	}
	ctx.DrawCircle(c, 3.2, paintengine2d.Fill(Hex("#2c3654")))
	ctx.DrawCircle(c, 2.5, paintengine2d.Radial(paintengine2d.RadialGradient{
		Center: paintengine2d.Pt(x-0.8, y-0.8), Radius: 3,
		Stops: []paintengine2d.GradientStop{stop(0, lerpColor(col, Hex("#ffffff"), 0.65)), stop(1, col)},
	}))
}

// svKnob is an equaliser thumb: a silver ball.
func svKnob(ctx *paintengine2d.Context, w, h float32, down bool) {
	c := paintengine2d.Pt(w/2, h/2)
	ctx.DrawCircle(c, w/2, paintengine2d.Fill(Hex("#2b2f39")))
	hi := Hex("#ffffff")
	if down {
		hi = Hex("#c9cdd6")
	}
	ctx.DrawCircle(c, w/2-1, paintengine2d.Radial(paintengine2d.RadialGradient{
		Center: paintengine2d.Pt(c.X-1.5, c.Y-1.5), Radius: w * 0.6,
		Stops: []paintengine2d.GradientStop{stop(0, hi), stop(0.6, Hex("#d4d8e0")), stop(1, Hex("#8c929e"))},
	}))
}

// svListThumb is the playlist's scroll thumb: silver with three grip lines.
func svListThumb(ctx *paintengine2d.Context, w, h float32, down bool) {
	r := paintengine2d.XYWH(0, 0, w, h)
	ctx.DrawRoundRect(r, 2, 2, paintengine2d.Fill(Hex("#2e3340")))
	top := Hex("#ffffff")
	if down {
		top = Hex("#c2c7d1")
	}
	vgrad(ctx, r.Inset(1), 1.5, stop(0, top), stop(1, Hex("#aeb4c0")))
	for i := float32(0); i < 4; i++ {
		ctx.DrawRect(paintengine2d.XYWH(1.5, h/2-4+i*2.5, w-3, 1), paintengine2d.Fill(Hex("#6c7282")))
	}
}

// svGlyphMark is the mark on a round key or a capsule, in near-black.
func svGlyphMark(ctx *paintengine2d.Context, w, h float32, g string, down bool) {
	c := paintengine2d.Pt(w/2, h/2)
	if down {
		c = c.Add(paintengine2d.Pt(0.5, 0.5))
	}
	ink := paintengine2d.Fill(Hex(svGlyph))
	tri := func(x, y, s float32, right bool) {
		p := paintengine2d.NewPath()
		if right {
			p.MoveTo(x, y-s)
			p.LineTo(x+s*1.1, y)
			p.LineTo(x, y+s)
		} else {
			p.MoveTo(x, y-s)
			p.LineTo(x-s*1.1, y)
			p.LineTo(x, y+s)
		}
		p.Close()
		ctx.DrawPath(p, ink)
	}
	u := min(w, h) / 21
	switch g {
	case "prev":
		tri(c.X, c.Y, 3*u, false)
		tri(c.X+3.4*u, c.Y, 3*u, false)
	case "next":
		tri(c.X-3.4*u, c.Y, 3*u, true)
		tri(c.X, c.Y, 3*u, true)
	case "play":
		tri(c.X-1.8*u, c.Y, 4*u, true)
	case "pause":
		ctx.DrawRect(paintengine2d.XYWH(c.X-3*u, c.Y-3.5*u, 2.2*u, 7*u), ink)
		ctx.DrawRect(paintengine2d.XYWH(c.X+0.8*u, c.Y-3.5*u, 2.2*u, 7*u), ink)
	case "stop":
		ctx.DrawRect(paintengine2d.XYWH(c.X-3.2*u, c.Y-3.2*u, 6.4*u, 6.4*u), ink)
	case "eject":
		p := paintengine2d.NewPath()
		p.MoveTo(c.X-4*u, c.Y+0.5*u)
		p.LineTo(c.X, c.Y-4*u)
		p.LineTo(c.X+4*u, c.Y+0.5*u)
		p.Close()
		ctx.DrawPath(p, ink)
		ctx.DrawRect(paintengine2d.XYWH(c.X-4*u, c.Y+2*u, 8*u, 1.8*u), ink)
	case "shuffle":
		pixLabel(ctx, float32(int(c.X-5)), float32(int(c.Y-3)), "123", Hex(svGlyph))
	case "repeat":
		st := paintengine2d.StrokePaint(Hex(svGlyph), 1.2)
		ctx.DrawArc(c, 4, 4, 0.9, 5.2, st)
		p := paintengine2d.NewPath()
		p.MoveTo(c.X+2.2, c.Y-4.4)
		p.LineTo(c.X+4.8, c.Y-2.2)
		p.LineTo(c.X+1.6, c.Y-1.2)
		p.Close()
		ctx.DrawPath(p, ink)
	case "add", "rem":
		ctx.DrawCircle(c, 5*u*1.3, ink)
		ctx.DrawRect(paintengine2d.XYWH(c.X-3.5*u, c.Y-0.8*u, 7*u, 1.6*u), paintengine2d.Fill(Hex("#ffffff")))
		if g == "add" {
			ctx.DrawRect(paintengine2d.XYWH(c.X-0.8*u, c.Y-3.5*u, 1.6*u, 7*u), paintengine2d.Fill(Hex("#ffffff")))
		}
	case "sel":
		for i := float32(-1); i <= 1; i++ {
			ctx.DrawRect(paintengine2d.XYWH(c.X-4*u, c.Y+i*2.6*u-0.6*u, 1.4*u, 1.4*u), ink)
			ctx.DrawRect(paintengine2d.XYWH(c.X-1.8*u, c.Y+i*2.6*u-0.6*u, 6*u, 1.4*u), ink)
		}
	case "misc":
		for i := 0; i < 8; i++ {
			a := float32(i) * 3.14159 / 4
			ctx.DrawCircle(paintengine2d.Pt(c.X+4*u*cos32(a), c.Y+4*u*sin32(a)), 0.9*u, ink)
		}
		ctx.DrawCircle(c, 1.2*u, ink)
	case "opts":
		// A folder: the options for the list it holds.
		f := paintengine2d.XYWH(c.X-5*u, c.Y-3*u, 10*u, 7*u)
		ctx.DrawRoundRect(f, 1, 1, paintengine2d.StrokePaint(Hex(svGlyph), 1.2))
		ctx.DrawRect(paintengine2d.XYWH(f.Min.X, f.Min.Y-1.4*u, 4*u, 1.6*u), ink)
	}
}

// svDots prints a line of the capital face on the display, each pixel a dot.
func svDots(ctx *paintengine2d.Context, x, y float32, s string, col paintengine2d.Color) {
	for _, r := range strings.ToUpper(s) {
		rows, ok := pixGlyphs[r]
		if !ok {
			rows = pixGlyphs['?']
		}
		for yy, line := range rows {
			for xx, c := range line {
				if c == '#' {
					svPix(ctx, x+float32(xx), y+float32(yy), col)
				}
			}
		}
		x += float32(len(rows[0]) + 1)
	}
}

// svPix is one lit pixel of the dot matrix: nearly its whole cell, so at 1×
// it is a pixel and at 2× a dot with a hair of display between its
// neighbours.
func svPix(ctx *paintengine2d.Context, x, y float32, col paintengine2d.Color) {
	ctx.DrawRoundRect(paintengine2d.XYWH(x+0.05, y+0.05, 0.9, 0.9), 0.2, 0.2, paintengine2d.Fill(col))
}

// svDot is one cell of the big clock: two pixels square. It is a whole
// square rather than a round dot so it stays crisp at 1×, where a dot two
// pixels across is a smudge; the dot-matrix is the display's grid under it.
func svDot(ctx *paintengine2d.Context, x, y float32, col paintengine2d.Color) {
	ctx.DrawRect(paintengine2d.XYWH(x, y, 2, 2), paintengine2d.Fill(col))
}

// svDotDigit is a clock digit in dots: five by seven cells of two pixels.
func svDotDigit(ctx *paintengine2d.Context, rows []string) {
	for y, line := range rows {
		for x, c := range line {
			if c == '#' {
				svDot(ctx, float32(x)*2, float32(y)*2, Hex(svInk))
			}
		}
	}
}

// svBand is the navy title band: a lit top edge, round top corners, and a
// silver groove from the corner mark to the caption keys.
func svBand(ctx *paintengine2d.Context, w, h float32, active bool) {
	r := paintengine2d.XYWH(0, 0, w, h)
	top, bot := Hex(svNavyHi), Hex(svNavy)
	if !active {
		top, bot = Hex("#56607a"), Hex("#3a4258")
	}
	ctx.DrawRoundRectCorners(r, svCorner, svCorner, 0, 0, paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, 0), End: paintengine2d.Pt(0, h),
		Stops: []paintengine2d.GradientStop{stop(0, top), stop(1, bot)},
	}))
	ctx.Save()
	ctx.ClipRect(paintengine2d.XYWH(0, 1, w, 1))
	ctx.DrawRoundRectCorners(r.Inset(1), svCorner-1, svCorner-1, 0, 0, paintengine2d.Fill(Hex("#ffffff30")))
	ctx.Restore()
	svGrooveLine(ctx, 17, w-28, h, active)
	paintPlayerLogo(ctx, paintengine2d.XYWH(5, float32(int((h-10)/2)), 10, 10))
}

// svGrooveLine is the band's groove: a lit line on a dark one, round ends.
func svGrooveLine(ctx *paintengine2d.Context, x0, x1, h float32, active bool) {
	y := float32(int(h/2)) - 1
	hi := Hex("#c6cbd6")
	if !active {
		hi = Hex("#8f97aa")
	}
	ctx.DrawRoundRect(paintengine2d.XYWH(x0, y-0.5, x1-x0, 3.5), 1.7, 1.7, paintengine2d.Fill(Hex("#0d1428")))
	ctx.DrawRoundRect(paintengine2d.XYWH(x0+0.5, y, x1-x0-1, 2), 1, 1, paintengine2d.Fill(hi))
}

// svPlate is the plate under the title: plain band, stopping the groove with
// its round end either side of the words.
func svPlate(ctx *paintengine2d.Context, w, h float32, active bool) {
	top, bot := Hex(svNavyHi), Hex(svNavy)
	if !active {
		top, bot = Hex("#56607a"), Hex("#3a4258")
	}
	ctx.DrawRect(paintengine2d.XYWH(0, 0, w, h), paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(0, 0), End: paintengine2d.Pt(0, h),
		Stops: []paintengine2d.GradientStop{stop(0, top), stop(1, bot)},
	}))
	ctx.DrawRect(paintengine2d.XYWH(0, 1, w, 1), paintengine2d.Fill(Hex("#ffffff30")))
	ctx.Save()
	ctx.ClipRect(paintengine2d.XYWH(0, 0, 2, h))
	svGrooveLine(ctx, -6, 2, h, active)
	ctx.Restore()
	ctx.Save()
	ctx.ClipRect(paintengine2d.XYWH(w-2, 0, 2, h))
	svGrooveLine(ctx, w-2, w+6, h, active)
	ctx.Restore()
}

// svMark is the mark at the strip's lower right: the original player logo.
func svMark(ctx *paintengine2d.Context, r paintengine2d.Rect) {
	paintPlayerLogo(ctx, r)
}

// svGrip is the resize grip's diagonal lines in the lower right corner —
// decoration here, since the windows keep the size they were drawn at.
func svGrip(ctx *paintengine2d.Context, w, h float32) {
	for i := float32(0); i < 3; i++ {
		d := 4 + i*3
		ctx.DrawLine(paintengine2d.Pt(w-d, h-2), paintengine2d.Pt(w-2, h-d), paintengine2d.StrokePaint(Hex("#ffffffa0"), 0.8))
		ctx.DrawLine(paintengine2d.Pt(w-d+1, h-2), paintengine2d.Pt(w-2, h-d+1), paintengine2d.StrokePaint(Hex("#4a5060"), 0.8))
	}
}

func cos32(a float32) float32 { return float32(math.Cos(float64(a))) }
func sin32(a float32) float32 { return float32(math.Sin(float64(a))) }
