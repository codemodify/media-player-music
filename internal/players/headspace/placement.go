package headspace

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
)

// Retry only during startup. A desktop may refuse a request, and a user's
// subsequent move must remain their new placement rather than a restore target.
const placementRetryLimit = 12
const placementRestoreTimeout = 2 * time.Second

type placementRestore struct {
	desired      players.WindowPlacement
	elapsed      time.Duration
	attempts     int
	lastX, lastY int
	lastKnown    bool
}

// CapturePlacement keeps Headspace independent of the other appearances.
// The toolkit reports both size and position in logical desktop pixels.
func (p *Player) CapturePlacement() map[string]players.WindowPlacement {
	if p.Main == nil || p.Main.Closed() {
		return nil
	}
	x, y, known := p.Main.Position()
	w, h := p.Main.Size()
	if pending := p.restore; pending != nil {
		if pending.desired.Positioned {
			x, y, known = pending.desired.X, pending.desired.Y, true
		}
		// Keep the skin's fixed design size during initial native mapping.
		if !p.placementReady || w != pending.desired.Width || h != pending.desired.Height {
			w, h = pending.desired.Width, pending.desired.Height
		}
	}
	return map[string]players.WindowPlacement{
		"main": {X: x, Y: y, Width: w, Height: h, Positioned: known},
	}
}

// RestorePlacement restores position while retaining the artwork's fixed size.
// Unknown positions never become a request to move to the origin.
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	p.restore = nil
	placement, ok := saved["main"]
	if !ok || p.Main == nil || p.Main.Closed() {
		return
	}
	placement.Width, placement.Height = MainW, MainH
	placement.Positioned = placement.Positioned && p.Main.CanMove()
	p.restore = &placementRestore{desired: placement}
	if p.placementReady {
		p.retryPlacement(0)
	}
}

func (p *Player) retryPlacement(dt time.Duration) {
	pending := p.restore
	if pending == nil || p.Main == nil || p.Main.Closed() {
		return
	}
	pending.elapsed += dt
	if pending.elapsed >= placementRestoreTimeout || pending.attempts >= placementRetryLimit {
		p.restore = nil
		return
	}
	if !p.placementReady {
		return
	}
	w, h := p.Main.Size()
	sizeObserved := w == pending.desired.Width && h == pending.desired.Height
	if !pending.desired.Positioned || !p.Main.CanMove() {
		if sizeObserved {
			p.restore = nil
		}
		return
	}
	x, y, known := p.Main.Position()
	if pending.attempts > 0 && known {
		if x == pending.desired.X && y == pending.desired.Y {
			if sizeObserved {
				p.restore = nil
			}
			return
		}
		// Once a request is pending, an unexpected new position is a
		// desktop adjustment or user move. Adopt it instead of fighting it.
		if pending.lastKnown && (x != pending.lastX || y != pending.lastY) {
			p.restore = nil
			return
		}
	}
	pending.lastX, pending.lastY, pending.lastKnown = x, y, known
	if !p.Main.Move(pending.desired.X, pending.desired.Y) {
		p.restore = nil
		return
	}
	pending.attempts++
}
