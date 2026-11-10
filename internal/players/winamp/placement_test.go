package winamp

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
)

func TestSatelliteWindowsRequestAnAuxiliaryRoleWithoutMinimize(t *testing.T) {
	for _, skin := range Skins {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1)
			for _, look := range []string{skin, otherPanelSkin(skin)} {
				p.Host.ApplySkin(look)
				a.PumpOnce()
				for name, window := range map[string]*ui.Window{"main": p.Main(), "equaliser": p.Eq(), "playlist": p.List()} {
					wantRole := platform.RoleUtility
					if name == "main" {
						wantRole = platform.RoleNormal
					}
					surface := window.Surface().(*platform.Offscreen)
					if got := surface.WindowRole(); got != wantRole {
						t.Errorf("%s requested role %v, want %v", name, got, wantRole)
					}
					if name != "main" && (surface.Owner() != p.Main().Surface() || !surface.SkipsTaskbar()) {
						t.Errorf("%s did not declare Main ownership and taskbar policy", name)
					}
					left, right := window.Caption().Controls()
					shown := append(left.Shown(), right.Shown()...)
					hasMinimize, hasClose := false, false
					for _, button := range shown {
						hasMinimize = hasMinimize || button == platform.CaptionMinimize
						hasClose = hasClose || button == platform.CaptionClose
					}
					if hasMinimize != (name == "main") || !hasClose {
						t.Errorf("%s/%s caption buttons %v", look, name, shown)
					}
				}
			}
		})
	}
}

func TestPlacementSurvivesSkinChangeWithHiddenDockAndDetachedPlaylist(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		for _, bond := range []rack.Bond{
			{Side: rack.SideTop, Along: 37},
			{Side: rack.SideBottom, Along: -29},
			{Side: rack.SideLeft, Along: 31},
			{Side: rack.SideRight, Along: -23},
		} {
			t.Run(skin+"/"+bond.Side.String(), func(t *testing.T) {
				a, p := open(t, skin, 1.75)
				p.Advance(0)
				dockingMove(t, p, p.iMain, 410, 270)
				dockingMove(t, p, p.iList, 1500, 900)
				main := p.g.Rack.Pane(p.iMain).Box
				eq := p.g.Rack.Pane(p.iEq).Box
				near := dockingNear(bond.Place(main, eq.W, eq.H), bond.Side)
				dockingMove(t, p, p.iEq, near.X, near.Y)
				p.ShowEqualiser(false)
				before := p.g.Rack.Boxes()
				saved := placementJSONRoundTrip(t, p.CapturePlacement())
				if dock := saved["equaliser"].Dock; dock == nil || dock.Parent != "main" ||
					dock.Side != bond.Side.String() || dock.Along != bond.Along {
					t.Fatalf("did not save the equaliser's exact bond: %+v", dock)
				}
				if saved["playlist"].Dock != nil || !saved["playlist"].Positioned {
					t.Fatal("the detached playlist was saved as docked or unpositioned")
				}
				p.Close()
				p.Host.ApplySkin(otherPanelSkin(skin))
				reopened, err := New(p.Host, Options{Headless: true, Scale: 1.75})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(reopened.Close)
				reopened.RestorePlacement(saved)
				playertest.SettleGroup(t, a, reopened.g)
				if got := reopened.g.Rack.Boxes(); !reflect.DeepEqual(got, before) {
					t.Fatalf("reopening replaced saved positions with a default stack: got %+v, want %+v", got, before)
				}
				if reopened.Eq().Visible() || reopened.g.Rack.Pane(reopened.iEq).Shown {
					t.Fatal("restoring placement showed the hidden equaliser")
				}
				dockingMove(t, reopened, reopened.iMain, main.X+70, main.Y+45)
				if got := reopened.g.Rack.Pane(reopened.iEq).Box; got != before[p.iEq].Moved(70, 45) {
					t.Fatal("the hidden equaliser did not keep following its restored anchor")
				}
				if got := reopened.g.Rack.Pane(reopened.iList).Box; got != before[p.iList] {
					t.Fatal("the restored detached playlist followed Main")
				}
				reopened.ShowEqualiser(true)
				dockingAssertPositions(t, reopened)
			})
		}
	}
}

func TestPlacementRestoresDetachedPanelSubtreeAndKnownOrigin(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			p.RestorePlacement(map[string]players.WindowPlacement{
				"main":      {Positioned: true, X: 0, Y: 0, Width: 900, Height: 700},
				"playlist":  {Positioned: true, X: -600, Y: 800},
				"equaliser": {Positioned: true, Dock: &players.WindowDock{Parent: "playlist", Side: "right", Along: 28}},
			})
			playertest.Follow(p.App)
			if x, y, ok := p.Main().Position(); !ok || x != 0 || y != 0 {
				t.Fatalf("the known origin was discarded: %d,%d, known=%t", x, y, ok)
			}
			if w, h := p.Main().Size(); w != StripW || h != StripH {
				t.Fatalf("saved dimensions resized a fixed skin: %dx%d", w, h)
			}
			if p.g.Rack.Pane(p.iList).To != -1 || p.g.Rack.Pane(p.iEq).To != p.iList {
				t.Fatal("the detached playlist's equaliser subtree was replaced by the default stack")
			}
			before := p.g.Rack.Boxes()
			dockingMove(t, p, p.iList, -530, 840)
			if p.g.Rack.Pane(p.iMain).Box != before[p.iMain] ||
				p.g.Rack.Pane(p.iEq).Box != before[p.iEq].Moved(70, 40) {
				t.Fatal("moving a restored detached anchor did not carry only its own subtree")
			}
		})
	}
}

func TestMalformedPlacementCannotCreateDockingCycles(t *testing.T) {
	_, p := open(t, Skin, 1)
	p.RestorePlacement(map[string]players.WindowPlacement{
		"main":      {Positioned: true, X: 220, Y: 130, Dock: &players.WindowDock{Parent: "playlist", Side: "top"}},
		"equaliser": {Positioned: true, X: 900, Y: 600, Dock: &players.WindowDock{Parent: "playlist", Side: "left"}},
		"playlist":  {Positioned: true, X: 1200, Y: 800, Dock: &players.WindowDock{Parent: "equaliser", Side: "right"}},
		"unknown":   {Positioned: true, X: 4000, Y: 3000},
	})
	for i := 0; i < p.g.Rack.Len(); i++ {
		seen := make(map[int]bool)
		for at := i; at >= 0; at = p.g.Rack.Pane(at).To {
			if seen[at] {
				t.Fatal("restored docking contains a cycle")
			}
			seen[at] = true
		}
	}
	if p.g.Rack.Pane(p.iMain).To != -1 {
		t.Fatal("malformed settings made Main follow a satellite")
	}
	playertest.Follow(p.App)
	dockingAssertPositions(t, p)
}

func placementJSONRoundTrip(t *testing.T, placement map[string]players.WindowPlacement) map[string]players.WindowPlacement {
	t.Helper()
	data, err := json.Marshal(placement)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]players.WindowPlacement
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func otherPanelSkin(skin string) string {
	if skin == Skin {
		return SkinSilver
	}
	return Skin
}
