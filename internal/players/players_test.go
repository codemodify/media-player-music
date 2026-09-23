package players

import (
	"testing"
	"time"
)

// The transport is a clock, and the things worth pinning about a clock are
// the ends: what happens when it runs off the end of a track, what happens
// when the list runs out, and what the fraction a seek bar reads says at
// both ends.

func TestTheHeadRollsIntoTheNextTrack(t *testing.T) {
	list := NewLibrary()
	tr := NewTransport(list)
	tr.Play()
	first := list.Current()
	tr.Tick(first.Length - time.Second)
	if list.Index() != 0 {
		t.Fatalf("still on track 0 a second from the end: %d", list.Index())
	}
	tr.Tick(2 * time.Second)
	if list.Index() != 1 {
		t.Fatalf("track after the roll: got %d, want 1", list.Index())
	}
	if want := time.Second; tr.Pos != want {
		t.Errorf("position after the roll: got %v, want %v", tr.Pos, want)
	}
	if tr.State != Playing {
		t.Errorf("state after the roll: %v", tr.State)
	}
}

// A tick longer than a whole track must not land in the middle of one it
// skipped over: the demos repaint when the compositor lets them, and a
// window that was not repainted for a minute gets the whole minute at once.
func TestOneLongTickCrossesSeveralTracks(t *testing.T) {
	list := NewLibrary()
	tr := NewTransport(list)
	tr.Repeat = RepeatAll
	tr.Play()
	var want time.Duration
	for i := 0; i < 3; i++ {
		want += list.At(i).Length
	}
	tr.Tick(want + 5*time.Second)
	if list.Index() != 3 {
		t.Fatalf("three tracks and five seconds on: got track %d, want 3", list.Index())
	}
	if tr.Pos != 5*time.Second {
		t.Errorf("position: got %v, want 5s", tr.Pos)
	}
}

func TestTheListStopsAtItsEndUnlessItRepeats(t *testing.T) {
	list := NewLibrary()
	last := list.Len() - 1
	tr := NewTransport(list)
	tr.SelectTrack(last)
	tr.Play()
	tr.Tick(list.At(last).Length + time.Second)
	if tr.State != Stopped {
		t.Errorf("state past the end of the list: got %v, want Stopped", tr.State)
	}
	if tr.Pos != 0 {
		t.Errorf("position past the end: got %v, want 0", tr.Pos)
	}

	tr.Repeat = RepeatAll
	tr.SelectTrack(last)
	tr.Play()
	tr.Tick(list.At(last).Length + time.Second)
	if tr.State != Playing || list.Index() != 0 {
		t.Errorf("repeat all: state %v, track %d — want Playing, 0", tr.State, list.Index())
	}

	tr.Repeat = RepeatOne
	tr.SelectTrack(4)
	tr.Play()
	tr.Tick(list.At(4).Length + time.Second)
	if list.Index() != 4 || tr.State != Playing {
		t.Errorf("repeat one: track %d state %v — want 4, Playing", list.Index(), tr.State)
	}
}

func TestSeekAndTheFractionAgree(t *testing.T) {
	tr := NewTransport(NewLibrary())
	for _, f := range []float32{0, 0.25, 0.5, 0.999, 1} {
		tr.Seek(f)
		if got := tr.Fraction(); abs32(got-f) > 0.002 {
			t.Errorf("seek %.3f then read %.3f", f, got)
		}
	}
	// A drag that left the window is a fraction outside the bar.
	tr.Seek(-3)
	if tr.Pos != 0 {
		t.Errorf("seek below zero: %v", tr.Pos)
	}
	tr.Seek(9)
	if tr.Pos != tr.Length() {
		t.Errorf("seek past the end: %v, want %v", tr.Pos, tr.Length())
	}
	if tr.Remaining() != 0 {
		t.Errorf("remaining at the end: %v", tr.Remaining())
	}
}

func TestPreviousRestartsTheTrackOnceItIsUnderway(t *testing.T) {
	list := NewLibrary()
	tr := NewTransport(list)
	tr.SelectTrack(3)
	tr.Pos = 30 * time.Second
	tr.Prev()
	if list.Index() != 3 || tr.Pos != 0 {
		t.Fatalf("thirty seconds in, Prev went to track %d at %v", list.Index(), tr.Pos)
	}
	tr.Prev()
	if list.Index() != 2 {
		t.Fatalf("at the top of the track, Prev went to %d, want 2", list.Index())
	}
}

func TestSkipStopsAtTheEndsOfTheTrack(t *testing.T) {
	list := NewLibrary()
	tr := NewTransport(list)
	tr.Skip(-time.Minute)
	if tr.Pos != 0 || list.Index() != 0 {
		t.Errorf("skipping back off the front: %v on track %d", tr.Pos, list.Index())
	}
	tr.Skip(time.Hour)
	if tr.Pos != tr.Length() || list.Index() != 0 {
		t.Errorf("skipping past the end: %v on track %d", tr.Pos, list.Index())
	}
}

// Shuffle is a permutation beside the list, not of it: the order the user
// typed comes back when it is turned off, and the track playing does not
// change when it is turned on.
func TestShuffleKeepsTheListAndTheCurrentTrack(t *testing.T) {
	list := NewLibrary()
	before := append([]Track(nil), list.Tracks...)
	list.Select(4)
	list.Shuffle(true)
	if list.Index() != 4 {
		t.Errorf("shuffle moved the current track to %d", list.Index())
	}
	for i := range before {
		if list.Tracks[i].Title != before[i].Title {
			t.Fatalf("shuffle reordered the list at %d", i)
		}
	}
	seen := map[int]bool{}
	for i := 0; i < list.Len(); i++ {
		seen[list.Index()] = true
		list.Advance(1)
	}
	if len(seen) != list.Len() {
		t.Errorf("a pass through the shuffled order saw %d of %d tracks", len(seen), list.Len())
	}
	list.Shuffle(false)
	list.Select(0)
	list.Advance(1)
	if list.Index() != 1 {
		t.Errorf("shuffle off: next from 0 went to %d, want 1", list.Index())
	}
}

func TestClockReads(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00"},
		{59 * time.Second, "0:59"},
		{3*time.Minute + 47*time.Second, "3:47"},
		{62 * time.Minute, "1:02:00"},
		{-time.Second, "0:00"},
	} {
		if got := Clock(c.d); got != c.want {
			t.Errorf("Clock(%v) = %q, want %q", c.d, got, c.want)
		}
	}
	if got := Countdown(90 * time.Second); got != "-1:30" {
		t.Errorf("Countdown = %q", got)
	}
}

// The analyser must be a function of the position and nothing else, or two
// screenshots of the same moment are two different pictures.
func TestTheAnalyserIsReproducible(t *testing.T) {
	a, b := NewSpectrum(16), NewSpectrum(16)
	pos := 41 * time.Second
	// Same position, reached in one step and in twenty.
	a.Advance(pos, true, 50*time.Millisecond)
	for i := 1; i <= 20; i++ {
		b.Advance(time.Duration(i)*pos/20, true, 20*time.Millisecond)
	}
	for i := range a.Bars {
		if abs32(a.Bars[i]-b.Bars[i]) > 1e-5 {
			t.Fatalf("band %d: one step %.4f, twenty steps %.4f", i, a.Bars[i], b.Bars[i])
		}
	}
	for i, v := range a.Bars {
		if v < 0 || v > 1 {
			t.Errorf("band %d out of range: %v", i, v)
		}
		if a.Peaks[i] < v {
			t.Errorf("band %d: peak %v under bar %v", i, a.Peaks[i], v)
		}
	}
}

func TestTheAnalyserFallsSilentWhenStopped(t *testing.T) {
	s := NewSpectrum(12)
	s.Advance(30*time.Second, true, 50*time.Millisecond)
	loud := false
	for _, v := range s.Bars {
		if v > 0.05 {
			loud = true
		}
	}
	if !loud {
		t.Fatal("nothing moved while playing")
	}
	for i := 0; i < 40; i++ {
		s.Advance(30*time.Second, false, 50*time.Millisecond)
	}
	for i, v := range s.Bars {
		if v != 0 {
			t.Errorf("band %d still at %v two seconds after the music stopped", i, v)
		}
	}
}

func TestTheEqualiserClampsAndPresets(t *testing.T) {
	eq := NewEqualizer()
	eq.Set(0, 99)
	if eq.Gains[0] != EqRange {
		t.Errorf("clamped high to %v", eq.Gains[0])
	}
	eq.Set(1, -99)
	if eq.Gains[1] != -EqRange {
		t.Errorf("clamped low to %v", eq.Gains[1])
	}
	if eq.Preset != "Custom" {
		t.Errorf("a moved slider left the preset at %q", eq.Preset)
	}
	if !eq.Apply("Full bass") || eq.Preset != "Full bass" {
		t.Fatalf("applying a preset: %q", eq.Preset)
	}
	if eq.Gains[0] <= eq.Gains[len(eq.Gains)-1] {
		t.Errorf("full bass is not bass-heavy: %v", eq.Gains)
	}
	if eq.Apply("No Such Preset") {
		t.Error("an unknown preset applied")
	}
	eq.Flat()
	for i, v := range eq.Gains {
		if v != 0 {
			t.Errorf("band %d after Flat: %v", i, v)
		}
	}
	// The curve behind the sliders is normalised and as long as asked.
	eq.Apply("Full treble")
	curve := eq.Curve(64)
	if len(curve) != 64 {
		t.Fatalf("curve length %d", len(curve))
	}
	for i, v := range curve {
		if v < -1.001 || v > 1.001 {
			t.Errorf("curve[%d] = %v outside -1..1", i, v)
		}
	}
	if curve[0] >= curve[len(curve)-1] {
		t.Errorf("full treble's curve does not rise: %v .. %v", curve[0], curve[len(curve)-1])
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
