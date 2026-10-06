package shell

import (
	"log"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
)

// openTray uses the application's logo, which main sets before opening the
// shell. The tray belongs to the shell rather than a presentation: changing
// skins replaces windows but keeps this one item and its current-window actions.
func (s *Shell) openTray() {
	if s.opts.Headless || s.closed || s.tray != nil {
		return
	}
	var image *paintengine2d.Image
	for _, candidate := range s.App.Icon() {
		if candidate != nil && candidate.Width > 0 && candidate.Width == candidate.Height &&
			(image == nil || candidate.Width > image.Width) {
			image = candidate
		}
	}
	item, err := s.App.NewStatusItem(platform.StatusItemOptions{
		ID: "media-player-music", Title: "Music Player", Tooltip: "Music Player",
		// Plasma resolves IconName before IconPixmap. This app-specific
		// unresolved name lets it use the supplied logo instead of the
		// toolkit's generic application-default-icon fallback.
		Icon:    platform.StatusIcon{Name: "media-player-music", Image: image},
		OnClick: s.showPlayer,
		Menu: []platform.StatusMenuItem{
			{Text: "Show player", OnClick: s.showPlayer},
			{Text: "Quit", OnClick: s.quitPlayer},
		},
	})
	if err != nil {
		if item != nil {
			_ = item.Close()
		}
		log.Printf("system tray: %v", err)
		return
	}
	s.tray = item
	item.SetOnShownChange(func(shown bool) {
		if !shown && s.tray == item && s.trayHidden != nil {
			s.showPlayer()
		}
	})
}

// closePlayerWindow vetoes destruction only when the tray has a visible host.
// Hiding native windows leaves the model's panel preferences and playback
// untouched; those preferences still describe the next restored session.
func (s *Shell) closePlayerWindow() bool {
	if s.tray == nil || !s.tray.Shown() {
		s.quitPlayer()
		return true
	}
	if s.trayHidden == nil {
		s.remember()
		s.trayHidden = make(map[string]bool)
	}
	s.hidePlayerWindows()
	return false
}

func (s *Shell) hidePlayerWindows() {
	for _, named := range s.Windows() {
		name := named.Name
		if named.Window == s.MainWindow() {
			name = "main"
		}
		// A new presentation may add panels while the player is in the
		// tray. It creates them from the model's existing panel preferences.
		if _, known := s.trayHidden[name]; !known {
			s.trayHidden[name] = named.Window.Visible()
		}
		named.Window.Hide()
	}
	if w := s.fileWindow; w != nil && !w.Closed() {
		if _, known := s.trayHidden["file-dialog"]; !known {
			s.trayHidden["file-dialog"] = w.Visible()
		}
		w.Hide()
	}
}

func (s *Shell) showPlayer() {
	if s.closed || s.App.Quitting() || s.presentation == nil {
		return
	}
	window := s.MainWindow()
	if window == nil || window.Closed() {
		return
	}
	if visible := s.trayHidden; visible != nil {
		s.trayHidden = nil
		for _, named := range s.Windows() {
			name := named.Name
			if named.Window == window {
				name = "main"
			}
			switch name {
			case "equaliser":
				s.presentation.ShowEqualiser(visible[name])
			case "playlist":
				s.presentation.ShowPlaylist(visible[name])
			default:
				if visible[name] {
					named.Window.Show()
				}
			}
		}
		if w := s.fileWindow; visible["file-dialog"] && w != nil && !w.Closed() {
			w.Show()
		}
	}
	window.Show()
	window.Activate()
}

func (s *Shell) quitPlayer() {
	if s.closed || s.App.Quitting() {
		return
	}
	s.remember()
	s.pulse.Stop()
	s.App.Quit()
}

func (s *Shell) closeTray() {
	if s.tray == nil {
		return
	}
	s.tray.SetOnShownChange(nil)
	if err := s.tray.Close(); err != nil {
		log.Printf("close system tray: %v", err)
	}
	s.tray = nil
}
