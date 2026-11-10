package fandom

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

func groupFixture(t *testing.T, scale float32) (*ui.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, "dark")
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	p := &Player{App: a, Host: h, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = players.OpenSized(a, platform.WindowOptions{Headless: true, Decorations: platform.DecorationsNone}, MainW, MainH, true)
	if err != nil {
		t.Fatal(err)
	}
	p.List, err = players.OpenSized(a, platform.WindowOptions{Headless: true, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: p.Main.Surface(), MinWidth: MinListW, MinHeight: MinListH}, ListW, ListH, false)
	if err != nil {
		p.Main.Close()
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	p.initGroup()
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	p.AdvanceGeometry(0)
	return a, p
}

func assertGroupPositions(t *testing.T, p *Player) {
	t.Helper()
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane, w := p.Desk.Rack.Pane(i), p.Desk.Window(i)
		if !pane.Shown {
			continue
		}
		x, y, known := w.Position()
		if !known || x != pane.Box.X || y != pane.Box.Y {
			t.Fatalf("%s native position %d,%d known=%t differs from rack %+v", pane.Name, x, y, known, pane.Box)
		}
	}
}

func TestDesktopMoveCarriesOnlyTheSnappedPlaylist(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := groupFixture(t, scale)
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			main, list := p.Desk.Rack.Pane(p.iMain), p.Desk.Rack.Pane(p.iList)
			if list.To != p.iMain || list.Bond != (rack.Bond{Side: rack.SideRight, Along: 46}) || list.Box.X != main.Box.Right() || list.Box.Y != main.Box.Y+46 {
				t.Fatal("reference side-by-side arrangement changed")
			}
			before := p.Desk.Rack.Boxes()
			for i := 0; i < 40; i++ {
				p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(420+i, 250+i)
			}
			a.PumpOnce()
			a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
			for i, old := range before {
				if got := p.Desk.Rack.Pane(i).Box; got != old.Moved(459-before[p.iMain].X, 289-before[p.iMain].Y) {
					t.Fatalf("pane%d did not follow final event: %+v", i, got)
				}
			}
			p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(1500, 900)
			a.PumpOnce()
			a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
			if p.Desk.Rack.Pane(p.iList).To != -1 {
				t.Fatal("dragged playlist stayed attached")
			}
			detached := p.Desk.Rack.Pane(p.iList).Box
			p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(510, 330)
			a.PumpOnce()
			a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
			if p.Desk.Rack.Pane(p.iList).Box != detached {
				t.Fatal("detached playlist followed Main")
			}
			if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
				t.Fatal("window motion changed playback")
			}
			assertGroupPositions(t, p)
		})
	}
}

func TestMainRejoinKeepsAStationaryPlaylistInPlace(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(1400, 800)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	list := p.Desk.Rack.Pane(p.iList).Box
	want := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(list, MainW, MainH)
	p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(want.X+2, want.Y)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	if p.Desk.Rack.Pane(p.iMain).Box != want || p.Desk.Rack.Pane(p.iList).To != p.iMain || p.Desk.Rack.Pane(p.iList).Box != list {
		t.Fatal("Main rejoin shifted stationary playlist geometry")
	}
	assertGroupPositions(t, p)
}

func TestPlacementRestoresResizedHiddenPlaylistWithoutResizingMain(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	p.List.SetSize(330, 390)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	p.AdvanceGeometry(0)
	p.ShowPlaylist(false)
	before := p.Desk.Rack.Boxes()
	saved := p.CapturePlacement()
	main := saved["main"]
	main.Width, main.Height = 900, 700
	saved["main"] = main
	p.Close()
	a, reopened := groupFixture(t, 1.75)
	reopened.ShowPlaylist(false)
	reopened.RestorePlacement(saved)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	reopened.AdvanceGeometry(0)
	if got := reopened.Desk.Rack.Boxes(); !reflect.DeepEqual(got, before) {
		t.Fatalf("restore replaced original group: got%+v want%+v", got, before)
	}
	if reopened.List.Visible() || reopened.Host.Model.PlaylistVisible {
		t.Fatal("restore showed hidden playlist")
	}
	if w, h := reopened.Main.Size(); w != MainW || h != MainH {
		t.Fatal("saved dimensions resized fixed Main")
	}
	reopened.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(490, 295)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	if reopened.Desk.Rack.Pane(reopened.iList).Box != before[p.iList].Moved(70, 45) {
		t.Fatal("hidden playlist lost its restored bond")
	}
	reopened.ShowPlaylist(true)
	assertGroupPositions(t, reopened)
}

func TestNativeDragCancelsPendingStartupPlacement(t *testing.T) {
	a, p := groupFixture(t, 1)
	p.RestorePlacement(map[string]players.WindowPlacement{
		"main":     {Positioned: true, X: 1100, Y: 400},
		"playlist": {Positioned: true, Dock: &players.WindowDock{Parent: "main", Side: "right", Along: 46}},
	})
	p.userMoved("playlist")
	p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(2200, 850)
	a.PumpOnce()
	a.PumpOnce() // Desk follows after the configure batch, on the next UI turn.
	p.AdvanceGeometry(0)
	if p.restoredPlacement != nil || !p.stacked || p.Desk.Rack.Pane(p.iList).To != -1 {
		t.Fatal("native drag retained a pending restore/default arrangement")
	}
	if x, y, known := p.List.Position(); !known || x != 2200 || y != 850 {
		t.Fatal("startup placement fought a user drag")
	}
	assertGroupPositions(t, p)
}
