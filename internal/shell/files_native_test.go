package shell

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

// This opt-in integration crosses the actual native XDND selection protocol,
// rather than injecting an already decoded DropEvent. It creates only its own
// windows and uses a recording playback backend, so no audio is played.
func TestNativeXDNDPlaylistFolderDrop(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MUSIC_PLAYER_NATIVE_XDND_TEST") != "1" {
		t.Skip("set MUSIC_PLAYER_NATIVE_XDND_TEST=1 with an X11 display to test external XDND")
	}
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("native XDND sender requires cc and X11 development headers")
	}
	work := t.TempDir()
	source := filepath.Join(work, "xdnd-sender.c")
	sender := filepath.Join(work, "xdnd-sender")
	if err := os.WriteFile(source, []byte(nativeXDNDSource), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(compiler, source, "-o", sender, "-lX11").CombinedOutput(); err != nil {
		t.Fatalf("compile native XDND sender: %v\n%s", err, output)
	}
	folder := filepath.Join(work, "music folder")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	writeMusicEntry(t, folder, "A-song.MP4")
	writeMusicEntry(t, folder, "nested/B-song.flac")
	writeMusicEntry(t, folder, "cover.jpg")
	if requested := os.Getenv("MUSIC_PLAYER_NATIVE_DROP_FOLDER"); requested != "" {
		folder = requested
	}
	want, err := expandMusicPaths(context.Background(), []string{folder})
	if err != nil {
		t.Fatal(err)
	}
	if len(want.tracks) == 0 {
		t.Fatal("native test folder has no playable entries")
	}
	t.Setenv(platform.EnvBackend, "x11")
	for _, skin := range []string{players.DefaultSkin, winamp.Skin, winamp.SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, LookFor(skin))
			a := ui.New(ui.Options{Scale: 1.75, DisableLookWatch: true})
			backend := &recordingBackend{events: make(chan playback.Event, 100)}
			s, err := New(a, Options{Skin: skin, Scale: 1.75, Backend: backend})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(s.Close)
			window := nativePlaylistWindow(s)
			inspectNativeXDND(t, sender, window, "before first frame")
			a.PumpOnce()
			// Application.Run normally maps windows; this test drives the
			// native pump directly, so map its owned windows explicitly.
			for _, named := range s.Windows() {
				named.Window.Show()
			}
			s.Start()
			deadline := time.Now().Add(1500 * time.Millisecond)
			for time.Now().Before(deadline) {
				a.PumpOnce()
				time.Sleep(time.Millisecond)
			}
			if !inspectNativeXDND(t, sender, window, "after frame setup and utility settling") {
				t.Skip("native playlist is not viewable; visible XDND discovery is unavailable while the desktop is locked or the window is hidden")
			}
			var list *a11y.Node
			window.AccessibleTree().Walk(func(node *a11y.Node) bool {
				if node.Role == a11y.RoleList {
					list = node
				}
				return true
			})
			if list == nil || list.Bounds.Empty() {
				t.Fatal("playlist has no native drop bounds")
			}
			for _, preferred := range []string{"copy", "move", "copy-unlisted"} {
				before := s.Host.Model.Transport.List.Len()
				point := list.Bounds.Center()
				payload := (&url.URL{Scheme: "file", Path: folder}).String() + "\r\n"
				parent := window.PortalParent()
				if !strings.HasPrefix(parent, "x11:") {
					t.Fatalf("native test requires X11, got %q", parent)
				}
				nativeXDND(t, a, sender, strings.TrimPrefix(parent, "x11:"), int(point.X), int(point.Y), payload, preferred)
				waitFileLoad(t, a, "native folder drop", func() bool {
					return s.Host.Model.Transport.List.Len() == before+len(want.tracks)
				})
				for i, track := range want.tracks {
					if got := s.Host.Model.Transport.List.At(before + i); got.Path != track.Path {
						t.Fatalf("native folder entry %d differs from recursive expansion", i)
					}
				}
				if s.Host.Model.Error != "" {
					t.Fatalf("native folder loading failed: %s", s.Host.Model.Error)
				}
				t.Logf("native %s-preferred folder drop negotiated Copy and added %d entries (recording backend; no audio)", preferred, len(want.tracks))
			}
		})
	}
}

func nativePlaylistWindow(s *Shell) *ui.Window {
	for _, named := range s.Windows() {
		if named.Name == "playlist" {
			return named.Window
		}
	}
	return s.MainWindow()
}

func inspectNativeXDND(t *testing.T, sender string, window *ui.Window, phase string) bool {
	t.Helper()
	parent := window.PortalParent()
	if !strings.HasPrefix(parent, "x11:") {
		t.Fatalf("native test requires X11, got %q", parent)
	}
	output, err := exec.Command(sender, strings.TrimPrefix(parent, "x11:"), "inspect").CombinedOutput()
	if err != nil {
		t.Fatalf("XDND advertisement %s failed: %v\n%s", phase, err, output)
	}
	t.Logf("%s: %s", phase, strings.TrimSpace(string(output)))
	return strings.Contains(string(output), "mapped=2 ")
}

func nativeXDND(t *testing.T, a *ui.Application, sender, target string, x, y int, payload, preferred string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, sender, target, fmt.Sprint(x), fmt.Sprint(y), payload, preferred)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		a.PumpOnce()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("native XDND sender failed: %v\n%s", err, output.String())
			}
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

const nativeXDNDSource = `
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <time.h>

static Window destination;
static unsigned long property(Display *d, Window w, const char *name) {
  Atom type; int format; unsigned long count, left, value = 0; unsigned char *data = NULL;
  if (XGetWindowProperty(d, w, XInternAtom(d, name, False), 0, 1, False, AnyPropertyType,
                        &type, &format, &count, &left, &data) == Success && format == 32 && count == 1)
    value = ((unsigned long *)data)[0];
  if (data) XFree(data); return value;
}
static Window discover(Display *d, Window root, int rx, int ry) {
  Window current = root, found = None;
  for (int depth = 0; current && depth < 32; depth++) {
    if (property(d, current, "XdndAware") >= 3) found = current;
    int lx, ly; Window child;
    if (!XTranslateCoordinates(d, root, current, rx, ry, &lx, &ly, &child) || !child) break;
    current = child;
  }
  return found;
}
static int advertisement(Display *d, Window w) {
  unsigned long version = property(d, w, "XdndAware");
  Window proxy = property(d, w, "XdndProxy");
  XWindowAttributes a;
  if (!XGetWindowAttributes(d, w, &a)) return 0;
  Window r, parent, *children = NULL; unsigned int count;
  XQueryTree(d, w, &r, &parent, &children, &count); if (children) XFree(children);
  printf("window=%#lx XdndAware=%lu XdndProxy=%#lx depth=%d mapped=%d parent=%#lx\n", w, version, proxy, a.depth, a.map_state, parent);
  if (version < 3) return 0;
  if (proxy && property(d, proxy, "XdndProxy") != proxy) return 0;
  return 1;
}
static void message(Display *d, Window target, Atom type, long a, long b, long c, long e, long f) {
  XEvent ev = {0};
  ev.xclient.type = ClientMessage; ev.xclient.display = d;
  ev.xclient.window = target; ev.xclient.message_type = type; ev.xclient.format = 32;
  ev.xclient.data.l[0] = a; ev.xclient.data.l[1] = b; ev.xclient.data.l[2] = c;
  ev.xclient.data.l[3] = e; ev.xclient.data.l[4] = f;
  XSendEvent(d, destination ? destination : target, False, NoEventMask, &ev); XFlush(d);
}
int main(int argc, char **argv) {
  if (argc != 6 && argc != 3) return 2;
  Display *d = XOpenDisplay(NULL); if (!d) return 3;
  Window root = DefaultRootWindow(d), target = strtoul(argv[1], NULL, 16);
  if (argc == 3) { int ok = advertisement(d, target); XCloseDisplay(d); return ok ? 0 : 7; }
  Window source = XCreateSimpleWindow(d, root, 0, 0, 1, 1, 0, 0, 0);
  Atom selection = XInternAtom(d, "XdndSelection", False);
  Atom enter = XInternAtom(d, "XdndEnter", False), position = XInternAtom(d, "XdndPosition", False);
  Atom status = XInternAtom(d, "XdndStatus", False), drop = XInternAtom(d, "XdndDrop", False);
  Atom finished = XInternAtom(d, "XdndFinished", False), uri = XInternAtom(d, "text/uri-list", False);
  Atom copy = XInternAtom(d, "XdndActionCopy", False), move = XInternAtom(d, "XdndActionMove", False);
  Atom actions[] = {copy, move};
  if (!strstr(argv[5], "unlisted"))
    XChangeProperty(d, source, XInternAtom(d, "XdndActionList", False), XA_ATOM, 32, PropModeReplace, (unsigned char *)actions, 2);
  XSetSelectionOwner(d, selection, source, CurrentTime);
  int rx, ry; Window child;
  XTranslateCoordinates(d, target, root, atoi(argv[2]), atoi(argv[3]), &rx, &ry, &child);
  Window found = discover(d, root, rx, ry);
  if (found != target || !advertisement(d, found)) {
    fprintf(stderr, "native discovery at %d,%d found %#lx instead of %#lx\n", rx, ry, found, target); return 8;
  }
  target = found; destination = property(d, target, "XdndProxy");
  message(d, target, enter, source, 5L << 24, uri, 0, 0);
  message(d, target, position, source, 0, ((rx & 65535L) << 16) | (ry & 65535L), CurrentTime,
          strcmp(argv[5], "move") == 0 ? move : copy);
  time_t until = time(NULL) + 10;
  while (time(NULL) < until) {
    if (!XPending(d)) {
      fd_set fds; FD_ZERO(&fds); FD_SET(ConnectionNumber(d), &fds);
      struct timeval wait = {0, 100000}; select(ConnectionNumber(d)+1, &fds, NULL, NULL, &wait); continue;
    }
    XEvent ev; XNextEvent(d, &ev);
    if (ev.type == ClientMessage && ev.xclient.message_type == status) {
      if (!(ev.xclient.data.l[1] & 1) || ev.xclient.data.l[4] != copy) {
        fprintf(stderr, "target refused URI Copy negotiation\n"); return 4;
      }
      message(d, target, drop, source, 0, CurrentTime, 0, 0);
    } else if (ev.type == SelectionRequest) {
      XSelectionRequestEvent *r = &ev.xselectionrequest;
      Atom prop = r->property ? r->property : r->target;
      XEvent reply = {0}; reply.xselection.type = SelectionNotify; reply.xselection.display = d;
      reply.xselection.requestor = r->requestor; reply.xselection.selection = r->selection;
      reply.xselection.target = r->target; reply.xselection.time = r->time;
      if (r->target == uri) {
        XChangeProperty(d, r->requestor, prop, uri, 8, PropModeReplace, (unsigned char *)argv[4], strlen(argv[4]));
        reply.xselection.property = prop;
      }
      XSendEvent(d, r->requestor, False, NoEventMask, &reply); XFlush(d);
    } else if (ev.type == ClientMessage && ev.xclient.message_type == finished) {
      int ok = (ev.xclient.data.l[1] & 1) && ev.xclient.data.l[2] == copy;
      XDestroyWindow(d, source); XCloseDisplay(d); return ok ? 0 : 5;
    }
  }
  fprintf(stderr, "timed out waiting for native XDND completion\n"); return 6;
}
`
