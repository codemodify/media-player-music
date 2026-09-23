// Package panel is where the two panel faces of the compact player are, as
// numbers: every well, key, slider and label, in the design pixels the art is
// drawn in.
//
// Minim wears three skins. The first, "minim", dresses the ordinary widgets
// of an ordinary layout. The other two, "minim-classic" and "minim-silver",
// are panels: the window is a picture with holes in it where the controls
// are, which is how a player of this shape was always built.
//
// This package is where the generator (internal/skinart) reads them from. It
// draws each face's backgrounds with its wells, grooves and printed labels at
// these rects, and writes the same rects into each skin's manifest as a
// fixed layout (Layouts): named slots the player binds its controls to
// (docs/skins.md, "Fixed layouts"). The player takes nothing from this
// package but the layouts' names — it asks the skin it is wearing where each
// slot is — so the art and the layout cannot disagree about where a key is,
// and a skin someone else draws for the player places its keys wherever its
// own art has them.
//
// Every rect is {x, y, w, h} in the window's *content* box: the window less
// the skin's border and caption, which are stated here too because the art
// needs them and they are the skin's to say.
//
// The positions of the classic face follow the published layout of the
// 275×116 skin format of the era (a 14-pixel title bar, a clock at 48,26 in
// 9×13 digits, a 248-pixel seek bar at 16,72 and so on), which is widely
// documented and is a fact about a file format rather than anybody's art.
// The silver face's are measured off the proportions of the later rounded
// look. Nothing here is a picture.
package panel

// R is a rect in design pixels: x, y, w, h.
type R [4]int

// X, Y, W and H read a rect.
func (r R) X() int { return r[0] }
func (r R) Y() int { return r[1] }
func (r R) W() int { return r[2] }
func (r R) H() int { return r[3] }

// Right and Bottom are the far edges.
func (r R) Right() int  { return r[0] + r[2] }
func (r R) Bottom() int { return r[1] + r[3] }

// Grow is the rect with n more on every side.
func (r R) Grow(n int) R { return R{r[0] - n, r[1] - n, r[2] + 2*n, r[3] + 2*n} }

// The windows, in design pixels. They are the same three sizes in every face,
// so switching skins never moves or resizes a window.
const (
	WindowW = 275
	MainH   = 116
	EqH     = 116
	ListH   = 232
)

// Face is one panel face: its frame, and the three windows' layouts.
type Face struct {
	// Caption is the title band's height and Border the frame round the
	// content (top, right, bottom, left), in design pixels. EqCaption is
	// the equaliser's own where it has a caption of its own (a window
	// variant, docs/skins.md): the silver face's is its tab. Zero: Caption.
	Caption   int
	EqCaption int
	Border    [4]int

	Main Main
	Eq   Eq
	List List
}

// ContentW is the content box's width.
func (f *Face) ContentW() int { return WindowW - f.Border[1] - f.Border[3] }

// ContentH is the content box's height for a window of height h under the
// face's own caption.
func (f *Face) ContentH(h int) int { return h - f.Caption - f.Border[0] - f.Border[2] }

// EqContentH is the equaliser's content height, under its own caption.
func (f *Face) EqContentH() int { return EqH - f.eqCaption() - f.Border[0] - f.Border[2] }

func (f *Face) eqCaption() int {
	if f.EqCaption > 0 {
		return f.EqCaption
	}
	return f.Caption
}

// Main is the strip.
type Main struct {
	// Skin is the key that cycles the skins, in the place the era kept its
	// row of little option letters.
	Skin R
	// Display is the clock's well; State the play/pause/stop mark; Digits
	// the four digits' origins (the colon sits between the second and the
	// third) and Colon the colon's box.
	Display, State R
	Digits         [4]R
	Colon          R
	// Analyser is the spectrum inside the display.
	Analyser R
	// Title is the scrolling title's well and TitleText the line inside it.
	Title, TitleText R
	// Rate and Freq are the two small wells ("128" kbps, "44" kHz) and
	// RateText and FreqText the figures in them.
	Rate, Freq, RateText, FreqText R
	// Mono and Stereo are the two channel lamps.
	Mono, Stereo R
	// Volume and Balance are the two short sliders, and EQ and PL the
	// buttons that open the other two windows.
	Volume, Balance, EQ, PL R
	// Seek is the long bar.
	Seek R
	// Prev, Play, Pause, Stop and Next are the transport; Eject the key
	// beside it, and Shuffle and Repeat the two toggles.
	Prev, Play, Pause, Stop, Next, Eject, Shuffle, Repeat R
	// Mark is where the player's own mark is printed.
	Mark R
}

// Eq is the equaliser.
type Eq struct {
	// On, Auto and Presets are the three keys along the top, and Graph
	// the well the curve is drawn in.
	On, Auto, Presets, Graph R
	// Preamp is the preamp fader, and Bands the first band's; the others
	// follow at BandStep.
	Preamp, Bands R
	BandStep      int
	// Labels is the row the frequencies are printed on, and DB the column
	// the +12 / 0 / -12 marks are printed in.
	Labels, DB R
}

// Band is band i's fader.
func (e *Eq) Band(i int) R {
	b := e.Bands
	b[0] += i * e.BandStep
	return b
}

// List is the playlist.
type List struct {
	// Rows is the list's well and Scroll its scroll bar's groove.
	Rows, Scroll R
	// RowH is one row, and Font the size its text is set at, in the bold
	// weight when Bold says so.
	RowH, Font int
	Bold       bool
	// Add, Rem, Sel and Misc are the four keys at the bottom left, and
	// Opts the one at the right.
	Add, Rem, Sel, Misc, Opts R
	// Info is the small display with the selected track's length over the
	// queue's; Mini the six small transport keys, and Clock the small clock.
	Info  R
	Mini  [6]R
	Clock R
}

// Classic is the base-skin look of the era: slate-blue bevelled chrome, a
// black display with green segments, grey keys, an orange volume bar and a
// green balance bar.
var Classic = &Face{
	Caption: 14,
	Border:  [4]int{0, 1, 1, 1},
	Main: Main{
		Skin:      R{9, 8, 8, 40},
		Display:   R{8, 6, 95, 42},
		State:     R{25, 14, 9, 9},
		Digits:    [4]R{{47, 12, 9, 13}, {59, 12, 9, 13}, {77, 12, 9, 13}, {89, 12, 9, 13}},
		Colon:     R{70, 12, 3, 13},
		Analyser:  R{23, 29, 76, 16},
		Title:     R{107, 9, 158, 13},
		TitleText: R{110, 12, 152, 6},
		Rate:      R{107, 27, 19, 11},
		Freq:      R{152, 27, 15, 11},
		RateText:  R{110, 30, 15, 6},
		FreqText:  R{155, 30, 10, 6},
		Mono:      R{211, 28, 25, 10},
		Stereo:    R{237, 28, 30, 10},
		Volume:    R{106, 43, 68, 13},
		Balance:   R{176, 43, 38, 13},
		EQ:        R{218, 44, 23, 12},
		PL:        R{241, 44, 23, 12},
		Seek:      R{15, 58, 248, 10},
		Prev:      R{15, 74, 23, 18},
		Play:      R{38, 74, 23, 18},
		Pause:     R{61, 74, 23, 18},
		Stop:      R{84, 74, 23, 18},
		Next:      R{107, 74, 22, 18},
		Eject:     R{135, 75, 22, 16},
		Shuffle:   R{163, 75, 47, 15},
		Repeat:    R{209, 75, 28, 15},
		Mark:      R{250, 76, 15, 16},
	},
	Eq: Eq{
		On:       R{13, 4, 26, 12},
		Auto:     R{39, 4, 32, 12},
		Presets:  R{216, 4, 44, 12},
		Graph:    R{85, 2, 113, 19},
		Preamp:   R{20, 24, 14, 63},
		Bands:    R{77, 24, 14, 63},
		BandStep: 18,
		Labels:   R{0, 89, 273, 6},
		DB:       R{40, 24, 32, 63},
	},
	List: List{
		Rows:   R{11, 5, 244, 175},
		Scroll: R{258, 5, 9, 175},
		RowH:   15,
		Font:   11,
		Bold:   true,
		Add:    R{13, 188, 25, 18},
		Rem:    R{42, 188, 25, 18},
		Sel:    R{71, 188, 25, 18},
		Misc:   R{100, 188, 25, 18},
		Opts:   R{227, 188, 22, 18},
		Info:   R{129, 187, 92, 11},
		Mini: [6]R{
			{129, 201, 8, 8}, {137, 201, 8, 8}, {145, 201, 8, 8},
			{153, 201, 8, 8}, {161, 201, 8, 8}, {169, 201, 9, 8},
		},
		Clock: R{182, 200, 39, 10},
	},
}

// Silver is the rounded silver-and-blue look of the later era: silver
// chrome, a blue dot-matrix display, glossy round keys and capsule toggles.
var Silver = &Face{
	Caption:   15,
	EqCaption: 13,
	Border:    [4]int{0, 1, 1, 1},
	Main: Main{
		Skin:      R{9, 8, 7, 40},
		Display:   R{4, 3, 265, 54},
		State:     R{21, 13, 9, 7},
		Digits:    [4]R{{44, 11, 10, 14}, {57, 11, 10, 14}, {75, 11, 10, 14}, {88, 11, 10, 14}},
		Colon:     R{69, 11, 4, 14},
		Analyser:  R{21, 30, 66, 17},
		Title:     R{108, 9, 156, 12},
		TitleText: R{110, 12, 152, 6},
		Rate:      R{108, 27, 20, 10},
		Freq:      R{158, 27, 14, 10},
		RateText:  R{110, 29, 15, 6},
		FreqText:  R{160, 29, 10, 6},
		Mono:      R{205, 28, 24, 8},
		Stereo:    R{238, 28, 28, 8},
		Volume:    R{104, 44, 66, 10},
		Balance:   R{175, 44, 40, 10},
		EQ:        R{222, 43, 19, 12},
		PL:        R{243, 43, 19, 12},
		Seek:      R{12, 58, 250, 9},
		Prev:      R{15, 70, 21, 21},
		Play:      R{38, 70, 21, 21},
		Pause:     R{61, 70, 21, 21},
		Stop:      R{84, 70, 21, 21},
		Next:      R{107, 70, 21, 21},
		Eject:     R{136, 72, 17, 17},
		Shuffle:   R{168, 71, 33, 19},
		Repeat:    R{207, 71, 33, 19},
		Mark:      R{245, 74, 16, 16},
	},
	// The equaliser's header is its tab, which is its caption (EqCaption):
	// the face under it starts at the tab's foot, so the keys come up to
	// the top and the faders take the room the tab left.
	Eq: Eq{
		On:       R{14, 4, 22, 11},
		Auto:     R{39, 4, 30, 11},
		Presets:  R{222, 3, 42, 12},
		Graph:    R{88, 2, 110, 14},
		Preamp:   R{14, 19, 14, 71},
		Bands:    R{79, 19, 14, 71},
		BandStep: 18,
		Labels:   R{8, 92, 257, 8},
		DB:       R{35, 19, 38, 71},
	},
	List: List{
		Rows:   R{5, 3, 252, 176},
		Scroll: R{259, 3, 9, 176},
		RowH:   14,
		Font:   11,
		Add:    R{12, 188, 22, 20},
		Rem:    R{38, 188, 22, 20},
		Sel:    R{64, 188, 22, 20},
		Misc:   R{90, 188, 22, 20},
		Opts:   R{236, 188, 22, 20},
		Info:   R{120, 183, 104, 30},
		Mini: [6]R{
			{126, 201, 7, 7}, {134, 201, 7, 7}, {142, 201, 7, 7},
			{150, 201, 7, 7}, {158, 201, 7, 7}, {166, 201, 8, 7},
		},
		Clock: R{186, 200, 34, 9},
	},
}

// ---- the layouts the skins state -------------------------------------------------

// The names of the three layouts, and of the slots in them, are the player's
// vocabulary: it binds its controls to these names and asks the skin where
// each one is. A skin drawn for Minim states these three.
const (
	LayoutStrip     = "minim.strip"
	LayoutEqualiser = "minim.equaliser"
	LayoutPlaylist  = "minim.playlist"
)

// Slot is one named rect of a layout, and the sprite its control is painted
// from, when it is one of the keys ("" for a slot whose control paints
// itself or that the player prints into).
type Slot struct {
	Name string
	R    R
	Key  string
}

// Layout is one window's layout: its content size, the face under it and
// its slots.
type Layout struct {
	Name  string
	W, H  int
	Face  string
	Slots []Slot
}

// Layouts are the face's three layouts, as the skin states them.
func (f *Face) Layouts() []Layout {
	m, q, li := f.Main, f.Eq, f.List
	strip := Layout{Name: LayoutStrip, W: f.ContentW(), H: f.ContentH(MainH), Face: "main.face", Slots: []Slot{
		{"skin", m.Skin, "key.skin"},
		{"display", m.Display, ""},
		{"state", m.State, ""},
		{"colon", m.Colon, ""},
		{"analyser", m.Analyser, ""},
		{"title", m.TitleText, ""},
		{"rate", m.RateText, ""},
		{"freq", m.FreqText, ""},
		{"mono", m.Mono, ""},
		{"stereo", m.Stereo, ""},
		{"volume", m.Volume, ""},
		{"balance", m.Balance, ""},
		{"eq", m.EQ, "key.eq"},
		{"pl", m.PL, "key.pl"},
		{"seek", m.Seek, ""},
		{"prev", m.Prev, "key.prev"},
		{"play", m.Play, "key.play"},
		{"pause", m.Pause, "key.pause"},
		{"stop", m.Stop, "key.stop"},
		{"next", m.Next, "key.next"},
		{"eject", m.Eject, "key.eject"},
		{"shuffle", m.Shuffle, "key.shuffle"},
		{"repeat", m.Repeat, "key.repeat"},
	}}
	for i, d := range m.Digits {
		strip.Slots = append(strip.Slots, Slot{Name: "digit." + string(rune('0'+i)), R: d})
	}
	eq := Layout{Name: LayoutEqualiser, W: f.ContentW(), H: f.EqContentH(), Face: "eq.face", Slots: []Slot{
		{"on", q.On, "key.on"},
		{"auto", q.Auto, "key.auto"},
		{"presets", q.Presets, "key.presets"},
		{"graph", q.Graph, ""},
		{"preamp", q.Preamp, ""},
	}}
	for i := 0; i < 10; i++ {
		eq.Slots = append(eq.Slots, Slot{Name: "band." + string(rune('0'+i)), R: q.Band(i)})
	}
	list := Layout{Name: LayoutPlaylist, W: f.ContentW(), H: f.ContentH(ListH), Face: "list.face", Slots: []Slot{
		// The list takes its rows and its scroll bar, which sit side by
		// side; the rows, the bar and the first row are where it paints.
		{"list", R{li.Rows.X(), li.Rows.Y(), li.Scroll.Right() - li.Rows.X(), li.Rows.H()}, ""},
		{"rows", li.Rows, ""},
		{"scroll", li.Scroll, ""},
		{"row", R{li.Rows.X(), li.Rows.Y(), li.Rows.W(), li.RowH}, ""},
		{"info", li.Info, ""},
		{"clock", li.Clock, ""},
		{"add", li.Add, "key.add"},
		{"rem", li.Rem, "key.rem"},
		{"sel", li.Sel, "key.sel"},
		{"misc", li.Misc, "key.misc"},
		{"opts", li.Opts, "key.opts"},
	}}
	for i, g := range []string{"prev", "play", "pause", "stop", "next", "eject"} {
		list.Slots = append(list.Slots, Slot{Name: "mini." + string(rune('0'+i)), R: li.Mini[i], Key: "mini." + g})
	}
	return []Layout{strip, eq, list}
}
