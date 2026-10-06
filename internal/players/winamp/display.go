package winamp

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/style"
)

// ---- the display -------------------------------------------------------------

// readout gives the panel's printed clock and track an accessible name and
// a window drag handle. The strip paints the visible display over the art.
type readout struct {
	players.DragsWindow
	p *Player
}

func newReadout(p *Player) *readout {
	r := &readout{p: p}
	r.Init(r)
	r.Win = p.Main
	return r
}

func (r *readout) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(r.Look(), 200), style.Dip(r.Look(), 38)))
}

func (r *readout) Arrange(b paintengine2d.Rect) { r.SetBounds(b) }

// Describe is the display as a screen reader meets it: one label whose name
// is everything the well says, so a reader announces the track and the time
// rather than a blank rectangle.
func (r *readout) Describe(n *a11y.Node) {
	t := r.p.Transport
	n.Role = a11y.RoleLabel
	if t.List.Len() == 0 {
		n.Name = "Open audio files to play. " + t.State.String() + "."
	} else {
		n.Name = fmt.Sprintf("%s. %s of %s. %s.",
			t.Track().Label(), players.Clock(t.Pos), durationText(t.Length()), t.State)
	}
	n.Description = r.p.Host.StatusText()
}

func durationText(d time.Duration) string {
	if d <= 0 {
		return "--:--"
	}
	return players.Clock(d)
}

func playlistDurationText(pl *players.Playlist) string {
	if pl.Len() == 0 {
		return "--:--"
	}
	for _, tr := range pl.Tracks {
		if tr.Length <= 0 {
			return "--:--"
		}
	}
	return players.Clock(pl.Total())
}

func currentTitle(t *players.Transport) string {
	if t.List.Len() == 0 {
		return "Open audio files to play"
	}
	tr := t.Track()
	title := fmt.Sprintf("%d. %s", t.List.Index()+1, tr.Label())
	if tr.Length > 0 {
		title += " (" + players.Clock(tr.Length) + ")"
	}
	return title
}
