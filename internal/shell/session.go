package shell

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/codemodify/media-player-music/internal/players"
)

// Winamp skins share their arrangement. Other presentations keep their own.
func presentationFamily(skin string) string {
	if players.IsPanelSkin(skin) {
		return players.WinampSkin
	}
	return skin
}

func (s *Shell) restoreSession() {
	m := s.Host.Model
	if panels := s.settings.Panels; panels != nil {
		m.EqualizerVisible, m.PlaylistVisible = panels.Equaliser, panels.Playlist
	}
}

func (s *Shell) capturePlacement(p Presentation, skin string) {
	family := presentationFamily(skin)
	if stateful, ok := p.(StatefulPresentation); ok {
		if s.settings.Presentations == nil {
			s.settings.Presentations = make(map[string]players.PresentationState)
		}
		s.settings.Presentations[family] = copyPresentationState(stateful.CaptureState())
	}
	placements := p.CapturePlacement()
	if len(placements) == 0 {
		return
	}
	if s.settings.Windows == nil {
		s.settings.Windows = make(map[string]map[string]players.WindowPlacement)
	}
	if s.settings.Windows[family] == nil {
		s.settings.Windows[family] = make(map[string]players.WindowPlacement)
	}
	for name, placement := range placements {
		// A desktop that cannot report window positions must not erase a
		// previously known location. It may still report a useful size.
		if !placement.Positioned {
			if previous, ok := s.settings.Windows[family][name]; ok && previous.Positioned {
				previous.Width, previous.Height = placement.Width, placement.Height
				placement = previous
			}
		}
		s.settings.Windows[family][name] = placement
	}
}

// Detach saved state from a presentation's mutable mode cache on both capture
// and restore. The persisted snapshot changes only at an explicit capture.
func copyPresentationState(state players.PresentationState) players.PresentationState {
	if state.Sizes == nil {
		return state
	}
	sizes := make(map[string]players.PresentationSize, len(state.Sizes))
	for mode, size := range state.Sizes {
		sizes[mode] = size
	}
	state.Sizes = sizes
	return state
}

func (s *Shell) remember() {
	s.rememberIn = 0
	s.settings.Skin = s.skin
	if s.Host.Model.Live {
		// the queue and where it is are the daemon's to keep
		s.settings.Panels = &players.SavedPanels{
			Equaliser: s.Host.Model.EqualizerVisible, Playlist: s.Host.Model.PlaylistVisible,
		}
		if s.presentation != nil {
			s.capturePlacement(s.presentation, s.skin)
		}
	}
	if !s.opts.Remember {
		return
	}
	data, err := json.Marshal(s.settings)
	if err != nil {
		log.Printf("save player session: %v", err)
		return
	}
	if bytes.Equal(data, s.lastSaved) {
		return
	}
	if err := s.settings.Save(); err != nil {
		log.Printf("save player session: %v", err)
		return
	}
	s.lastSaved = data
}
