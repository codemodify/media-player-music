package panel

import (
	"testing"

	"goframework.io/desktop-ui-rack"
)

func TestDraggingMainToALooseSatelliteKeepsMainAsTheAnchor(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		for _, bond := range []rack.Bond{
			{Side: rack.SideTop, Along: 34},
			{Side: rack.SideBottom, Along: -26},
			{Side: rack.SideLeft, Along: 29},
			{Side: rack.SideRight, Along: -21},
		} {
			t.Run(skin+"/"+bond.Side.String(), func(t *testing.T) {
				_, p := open(t, skin, 1)
				p.Advance(0)
				dockingMove(t, p, p.iList, 900, 700)
				if p.g.Rack.Pane(p.iList).To != -1 {
					t.Fatal("moving the playlist away did not detach it")
				}
				listBefore := p.g.Rack.Pane(p.iList).Box
				mainBefore := p.g.Rack.Pane(p.iMain).Box
				eqBefore := p.g.Rack.Pane(p.iEq).Box
				eqBond := p.g.Rack.Pane(p.iEq).Bond
				wantMain := bond.Place(listBefore, mainBefore.W, mainBefore.H)
				near := dockingNear(wantMain, bond.Side)
				dockingMove(t, p, p.iMain, near.X, near.Y)

				if p.g.Rack.Pane(p.iMain).To != -1 || p.g.Rack.Pane(p.iList).To != p.iMain {
					t.Fatal("snapping Main to the playlist did not make the playlist follow Main")
				}
				if got := p.g.Rack.Pane(p.iMain).Box; got != wantMain {
					t.Fatalf("snapped Main to %+v, want %+v", got, wantMain)
				}
				if got := p.g.Rack.Pane(p.iList).Box; got != listBefore {
					t.Fatalf("reorienting the group moved the playlist: %+v, want %+v", got, listBefore)
				}
				eq := p.g.Rack.Pane(p.iEq)
				if eq.To != p.iMain || eq.Bond != eqBond || eq.Box != eqBefore.Moved(wantMain.X-mainBefore.X, wantMain.Y-mainBefore.Y) {
					t.Fatal("snapping to the playlist disturbed the equaliser's existing bond")
				}
				dockingAssertPositions(t, p)

				before := p.g.Rack.Boxes()
				dockingMove(t, p, p.iMain, 400, 260)
				dockingAssertTranslated(t, p, before, 400-wantMain.X, 260-wantMain.Y)
			})
		}
	}
}

func TestAReorientedSatelliteCanDetachMoveAloneAndSnapBack(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			p.Advance(0)
			dockingMove(t, p, p.iList, 900, 700)
			list := p.g.Rack.Pane(p.iList).Box
			dockingMove(t, p, p.iMain, list.X, list.Bottom()+3)
			dockingMove(t, p, p.iList, 1400, 900)
			if p.g.Rack.Pane(p.iList).To != -1 {
				t.Fatal("the reoriented playlist stayed attached after dragging away")
			}
			loose := p.g.Rack.Pane(p.iList).Box
			dockingMove(t, p, p.iMain, 300, 200)
			if got := p.g.Rack.Pane(p.iList).Box; got != loose {
				t.Fatalf("the detached playlist followed Main to %+v", got)
			}
			main := p.g.Rack.Pane(p.iMain).Box
			eq := p.g.Rack.Pane(p.iEq).Box
			dockingMove(t, p, p.iList, 1700, 1100)
			if p.g.Rack.Pane(p.iMain).Box != main || p.g.Rack.Pane(p.iEq).Box != eq {
				t.Fatal("moving the loose playlist moved Main or the equaliser")
			}
			dockingMove(t, p, p.iList, eq.X+4, eq.Bottom()+3)
			if pane := p.g.Rack.Pane(p.iList); pane.To != p.iEq || pane.Bond.Side != rack.SideBottom {
				t.Fatal("the playlist did not snap back under the equaliser")
			}
			before := p.g.Rack.Boxes()
			dockingMove(t, p, p.iMain, 550, 350)
			dockingAssertTranslated(t, p, before, 550-main.X, 350-main.Y)
		})
	}
}

func dockingMove(t *testing.T, p *Player, i, x, y int) {
	t.Helper()
	w := p.g.Window(i)
	if w == nil {
		t.Fatal("the panel has no window")
	}
	// The user's drag, as the desktop reports it; the group follows.
	desktopMove(p.App, w, x, y)
	dockingAssertPositions(t, p)
}

func dockingNear(box rack.Box, side rack.Side) rack.Box {
	switch side {
	case rack.SideTop:
		box.Y--
	case rack.SideBottom:
		box.Y++
	case rack.SideLeft:
		box.X--
	case rack.SideRight:
		box.X++
	}
	return box
}

func dockingAssertTranslated(t *testing.T, p *Player, before []rack.Box, dx, dy int) {
	t.Helper()
	for i, box := range before {
		if got, want := p.g.Rack.Pane(i).Box, box.Moved(dx, dy); got != want {
			t.Errorf("%s moved to %+v, want %+v", p.g.Rack.Pane(i).Name, got, want)
		}
	}
	dockingAssertPositions(t, p)
}

func dockingAssertPositions(t *testing.T, p *Player) {
	t.Helper()
	for i := 0; i < p.g.Rack.Len(); i++ {
		box := p.g.Rack.Pane(i).Box
		w := p.g.Window(i)
		if !p.g.Rack.Pane(i).Shown {
			continue
		}
		if x, y, ok := w.Position(); !ok || x != box.X || y != box.Y {
			t.Errorf("%s window is at %d,%d (known=%t), rack says %+v", p.g.Rack.Pane(i).Name, x, y, ok, box)
		}
	}
}
