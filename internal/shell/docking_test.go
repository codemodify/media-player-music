package shell

import (
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
)

// Pump the actual UI timers and desktop events rather than calling Follow.
func awaitDocking(t *testing.T, a *ui.Application, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		a.PumpOnce()
		if ready() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("window maintenance did not update the snapped group")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func awaitPositions(t *testing.T, a *ui.Application, windows []*ui.Window, positions [][2]int) {
	t.Helper()
	awaitDocking(t, a, func() bool {
		for i, w := range windows {
			x, y, ok := w.Position()
			if !ok || [2]int{x, y} != positions[i] {
				return false
			}
		}
		return true
	})
}

// awaitSettled pumps a's loop until s's face has put its windows in place.
// A racked face (winamp, fancy, fandom, halflife2) keeps them on a
// rack.Group, which, while it settles, puts back a window the desktop moves
// where its bond or saved place says (the window manager placing it, not the
// user). The other faces have nothing to settle.
func awaitSettled(t *testing.T, a *ui.Application, s *Shell) {
	t.Helper()
	racked, ok := s.presentation.(interface{ Settled() bool })
	if !ok {
		return
	}
	for deadline := time.Now().Add(10 * time.Second); !racked.Settled(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the face's windows did not settle")
		}
		a.PumpOnce()
	}
	a.PumpOnce()
}

// desktopMove is the user moving w to x, y, as the desktop reports it, once
// s's face has settled; then the two turns of a's loop a racked face's group
// takes to follow it (playertest.Follow). A program's w.Move is the
// application's own request and the desktop reports no move for it, so a
// rack.Group does not follow it and its saved placement keeps the panes where
// they were.
func desktopMove(t *testing.T, a *ui.Application, s *Shell, w *ui.Window, x, y int) {
	t.Helper()
	awaitSettled(t, a, s)
	w.Surface().(*platform.Offscreen).SimulateDesktopMove(x, y)
	playertest.Follow(a)
}

func TestSnappedWindowsFollowWhilePlaybackIsPausedOrStopped(t *testing.T) {
	for _, skin := range []string{winamp.Skin, winamp.SkinSilver} {
		for _, state := range []players.State{players.Stopped, players.Paused} {
			t.Run(skin+"/"+state.String(), func(t *testing.T) {
				a, s := open(t, players.DefaultSkin)
				transport := s.Host.Model.Transport
				transport.State, transport.Pos = state, 17*time.Second
				if err := s.SetSkin(skin); err != nil {
					t.Fatal(err)
				}
				if !s.Pulse().Running() {
					t.Fatal("entering a skin without playback did not arm window maintenance")
				}
				windows := s.Windows()
				main, eq, list := windows[0].Window, windows[1].Window, windows[2].Window
				if !main.CanMove() {
					t.Fatal("headless fixture must support window placement")
				}
				// Allow the initial mapping/stacking maintenance to settle:
				// until the group has, a desktop move is put back.
				awaitSettled(t, a, s)
				_, mainH := main.Size()
				_, eqH := eq.Size()
				x, y, _ := main.Position()
				all := []*ui.Window{main, eq, list}
				awaitPositions(t, a, all, [][2]int{{x, y}, {x, y + mainH}, {x, y + mainH + eqH}})
				// Wait for its scheduled first tick before moving, because
				// startup adoption intentionally takes the desktop's boxes.
				ticks := 0
				s.Pulse().SetClock(func() time.Time { ticks++; return time.Now() })
				awaitDocking(t, a, func() bool { return ticks > 0 })
				// Desktop movement now has a toolkit event. Offscreen.Move is
				// an application request and intentionally emits no such event.
				// Stop the playback timer to prove dragging uses notifications.
				s.Pulse().Stop()
				move := func(w *ui.Window, x, y int) {
					w.Surface().(*platform.Offscreen).SimulateDesktopMove(x, y)
				}
				move(main, 400, 260)
				awaitPositions(t, a, all, [][2]int{{400, 260}, {400, 260 + mainH}, {400, 260 + mainH + eqH}})

				// Pull the playlist away, move the main group, and verify
				// the detached playlist stays put.
				move(list, 900, 700)
				// The group follows a batch of desktop moves at once: the
				// playlist must have come loose before the player moves,
				// or it goes along.
				awaitPositions(t, a, all, [][2]int{{400, 260}, {400, 260 + mainH}, {900, 700}})
				move(main, 430, 290)
				awaitPositions(t, a, all, [][2]int{{430, 290}, {430, 290 + mainH}, {900, 700}})
				move(list, 434, 290+mainH+eqH+3)
				awaitPositions(t, a, all, [][2]int{{430, 290}, {430, 290 + mainH}, {430, 290 + mainH + eqH}})
				move(main, 500, 300)
				awaitPositions(t, a, all, [][2]int{{500, 300}, {500, 300 + mainH}, {500, 300 + mainH + eqH}})
				if transport.State != state || transport.Pos != 17*time.Second {
					t.Fatal("following windows started or advanced inactive playback")
				}
			})
		}
	}
}
