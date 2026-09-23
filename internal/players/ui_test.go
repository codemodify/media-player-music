package players

import (
	"testing"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// ---- the transport button ------------------------------------------------------

// A glyph has no words on it, so everything that would normally come from a
// label has to come from somewhere else: the name, the tooltip, the
// accessibility node and the action a screen reader can run.
func TestAGlyphButtonIsAWholeControl(t *testing.T) {
	hits := 0
	b := NewGlyphButton(GlyphPlay, "Play", func() { hits++ })
	if !b.WantsFocus() {
		t.Error("a transport button does not take the focus")
	}
	if b.Tooltip() != "Play" {
		t.Errorf("tooltip %q", b.Tooltip())
	}
	if b.ShapeRole() != style.RoleTool {
		t.Errorf("shape role %v", b.ShapeRole())
	}
	for _, k := range []platform.Key{platform.KeyReturn, platform.KeySpace} {
		if !b.KeyPress(widget.KeyEvent{Key: k}) {
			t.Errorf("%v did not work the button", k)
		}
	}
	if hits != 2 {
		t.Errorf("the button fired %d times, want 2", hits)
	}

	var n a11y.Node
	b.Describe(&n)
	if n.Role != a11y.RoleButton || n.Name != "Play" {
		t.Errorf("described as %v %q", n.Role, n.Name)
	}
	if !n.Actions.Has(a11y.ActionDefault) {
		t.Error("a screen reader cannot press it")
	}
	if !b.AccessibleAction(0, a11y.ActionDefault) || hits != 3 {
		t.Error("the accessibility action did not press it")
	}

	// A toggle says so, and says which way it is.
	b.Toggle = true
	b.SetChecked(true)
	n = a11y.Node{}
	b.Describe(&n)
	if n.Role != a11y.RoleToggleButton || !n.State.Has(a11y.StateChecked) {
		t.Errorf("a checked toggle described as %v state %v", n.Role, n.State)
	}

	// A disabled one does nothing at all.
	b.SetEnabled(false)
	before := hits
	b.KeyPress(widget.KeyEvent{Key: platform.KeySpace})
	b.AccessibleAction(0, a11y.ActionDefault)
	if hits != before {
		t.Error("a disabled button fired")
	}
}

// Every glyph draws something, and none of them draws outside its box.
func TestEveryGlyphDrawsInsideItsBox(t *testing.T) {
	const side = 40
	for g := GlyphPlay; g <= GlyphSkin; g++ {
		img := paintengine2d.NewImage(side, side)
		ctx := paintengine2d.NewContext(img)
		ctx.Clear(paintengine2d.Color{})
		DrawGlyph(ctx, paintengine2d.XYWH(0, 0, side, side), g, paintengine2d.RGB(1, 1, 1), 2)
		ink := 0
		edge := 0
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				_, _, _, a := img.PremulAt(x, y)
				if a == 0 {
					continue
				}
				ink++
				if x == 0 || y == 0 || x == side-1 || y == side-1 {
					edge++
				}
			}
		}
		if ink < 20 {
			t.Errorf("glyph %d drew %d pixels", g, ink)
		}
		if edge > 0 {
			t.Errorf("glyph %d put %d pixels on the very edge of its box", g, edge)
		}
	}
}

// ---- the fader -----------------------------------------------------------------

func TestTheFaderIsAVerticalSlider(t *testing.T) {
	last := float32(0)
	f := NewFader(-EqRange, EqRange, 0, "62 Hz", func(v float32) { last = v })
	f.Format = Decibels
	if !f.WantsFocus() {
		t.Error("a fader does not take the focus")
	}
	// Up is more, which is the whole reason it is not a slider on its side.
	f.KeyPress(widget.KeyEvent{Key: platform.KeyUp})
	if f.Value <= 0 {
		t.Errorf("Up moved the fader to %v", f.Value)
	}
	f.KeyPress(widget.KeyEvent{Key: platform.KeyDown})
	f.KeyPress(widget.KeyEvent{Key: platform.KeyDown})
	if f.Value >= 0 {
		t.Errorf("Down moved the fader to %v", f.Value)
	}
	f.KeyPress(widget.KeyEvent{Key: platform.KeyHome})
	if f.Value != EqRange {
		t.Errorf("Home is %v, want the top (%v)", f.Value, EqRange)
	}
	f.KeyPress(widget.KeyEvent{Key: platform.KeyEnd})
	if f.Value != -EqRange {
		t.Errorf("End is %v, want the bottom (%v)", f.Value, -EqRange)
	}
	if last != f.Value {
		t.Errorf("the owner was told %v, the fader is at %v", last, f.Value)
	}
	// Set moves it without telling the owner: a preset moves eleven of
	// them and reports the change once.
	last = 99
	f.Set(3)
	if last != 99 || f.Value != 3 {
		t.Errorf("Set told the owner %v and left the fader at %v", last, f.Value)
	}

	var n a11y.Node
	f.Describe(&n)
	if n.Role != a11y.RoleSlider || !n.State.Has(a11y.StateVertical) {
		t.Errorf("described as %v state %v", n.Role, n.State)
	}
	if n.Name != "62 Hz" || n.Value != "+3.0 dB" {
		t.Errorf("named %q reading %q", n.Name, n.Value)
	}
	if !n.HasRange || n.Min != -EqRange || n.Max != EqRange {
		t.Errorf("range %v..%v (has=%v)", n.Min, n.Max, n.HasRange)
	}
	if !f.AccessibleAction(0, a11y.ActionIncrement) || f.Value <= 3 {
		t.Errorf("the accessibility action left it at %v", f.Value)
	}
}

// ---- the keyboard table --------------------------------------------------------

func TestTheKeyTableReadsKeysNotRunes(t *testing.T) {
	// The event a component is handed carries the key; the character is a
	// separate event, so a table that read runes would never fire.
	for _, c := range []struct {
		key  platform.Key
		want Command
	}{
		{platform.KeySpace, CmdPlayPause},
		{platform.KeyB, CmdNext},
		{platform.KeyZ, CmdPrev},
		{platform.KeyV, CmdStop},
		{platform.KeyM, CmdMute},
		{platform.KeyS, CmdShuffle},
		{platform.KeyR, CmdRepeat},
		{platform.KeyLeft, CmdSeekBack},
		{platform.KeyUp, CmdVolumeUp},
		{platform.KeyTab, CmdNone},
	} {
		if got := CommandFor(widget.KeyEvent{Key: c.key}); got != c.want {
			t.Errorf("key %v gave command %v, want %v", c.key, got, c.want)
		}
	}
	// A chord is somebody else's: the players' own shortcuts are chords
	// and would collide.
	if got := CommandFor(widget.KeyEvent{Key: platform.KeyM, Mods: platform.ModCtrl}); got != CmdNone {
		t.Errorf("Ctrl+M gave %v", got)
	}
}

func TestTypingKnowsAFieldFromAButton(t *testing.T) {
	if Typing(nil) {
		t.Error("nothing is typing")
	}
	if Typing(NewGlyphButton(GlyphPlay, "Play", nil)) {
		t.Error("a transport button counted as a text field")
	}
	if !Typing(widgets.NewTextField("", "Filter", nil)) {
		t.Error("a text field did not count as one")
	}
	if !Typing(widgets.NewComboBox([]string{"a"}, 0, nil)) {
		t.Error("a combo box did not count: its letters are its own type-ahead")
	}
}

// ---- the clock -----------------------------------------------------------------

func TestThePulseMeasuresRealTimeAndClampsALostOne(t *testing.T) {
	now := time.Unix(0, 0)
	var seen []time.Duration
	p := NewPulse(50*time.Millisecond, func(dt time.Duration) { seen = append(seen, dt) })
	p.SetClock(func() time.Time { return now })
	p.last = now

	now = now.Add(60 * time.Millisecond)
	p.Tick()
	now = now.Add(40 * time.Millisecond)
	p.Tick()
	// A machine that was asleep for a minute: the tick is clamped, or the
	// analyser's peak markers fall for a minute in one frame.
	now = now.Add(time.Minute)
	p.Tick()

	if len(seen) != 3 {
		t.Fatalf("%d ticks", len(seen))
	}
	if seen[0] != 60*time.Millisecond || seen[1] != 40*time.Millisecond {
		t.Errorf("the first two ticks were %v and %v", seen[0], seen[1])
	}
	if want := 8 * 50 * time.Millisecond; seen[2] != want {
		t.Errorf("a minute of lost time arrived as %v, want it clamped to %v", seen[2], want)
	}
}
