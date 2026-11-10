package winamp

import (
	"reflect"
	"testing"

	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
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
				if p.Desk.Rack.Pane(p.iList).To != -1 {
					t.Fatal("moving the playlist away did not detach it")
				}
				listBefore := p.Desk.Rack.Pane(p.iList).Box
				mainBefore := p.Desk.Rack.Pane(p.iMain).Box
				eqBefore := p.Desk.Rack.Pane(p.iEq).Box
				eqBond := p.Desk.Rack.Pane(p.iEq).Bond
				wantMain := bond.Place(listBefore, mainBefore.W, mainBefore.H)
				near := dockingNear(wantMain, bond.Side)
				dockingMove(t, p, p.iMain, near.X, near.Y)

				if p.Desk.Rack.Pane(p.iMain).To != -1 || p.Desk.Rack.Pane(p.iList).To != p.iMain {
					t.Fatal("snapping Main to the playlist did not make the playlist follow Main")
				}
				if got := p.Desk.Rack.Pane(p.iMain).Box; got != wantMain {
					t.Fatalf("snapped Main to %+v, want %+v", got, wantMain)
				}
				if got := p.Desk.Rack.Pane(p.iList).Box; got != listBefore {
					t.Fatalf("reorienting the group moved the playlist: %+v, want %+v", got, listBefore)
				}
				eq := p.Desk.Rack.Pane(p.iEq)
				if eq.To != p.iMain || eq.Bond != eqBond || eq.Box != eqBefore.Moved(wantMain.X-mainBefore.X, wantMain.Y-mainBefore.Y) {
					t.Fatal("snapping to the playlist disturbed the equaliser's existing bond")
				}
				dockingAssertPositions(t, p)

				before := p.Desk.Rack.Boxes()
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
			list := p.Desk.Rack.Pane(p.iList).Box
			dockingMove(t, p, p.iMain, list.X, list.Bottom()+3)
			dockingMove(t, p, p.iList, 1400, 900)
			if p.Desk.Rack.Pane(p.iList).To != -1 {
				t.Fatal("the reoriented playlist stayed attached after dragging away")
			}
			loose := p.Desk.Rack.Pane(p.iList).Box
			dockingMove(t, p, p.iMain, 300, 200)
			if got := p.Desk.Rack.Pane(p.iList).Box; got != loose {
				t.Fatalf("the detached playlist followed Main to %+v", got)
			}
			main := p.Desk.Rack.Pane(p.iMain).Box
			eq := p.Desk.Rack.Pane(p.iEq).Box
			dockingMove(t, p, p.iList, 1700, 1100)
			if p.Desk.Rack.Pane(p.iMain).Box != main || p.Desk.Rack.Pane(p.iEq).Box != eq {
				t.Fatal("moving the loose playlist moved Main or the equaliser")
			}
			dockingMove(t, p, p.iList, eq.X+4, eq.Bottom()+3)
			if pane := p.Desk.Rack.Pane(p.iList); pane.To != p.iEq || pane.Bond.Side != rack.SideBottom {
				t.Fatal("the playlist did not snap back under the equaliser")
			}
			before := p.Desk.Rack.Boxes()
			dockingMove(t, p, p.iMain, 550, 350)
			dockingAssertTranslated(t, p, before, 550-main.X, 350-main.Y)
		})
	}
}

func TestReorientingAPathPreservesOffsetsAndAHiddenSiblingSubtree(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			p.Advance(0)
			dockingMove(t, p, p.iList, 1000, 600)
			list := p.Desk.Rack.Pane(p.iList).Box
			dockingMove(t, p, p.iEq, list.X-37, list.Bottom()+3)
			if p.Desk.Rack.Pane(p.iEq).To != p.iList {
				t.Fatal("the equaliser did not attach to the loose playlist")
			}

			branch := dockingAddWindow(t, p, "branch", 93, 58)
			p.Desk.Attach(branch, p.iEq, rack.SideLeft)
			hidden := dockingAddWindow(t, p, "hidden child", 67, 41)
			p.Desk.Attach(hidden, branch, rack.SideBottom)
			p.Desk.Show(hidden, false)
			branchBefore, hiddenBefore := *p.Desk.Rack.Pane(branch), *p.Desk.Rack.Pane(hidden)

			// Join through EQ, which already hangs from Playlist. The stock
			// v0.23.3 path reversal loops here; the guarded app pass must join
			// the entire component without moving its stationary panes.
			eq := p.Desk.Rack.Pane(p.iEq).Box
			if !p.Main.Move(eq.Right()+2, eq.Y+23) {
				t.Fatal("the headless desktop rejected a Main move")
			}
			p.followWindows()
			before := p.Desk.Rack.Boxes()
			p.followWindows()
			if got := p.Desk.Rack.Boxes(); !reflect.DeepEqual(got, before) {
				t.Fatalf("reorienting the path changed boxes: got %+v, want %+v", got, before)
			}
			if p.Desk.Rack.Pane(p.iMain).To != -1 || p.Desk.Rack.Pane(p.iEq).To != p.iMain || p.Desk.Rack.Pane(p.iList).To != p.iEq {
				t.Fatal("the full path was not reoriented toward Main")
			}
			if *p.Desk.Rack.Pane(branch) != branchBefore || *p.Desk.Rack.Pane(hidden) != hiddenBefore {
				t.Fatal("reorienting the path changed a sibling or hidden-child bond")
			}
			dockingAssertPositions(t, p)

			main := p.Desk.Rack.Pane(p.iMain).Box
			dockingMove(t, p, p.iMain, 400, 250)
			dockingAssertTranslated(t, p, before, 400-main.X, 250-main.Y)
			if p.Desk.Window(hidden).Visible() || p.Desk.Rack.Pane(hidden).Shown {
				t.Fatal("moving the group showed its hidden child")
			}
		})
	}
}

func dockingMove(t *testing.T, p *Player, i, x, y int) {
	t.Helper()
	if !p.Desk.Window(i).Move(x, y) {
		t.Fatal("the headless desktop rejected a move")
	}
	p.followWindows()
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

func dockingAddWindow(t *testing.T, p *Player, name string, width, height int) int {
	t.Helper()
	w, err := p.App.NewWindow(platform.WindowOptions{
		Title: name, Headless: true, Width: width, Height: height,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	return p.Desk.Add(name, w)
}

func dockingAssertTranslated(t *testing.T, p *Player, before []rack.Box, dx, dy int) {
	t.Helper()
	for i, box := range before {
		if got, want := p.Desk.Rack.Pane(i).Box, box.Moved(dx, dy); got != want {
			t.Errorf("%s moved to %+v, want %+v", p.Desk.Rack.Pane(i).Name, got, want)
		}
	}
	dockingAssertPositions(t, p)
}

func dockingAssertPositions(t *testing.T, p *Player) {
	t.Helper()
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		box := p.Desk.Rack.Pane(i).Box
		w := p.Desk.Window(i)
		if x, y, ok := w.Position(); !ok || x != box.X || y != box.Y {
			t.Errorf("%s window is at %d,%d (known=%t), rack says %+v", p.Desk.Rack.Pane(i).Name, x, y, ok, box)
		}
	}
}
