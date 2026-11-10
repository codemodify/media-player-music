package winamp

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

// This opt-in opens only its own synthetic player. The event-follow checks pump
// native events without Advance, timers or a playback backend.
func TestNativeDesktopMovesFollowAndRejoinWithoutPlaybackTicks(t *testing.T) {
	if os.Getenv("MEDIA_PLAYER_NATIVE_WINDOW_TEST") != "1" {
		t.Skip("set MEDIA_PLAYER_NATIVE_WINDOW_TEST=1 on an X11 desktop")
	}
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv(platform.EnvBackend, "x11")
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, skin)
			a := ui.New(ui.Options{Scale: 1.75, DisableLookWatch: true})
			p, err := New(players.NewHost(a), Options{Scale: 1.75})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			if !strings.HasPrefix(nativeWindowID(p.Main()), "x11:") {
				t.Skip("the X11 backend is unavailable")
			}
			opened := time.Now()
			pumpNativeUtility(a.PumpOnce, p)
			// The group puts its declared bonds back until the windows have
			// stood still a fifth of a second (two seconds at most); a move
			// before then is undone (R21: a press during the settle does not
			// end it).
			playertest.SettleGroup(t, a, p.g)
			t.Logf("the group settled %v after opening", time.Since(opened).Round(10*time.Millisecond))
			p.Transport.Stop()
			p.Transport.Pos = 9 * time.Second
			pumpUntil := func(reason string, ready func() bool) {
				t.Helper()
				deadline := time.Now().Add(2 * time.Second)
				for {
					a.PumpOnce()
					if ready() {
						return
					}
					if time.Now().After(deadline) {
						for i := 0; i < p.g.Rack.Len(); i++ {
							x, y, known := p.g.Window(i).Position()
							t.Logf("native %s position=%d,%d known=%t", p.g.Rack.Pane(i).Name, x, y, known)
						}
						t.Fatalf("%s did not arrive: boxes=%+v", reason, p.g.Rack.Boxes())
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
			at := func(i, x, y int) bool {
				wx, wy, known := p.g.Window(i).Position()
				box := p.g.Rack.Pane(i).Box
				return known && wx == x && wy == y && box.X == x && box.Y == y
			}
			if !p.Main().Move(200, 100) {
				t.Fatal("X11 rejected the synthetic Main move")
			}
			pumpUntil("Main and its followers", func() bool {
				return at(p.iMain, 200, 100) && at(p.iEq, 200, 100+StripH) && at(p.iList, 200, 100+2*StripH)
			})
			p.List().Move(550, 120)
			pumpUntil("playlist detachment", func() bool {
				return at(p.iList, 550, 120) && p.g.Rack.Pane(p.iList).To == -1
			})
			p.Eq().Move(513, 120+ListH+3)
			pumpUntil("detached two-panel component", func() bool {
				return p.g.Rack.Pane(p.iEq).To == p.iList && groupPlaced(p)
			})
			eq, list := *p.g.Rack.Pane(p.iEq), *p.g.Rack.Pane(p.iList)
			main := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(eq.Box, StripW, StripH)
			p.Main().Move(main.X+2, main.Y)
			pumpUntil("Main rejoining a non-root equaliser", func() bool {
				return at(p.iMain, main.X, main.Y) && p.g.Rack.Pane(p.iMain).To == -1 &&
					p.g.Rack.Pane(p.iEq).To == p.iMain && p.g.Rack.Pane(p.iList).To == p.iEq && groupPlaced(p)
			})
			if p.g.Rack.Pane(p.iEq).Box != eq.Box || p.g.Rack.Pane(p.iList).Box != list.Box {
				t.Fatal("rejoining Main moved the stationary component")
			}
			p.Main().Move(420, 350)
			pumpUntil("the rejoined group following Main", func() bool {
				return at(p.iMain, 420, 350) && groupPlaced(p) &&
					p.g.Rack.Pane(p.iEq).Box == eq.Box.Moved(420-main.X, 350-main.Y) &&
					p.g.Rack.Pane(p.iList).Box == list.Box.Moved(420-main.X, 350-main.Y)
			})
			if p.Transport.State != players.Stopped || p.Transport.Pos != 9*time.Second {
				t.Fatal("native event following advanced the synthetic playback state")
			}
			t.Logf("native 1.75x %s event following and non-root rejoin passed", skin)
		})
	}
}

// groupPlaced reports whether every shown panel's window is where the group's
// rack says.
func groupPlaced(p *Player) bool {
	for i := 0; i < p.g.Rack.Len(); i++ {
		pane, w := p.g.Rack.Pane(i), p.g.Window(i)
		if pane == nil || !pane.Shown || w == nil || w.Closed() {
			continue
		}
		if x, y, ok := w.Position(); !ok || x != pane.Box.X || y != pane.Box.Y {
			return false
		}
	}
	return true
}
