package players

import (
	"os"
	"testing"

	"goframework.io/desktop-ui/platform/platformtest"
)

// TestMain keeps the tests off the person's look, folders and session
// (desktop-ui's platformtest, R102).
func TestMain(m *testing.M) { os.Exit(platformtest.Main(m)) }
