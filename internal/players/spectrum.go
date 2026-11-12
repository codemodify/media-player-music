package players

import (
	"math"
	"time"
)

// Live playback supplies measured frequency-band levels through SetLevels.
// Advance provides deterministic synthetic values only for screenshot fixtures.

// Spectrum is a bar analyser with falling peak markers.
type Spectrum struct {
	// Live distinguishes measured playback from screenshot fixtures.
	Live bool
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
	// age detects a stalled measurement stream without keeping stale bars lit.
	age time.Duration
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

// SetLevels consumes normalized measured bands, interpolating when the
// producer's band count differs from the view's. Peak markers fall on UI ticks.
func (s *Spectrum) SetLevels(levels []float32) {
	if s == nil || len(levels) == 0 || len(s.Bars) == 0 {
		return
	}
	clean := func(v float32) float32 {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return 0
		}
		return clamp01(v)
	}
	for i := range s.Bars {
		at := float64(i) * float64(len(levels)-1) / float64(maxInt(len(s.Bars)-1, 1))
		lo := int(at)
		hi := min(lo+1, len(levels)-1)
		fraction := float32(at - float64(lo))
		s.Bars[i] = clean(levels[lo])*(1-fraction) + clean(levels[hi])*fraction
		s.Peaks[i] = max(s.Peaks[i], s.Bars[i])
	}
	s.age = 0
}

// AdvanceLive animates only decay and peaks; it never invents signal data.
func (s *Spectrum) AdvanceLive(playing bool, dt time.Duration) {
	if s == nil {
		return
	}
	dt = max(dt, 0)
	s.age += dt
	step := float32(dt.Seconds())
	for i := range s.Bars {
		if !playing || s.age > 300*time.Millisecond {
			s.Bars[i] = max(s.Bars[i]-s.Decay*step, 0)
		}
		s.Peaks[i] = max(s.Peaks[i]-s.Fall*step, s.Bars[i])
	}
}

// Reset takes every band and marker back to nothing.
func (s *Spectrum) Reset() {
	s.age = 0
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
