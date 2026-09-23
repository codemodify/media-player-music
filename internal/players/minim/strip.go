package minim

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// ---- the strip ---------------------------------------------------------------

// strip is the main window's content: a display, a seek bar and a transport
// row, laid out by hand because a compact player's proportions *are* its
// design and a flex box would give them away.
//
// It is laid out one of three ways (face.go). The widget face is Minim's own
// row of glyphs, placed here; in the two panel faces the skin places the
// same controls, and the three a panel of the era also had — a pause key of
// its own, an eject key and a balance slider — on the slots of its strip
// layout, where its art has holes for them.
type strip struct {
	widget.Base
	p *Player

	readout  *readout
	analyser *players.AnalyserView
	seek     *players.Fader
	volume   *players.Fader
	balance  *players.Fader

	prev, play, pause, stop, next *players.GlyphButton
	eject, mute                   *players.GlyphButton
	shuffle, repeat               *players.GlyphButton
	eqBtn, listBtn, skinBtn       *players.GlyphButton

	// slots is where each control goes in a panel face: the skin's strip
	// layout says where each slot is.
	slots *widget.Slots
}

func newStrip(p *Player) *strip {
	s := &strip{p: p}
	s.Init(s)
	s.SetManagesChildren(true)

	s.readout = newReadout(p)
	s.analyser = players.NewAnalyserView(p.Spectrum)
	s.analyser.Drag = p.Main
	s.analyser.Peaks = true

	// The seek bar is a fader on its side rather than the toolkit's slider,
	// because a panel draws its thumb as a picture and the toolkit's slider
	// draws its own. The clock does not move it while the pointer does.
	s.seek = players.NewBar(0, 1, 0, "Seek", func(v float32) { p.Transport.Seek(v) })
	s.seek.Step, s.seek.Page = 0.02, 0.1
	s.seek.Format = func(v float32) string {
		n := p.Transport.Length()
		return players.Clock(time.Duration(float64(n)*float64(v))) + " of " + players.Clock(n)
	}
	s.volume = players.NewBar(0, 100, p.Transport.Volume*100, "Volume", func(v float32) {
		p.Transport.SetVolume(v / 100)
	})
	s.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+0.5)) }
	s.balance = players.NewBar(-1, 1, p.Transport.Pan, "Balance", func(v float32) { p.Transport.SetPan(v) })
	s.balance.Format = balanceText

	s.prev = players.NewGlyphButton(players.GlyphPrev, "Previous track", func() { p.Command(players.CmdPrev) })
	s.play = players.NewGlyphButton(players.GlyphPlay, "Play", func() { p.Command(players.CmdPlayPause) })
	s.pause = players.NewGlyphButton(players.GlyphPause, "Pause", func() {
		// The panel's own pause key: it pauses and resumes and never
		// starts, which is the one thing it has over the play key.
		if p.Transport.State != players.Stopped {
			p.Command(players.CmdPlayPause)
		}
	})
	s.stop = players.NewGlyphButton(players.GlyphStop, "Stop", func() { p.Command(players.CmdStop) })
	s.next = players.NewGlyphButton(players.GlyphNext, "Next track", func() { p.Command(players.CmdNext) })
	s.eject = players.NewGlyphButton(players.GlyphEject, "Eject: stop and go back to the first track", func() {
		// There is nothing to load — the player plays nothing — so the
		// eject key does the half of ejecting that is true here: it takes
		// the queue out and puts it back at the top.
		p.Command(players.CmdStop)
		p.Transport.SelectTrack(0)
		p.Transport.Stop()
	})
	s.mute = players.NewGlyphButton(players.GlyphVolume, "Mute", func() { p.Command(players.CmdMute) })
	s.mute.Toggle = true

	s.shuffle = players.NewGlyphButton(players.GlyphShuffle, "Shuffle", func() { p.Command(players.CmdShuffle) })
	s.shuffle.Toggle = true
	s.repeat = players.NewGlyphButton(players.GlyphRepeat, "Repeat", func() { p.Command(players.CmdRepeat) })
	s.repeat.Toggle = true
	s.eqBtn = players.NewGlyphButton(players.GlyphSliders, "Equaliser window", func() { p.toggle(p.iEq) })
	s.eqBtn.Toggle = true
	s.listBtn = players.NewGlyphButton(players.GlyphList, "Playlist window", func() { p.toggle(p.iList) })
	s.listBtn.Toggle = true
	s.skinBtn = players.NewGlyphButton(players.GlyphSkin, "Skin", func() { p.NextSkin() })

	// In a panel face every control sits in a slot of the skin's strip
	// layout, and every key is the picture its slot gives it (face.go). The
	// mute key has no slot: the panels have none, and it is hidden there.
	s.slots = widget.NewSlots(layoutStrip).
		Bind("display", s.readout).Bind("analyser", s.analyser).Bind("seek", s.seek).
		Bind("volume", s.volume).Bind("balance", s.balance)
	for b, slot := range map[*players.GlyphButton]string{
		s.prev: "prev", s.play: "play", s.pause: "pause", s.stop: "stop",
		s.next: "next", s.eject: "eject", s.shuffle: "shuffle", s.repeat: "repeat",
		s.eqBtn: "eq", s.listBtn: "pl", s.skinBtn: "skin",
	} {
		s.slots.Bind(slot, b)
		paintAsKey(b, layoutStrip, slot)
	}
	paintAsThumb(s.seek, "seek.thumb", layoutStrip, "seek")
	paintAsThumb(s.volume, "thumb", layoutStrip, "volume")
	paintAsThumb(s.balance, "thumb", layoutStrip, "balance")

	for _, c := range s.kids() {
		s.Add(c)
	}
	return s
}

// kids is the strip's controls in tab order. It is component order, not art
// order, and the same in every face: the transport, then the levels, then
// the toggles, then the skin key.
func (s *strip) kids() []widget.Component {
	return []widget.Component{
		s.readout, s.analyser, s.seek,
		s.prev, s.play, s.pause, s.stop, s.next, s.eject,
		s.mute, s.volume, s.balance,
		s.shuffle, s.repeat, s.eqBtn, s.listBtn, s.skinBtn,
	}
}

// show hides what a face has no place for: the panels have their own pause,
// eject and balance and no mute key; the widget face the other way round.
func (s *strip) show(f face) {
	panelled := f.panelled()
	s.pause.SetVisible(panelled)
	s.eject.SetVisible(panelled)
	s.balance.SetVisible(panelled)
	s.mute.SetVisible(!panelled)
	s.skinBtn.SetAccessibleName("Skin: " + SkinLabel(s.p.Worn()) + " (next skin, Ctrl+K)")
	s.skinBtn.Label = "Skin: " + SkinLabel(s.p.Worn()) + " — Ctrl+K for the next, right-click for all"
}

// toggle opens or closes one of the satellite windows.
func (p *Player) toggle(i int) {
	pane := p.Desk.Rack.Pane(i)
	if pane == nil {
		return
	}
	p.Desk.Show(i, !pane.Shown)
	p.refresh()
}

// sync puts the model into the controls: the play button's glyph, the
// toggles, the seek bar's thumb.
func (s *strip) sync() {
	t := s.p.Transport
	panelled := faceOf(s.Look()).panelled()
	switch {
	case panelled:
		// A panel has a key for each, so the play key stays a play key.
		s.play.SetGlyph(players.GlyphPlay, "Play")
		s.pause.SetChecked(t.State == players.Paused)
	case t.State == players.Playing:
		s.play.SetGlyph(players.GlyphPause, "Pause")
	default:
		s.play.SetGlyph(players.GlyphPlay, "Play")
	}
	s.mute.SetGlyph(glyphForVolume(t), muteLabel(t))
	s.mute.SetChecked(t.Muted())
	s.shuffle.SetChecked(t.Random)
	s.repeat.SetChecked(t.Repeat != players.RepeatOff)
	if t.Repeat == players.RepeatOne {
		s.repeat.SetGlyph(players.GlyphRepeatOne, "Repeat one")
	} else {
		s.repeat.SetGlyph(players.GlyphRepeat, t.Repeat.String())
	}
	if p := s.p.Desk.Rack.Pane(s.p.iEq); p != nil {
		s.eqBtn.SetChecked(p.Shown)
	}
	if p := s.p.Desk.Rack.Pane(s.p.iList); p != nil {
		s.listBtn.SetChecked(p.Shown)
	}
	if !s.seek.Dragging() {
		s.seek.Set(t.Fraction())
	}
	s.volume.Set(t.Volume * 100)
	s.balance.Set(t.Pan)
	s.readout.Invalidate()
	s.analyser.Invalidate()
	s.seek.Invalidate()
	s.volume.Invalidate()
	if panelled {
		// The clock, the title and the lamps are printed on the strip
		// itself in a panel face.
		s.Invalidate()
	}
}

func glyphForVolume(t *players.Transport) players.Glyph {
	if t.Muted() {
		return players.GlyphMute
	}
	return players.GlyphVolume
}

func muteLabel(t *players.Transport) string {
	if t.Muted() {
		return "Unmute"
	}
	return "Mute"
}

// Arrange lays the strip out in its face.
func (s *strip) Arrange(r paintengine2d.Rect) {
	s.SetBounds(r)
	f := faceOf(s.Look())
	s.show(f)
	s.styleAnalyser(f)
	if f.panelled() {
		// Every control on its hole in the panel, where the skin says. A
		// control the skin has no slot for is left where the face's show
		// put it — off — rather than anywhere the skin did not say.
		s.slots.Arrange(s.Look(), s.LocalBounds())
		s.mute.Arrange(paintengine2d.Rect{})
		return
	}
	s.arrangeWidgets()
}

// arrangeWidgets is Minim's own face: three rows in eighty design pixels,
// stated in the units the art was drawn in.
func (s *strip) arrangeWidgets() {
	lk := s.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	b := s.LocalBounds()
	w := b.Dx()

	// Row A: the display, with the analyser inside its right-hand end.
	displayH := dip(38)
	display := paintengine2d.XYWH(0, 0, w, displayH)
	s.readout.Arrange(display)
	an := dip(AnalyserWidth) - dip(6)
	s.analyser.Arrange(paintengine2d.XYWH(display.Max.X-an-dip(3), display.Min.Y+dip(4), an, displayH-dip(9)))

	// Row B: the seek bar, the full width of the strip.
	y := displayH + dip(5)
	seekH := dip(10)
	s.seek.Arrange(paintengine2d.XYWH(0, y, w, seekH))

	// Row C: transport, volume, toggles.
	y += seekH + dip(5)
	btn := dip(21)
	x := float32(0)
	for _, c := range []*players.GlyphButton{s.prev, s.play, s.stop, s.next} {
		c.Arrange(paintengine2d.XYWH(x, y, btn, btn))
		x += btn + dip(1)
	}
	x += dip(5)
	s.mute.Arrange(paintengine2d.XYWH(x, y, dip(18), btn))
	x += dip(19)

	// The toggles are pinned to the right-hand end and the volume slider
	// takes whatever is between: the row keeps its shape when the strip is
	// wider than it was drawn. The skin key is the last of them.
	small := dip(19)
	toggles := []*players.GlyphButton{s.shuffle, s.repeat, s.eqBtn, s.listBtn, s.skinBtn}
	tw := float32(len(toggles))*small + float32(len(toggles)-1)*dip(1)
	tx := w - tw
	volW := max(tx-dip(6)-x, dip(24))
	s.volume.Arrange(paintengine2d.XYWH(x, y+(btn-dip(10))/2, volW, dip(10)))
	for _, c := range toggles {
		c.Arrange(paintengine2d.XYWH(tx, y+(btn-small)/2, small, small))
		tx += small + dip(1)
	}
	for _, c := range []widget.Component{s.pause, s.eject, s.balance} {
		c.Arrange(paintengine2d.Rect{})
	}
}

// styleAnalyser dresses the analyser for the face: the phosphor bars of
// Minim's own, the green-to-orange climb of the classic panel, the dotted
// columns of the silver one.
func (s *strip) styleAnalyser(f face) {
	a := s.analyser
	a.Pixelated, a.Gap = true, 1
	a.Ramp, a.PeakTint = nil, paintengine2d.Color{}
	a.MinBarW = 2
	if in := f.ink(); in != nil {
		a.Ramp, a.PeakTint = in.ramp, in.peak
		a.MinBarW = 3
	}
}

func (s *strip) Measure(c layout.Constraints) paintengine2d.Point {
	lk := s.Look()
	if sz, ok := s.slots.Measure(lk, c); ok {
		return sz
	}
	return c.Constrain(paintengine2d.Pt(style.Dip(lk, StripW), style.Dip(lk, 80)))
}

// Paint is nothing in the widget face — the window's own background is the
// panel there — and the whole printed face of a panel: the art, and over it
// the clock, the title, the figures and the lamps.
func (s *strip) Paint(ctx *paintengine2d.Context) {
	lk := s.Look()
	f := faceOf(lk)
	if !f.panelled() {
		return
	}
	b := s.LocalBounds()
	style.DrawSkinLayout(lk, ctx, b, layoutStrip)
	at := func(slot string) paintengine2d.Rect { return slotRect(lk, layoutStrip, slot, b) }
	in := f.ink()
	t := s.p.Transport
	tr := t.Track()

	state := "state.stop"
	switch t.State {
	case players.Playing:
		state = "state.play"
	case players.Paused:
		state = "state.pause"
	}
	art(lk, ctx, at("state"), state)
	if t.State != players.Stopped {
		ledClock(lk, ctx, b, players.Clock(t.Pos))
	}

	// The title as the era printed it: the track's place, the artist and
	// title, and its length.
	title := fmt.Sprintf("%d. %s (%s)", t.List.Index()+1, tr.Label(), players.Clock(tr.Length))
	pixScroll(lk, ctx, at("title"), title, in.text, s.p)

	rt, ft := at("rate"), at("freq")
	rate := fmt.Sprint(kbps(tr))
	pixText(lk, ctx, rt.Max.X-pixWidth(lk, rate), rt.Min.Y, rate, in.text)
	freq := fmt.Sprint(tr.Rate)
	pixText(lk, ctx, ft.Max.X-pixWidth(lk, freq), ft.Min.Y, freq, in.text)
	if tr.Channels >= 2 {
		art(lk, ctx, at("stereo"), "lamp.stereo")
	} else {
		art(lk, ctx, at("mono"), "lamp.mono")
	}
}

// kbps is the invented bit rate the display prints: a player of the era
// printed one whether anyone read it or not, and an invented track has an
// invented one.
func kbps(t players.Track) int {
	if t.Rate >= 48 {
		return 192
	}
	return 128
}

// MousePress on the strip itself starts a window move, so the shell between
// the controls is a drag handle — which is what the whole face of a player
// this size has always been. The secondary button opens the skin menu.
func (s *strip) MousePress(e widget.MouseEvent) bool {
	if s.p.rightClick(s, e) {
		return true
	}
	s.p.Main.StartMove()
	return true
}

func (s *strip) KeyPress(e widget.KeyEvent) bool { return s.p.Keys(s.p.Main, e) }

// CaptionAt tells the frame that the strip's own background is caption: a
// press there moves the window rather than landing in the app.
func (s *strip) CaptionAt(paintengine2d.Point) bool { return true }

func (s *strip) Describe(n *a11y.Node) {
	n.Role = a11y.RolePane
	if n.Name == "" {
		n.Name = "Player"
	}
	n.Description = players.Disclaimer
}

// balanceText reads a balance the way a person says it.
func balanceText(v float32) string {
	switch {
	case v < -0.02:
		return fmt.Sprintf("%d%% left", int(-v*100+0.5))
	case v > 0.02:
		return fmt.Sprintf("%d%% right", int(v*100+0.5))
	}
	return "centre"
}
