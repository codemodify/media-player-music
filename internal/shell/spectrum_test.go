package shell

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
)

func spectrumEvent(generation int64, position float64, level float32) playback.Event {
	data, _ := json.Marshal(playback.SpectrumData{Generation: generation, Time: position, Bands: []float32{level, level}})
	return playback.Event{Event: "spectrum", Data: data}
}

func TestMeasuredSpectrumRejectsOldLoadsSeeksAndPausedFrames(t *testing.T) {
	_, s, _ := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	identify(s, 11)
	epoch := s.playback.spectrumEpoch
	s.playbackEvent(spectrumEvent(epoch, 0, .8))
	if s.Host.Model.Spectrum.Bars[0] != .8 {
		t.Fatal("a current decoder measurement did not reach the visualizer")
	}
	s.Host.Command(players.CmdNext)
	s.playbackEvent(spectrumEvent(epoch, 0, 1))
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("old track measurements flashed in the new track")
	}
	identify(s, 12)
	epoch = s.playback.spectrumEpoch
	s.Host.Model.Transport.List.Tracks[1].Length = time.Minute
	s.Host.Seek(.5)
	s.playbackEvent(spectrumEvent(epoch, 0, 1))
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("pre-seek measurements flashed at the new position")
	}
	s.playbackEvent(playback.Event{Event: "seek"})
	s.playbackEvent(playback.Event{Event: "playback-restart"})
	s.playbackEvent(spectrumEvent(s.playback.spectrumEpoch, 30, .6))
	if s.Host.Model.Spectrum.Bars[0] != .6 {
		t.Fatal("new seek measurements were not accepted")
	}
	s.Host.Command(players.CmdPlayPause)
	s.playbackEvent(spectrumEvent(s.playback.spectrumEpoch, 30, 1))
	if s.Host.Model.Spectrum.Bars[0] != .6 {
		t.Fatal("a paused decoder measurement changed the display")
	}
	s.tick(3 * time.Second)
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("the paused visualizer did not fall to silence")
	}
	s.Host.Play()
	s.playbackEvent(spectrumEvent(s.playback.spectrumEpoch, 55, 1))
	if s.Host.Model.Spectrum.Bars[0] != 0 {
		t.Fatal("far-future decoder measurements were accepted")
	}
}
