package winamp

import "github.com/codemodify/uitoolkit/rack"

// followWindows delegates snapping and placement to the toolkit. v0.23.4's
// Main rejoin drops links on longer foreign paths and moves stationary panes
// when snapping across a small gap. Choose
// the anchor in a temporary public Rack with free foreign panes, then
// hide those anchors from the live pass until their component path is restored.
// Remove this guard once Rack.hangFrom preserves paths and stationary geometry.
func (p *Player) followWindows() {
	if p.followingWindows {
		p.followWindowsAgain = true
		return
	}
	p.followingWindows = true
	defer func() {
		p.followingWindows = false
		// Maintain unresolved geometry on the application pulse while the
		// Desk waits for its clock-based native acknowledgement deadline.
		p.pendingWindowGeometry = !p.placed()
	}()
	for {
		p.followWindowsAgain = false
		p.followWindowGeometry()
		if !p.followWindowsAgain {
			return
		}
	}
}

func (p *Player) followWindowGeometry() {
	rejoin := p.prepareMainRejoin()
	if rejoin != nil {
		defer p.restoreRejoinVisibility(rejoin)
	}
	p.Desk.Follow()
	if rejoin == nil {
		return
	}
	p.restoreRejoinVisibility(rejoin)
	main := p.Desk.Rack.Pane(p.iMain)
	// Follow can still be waiting for an earlier native move. Preserve that
	// acknowledgement policy rather than accepting a position it rejected.
	if main.Box.X != rejoin.x || main.Box.Y != rejoin.y {
		return
	}
	// Another desktop move or resize in the same burst can invalidate the
	// preview. Preserve that newer pose rather than replaying its old path.
	for i, before := range rejoin.foreign {
		pane := p.Desk.Rack.Pane(i)
		if pane.Box != before.Box || pane.To != before.To || pane.Bond != before.Bond {
			return
		}
	}
	joined, foreign := rejoin.joined, rejoin.foreign
	if joined >= 0 {
		anchor := foreign[joined]
		// Keep the stationary component in place while Main snaps to it,
		// using the same public geometry used for satellite snapping.
		if bond, ok := rack.Snap(anchor.Box, main.Box, p.Desk.Rack.Reach); ok {
			main.Box = bond.Place(anchor.Box, main.Box.W, main.Box.H)
			p.Desk.Rack.Pane(joined).To = p.iMain
			p.Desk.Rack.Pane(joined).Bond = reverseDockBond(bond)
			for from, to := joined, anchor.To; to >= 0; {
				before, ok := foreign[to]
				if !ok {
					break
				}
				parent := p.Desk.Rack.Pane(to)
				parent.To, parent.Bond = from, reverseDockBond(foreign[from].Bond)
				from, to = to, before.To
			}
		}
	}
	p.placeDockedWindows()
	p.Desk.Apply()
}

type mainRejoin struct {
	foreign map[int]rack.Pane
	joined  int
	x, y    int
}

// prepareMainRejoin uses stock Rack.MoveTo to choose the nearest joining pane.
// The temporary rack has no desktop: its tentative snap cannot submit a move
// which races the final geometry while native acknowledgements are pending.
func (p *Player) prepareMainRejoin() *mainRejoin {
	main := p.Desk.Rack.Pane(p.iMain)
	x, y, known := p.Main.Position()
	if main == nil || !known || x == main.Box.X && y == main.Box.Y {
		return nil
	}
	saved := make(map[int]rack.Pane)
	trial := rack.New(p.Desk.Rack.Reach)
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane := p.Desk.Rack.Pane(i)
		trial.Add(pane.Name, pane.Box)
		trial.Pane(i).To, trial.Pane(i).Bond, trial.Pane(i).Shown = pane.To, pane.Bond, pane.Shown
	}
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		underMain := false
		seen := make(map[int]bool)
		for at := i; at >= 0 && !seen[at]; {
			if at == p.iMain {
				underMain = true
				break
			}
			seen[at] = true
			pane := p.Desk.Rack.Pane(at)
			if pane == nil {
				break
			}
			at = pane.To
		}
		if !underMain {
			saved[i] = *p.Desk.Rack.Pane(i)
			trial.Detach(i)
		}
	}
	trial.MoveTo(p.iMain, x, y)
	joined := -1
	for i := range saved {
		if trial.Pane(i).To == p.iMain {
			joined = i
		}
	}
	if joined < 0 {
		return nil
	}
	for i := range saved {
		p.Desk.Rack.Show(i, false)
	}
	return &mainRejoin{foreign: saved, joined: joined, x: x, y: y}
}

func (p *Player) restoreRejoinVisibility(rejoin *mainRejoin) {
	for i, before := range rejoin.foreign {
		p.Desk.Rack.Show(i, before.Shown)
	}
}

// Toolkit desks install an unguarded OnMove handler in Add. Bind this small
// controller instead until the Main rejoin regressions above are fixed.
func (p *Player) watchWindowMoves() {
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		if w := p.Desk.Window(i); w != nil && w.SendsMoveEvents() {
			w.OnMove(func(int, int) {
				if p.stacked && p.restoredPlacement == nil {
					p.followWindows()
				}
			})
		}
	}
}

func (p *Player) needsFollow(dtZero bool) bool {
	if p.pendingWindowGeometry {
		return true
	}
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		w, pane := p.Desk.Window(i), p.Desk.Rack.Pane(i)
		if w == nil || w.Closed() || pane == nil {
			continue
		}
		width, height := w.Size()
		if dtZero || !w.SendsMoveEvents() || width != pane.Box.W || height != pane.Box.H {
			return true
		}
	}
	return false
}

func reverseDockBond(bond rack.Bond) rack.Bond {
	switch bond.Side {
	case rack.SideTop:
		bond.Side = rack.SideBottom
	case rack.SideBottom:
		bond.Side = rack.SideTop
	case rack.SideLeft:
		bond.Side = rack.SideRight
	case rack.SideRight:
		bond.Side = rack.SideLeft
	}
	bond.Along = -bond.Along
	return bond
}
