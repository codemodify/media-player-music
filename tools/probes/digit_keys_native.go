//go:build ignore

// Targeted native X11 key events, sent only to this probe's own Surface.
// Requires an isolated owned display and MUSIC_PLAYER_PROBE_PRIVATE_NATIVE=1.
package main

/*
#cgo pkg-config: x11
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <string.h>
static unsigned int num_lock_mask(Display *d) {
  XModifierKeymap *map = XGetModifierMapping(d);
  KeyCode num = XKeysymToKeycode(d, XK_Num_Lock);
  unsigned int mask = 0;
  if (map) {
    for (int mod = 0; mod < 8; mod++)
      for (int i = 0; i < map->max_keypermod; i++)
        if (num && map->modifiermap[mod * map->max_keypermod + i] == num)
          mask |= (1u << mod);
    XFreeModifiermap(map);
  }
  return mask;
}
static int send_key(Display *d, unsigned long id, unsigned long sym, unsigned int state) {
  KeyCode code = XKeysymToKeycode(d, sym);
  if (!code) return 0;
  XEvent ev;
  memset(&ev, 0, sizeof(ev));
  ev.xkey.type = KeyPress;
  ev.xkey.display = d;
  ev.xkey.window = id;
  ev.xkey.root = DefaultRootWindow(d);
  ev.xkey.time = CurrentTime;
  ev.xkey.x = ev.xkey.y = ev.xkey.x_root = ev.xkey.y_root = 1;
  ev.xkey.same_screen = True;
  ev.xkey.keycode = code;
  ev.xkey.state = state;
  int result = XSendEvent(d, id, False, KeyPressMask, &ev);
  XSync(d, False);
  return result;
}
*/
import "C"

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
)

func main() {
	if os.Getenv("MUSIC_PLAYER_PROBE_PRIVATE_NATIVE") != "1" || os.Getenv("DISPLAY") == "" {
		panic("set MUSIC_PLAYER_PROBE_PRIVATE_NATIVE=1 on an isolated owned native display")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	backend := platform.Select("x11", false)
	if backend.Name() != "x11" {
		panic("native x11 backend unavailable")
	}
	s, err := backend.NewSurface(platform.WindowOptions{Title: "desktop-ui digit mapping probe", Width: 120, Height: 60, Decorations: platform.DecorationsNone})
	if err != nil {
		panic(err)
	}
	defer s.Close()
	id, err := strconv.ParseUint(strings.TrimPrefix(s.(platform.PortalParenter).PortalParent(), "x11:"), 16, 64)
	if err != nil {
		panic(err)
	}
	d := C.XOpenDisplay(nil)
	if d == nil {
		panic("cannot open the isolated X display")
	}
	defer C.XCloseDisplay(d)
	num := C.num_lock_mask(d)
	if num == 0 {
		panic("isolated X keymap has no Num Lock modifier")
	}
	// Consume startup events. No input focus, grabs, physical-device event
	// injection, keymap modification or other application's window is used.
	s.Poll()
	check := func(label string, sym uint64, state C.uint, want platform.Key, mods platform.Modifiers) platform.Event {
		if C.send_key(d, C.ulong(id), C.ulong(sym), state) == 0 {
			panic("cannot send " + label + " to the owned window")
		}
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			for _, ev := range s.Poll() {
				if ev.Kind != platform.EventKeyDown {
					continue
				}
				if ev.Key != want || ev.Mods.Chord() != mods {
					panic(fmt.Sprintf("%s: native key=%d mods=%x want key=%d mods=%x", label, ev.Key, ev.Mods.Chord(), want, mods))
				}
				return ev
			}
			time.Sleep(time.Millisecond)
		}
		panic("native event timeout for " + label)
	}
	for n := 0; n <= 9; n++ {
		key := platform.DigitKey(rune('0' + n))
		for _, keypad := range []bool{false, true} {
			sym := uint64(0x30 + n)
			state := C.uint(C.ControlMask)
			label := fmt.Sprintf("Ctrl+%d row", n)
			if keypad {
				sym = uint64(0xffb0 + n)
				state |= num
				label = fmt.Sprintf("Ctrl+KP%d NumLock", n)
			}
			ev := check(label, sym, state, key, platform.ModCtrl)
			calls := 0
			bar := widgets.NewMenuBar(widgets.NewMenu("Go", widgets.ItemAccel("Digit", fmt.Sprintf("Ctrl+%d", n), func() { calls++ })))
			if !bar.HandleAccelerator(ev.Key, ev.Mods) || calls != 1 {
				panic(label + " native event did not activate its public accelerator")
			}
		}
	}
	check("Ctrl+comma retained", 0x2c, C.ControlMask, platform.KeyComma, platform.ModCtrl)
	check("Ctrl+S retained", 0x73, C.ControlMask, platform.KeyS, platform.ModCtrl)
	check("Ctrl+Shift+3 hash retained", 0x33, C.ControlMask|C.ShiftMask, platform.KeyHash, platform.ModCtrl|platform.ModShift)
	check("keypad2 NumLock off is navigation", 0xffb2, 0, platform.KeyDown, 0)
	fmt.Printf("PASS: 20 native row/keypad digit events activate Ctrl+0..9 once; comma/S/hash retained; keypad NumLock-off navigation retained (display=%s NumLockMask=0x%x)\n", os.Getenv("DISPLAY"), uint(num))
}
