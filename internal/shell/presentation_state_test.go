package shell

import (
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
)

type unpositionedModePresentation struct {
	Presentation
	state players.PresentationState
}

func (p *unpositionedModePresentation) CapturePlacement() map[string]players.WindowPlacement {
	return nil
}
func (p *unpositionedModePresentation) CaptureState() players.PresentationState { return p.state }
func (p *unpositionedModePresentation) RestoreState(state players.PresentationState) {
	p.state = state
}

func TestPresentationStateSurvivesAnUnobservableDesktopWithoutAliasingItsModeCache(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := &unpositionedModePresentation{state: players.PresentationState{
		Mode: "mini", Sizes: map[string]players.PresentationSize{
			"full": {Width: 900, Height: 600}, "mini": {Width: 380, Height: 68},
		},
	}}
	s := &Shell{}
	s.capturePlacement(p, players.AluminiumSkin)
	if len(s.settings.Windows) != 0 || s.settings.Presentations[players.AluminiumSkin].Mode != "mini" {
		t.Fatal("a desktop without placements lost its independent view mode")
	}
	p.state.Sizes["full"] = players.PresentationSize{Width: 1200, Height: 800}
	if s.settings.Presentations[players.AluminiumSkin].Sizes["full"].Width != 900 {
		t.Fatal("a later resize mutated the captured session before it was saved")
	}
	if err := s.settings.Save(); err != nil {
		t.Fatal(err)
	}
	if got := players.LoadSettings().Presentations[players.AluminiumSkin]; got.Mode != "mini" || got.Sizes["full"].Width != 900 || got.Sizes["mini"].Height != 68 {
		t.Fatalf("mode state did not survive serialization: %+v", got)
	}
}
