package winamp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

// Native inspection belongs to this opt-in test, not the application.
func nativeWindowID(w *ui.Window) string {
	if w == nil || w.Closed() {
		return ""
	}
	return w.PortalParent()
}

func nativeUtilityStatus(w, owner *ui.Window) (supported, ready bool) {
	id, parent := nativeWindowID(w), nativeWindowID(owner)
	if !strings.HasPrefix(id, "x11:") || !strings.HasPrefix(parent, "x11:") {
		return false, false
	}
	data, err := exec.Command("xprop", "-id", "0x"+strings.TrimPrefix(id, "x11:"),
		"_NET_WM_WINDOW_TYPE", "_NET_WM_STATE", "WM_TRANSIENT_FOR").CombinedOutput()
	text := string(data)
	return true, err == nil && strings.Contains(text, "_NET_WM_WINDOW_TYPE_UTILITY") &&
		strings.Contains(text, "_NET_WM_STATE_SKIP_TASKBAR") &&
		strings.Contains(text, "_NET_WM_STATE_SKIP_PAGER") &&
		strings.Contains(text, "window id # 0x"+strings.TrimPrefix(parent, "x11:"))
}

// Explicit opt-in: this test opens only its own windows on the real desktop.
// xprop is a read-only verifier here; the application has no xprop dependency.
func TestNativeUtilityWindowsStayOffTaskbarAfterFrameRecreation(t *testing.T) {
	if os.Getenv("MEDIA_PLAYER_NATIVE_WINDOW_TEST") != "1" {
		t.Skip("set MEDIA_PLAYER_NATIVE_WINDOW_TEST=1 on an X11 desktop to inspect native window hints")
	}
	if _, err := exec.LookPath("xprop"); err != nil {
		t.Skip("the native hint verifier xprop is unavailable")
	}
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv(platform.EnvBackend, "x11")
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, skin)
			a := ui.New(ui.Options{Scale: 1, DisableLookWatch: true})
			p, err := New(players.NewHost(a), Options{Scale: 1})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			if !strings.HasPrefix(nativeWindowID(p.Main), "x11:") {
				t.Skip("the X11 backend is unavailable")
			}
			pumpNativeUtility(a.PumpOnce, p)
			assertNativeUtilityHints(t, p)
			assertKWinTaskbarPolicy(t, p)

			// A toolkit frame which newly needs alpha recreates native X11
			// windows. Exercise both child identity and main-owner changes.
			ownerBefore := nativeWindowID(p.Main)
			childBefore := nativeWindowID(p.Eq)
			mainFrame := p.Main.Surface().(platform.WindowFrame)
			frame := mainFrame.Frame()
			frame.Alpha = true
			mainFrame.SetFrame(frame)
			// Existing satellites must follow a recreated owner even when
			// their own native identity has not changed.
			pumpNativeUtility(a.PumpOnce, p)
			assertNativeUtilityHints(t, p)
			assertKWinTaskbarPolicy(t, p)
			eqFrame := p.Eq.Surface().(platform.WindowFrame)
			frame = eqFrame.Frame()
			frame.Alpha = true
			eqFrame.SetFrame(frame)
			pumpNativeUtility(a.PumpOnce, p)
			assertNativeUtilityHints(t, p)
			assertKWinTaskbarPolicy(t, p)
			if nativeWindowID(p.Main) == ownerBefore && nativeWindowID(p.Eq) == childBefore {
				t.Log("the desktop kept existing alpha visuals; utility hints remain correct")
			} else {
				t.Log("utility and owner hints survived toolkit native-window recreation")
			}
		})
	}
}

func TestNativeUtilitySetterRestoresMappedPolicyWithoutIdentityChange(t *testing.T) {
	if os.Getenv("MEDIA_PLAYER_NATIVE_WINDOW_TEST") != "1" {
		t.Skip("set MEDIA_PLAYER_NATIVE_WINDOW_TEST=1 on an X11 desktop")
	}
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("the owned native state requester needs cc and X11 headers")
	}
	dir := t.TempDir()
	source, request := filepath.Join(dir, "state.c"), filepath.Join(dir, "state")
	if err := os.WriteFile(source, []byte(nativeRemoveTaskbarState), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := exec.Command(compiler, source, "-o", request, "-lX11").CombinedOutput(); err != nil {
		t.Fatalf("compile the owned state requester: %v: %s", err, data)
	}
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv(platform.EnvBackend, "x11")
			t.Setenv(style.ThemeEnv, skin)
			a := ui.New(ui.Options{Scale: 1.75, DisableLookWatch: true})
			p, err := New(players.NewHost(a), Options{Scale: 1.75})
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			if !strings.HasPrefix(nativeWindowID(p.Main), "x11:") {
				t.Skip("the X11 backend is unavailable")
			}
			pumpNativeUtility(a.PumpOnce, p)
			assertKWinTaskbarPolicy(t, p)
			for _, named := range p.Windows()[1:] {
				id := nativeWindowID(named.Window)
				if supported, ready := nativeUtilityStatus(named.Window, p.Main); !supported || !ready {
					t.Fatalf("%s policy was not confirmed after mapping", named.Name)
				}
				if data, err := exec.Command(request, strings.TrimPrefix(id, "x11:")).CombinedOutput(); err != nil {
					t.Fatalf("request a policy reset for owned %s: %v: %s", named.Name, err, data)
				}
				// Let the actual WM process REMOVE, without app policy repair.
				deadline := time.Now().Add(time.Second)
				for {
					a.PumpOnce()
					if _, ready := nativeUtilityStatus(named.Window, p.Main); !ready {
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("the WM did not remove the owned %s taskbar state", named.Name)
					}
					time.Sleep(10 * time.Millisecond)
				}
				if nativeWindowID(named.Window) != id {
					t.Fatal("the policy reset unexpectedly recreated the window")
				}
			}
			// An explicit toolkit policy request must reach a mapped window
			// through its window manager, without rewriting initial properties.
			if !p.Eq.SetSkipTaskbar(true) || !p.List.SetSkipTaskbar(true) {
				t.Fatal("public utility policy request was rejected")
			}
			deadline := time.Now().Add(2500 * time.Millisecond)
			for {
				a.PumpOnce()
				p.Advance(70 * time.Millisecond)
				_, eqReady := nativeUtilityStatus(p.Eq, p.Main)
				_, listReady := nativeUtilityStatus(p.List, p.Main)
				if eqReady && listReady {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("explicit mapped policy requests were not accepted")
				}
				time.Sleep(20 * time.Millisecond)
			}
			assertNativeUtilityHints(t, p)
			assertKWinTaskbarPolicy(t, p)
			p.ShowEqualiser(false)
			p.ShowPlaylist(false)
			a.PumpOnce()
			p.ShowEqualiser(true)
			p.ShowPlaylist(true)
			pumpNativeUtility(a.PumpOnce, p)
			assertNativeUtilityHints(t, p)
			assertKWinTaskbarPolicy(t, p)
		})
	}
}

const nativeRemoveTaskbarState = `
#include <X11/Xlib.h>
#include <stdlib.h>
#include <string.h>
int main(int argc, char** argv) {
  if (argc != 2) return 2;
  Display* d = XOpenDisplay(NULL);
  if (!d) return 3;
  XEvent e; memset(&e, 0, sizeof(e));
  e.xclient.type = ClientMessage;
  e.xclient.window = strtoul(argv[1], NULL, 16);
  e.xclient.message_type = XInternAtom(d, "_NET_WM_STATE", False);
  e.xclient.format = 32;
  e.xclient.data.l[0] = 0; // REMOVE, requested only for this test's own windows.
  e.xclient.data.l[1] = XInternAtom(d, "_NET_WM_STATE_SKIP_TASKBAR", False);
  e.xclient.data.l[2] = XInternAtom(d, "_NET_WM_STATE_SKIP_PAGER", False);
  e.xclient.data.l[3] = 1;
  int sent = XSendEvent(d, DefaultRootWindow(d), False,
    SubstructureRedirectMask | SubstructureNotifyMask, &e);
  XSync(d, False); XCloseDisplay(d);
  return sent ? 0 : 4;
}
`

func pumpNativeUtility(pump func(), p *Player) {
	// Mapping and withdrawal acknowledgements are asynchronous. Allow the
	// bounded fast attempts plus a slow retry, rather than assuming the WM
	// accepted every policy within a fixed 400 ms.
	start := time.Now()
	deadline := start.Add(2500 * time.Millisecond)
	var readySince time.Time
	for time.Now().Before(deadline) {
		pump()
		p.Advance(0)
		ready := p.stacked && p.restoredPlacement == nil
		for _, named := range p.Windows()[1:] {
			if supported, accepted := nativeUtilityStatus(named.Window, p.Main); !supported || !accepted {
				ready = false
				break
			}
		}
		if !ready {
			readySince = time.Time{}
		} else if readySince.IsZero() {
			readySince = time.Now()
		}
		if !readySince.IsZero() && time.Since(start) >= 400*time.Millisecond && time.Since(readySince) >= 100*time.Millisecond {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func assertNativeUtilityHints(t *testing.T, p *Player) {
	t.Helper()
	owner := strings.TrimPrefix(nativeWindowID(p.Main), "x11:")
	for _, named := range p.Windows()[1:] {
		id := strings.TrimPrefix(nativeWindowID(named.Window), "x11:")
		data, err := exec.Command("xprop", "-id", "0x"+id,
			"_NET_WM_WINDOW_TYPE", "_NET_WM_STATE", "WM_TRANSIENT_FOR").CombinedOutput()
		if err != nil {
			t.Fatalf("inspect %s: %v: %s", named.Name, err, data)
		}
		text := string(data)
		if !strings.Contains(text, "_NET_WM_WINDOW_TYPE_UTILITY") ||
			!strings.Contains(text, "_NET_WM_STATE_SKIP_TASKBAR") ||
			!strings.Contains(text, "_NET_WM_STATE_SKIP_PAGER") ||
			!strings.Contains(text, "window id # 0x"+owner) {
			t.Fatalf("%s has incomplete auxiliary-window hints: %s", named.Name, text)
		}
		t.Logf("%s hints: %s", named.Name, text)
	}
}
