package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goframework.io/desktop-ui/style"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

// Every folder desktop-ui reads the person's look and files from is in the
// test's own folder, and no look variable is left set.
func TestNothingOfThePersonsIsReached(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(home)
	if !strings.HasPrefix(filepath.Base(root), "media-player-test-") {
		t.Fatalf("home is not the test's own: %s", home)
	}
	for what, path := range map[string]string{
		"config": style.ConfigDir(), "uitoolkit": style.UitoolkitConfigDir(),
		"cache": style.CacheDir(), "look.json": style.AppearanceSource(),
	} {
		if !strings.HasPrefix(path, root+string(filepath.Separator)) {
			t.Errorf("%s is outside the test's folder: %s", what, path)
		}
	}
	for _, name := range lookEnv {
		if v, set := os.LookupEnv(name); set {
			t.Errorf("%s is still set (%q)", name, v)
		}
	}
	if got, want := style.LoadAppearance().Name, style.DefaultAppearance().Name; got != want {
		t.Errorf("the look is %q, not the default %q", got, want)
	}
}
