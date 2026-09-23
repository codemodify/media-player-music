package players

import (
	"math"
	"time"
)

// The analyser.
//
// There is no signal to analyse, so this is the other half of the honest
// answer: a band of numbers that moves the way a spectrum does, generated
// from the transport's own clock. It is worth saying why it is not a random
// number generator called once a frame.
//
//   - It has to be **deterministic**. Two screenshots of the same app at
//     the same position must be the same picture, or nothing about the art
//     can be compared with the run before it.
//   - It has to be **frame-rate independent**. The same second of playing
//     must look the same whether the window repainted twelve times in it or
//     sixty, because one of the three demos runs at a different rate from
//     the others and all three run slower under a screen recorder.
//
// So the band values are a pure function of the track's position: a few
// sine waves at incommensurable rates per band, shaped by a falling curve
// so the bass end is loud and the top end is not. The peak markers are the
// one part with memory, because a peak that fell instantly would not be a
// peak marker.

// Spectrum is a bar analyser with falling peak markers.
type Spectrum struct {
	// Bars is the reading of each band, 0..1.
	Bars []float32
	// Peaks is the marker over each band, 0..1, and never below its bar.
	Peaks []float32
	// Fall is how far a peak marker drops per second.
	Fall float32
	// Decay is how fast the bars themselves fall away when the transport
	// is not playing, per second.
	Decay float32
	// seed offsets the whole pattern, so two analysers side by side in the
	// same app do not move in lockstep.
	seed float32
}

// NewSpectrum is an analyser of n bands, all at rest.
func NewSpectrum(n int) *Spectrum {
	if n < 1 {
		n = 1
	}
	return &Spectrum{
		Bars:  make([]float32, n),
		Peaks: make([]float32, n),
		Fall:  0.55,
		Decay: 2.4,
	}
}

// Bands is how many bands the analyser has.
func (s *Spectrum) Bands() int {
	if s == nil {
		return 0
	}
	return len(s.Bars)
}

// SetSeed offsets the pattern. Two analysers with different seeds never
// show the same picture at the same instant.
func (s *Spectrum) SetSeed(v float32) { s.seed = v }

// Advance moves the analyser to where it should be for a transport at
// position pos, dt after the last call.
//
// pos, not dt, decides the bars: that is what makes a screenshot
// reproducible. dt only drives the two things that have memory — the peak
// markers falling, and the bars decaying to nothing when the music stops.
func (s *Spectrum) Advance(pos time.Duration, playing bool, dt time.Duration) {
	if s == nil || len(s.Bars) == 0 {
		return
	}
	step := float32(dt) / float32(time.Second)
	if step < 0 {
		step = 0
	}
	t := float32(pos)/float32(time.Second) + s.seed
	n := len(s.Bars)
	for i := range s.Bars {
		want := float32(0)
		if playing {
			want = bandAt(i, n, t)
		} else {
			// Not playing: fall away rather than cut, so a paused
			// analyser settles instead of blinking off.
			want = s.Bars[i] - s.Decay*step
			if want < 0 {
				want = 0
			}
		}
		s.Bars[i] = want
		if p := s.Peaks[i] - s.Fall*step; p > want {
			s.Peaks[i] = p
		} else {
			s.Peaks[i] = want
		}
	}
}

// Reset takes every band and marker back to nothing.
func (s *Spectrum) Reset() {
	for i := range s.Bars {
		s.Bars[i], s.Peaks[i] = 0, 0
	}
}

// bandAt is one band's reading at time t: three sines whose periods share
// no common multiple, so the pattern does not repeat inside a track, under
// a tilt that makes the bass end loud and the treble end quiet.
//
// The numbers are chosen by eye against the picture, which is the only test
// that matters for something whose whole job is to look right.
func bandAt(i, n int, t float32) float32 {
	f := float32(i) / float32(maxInt(n-1, 1))
	// The tilt: about 1.0 at the bass end falling to about 0.38 at the top.
	tilt := 1 - 0.62*f
	phase := f * 6.2
	v := 0.52 +
		0.30*sin(t*2.7+phase*1.7) +
		0.22*sin(t*4.3+phase*2.9+1.1) +
		0.16*sin(t*8.9+phase*5.1+2.3)
	v *= tilt
	// A little life at the very bottom so a quiet band still twitches.
	v += 0.04 * sin(t*13.7+phase)
	return clamp01(v)
}

func sin(v float32) float32 { return float32(math.Sin(float64(v))) }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---- the equaliser ----------------------------------------------------------

// Equalizer is a preamp and a row of band gains. It filters nothing; the
// sliders move, the curve under them redraws, and that is the whole of it.
type Equalizer struct {
	On     bool
	Preamp float32   // dB
	Gains  []float32 // dB, one per band
	Preset string
	// Changed runs after any of the above moves.
	Changed func()
}

// EqRange is how far a band goes either way, in dB. Twelve is what the
// control of this era was marked at, and what the ten sliders of a compact
// player were tall enough to show.
const EqRange = 12

// EqBands are the centre frequencies the ten sliders are labelled with.
// They are the ISO octave centres every ten-band control used, which is a
// fact about the numbers and not anybody's artwork.
var EqBands = []string{"31", "62", "125", "250", "500", "1k", "2k", "4k", "8k", "16k"}

// NewEqualizer is a flat equaliser with as many bands as EqBands.
func NewEqualizer() *Equalizer {
	return &Equalizer{Gains: make([]float32, len(EqBands)), Preset: "Flat"}
}

// EqPreset is one named curve.
type EqPreset struct {
	Name  string
	Gains []float32
}

// EqPresets are the demo's own curves. The names are the ordinary words a
// tone control has always been labelled with; the numbers were drawn here.
func EqPresets() []EqPreset {
	return []EqPreset{
		{"Flat", []float32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"Full bass", []float32{9, 8, 6.5, 4, 1, -1, -2.5, -3, -3, -3}},
		{"Full treble", []float32{-4, -4, -3.5, -2, 0.5, 3, 6, 8, 9, 9.5}},
		{"Vocal", []float32{-3, -2, 0, 3, 5.5, 5, 3, 1, -1, -2}},
		{"Late night", []float32{5, 4, 2, 0, -1, -1, 0, 1.5, 2.5, 3}},
		{"Small speakers", []float32{6, 5, 3.5, 1.5, 0, -1, -1.5, -1, 1, 2}},
	}
}

// Apply sets the gains from a named preset and reports whether there was
// one by that name.
func (e *Equalizer) Apply(name string) bool {
	for _, p := range EqPresets() {
		if p.Name != name {
			continue
		}
		copy(e.Gains, p.Gains)
		for i := len(p.Gains); i < len(e.Gains); i++ {
			e.Gains[i] = 0
		}
		e.Preset = p.Name
		e.changed()
		return true
	}
	return false
}

// Set moves one band, clamped to the control's range, and drops the preset
// name: a curve a person has moved a slider on is no longer that preset.
func (e *Equalizer) Set(band int, db float32) {
	if e == nil || band < 0 || band >= len(e.Gains) {
		return
	}
	db = clampEq(db)
	if db == e.Gains[band] {
		return
	}
	e.Gains[band] = db
	e.Preset = "Custom"
	e.changed()
}

// SetPreamp moves the preamp.
func (e *Equalizer) SetPreamp(db float32) {
	db = clampEq(db)
	if db == e.Preamp {
		return
	}
	e.Preamp = db
	e.changed()
}

// Flat takes every band and the preamp back to zero.
func (e *Equalizer) Flat() {
	for i := range e.Gains {
		e.Gains[i] = 0
	}
	e.Preamp = 0
	e.Preset = "Flat"
	e.changed()
}

// Curve samples the gain across the bands at n points, normalised to
// -1..+1, so the panel behind the sliders can draw the line the sliders
// describe. Between bands it interpolates linearly, which is what the
// drawn curve of the era did and is honest about how little it knows.
func (e *Equalizer) Curve(n int) []float32 {
	if e == nil || len(e.Gains) == 0 || n < 1 {
		return nil
	}
	out := make([]float32, n)
	last := float32(len(e.Gains) - 1)
	for i := range out {
		at := float32(i) / float32(maxInt(n-1, 1)) * last
		lo := int(at)
		hi := lo + 1
		if hi > len(e.Gains)-1 {
			hi = len(e.Gains) - 1
		}
		f := at - float32(lo)
		out[i] = (e.Gains[lo]*(1-f) + e.Gains[hi]*f) / EqRange
	}
	return out
}

func (e *Equalizer) changed() {
	if e != nil && e.Changed != nil {
		e.Changed()
	}
}

func clampEq(v float32) float32 {
	if v < -EqRange {
		return -EqRange
	}
	if v > EqRange {
		return EqRange
	}
	return v
}
