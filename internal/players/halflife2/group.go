package halflife2

import (
	"time"

	"github.com/codemodify/uitoolkit/rack"
)

// initGroup creates the source skin's arrangement through the public desk.
// The desktop's initial mapping is adopted again by settle before the group
// is treated as ready for user moves.
func (p *Player) initGroup() {
	p.Desk = rack.NewDesk(rack.DefaultReach)
	p.iMain = p.Desk.Add("main", p.Main)
	p.iEq = p.Desk.Add("equaliser", p.Eq)
	p.iList = p.Desk.Add("playlist", p.List)
	p.attachDefaultGroup()
	p.Desk.Apply()
	p.watchWindowMoves()
}

func (p *Player) attachDefaultGroup() {
	p.Desk.Rack.Attach(p.iEq, p.iMain, rack.SideRight)
	p.Desk.Rack.Pane(p.iEq).Bond.Along = -80
	p.Desk.Rack.Attach(p.iList, p.iEq, rack.SideBottom)
	p.placeDockedWindows()
}

// AdvanceGeometry settles startup and maintains resized or unacknowledged
// geometry. Move callbacks follow an established group immediately; the pulse
// also covers backends without move events and the desk's native deadlines.
func (p *Player) AdvanceGeometry(dt time.Duration) {
	if p.Main == nil || p.Main.Closed() || p.Desk == nil {
		return
	}
	p.configureUtilityWindows()
	p.settle()
	if p.restoredPlacement == nil && p.needsFollow(dt == 0) {
		p.followWindows()
	}
}

// settle retries a requested startup arrangement until it is acknowledged,
// then lets subsequent desktop moves change the group normally. Native
// Wayland toplevels cannot be positioned, so they retain independent placement.
func (p *Player) settle() {
	if p.stacked || !p.Desk.Places() || !p.Desk.Adopt() {
		return
	}
	p.tries++
	if p.restoredPlacement != nil {
		p.restorePlaylistSize()
		p.applyPlacement(p.restoredPlacement)
	} else {
		p.attachDefaultGroup()
	}
	p.Desk.Apply()
	if p.placed() || p.tries >= settleTries {
		p.stacked = true
		p.restoredPlacement = nil
	}
}

const settleTries = 20

func (p *Player) placed() bool {
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane, window := p.Desk.Rack.Pane(i), p.Desk.Window(i)
		if pane == nil || window == nil || window.Closed() || !pane.Shown {
			continue
		}
		if x, y, known := window.Position(); !known || x != pane.Box.X || y != pane.Box.Y {
			return false
		}
		if w, h := window.Size(); w != pane.Box.W || h != pane.Box.H {
			return false
		}
	}
	return true
}

func (p *Player) showWindow(i int, on bool) {
	if p.Desk == nil || p.Desk.Window(i) == nil {
		return
	}
	if i == p.iEq {
		p.Host.Model.EqualizerVisible = on
	} else if i == p.iList {
		p.Host.Model.PlaylistVisible = on
	}
	p.Desk.Show(i, on)
	p.configureUtilityWindows()
	p.Refresh()
}
