//go:build ignore

package main

/*
#cgo pkg-config: x11 xext
#include <X11/Xlib.h>
#include <X11/extensions/shape.h>
static int region_check(unsigned long id, int kind, int x, int y, int *count) {
 Display *d=XOpenDisplay(NULL); if(!d)return -1;
 int order=0, inside=0;
 XRectangle *r=XShapeGetRectangles(d,id,kind,count,&order);
 if(r){for(int i=0;i<*count;i++){if(x>=r[i].x&&x<r[i].x+r[i].width&&y>=r[i].y&&y<r[i].y+r[i].height)inside=1;}XFree(r);}
 XCloseDisplay(d);return inside;
}
*/
import "C"

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/headspace"
	"github.com/codemodify/media-player-music/internal/skins"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := skins.Register(); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, players.HeadspaceSkin)
	for _, scale := range []float32{1, 1.75} {
		a := uitoolkit.New(uitoolkit.Options{Backend: "x11", Scale: scale, DisableLookWatch: true})
		p, err := headspace.New(players.NewHost(a), headspace.Options{Scale: scale})
		if err != nil {
			panic(err)
		}
		// Keep this verification window out of the user's desktop and taskbar.
		p.Main.Hide()
		for _, state := range []struct {
			name   string
			eq, pl bool
		}{{"both", true, true}, {"eq", true, false}, {"pl", false, true}, {"closed", false, false}} {
			p.ShowEqualiser(state.eq)
			p.ShowPlaylist(state.pl)
			deadline := time.Now().Add(100 * time.Millisecond)
			for time.Now().Before(deadline) {
				a.PumpOnce()
				time.Sleep(5 * time.Millisecond)
			}
			img := p.Main.CaptureSurface()
			id, err := strconv.ParseUint(strings.TrimPrefix(p.Main.PortalParent(), "x11:"), 16, 64)
			if err != nil {
				panic(err)
			}
			frame := platform.FrameOf(p.Main.Surface()).Frame()
			var bounding, input C.int
			corner := int(C.region_check(C.ulong(id), C.ShapeBounding, 0, 0, &bounding))
			left := int(C.region_check(C.ulong(id), C.ShapeInput, C.int(150*scale), C.int(175*scale), &input))
			right := int(C.region_check(C.ulong(id), C.ShapeInput, C.int(620*scale), C.int(175*scale), &input))
			if !p.Main.ShapeActive() || !frame.Alpha || corner != 0 || left != boolInt(state.eq) || right != boolInt(state.pl) || bounding < 10 || input < 10 || img.NRGBAAt(0, 0).A != 0 {
				panic(fmt.Sprintf("shape mismatch scale=%g state=%s bounding=%d input=%d corner=%d left=%d right=%d alpha=%d frameAlpha=%t", scale, state.name, bounding, input, corner, left, right, img.NRGBAAt(0, 0).A, frame.Alpha))
			}
			fmt.Printf("scale=%g state=%s nativeBounding=%d nativeInput=%d cornerAlpha=0 inputMatchesWings=true\n", scale, state.name, bounding, input)
		}
		p.Close()
	}
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
