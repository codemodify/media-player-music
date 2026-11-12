package service

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/internal/model"
	"github.com/codemodify/media-player-music/musicapi"
)

// Saved is what is kept of the player between runs (the daemon's
// session.json, through vfs-statefile): the queue, the track and where it
// was, the levels, the mode, the equaliser and ReplayGain. A default is
// not written.
type Saved struct {
	Queue      []musicapi.Track    `json:"queue,omitempty"`
	Index      int                 `json:"index,omitempty"`
	PositionMS int64               `json:"positionMs,omitempty"`
	Volume     *float64            `json:"volume,omitempty"` // nil: the default
	Balance    float64             `json:"balance,omitempty"`
	Repeat     string              `json:"repeat,omitempty"`
	Shuffle    bool                `json:"shuffle,omitempty"`
	Equalizer  *musicapi.Equalizer `json:"equalizer,omitempty"`
	// ReplayGain's mode "" is track gain, the default.
	ReplayGain musicapi.ReplayGain `json:"replayGain,omitzero"`
}

// Saved is what is kept of the player now, for the daemon's
// statefile.Saver, which calls it as each write begins.
func (thisRef *Service) Saved() Saved {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	t := thisRef.transport
	saved := Saved{Index: max(t.List.Index(), 0), PositionMS: max(t.Pos, 0).Milliseconds(), Balance: float64(t.Pan), Shuffle: t.Random}
	for _, each := range t.List.Tracks {
		saved.Queue = append(saved.Queue, track(each))
	}
	if t.Volume != model.NewTransport(nil).Volume {
		volume := float64(t.Volume)
		saved.Volume = &volume
	}
	if t.Repeat != model.RepeatOff {
		saved.Repeat = repeat(t.Repeat)
	}
	if e := thisRef.equalizer; e.On || e.Auto || e.Preamp != 0 || e.Preset != "Flat" {
		kept := equalizer(e)
		saved.Equalizer = &kept
	}
	saved.ReplayGain = thisRef.gain
	if saved.ReplayGain.Mode == musicapi.ReplayGainTrack {
		saved.ReplayGain.Mode = ""
	}
	return saved
}

// restore makes the player as saved, stopped at the track and the place it
// was: files moved or removed since are left out, the others kept in
// their order.
func (thisRef *Service) restore(saved Saved) {
	tracks := make([]model.Track, 0, len(saved.Queue))
	selected := -1
	for i, each := range saved.Queue {
		info, err := os.Stat(each.Path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if each.Title == "" {
			each.Title = strings.TrimSuffix(filepath.Base(each.Path), filepath.Ext(each.Path))
		}
		if i == saved.Index {
			selected = len(tracks)
		}
		tracks = append(tracks, modelTrack(each))
	}
	t := model.NewTransport(model.NewPlaylist(tracks...))
	if saved.Volume != nil {
		t.SetVolume(float32(*saved.Volume))
	}
	t.SetPan(float32(saved.Balance))
	t.Repeat = repeatOf(saved.Repeat)
	t.SetRandom(saved.Shuffle)
	if selected >= 0 {
		t.List.Select(selected)
		if saved.PositionMS > 0 {
			t.Pos = time.Duration(saved.PositionMS) * time.Millisecond
			if length := t.Length(); length > 0 && t.Pos > length {
				t.Pos = length
			}
		}
	}
	thisRef.transport = t
	thisRef.equalizer = model.NewEqualizer()
	if saved.Equalizer != nil {
		setEqualizer(thisRef.equalizer, *saved.Equalizer, -1)
	}
	thisRef.gain = NormalReplayGain(saved.ReplayGain)
	thisRef.savedAt = t.Pos
	thisRef.version = 1
}
