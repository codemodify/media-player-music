package shell

import (
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
)

// spectrum hands the shell bands measured at a position, of an epoch of the
// current load.
func spectrum(s *Shell, epoch int64, position float64, level float32) {
	update(s, playback.Update{Kind: playback.UpdateSpectrum, Epoch: epoch, Position: time.Duration(position * float64(time.Second)), Bands: []float32{level, level}})
}

func TestMeasuredSpectrumRejectsOldLoadsSeeksAndPausedFrames(t *testing.T) {
	_, s, _ := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	loaded(s)
	epoch := serviceOf(s).service.Epoch()
	spectrum(s, epoch, 0, .8)
	if s.Host.Model.Spectrum.Bars[0] != .8 {
		t.Fatal("a current decoder measurement did not reach the visualizer")
	}
	s.Host.Command(players.CmdNext)
	spectrum(s, epoch, 0, 1)
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("old track measurements flashed in the new track")
	}
	loaded(s)
	epoch = serviceOf(s).service.Epoch()
	s.Host.Model.Transport.List.Tracks[1].Length = time.Minute
	s.Host.Seek(.5)
	spectrum(s, epoch, 0, 1)
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("pre-seek measurements flashed at the new position")
	}
	seeked(s)
	spectrum(s, serviceOf(s).service.Epoch(), 30, .6)
	if s.Host.Model.Spectrum.Bars[0] != .6 {
		t.Fatal("new seek measurements were not accepted")
	}
	s.Host.Command(players.CmdPlayPause)
	spectrum(s, serviceOf(s).service.Epoch(), 30, 1)
	if s.Host.Model.Spectrum.Bars[0] != .6 {
		t.Fatal("a paused decoder measurement changed the display")
	}
	s.tick(3 * time.Second)
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("the paused visualizer did not fall to silence")
	}
	s.Host.Play()
	spectrum(s, serviceOf(s).service.Epoch(), 55, 1)
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("far-future decoder measurements were accepted")
	}
}
