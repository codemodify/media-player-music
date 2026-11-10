package fandom

import (
	"time"

	"goframework.io/desktop-ui-rack"
)

func (p *Player) initGroup() {
	p.Desk = rack.NewDesk(rack.DefaultReach)
	p.iMain = p.Desk.Add("main", p.Main)
	p.iList = p.Desk.Add("playlist", p.List)
	p.attachDefaultGroup()
	p.Desk.Apply()
}
func (p *Player) attachDefaultGroup() {
	p.Desk.Rack.Attach(p.iList, p.iMain, rack.SideRight)
	p.Desk.Rack.Pane(p.iList).Bond.Along = 46
	p.placeDockedWindows()
}
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
		pane, w := p.Desk.Rack.Pane(i), p.Desk.Window(i)
		if pane == nil || w == nil || w.Closed() || !pane.Shown {
			continue
		}
		if x, y, known := w.Position(); !known || x != pane.Box.X || y != pane.Box.Y {
			return false
		}
		if width, height := w.Size(); width != pane.Box.W || height != pane.Box.H {
			return false
		}
	}
	return true
}
func (p *Player) showWindow(i int, on bool) {
	if p.Desk == nil || p.Desk.Window(i) == nil {
		return
	}
	if i == p.iList {
		p.Host.Model.PlaylistVisible = on
	}
	p.Desk.Show(i, on)
	p.configureUtilityWindows()
	p.Refresh()
}
