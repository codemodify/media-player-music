package series9

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
)

const placementRetryLimit = 12
const placementRestoreTimeout = 2 * time.Second

type placementRestore struct {
	desired      players.WindowPlacement
	elapsed      time.Duration
	attempts     int
	lastX, lastY int
	lastKnown    bool
}

func (p *Player) CapturePlacement() map[string]players.WindowPlacement {
	if p.Main == nil || p.Main.Closed() {
		return nil
	}
	x, y, known := p.Main.Position()
	width, height := p.Main.Size()
	if p.modeResize != nil {
		width, height = p.modeResize.size.Width, p.modeResize.size.Height
	}
	if p.restore != nil {
		if p.restore.desired.Positioned {
			x, y, known = p.restore.desired.X, p.restore.desired.Y, true
		}
		width, height = p.restore.desired.Width, p.restore.desired.Height
	}
	return map[string]players.WindowPlacement{"main": {X: x, Y: y, Width: width, Height: height, Positioned: known}}
}
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	p.restore = nil
	pos, ok := saved["main"]
	if !ok || p.Main == nil || p.Main.Closed() {
		return
	}
	size := players.PresentationSize{Width: CompactW, Height: CompactH}
	if !p.Compact() {
		size = p.actualFullSize(p.fullSize)
		if pos.Width > 0 {
			size.Width = pos.Width
		}
		if pos.Height > 0 {
			size.Height = pos.Height
		}
		p.fullSize = p.baseFromActual(size)
		size = p.actualFullSize(p.fullSize)
	}
	pos.Width, pos.Height = size.Width, size.Height
	pos.Dock = nil
	pos.Positioned = pos.Positioned && p.Main.CanMove()
	p.modeResize = nil
	p.Main.SetSize(size.Width, size.Height)
	p.restore = &placementRestore{desired: pos}
	p.retryPlacement(0)
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
	width, height := p.Main.Size()
	observed := width == pending.desired.Width && height == pending.desired.Height
	if !observed {
		p.Main.SetSize(pending.desired.Width, pending.desired.Height)
	}
	if !pending.desired.Positioned || !p.Main.CanMove() {
		if observed {
			p.restore = nil
		}
		return
	}
	x, y, known := p.Main.Position()
	if pending.attempts > 0 && known {
		if x == pending.desired.X && y == pending.desired.Y {
			if observed {
				p.restore = nil
			}
			return
		}
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
