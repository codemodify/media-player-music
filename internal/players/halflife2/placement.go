package halflife2

import (
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/rack"
)

// CapturePlacement records the desk's logical positions and bonds, including
// hidden panels. Let the toolkit observe a final drag before taking the snapshot.
func (p *Player) CapturePlacement() map[string]players.WindowPlacement {
	if p.restoredPlacement == nil {
		p.followWindows()
	}
	out := make(map[string]players.WindowPlacement)
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane, window := p.Desk.Rack.Pane(i), p.Desk.Window(i)
		if pane == nil || window == nil || window.Closed() {
			continue
		}
		_, _, positioned := window.Position()
		placement := players.WindowPlacement{
			X: pane.Box.X, Y: pane.Box.Y, Width: pane.Box.W, Height: pane.Box.H,
			Positioned: positioned,
		}
		if parent := p.Desk.Rack.Pane(pane.To); parent != nil && pane.Bond.Stuck() {
			placement.Dock = &players.WindowDock{
				Parent: parent.Name, Side: pane.Bond.Side.String(), Along: pane.Bond.Along,
			}
		}
		out[pane.Name] = placement
	}
	return out
}

// RestorePlacement keeps the fixed skin sizes, restores the resizable playlist,
// and reinstates the last positions
// and docking relationships. Initial desktop mapping can happen after this
// call, so settle retries the requested arrangement rather than the default stack.
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	owned := make(map[string]players.WindowPlacement)
	for name, placement := range saved {
		if pane, _ := p.Desk.Rack.Find(name); pane == nil {
			continue
		}
		if placement.Dock != nil {
			dock := *placement.Dock
			placement.Dock = &dock
		}
		owned[name] = placement
	}
	if len(owned) == 0 {
		return
	}
	p.restoredPlacement, p.tries = owned, 0
	if placement, ok := owned["playlist"]; ok && p.List != nil {
		w, h := p.List.Size()
		if placement.Width > 0 {
			w = max(placement.Width, MinListW)
		}
		if placement.Height > 0 {
			h = max(placement.Height, MinListH)
		}
		p.List.SetSize(w, h)
		p.Desk.Rack.Resize(p.iList, w, h)
		placement.Width, placement.Height = w, h
		owned["playlist"] = placement
	}
	p.applyPlacement(owned)
	p.Desk.Apply()
	// The first native layout can still install a frame and map the window.
	// Its configure may supersede geometry requested here, so settle owns
	// acknowledgement after that layout rather than trusting this buffer.
	p.stacked = false
}

func (p *Player) restorePlaylistSize() {
	placement, ok := p.restoredPlacement["playlist"]
	if !ok || p.List == nil || p.List.Closed() {
		return
	}
	w, h := p.List.Size()
	if w == placement.Width && h == placement.Height {
		return
	}
	p.List.SetSize(placement.Width, placement.Height)
}

func (p *Player) applyPlacement(saved map[string]players.WindowPlacement) {
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane := p.Desk.Rack.Pane(i)
		placement, ok := saved[pane.Name]
		if !ok {
			continue
		}
		if i == p.iList {
			if placement.Width > 0 {
				pane.Box.W = max(placement.Width, MinListW)
			}
			if placement.Height > 0 {
				pane.Box.H = max(placement.Height, MinListH)
			}
		}
		if placement.Positioned {
			pane.Box = pane.Box.At(placement.X, placement.Y)
		}
		p.Desk.Rack.Detach(i)
		if placement.Dock == nil || i == p.iMain {
			continue
		}
		_, parent := p.Desk.Rack.Find(placement.Dock.Parent)
		side := savedDockSide(placement.Dock.Side)
		if parent < 0 || parent == i || side == rack.SideNone {
			continue
		}
		pane.To = parent
		pane.Bond = rack.Bond{Side: side, Along: placement.Dock.Along}
	}
	// Settings can be edited outside the app. Break invalid cycles before
	// asking the toolkit to place any children; Main always remains an anchor.
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		seen := make(map[int]bool)
		for at := i; at >= 0; {
			if seen[at] {
				p.Desk.Rack.Detach(i)
				break
			}
			seen[at] = true
			pane := p.Desk.Rack.Pane(at)
			if pane == nil {
				p.Desk.Rack.Detach(i)
				break
			}
			at = pane.To
		}
	}
	p.placeDockedWindows()
}

// Bond.Place is the toolkit's docking geometry. Compute parents first so
// saved offsets remain correct when only an anchor had a known position.
func (p *Player) placeDockedWindows() {
	placed := make(map[int]bool)
	var place func(int)
	place = func(i int) {
		if placed[i] {
			return
		}
		placed[i] = true
		pane := p.Desk.Rack.Pane(i)
		if pane.To >= 0 {
			place(pane.To)
			pane.Box = pane.Bond.Place(p.Desk.Rack.Pane(pane.To).Box, pane.Box.W, pane.Box.H)
		}
	}
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		place(i)
	}
}

func savedDockSide(side string) rack.Side {
	switch side {
	case "top":
		return rack.SideTop
	case "bottom":
		return rack.SideBottom
	case "left":
		return rack.SideLeft
	case "right":
		return rack.SideRight
	}
	return rack.SideNone
}
