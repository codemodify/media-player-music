package plans

import (
	"os"
	"testing"

	"github.com/codemodify/media-player-music/internal/testenv"
)

// TestMain keeps the tests off the person's look, folders and session
// (testenv, R102).
func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
