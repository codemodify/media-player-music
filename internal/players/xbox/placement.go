package xbox

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
	out := make(map[string]players.WindowPlacement)
	for _, named := range p.Windows() {
		w := named.Window
		if w == nil || w.Closed() {
			continue
		}
		x, y, known := w.Position()
		width, height := w.Size()
		if pending := p.restore[named.Name]; pending != nil {
			if pending.desired.Positioned {
				x, y, known = pending.desired.X, pending.desired.Y, true
			}
			width, height = pending.desired.Width, pending.desired.Height
		}
		out[named.Name] = players.WindowPlacement{X: x, Y: y, Width: width, Height: height, Positioned: known}
	}
	return out
}
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	p.restore = make(map[string]*placementRestore)
	for _, named := range p.Windows() {
		pos, ok := saved[named.Name]
		w := named.Window
		if !ok || w == nil || w.Closed() {
			continue
		}
		if named.Name == "main" {
			pos.Width, pos.Height = MainW, MainH
		} else {
			width, height := w.Size()
			if pos.Width > 0 {
				width = max(pos.Width, MinListW)
			}
			if pos.Height > 0 {
				height = max(pos.Height, MinListH)
			}
			pos.Width, pos.Height = width, height
			w.SetSize(width, height)
		}
		pos.Dock = nil
		pos.Positioned = pos.Positioned && w.CanMove()
		p.restore[named.Name] = &placementRestore{desired: pos}
	}
	p.retryPlacement(0)
}
func (p *Player) retryPlacement(dt time.Duration) {
	for _, named := range p.Windows() {
		pending := p.restore[named.Name]
		w := named.Window
		if pending == nil || w == nil || w.Closed() {
			continue
		}
		pending.elapsed += dt
		if pending.elapsed >= placementRestoreTimeout || pending.attempts >= placementRetryLimit {
			delete(p.restore, named.Name)
			continue
		}
		if !p.placementReady[named.Name] {
			continue
		}
		width, height := w.Size()
		observed := width == pending.desired.Width && height == pending.desired.Height
		if !pending.desired.Positioned || !w.CanMove() {
			if observed {
				delete(p.restore, named.Name)
			}
			continue
		}
		x, y, known := w.Position()
		if pending.attempts > 0 && known {
			if x == pending.desired.X && y == pending.desired.Y {
				if observed {
					delete(p.restore, named.Name)
				}
				continue
			}
			if pending.lastKnown && (x != pending.lastX || y != pending.lastY) {
				delete(p.restore, named.Name)
				continue
			}
		}
		pending.lastX, pending.lastY, pending.lastKnown = x, y, known
		if !w.Move(pending.desired.X, pending.desired.Y) {
			delete(p.restore, named.Name)
			continue
		}
		pending.attempts++
	}
}
