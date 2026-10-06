package players

import (
	"math"
	"testing"
	"time"
)

func TestLiveSpectrumUsesOnlyMeasuredLevelsAndDecaysWhenPausedOrStalled(t *testing.T) {
	s := NewSpectrum(3)
	s.Live = true
	s.AdvanceLive(true, time.Second)
	if s.Bars[0] != 0 || s.Peaks[0] != 0 {
		t.Fatal("live ticks invented an audio signal")
	}
	s.SetLevels([]float32{0, 1})
	if s.Bars[0] != 0 || s.Bars[1] != .5 || s.Bars[2] != 1 {
		t.Fatalf("measured bands lost their shape: %v", s.Bars)
	}
	s.SetLevels([]float32{0, 0, 0})
	if s.Bars[2] != 0 || s.Peaks[2] != 1 {
		t.Fatal("falling signal erased its peak immediately")
	}
	s.AdvanceLive(true, 100*time.Millisecond)
	if s.Peaks[2] >= 1 || s.Peaks[2] <= 0 {
		t.Fatal("peak did not fall smoothly")
	}
	for _, playing := range []bool{true, false} {
		s.SetLevels([]float32{1, 1, 1})
		s.AdvanceLive(playing, 3*time.Second)
		for i := range s.Bars {
			if s.Bars[i] != 0 || s.Peaks[i] != 0 {
				t.Fatalf("paused/stalled stream stayed lit: %v / %v", s.Bars, s.Peaks)
			}
		}
	}
}

func TestLiveSpectrumCannotPassInvalidLevelsToRendering(t *testing.T) {
	s := NewSpectrum(5)
	s.SetLevels([]float32{-1, 2, float32(math.NaN()), float32(math.Inf(1)), .25})
	want := []float32{0, 1, 0, 0, .25}
	for i, value := range s.Bars {
		if value != want[i] || s.Peaks[i] < value {
			t.Fatalf("invalid measured band %d: %v", i, value)
		}
	}
	s.Reset()
	for i := range s.Bars {
		if s.Bars[i] != 0 || s.Peaks[i] != 0 {
			t.Fatal("reset kept a signal from the previous track")
		}
	}
}
