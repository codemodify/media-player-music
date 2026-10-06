package winamp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
)

func TestNativePlaylistResizeAndSavedSize(t *testing.T) {
	if os.Getenv("MEDIA_PLAYER_NATIVE_WINDOW_TEST") != "1" {
		t.Skip("set MEDIA_PLAYER_NATIVE_WINDOW_TEST=1 on an X11 desktop to verify native resizing")
	}
	if _, err := exec.LookPath("xprop"); err != nil {
		t.Skip("the read-only native hint verifier xprop is unavailable")
	}
	const scale = 1.75
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv("UITK_BACKEND", "x11")
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, skin)
			a := uitoolkit.New(uitoolkit.Options{Scale: scale, DisableLookWatch: true})
			p, err := New(players.NewHost(a), Options{Scale: scale})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			if !strings.HasPrefix(nativeWindowID(p.List), "x11:") {
				t.Skip("the X11 backend is unavailable")
			}
			pumpNativeResize(a.PumpOnce, p)
			rowsBefore := p.listPane.list.geo.rows
			p.List.SetSize(510, 410)
			a.PumpOnce()
			p.List.Move(120, 160)
			pumpNativeResize(a.PumpOnce, p)
			id := strings.TrimPrefix(nativeWindowID(p.List), "x11:")
			data, err := exec.Command("xprop", "-id", "0x"+id, "WM_NORMAL_HINTS", "_NET_WM_ALLOWED_ACTIONS").CombinedOutput()
			t.Logf("%s native hints: %s", skin, data)
			if w, h := p.List.Size(); w != 510 || h != 410 {
				t.Fatalf("the desktop did not accept the larger playlist: %dx%d", w, h)
			}
			if p.listPane.list.geo.rows.Dx() <= rowsBefore.Dx() || p.listPane.list.geo.rows.Dy() <= rowsBefore.Dy() {
				t.Fatal("the native resize did not update the toolkit's playlist viewport")
			}
			if err != nil {
				t.Fatalf("inspect native resize policy: %v: %s", err, data)
			}
			margin := platform.FrameOf(p.List.Surface()).Frame().Margin
			minimum := fmt.Sprintf("minimum size: %d by %d", platform.DevicePixels(StripW, scale)+margin.Width(), platform.DevicePixels(ListH, scale)+margin.Height())
			if !strings.Contains(string(data), minimum) || !strings.Contains(string(data), "_NET_WM_ACTION_RESIZE") ||
				strings.Contains(string(data), "maximum size:") {
				t.Fatalf("the desktop did not receive a resizable playlist with the control minimum: %s", data)
			}
			assertNativeUtilityHints(t, p)
			assertPlaylistWellColour(t, p, skin, scale)
			if directory := os.Getenv("MEDIA_PLAYER_RESIZE_SCREENSHOTS"); directory != "" {
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
				file, err := os.Create(filepath.Join(directory, skin+"-playlist-resized.png"))
				if err != nil {
					t.Fatal(err)
				}
				err = p.List.Capture().WritePNG(file)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("save resized playlist: %v, %v", err, closeErr)
				}
			}
			saved := placementJSONRoundTrip(t, p.CapturePlacement())
			p.Close()
			p.Host.ApplySkin(otherPanelSkin(skin))
			p, err = New(p.Host, Options{Scale: scale})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			p.RestorePlacement(saved)
			pumpNativeResize(a.PumpOnce, p)
			if w, h := p.List.Size(); w != 510 || h != 410 {
				t.Fatalf("native skin switching discarded the saved playlist size: %dx%d", w, h)
			}
			if got := p.CapturePlacement()["playlist"]; got.X != saved["playlist"].X || got.Y != saved["playlist"].Y || got.Dock != nil {
				t.Fatalf("native size restoration changed the detached placement: %+v, want %+v", got, saved["playlist"])
			}
			assertNativeUtilityHints(t, p)
		})
	}
}

func pumpNativeResize(pump func(), p *Player) {
	// Allow the desktop to acknowledge initial mapping and the desk's bounded
	// stack/restore retries before simulating the next independent user action.
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		pump()
		p.Advance(0)
		time.Sleep(20 * time.Millisecond)
	}
}
