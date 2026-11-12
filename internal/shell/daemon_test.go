package shell

import (
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
)

// Starting media-player-musicd (R115, through link's Daemon): every face
// says "Starting…" where it says the player's errors; when it cannot be
// started they say why, and a window over the player says it too, with
// Retry (focused: Enter or Space retries) and Close (Escape); once it runs
// nothing is said.
func TestTheWindowSaysTheDaemonIsStartingOrWhyNot(t *testing.T) {
	for _, skin := range []string{players.DefaultSkin, players.SlateSkin} {
		t.Run(skin, func(t *testing.T) {
			a, s, _ := liveShell(t, skin)
			s.ShowDaemon("Starting media-player-musicd…", false, nil)
			a.PumpOnce()
			if text, problem, window := s.DaemonNote(); text != "Starting media-player-musicd…" || problem || window != nil {
				t.Fatalf("%q %v %v", text, problem, window)
			}
			if got := s.Host.StatusText(); got != "Starting media-player-musicd…" {
				t.Fatalf("the faces say %q", got)
			}

			retried := 0
			why := "media-player-musicd not found next to this program or on PATH"
			s.ShowDaemon(why, true, func() { retried++ })
			a.PumpOnce()
			_, problem, window := s.DaemonNote()
			if !problem || window == nil || s.Host.StatusText() != why {
				t.Fatalf("%v %v %q", problem, window, s.Host.StatusText())
			}
			tree := window.AccessibleTree()
			for _, want := range []string{"The music daemon", "Retry", "Close", why} {
				if !holds(tree, want) {
					t.Fatalf("the window's tree lacks %q", want)
				}
			}
			window.Press(platform.KeyReturn) // Retry has the focus
			a.PumpOnce()
			if _, _, open := s.DaemonNote(); retried != 1 || open != nil {
				t.Fatalf("retried %d, window %v", retried, open)
			}

			s.ShowDaemon(why, true, func() { retried++ })
			a.PumpOnce()
			_, _, window = s.DaemonNote()
			window.Press(platform.KeyEscape)
			a.PumpOnce()
			if _, _, open := s.DaemonNote(); retried != 1 || open != nil {
				t.Fatalf("Escape: retried %d, window %v", retried, open)
			}

			s.ShowDaemon("", false, nil)
			a.PumpOnce()
			if text, _, _ := s.DaemonNote(); text != "" || s.Host.Model.Error != "" {
				t.Fatalf("%q %q", text, s.Host.Model.Error)
			}
		})
	}
}

// holds tells whether a node of tree is named, or described, with text.
func holds(tree *a11y.Node, text string) bool {
	if tree == nil {
		return false
	}
	if strings.Contains(tree.Name, text) || strings.Contains(tree.Description, text) || strings.Contains(tree.Value, text) {
		return true
	}
	for _, child := range tree.Children {
		if holds(child, text) {
			return true
		}
	}
	return false
}
