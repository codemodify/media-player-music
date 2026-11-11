package fancy

import "github.com/codemodify/media-player-music/internal/players"

// CapturePlacement records each panel's box and bond, the hidden ones' too.
func (p *Player) CapturePlacement() map[string]players.WindowPlacement {
	return players.CaptureGroup(p.g)
}

// RestorePlacement keeps the fixed panels' sizes (the player's is its
// mode's), restores the resizable playlist's, and puts back the saved places
// and bonds.
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	players.RestoreGroup(p.g, saved, p.iList, MinListW, MinListH)
}
