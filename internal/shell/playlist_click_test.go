package shell

import (
	"github.com/codemodify/media-player-music/internal/playback"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
)

func TestPlaylistClickSelectsAndDoubleClickStartsBackend(t *testing.T) {
	for _, skin := range players.AppearanceIDs() {
		for _, state := range []players.State{players.Stopped, players.Paused, players.Playing} {
			t.Run(skin+"/"+state.String(), func(t *testing.T) {
				a, s, backend := liveShell(t, skin)
				if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
					t.Fatal(err)
				}
				transport := s.Host.Model.Transport
				transport.State, transport.Pos = state, 13*time.Second
				s.refresh()
				a.PumpOnce()
				window := nativePlaylistWindow(s)
				var list *a11y.Node
				window.AccessibleTree().Walk(func(node *a11y.Node) bool {
					if node.Role == a11y.RoleList {
						list = node
					}
					return true
				})
				if list == nil || len(list.Children) != 2 || list.Children[1].Bounds.Empty() {
					t.Fatal("the second playlist row is not visible")
				}
				row := list.Children[1]
				injectClick := func(target *a11y.Node) {
					window.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: target.Bounds.Center()})
					window.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: target.Bounds.Center()})
				}
				click := func() {
					injectClick(row)
					a.PumpOnce()
				}
				backend.commands = nil
				click()
				if len(backend.commands) != 0 || transport.List.Index() != 0 || transport.State != state || transport.Pos != 13*time.Second {
					t.Fatalf("one click changed playback: track=%d state=%s pos=%s commands=%v", transport.List.Index(), transport.State, transport.Pos, backend.commands)
				}
				// A normal UI pulse must retain the independent row selection.
				s.tick(70 * time.Millisecond)
				a.PumpOnce()
				selected := window.AccessibleTree().Find(row.ID)
				if selected == nil || selected.State&a11y.StateSelected == 0 {
					t.Fatal("the UI pulse discarded the clicked row selection")
				}
				if transport.List.Index() != 0 || transport.State != state || transport.Pos != 13*time.Second {
					t.Fatal("selecting a row changed the playing track or clock")
				}
				backend.commands = nil
				// Begin a fresh sequence on another row, then deliver the two
				// activation clicks in one batch, independent of paint timing.
				injectClick(list.Children[0])
				injectClick(row)
				injectClick(row)
				a.PumpOnce()
				if countCommand(backend, playback.OpLoad) != 1 || transport.List.Index() != 1 || transport.State != players.Playing || transport.Pos != 0 {
					t.Fatalf("double-click did not start exactly one track: track=%d state=%s pos=%s commands=%v", transport.List.Index(), transport.State, transport.Pos, backend.commands)
				}
			})
		}
	}
}
