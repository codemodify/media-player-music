package shell

import (
	"os"
	"testing"

	"github.com/codemodify/media-player-music/internal/players/playertest"
)

func TestMain(m *testing.M) { os.Exit(playertest.Main(m)) }
