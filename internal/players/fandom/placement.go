package fandom

import "github.com/codemodify/media-player-music/internal/players"

// CapturePlacement records each panel's box and bond, the hidden playlist's
// too.
func (p *Player) CapturePlacement() map[string]players.WindowPlacement {
	return players.CaptureGroup(p.g)
}

// RestorePlacement keeps the player's fixed size, restores the resizable
// playlist's, and puts back the saved places and bonds.
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	players.RestoreGroup(p.g, saved, p.iList, MinListW, MinListH)
}
