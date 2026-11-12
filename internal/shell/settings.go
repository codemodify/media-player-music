package shell

import (
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

func (s *Shell) settingsState() players.SettingsState {
	return players.SettingsState{Formats: playback.Formats, ReplayGain: s.settings.ReplayGain, Error: s.Host.Model.Error}
}

func (s *Shell) refreshSettings() {
	if s.settingsPanel != nil {
		s.settingsPanel.SetState(s.settingsState())
	}
}

// SetReplayGain changes the ReplayGain settings, saved, and heard from the
// next track.
func (s *Shell) SetReplayGain(gain players.ReplayGain) {
	gain = gain.Normal()
	if s.closed || gain == s.settings.ReplayGain {
		return
	}
	s.settings.ReplayGain = gain
	s.applyReplayGain()
	s.refreshSettings()
	s.remember()
}

// applyReplayGain gives the engine the saved ReplayGain settings.
func (s *Shell) applyReplayGain() {
	command := playback.Command{Op: playback.OpReplayGain, ReplayGain: playback.ReplayGain{Preamp: s.settings.ReplayGain.Preamp}}
	switch s.settings.ReplayGain.Mode {
	case players.ReplayGainTrack:
		command.ReplayGain.Mode = playback.ReplayGainTrack
	case players.ReplayGainAlbum:
		command.ReplayGain.Mode = playback.ReplayGainAlbum
	}
	s.apply(command)
}

func (s *Shell) openSettings() {
	if s.closed {
		return
	}
	s.App.Post(func() {
		if s.closed {
			return
		}
		if s.settingsWindow != nil && !s.settingsWindow.Closed() {
			s.refreshSettings()
			s.settingsWindow.Show()
			s.settingsWindow.Activate()
			return
		}
		w, err := players.OpenSized(s.App, platform.WindowOptions{Title: "Music Player Settings", Role: platform.RoleDialog, Center: true, Owner: s.MainWindow().Surface(), Headless: s.opts.Headless, Decorations: platform.DecorationsServer, MinWidth: players.SettingsMinWidth, MinHeight: players.SettingsMinHeight}, players.SettingsWidth, players.SettingsHeight, false)
		if err != nil {
			s.Host.Model.Error = err.Error()
			s.refresh()
			return
		}
		s.settingsWindow = w
		s.settingsPanel = players.NewSettings(s.settingsState(), s.SetReplayGain)
		appearance := style.LookAppearance(s.App.Look())
		appearance.Name, appearance.FollowDesktop = players.Themed, false
		s.settingsPanel.SetLook(style.WithAppearance(s.App.Look(), appearance))
		s.settingsPanel.OnClose = func() { w.Close(); s.settingsWindow = nil; s.settingsPanel = nil }
		w.SetContent(s.settingsPanel)
		w.SetOnCloseRequest(func() bool {
			if s.settingsWindow == w {
				s.settingsWindow = nil
				s.settingsPanel = nil
			}
			return true
		})
		widget.FocusFirstIn(s.settingsPanel)
		w.Activate()
	})
}
