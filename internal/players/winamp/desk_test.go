package winamp

import (
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"testing"
)

func TestDeferredDeskBatchAndToolkitDeadlines(t *testing.T) {
	playertest.AuditDeskRuntime(t, func(t *testing.T) playertest.DeskFixture {
		a, p := open(t, Skin, 1.75)
		p.Advance(0)
		return playertest.DeskFixture{App: a, Desk: p.Desk, Main: p.iMain, Satellite: p.iList, Show: p.ShowPlaylist}
	})
}
