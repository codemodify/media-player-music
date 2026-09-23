package minim

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/style"
)

// ---- the display -------------------------------------------------------------

// readout is the phosphor well: the clock, the track, and the flags a player
// of this shape printed beside them.
//
// It is painted rather than assembled from labels because the whole of it is
// one object — a display — and because the colours come from the look's
// palette, so the same component is a phosphor well under the skin and an
// ordinary sunken field under any of the other packs.
type readout struct {
	players.DragsWindow
	p *Player
}

func newReadout(p *Player) *readout {
	r := &readout{p: p}
	r.Init(r)
	r.Win = p.Main
	return r
}

func (r *readout) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(r.Look(), 200), style.Dip(r.Look(), 38)))
}

func (r *readout) Arrange(b paintengine2d.Rect) { r.SetBounds(b) }

// AnalyserWidth is how much of the display's right-hand end the analyser
// takes, in design pixels. The strip's Arrange puts it there and the
// readout leaves that much room, so the two agree in one place.
const AnalyserWidth = 74

func (r *readout) Paint(ctx *paintengine2d.Context) {
	lk, b := r.Look(), r.LocalBounds()
	if faceOf(lk).panelled() {
		// A panel's display is printed on the panel, by the strip, over
		// the art; this component is only its name and its drag handle.
		return
	}
	pal := lk.Palette()
	t := r.p.Transport
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	line := max(dip(1), 1)

	// The well. Palette colours, so this is a phosphor display under the
	// skin and an ordinary sunken field under any other pack.
	ctx.DrawRect(b, paintengine2d.Fill(pal.Field))
	ctx.DrawRect(paintengine2d.XYWH(b.Min.X, b.Min.Y, b.Dx(), line), paintengine2d.Fill(pal.FieldBorder))
	ctx.DrawRect(paintengine2d.XYWH(b.Min.X, b.Max.Y-line, b.Dx(), line), paintengine2d.Fill(pal.FieldBorder))

	pad := dip(4)
	right := b.Max.X - dip(AnalyserWidth)

	// The clock, in the pack's mono face at a display size: the one number
	// a player of this shape is read by.
	clock := players.Clock(t.Pos)
	if t.State == players.Stopped {
		clock = players.Clock(0)
	}
	big := players.ScaledFace(lk.MonoFont(), dip(17))
	cw := big.Advance(clock)
	players.DrawTextIn(ctx, big, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+dip(2), cw, dip(22)),
		clock, pal.Accent, style.AlignStart)

	// The flags beside it: what the transport is doing, and the invented
	// rate a player of this era printed whether anyone read it or not.
	tr := t.Track()
	x := b.Min.X + pad + cw + dip(6)
	small := players.ScaledFace(lk.Font(), dip(10))
	players.DrawTextIn(ctx, small, paintengine2d.XYWH(x, b.Min.Y+dip(3), max(right-x-dip(4), 0), dip(20)),
		fmt.Sprintf("%s · %d kHz · %s", shortState(t.State), tr.Rate, channels(tr)), pal.TextMuted, style.AlignStart)

	// The title under both, scrolling when it does not fit.
	players.DrawScrollingText(ctx, lk, lk.Font(),
		paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+dip(23), max(right-b.Min.X-2*pad, 0), dip(14)),
		tr.Label(), pal.Text, t.Pos)
}

func shortState(s players.State) string {
	switch s {
	case players.Playing:
		return "play"
	case players.Paused:
		return "pause"
	}
	return "stop"
}

func channels(t players.Track) string {
	if t.Channels >= 2 {
		return "stereo"
	}
	return "mono"
}

// Describe is the display as a screen reader meets it: one label whose name
// is everything the well says, so a reader announces the track and the time
// rather than a blank rectangle.
func (r *readout) Describe(n *a11y.Node) {
	t := r.p.Transport
	n.Role = a11y.RoleLabel
	n.Name = fmt.Sprintf("%s. %s of %s. %s.",
		t.Track().Label(), players.Clock(t.Pos), players.Clock(t.Length()), t.State)
	n.Description = players.Disclaimer
}
