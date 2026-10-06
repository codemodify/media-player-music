package shell

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func (s *Shell) detectBackends() []playback.BackendInfo {
	if s.opts.DetectBackends != nil {
		return s.opts.DetectBackends()
	}
	return playback.Detect()
}
func (s *Shell) openBackend(info playback.BackendInfo) (playback.Engine, error) {
	if s.opts.OpenBackend != nil {
		return s.opts.OpenBackend(info)
	}
	return playback.Open(info)
}
func (s *Shell) BackendID() string { return s.backendID }

func (s *Shell) settingsState() players.BackendSettingsState {
	state := players.BackendSettingsState{Selected: s.backendID, Busy: s.backendSwitching, Error: s.backendError}
	if state.Error == "" && !state.Busy {
		state.Error = s.Host.Model.Error
	}
	for _, info := range s.backendInfos {
		caps := info.Capabilities
		if info.ID == s.backendID && s.engine != nil {
			caps = s.engine.Capabilities()
		}
		state.Backends = append(state.Backends, players.BackendInfo{ID: info.ID, Name: info.Name, Executable: info.Executable, Available: info.Available, Equalizer: caps.Equalizer, Balance: caps.Balance, Visualizer: caps.Spectrum})
	}
	return state
}
func (s *Shell) refreshSettings() {
	if s.settingsPanel != nil {
		s.settingsPanel.SetState(s.settingsState())
	}
}

func (s *Shell) updateBackendStatus() {
	s.Host.AudioBackend = "No backend"
	if s.engine != nil {
		for _, info := range s.backendInfos {
			if info.ID == s.backendID {
				s.Host.AudioBackend = info.Name
				break
			}
		}
	}
	m := s.Host.Model
	m.BackendLimited = true
	m.BackendEqualizer, m.BackendBalance, m.BackendSpectrum = false, false, false
	if s.engine != nil {
		caps := s.engine.Capabilities()
		m.BackendEqualizer, m.BackendBalance, m.BackendSpectrum = caps.Equalizer, caps.Balance, caps.Spectrum
	}
	s.refreshSettings()
	s.refresh()
}

func (s *Shell) openSettings() {
	if s.closed {
		return
	}
	s.App.Post(func() {
		if s.closed {
			return
		}
		s.backendInfos = s.detectBackends()
		if s.settingsWindow != nil && !s.settingsWindow.Closed() {
			s.refreshSettings()
			s.settingsWindow.Show()
			s.settingsWindow.Activate()
			return
		}
		w, err := players.OpenSized(s.App, platform.WindowOptions{Title: "Music Player Settings", Role: platform.RoleDialog, Center: true, Owner: s.MainWindow().Surface(), Headless: s.opts.Headless, Decorations: platform.DecorationsServer, MinWidth: 560, MinHeight: 440}, 600, 470, false)
		if err != nil {
			s.backendError = err.Error()
			s.Host.Model.Error = err.Error()
			s.refresh()
			return
		}
		s.settingsWindow = w
		s.settingsPanel = players.NewBackendSettings(s.settingsState(), func(id string) {
			if err := s.SetBackend(id); err != nil {
				s.backendError = err.Error()
				s.refreshSettings()
			}
		}, func() { s.backendInfos = s.detectBackends(); s.refreshSettings() })
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

// SetBackend starts a replacement asynchronously. The existing engine survives
// startup failure. A successful switch joins the old process before restoring
// the latest track, position, pause state, volume and supported DSP.
func (s *Shell) SetBackend(id string) error {
	if s.closed {
		return fmt.Errorf("the player is closed")
	}
	if s.backendSwitching {
		return fmt.Errorf("a playback backend is already starting")
	}
	if id == s.backendID && s.engine != nil && s.backendError == "" {
		return nil
	}
	s.backendInfos = s.detectBackends()
	var chosen playback.BackendInfo
	for _, info := range s.backendInfos {
		if info.ID == id {
			chosen = info
			break
		}
	}
	if chosen.ID == "" {
		return fmt.Errorf("unknown playback backend %q", id)
	}
	if !chosen.Available {
		return fmt.Errorf("%s is not installed; install it and refresh Settings", chosen.Name)
	}
	s.backendSwitching = true
	s.backendError = ""
	s.refreshSettings()
	s.backendWorkers.Add(1)
	go func() {
		defer s.backendWorkers.Done()
		next, err := s.openBackend(chosen)
		if err == nil && next == nil {
			err = fmt.Errorf("%s did not provide a playback engine", chosen.Name)
		}
		type handoff struct {
			accepted bool
			old      playback.Engine
			done     <-chan struct{}
		}
		ack := make(chan handoff, 1)
		s.App.Post(func() {
			if s.closed {
				ack <- handoff{}
				return
			}
			if err != nil {
				s.backendSwitching = false
				s.backendError = err.Error()
				s.refreshSettings()
				ack <- handoff{}
				return
			}
			old, stop, done := s.engine, s.eventStop, s.eventDone
			s.backendTransition = true
			s.engineEpoch++
			s.engine, s.backendID = next, id
			s.eventStop, s.eventDone = nil, nil
			generation, epoch := s.playback.generation+1, s.playback.spectrumEpoch+1
			s.playback = playbackState{generation: generation, spectrumEpoch: epoch}
			s.Host.Model.Error = ""
			s.Host.Model.Spectrum.Reset()
			if stop != nil {
				close(stop)
			}
			s.startPlaybackEvents()
			s.updateBackendStatus()
			ack <- handoff{accepted: true, old: old, done: done}
		})
		var transfer handoff
		select {
		case transfer = <-ack:
		case <-s.lifecycleDone:
			// A committed handoff is already queued before the UI can close.
			select {
			case transfer = <-ack:
			default:
			}
		}
		if !transfer.accepted {
			if next != nil {
				_ = next.Close()
			}
			return
		}
		if transfer.old != nil {
			_ = transfer.old.Close()
		}
		if transfer.done != nil {
			<-transfer.done
		}
		s.App.Post(func() {
			if s.closed || s.engine != next {
				return
			}
			s.backendSwitching, s.backendTransition = false, false
			s.settings.AudioBackend = id
			s.syncPlayback()
			s.refreshSettings()
			s.remember()
		})
	}()
	return nil
}
