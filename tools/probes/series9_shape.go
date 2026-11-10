//go:build ignore

// Series 9 drawers/full/compact presentation with read-only native X11 SHAPE and owner checks.
// Run with a native DISPLAY and private bus/XDG directories. The probe hides
// only its own windows immediately and never starts an audio backend.
package main

/*
#cgo pkg-config: x11 xext
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/extensions/shape.h>
#include <stdio.h>

typedef struct {
 int count, corner, far_corner, inside, outside_old, max_x, max_y;
} NativeShape;
static NativeShape read_shape(Display *d, Window id, int kind, int width, int height,
 int x, int y, int extra_x, int extra_y) {
 NativeShape out={0}; int order=0;
 XRectangle *r=XShapeGetRectangles(d,id,kind,&out.count,&order);
 if(r) {
  for(int i=0;i<out.count;i++) {
   int x1=r[i].x, y1=r[i].y, x2=x1+r[i].width, y2=y1+r[i].height;
   if(x1<=0 && 0<x2 && y1<=0 && 0<y2) out.corner=1;
   if(x1<=width-1 && width-1<x2 && y1<=height-1 && height-1<y2) out.far_corner=1;
   if(x1<=x && x<x2 && y1<=y && y<y2) out.inside=1;
   if(x1<=extra_x && extra_x<x2 && y1<=extra_y && extra_y<y2) out.outside_old=1;
   if(x2>out.max_x)out.max_x=x2;
   if(y2>out.max_y)out.max_y=y2;
  }
  XFree(r);
 }
 return out;
}
static int has_atom(Display *d, Window id, const char *property, const char *name) {
 Atom actual=None; int format=0; unsigned long n=0, remaining=0; unsigned char *data=NULL;
 int found=0;
 if(XGetWindowProperty(d,id,XInternAtom(d,property,False),0,64,False,XA_ATOM,
   &actual,&format,&n,&remaining,&data)==Success && actual==XA_ATOM && format==32) {
  Atom wanted=XInternAtom(d,name,False); Atom *items=(Atom *)data;
  for(unsigned long i=0;i<n;i++)if(items[i]==wanted)found=1;
 }
 if(data)XFree(data);
 return found;
}
typedef struct {
 int valid, width, height, depth, mapped, compositor, skip_taskbar, skip_pager, utility;
 int bounding_matches, input_matches, min_width, min_height, max_width, max_height;
 unsigned long hint_flags;
 unsigned long owner;
 NativeShape bounding, input;
} NativeWindow;
static int shape_matches(Display *d, Window id, int kind, XRectangle *wanted, int want_count) {
 int n=0, order=0; XRectangle *actual=XShapeGetRectangles(d,id,kind,&n,&order);
 Region got=XCreateRegion(), want=XCreateRegion();
 for(int i=0;i<n;i++)XUnionRectWithRegion(&actual[i],got,got);
 for(int i=0;i<want_count;i++)XUnionRectWithRegion(&wanted[i],want,want);
 int equal=XEqualRegion(got,want);
 if(actual)XFree(actual); XDestroyRegion(got);XDestroyRegion(want);return equal;
}
static NativeWindow read_window(unsigned long id, int x, int y, int extra_x, int extra_y,
 XRectangle *wanted, int want_count) {
 NativeWindow out={0}; Display *d=XOpenDisplay(NULL); if(!d)return out;
 XWindowAttributes a;
 if(XGetWindowAttributes(d,id,&a)) {
  out.valid=1; out.width=a.width;out.height=a.height;out.depth=a.depth;
  out.mapped=a.map_state!=IsUnmapped;
  char selection[64]; snprintf(selection,sizeof(selection),"_NET_WM_CM_S%d",DefaultScreen(d));
  out.compositor=XGetSelectionOwner(d,XInternAtom(d,selection,False))!=None;
  out.bounding=read_shape(d,id,ShapeBounding,a.width,a.height,x,y,extra_x,extra_y);
  out.input=read_shape(d,id,ShapeInput,a.width,a.height,x,y,extra_x,extra_y);
  out.bounding_matches=shape_matches(d,id,ShapeBounding,wanted,want_count);
  out.input_matches=shape_matches(d,id,ShapeInput,wanted,want_count);
  Window owner=None; if(XGetTransientForHint(d,id,&owner))out.owner=owner;
  out.skip_taskbar=has_atom(d,id,"_NET_WM_STATE","_NET_WM_STATE_SKIP_TASKBAR");
  out.skip_pager=has_atom(d,id,"_NET_WM_STATE","_NET_WM_STATE_SKIP_PAGER");
  out.utility=has_atom(d,id,"_NET_WM_WINDOW_TYPE","_NET_WM_WINDOW_TYPE_UTILITY");
  XSizeHints hints={0};long provided=0;
  if(XGetWMNormalHints(d,id,&hints,&provided)) {
   out.hint_flags=hints.flags;out.min_width=hints.min_width;out.min_height=hints.min_height;out.max_width=hints.max_width;out.max_height=hints.max_height;
  }
 }
 XCloseDisplay(d);return out;
}
*/
import "C"

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/series9"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		panic("native X11 DISPLAY required")
	}
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.Setenv(style.ThemeEnv, players.Series9Skin); err != nil {
		panic(err)
	}
	for _, scale := range []float32{1, 1.75} {
		probe(scale)
		probeStartup(scale, false)
		probeStartup(scale, true)
	}
}
func probe(scale float32) {
	a := ui.New(ui.Options{Backend: "x11", Scale: scale, DisableLookWatch: true})
	p, err := series9.New(players.NewHost(a), series9.Options{Scale: scale})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Main.Hide()
	p.Main.SetTitle("Series 9 native shape probe")
	removeTrace := p.Main.OnResize(func(width, height int) {
		if os.Getenv("MUSIC_PLAYER_PROBE_TRACE") == "1" {
			fmt.Printf("resize scale=%g size=%dx%d compact=%t drawers=%t/%t state=%+v\n", scale, width, height, p.Compact(), p.Host.Model.PlaylistVisible, p.Host.Model.EqualizerVisible, p.CaptureState())
		}
	})
	defer removeTrace()
	pump := func() {
		start, previous := time.Now(), time.Now()
		for steps := 0; steps < 8 || time.Since(start) < 500*time.Millisecond; steps++ {
			a.PumpOnce()
			now := time.Now()
			p.Advance(now.Sub(previous))
			previous = now
			time.Sleep(20 * time.Millisecond)
		}
	}
	pump()
	initialID := nativeID(p.Main)
	if !p.Main.Move(170, 210) {
		panic("X11 position unavailable")
	}
	pump()
	x, y, known := p.Main.Position()
	if !known {
		panic("position missing")
	}
	assertPosition := func() {
		nx, ny, nk := p.Main.Position()
		if !nk || nx != x || ny != y {
			panic(fmt.Sprintf("mode transition moved native Main: %d,%d->%d,%d", x, y, nx, ny))
		}
		if nativeID(p.Main) != initialID {
			panic("mode transition recreated native Main")
		}
	}
	for cycle := 0; cycle < 2; cycle++ {
		for _, flags := range []struct{ playlist, eq bool }{{false, false}, {true, false}, {true, true}, {false, true}, {true, true}} {
			p.ShowPlaylist(flags.playlist)
			p.ShowEqualiser(flags.eq)
			pump()
			width, height, mainX := 346, 344, 0
			if flags.playlist {
				width += 250
				mainX = 250
			}
			if flags.eq {
				height += 124
			}
			if os.Getenv("MUSIC_PLAYER_PROBE_TRACE") == "1" {
				lw, lh := p.Main.Size()
				pw, ph := p.Main.PixelSize()
				fmt.Printf("scale=%g drawers=%t/%t requested=%dx%d logical=%dx%d device=%dx%d cachedBase=%+v\n", scale, flags.playlist, flags.eq, width, height, lw, lh, pw, ph, p.CaptureState().Sizes["full"])
			}
			verify(p.Main, nil, scale, width, height, mainX+170, 120, mainX+220, 150, true)
			verifyLimits(p.Main, scale, width, height, 0, 0)
			assertPosition()
		}
		p.Main.SetSize(700, 524)
		pump()
		p.SetCompact(true)
		pump()
		verify(p.Main, nil, scale, 346, 103, 180, 70, 220, 75, true)
		verifyLimits(p.Main, scale, 346, 103, 346, 103)
		assertPosition()
		if !p.Host.Model.EqualizerVisible || !p.Host.Model.PlaylistVisible {
			panic("compact discarded remembered drawers")
		}
		p.SetCompact(false)
		pump()
		verify(p.Main, nil, scale, 700, 524, 420, 120, 470, 150, true)
		verifyLimits(p.Main, scale, 596, 468, 0, 0)
		assertPosition()
		if size := p.CaptureState().Sizes["full"]; size != (players.PresentationSize{Width: 450, Height: 400}) {
			panic(fmt.Sprintf("base cache changed:%+v", size))
		}
		p.Main.SetSize(596, 468)
		pump()
	}
	fmt.Printf("scale=%g fullCompactRoundTrips=true stableNativeMain=true stablePosition=%d,%d drawerLimits=true originalBaseCache=true hiddenOnlyOwnedWindow=true\n", scale, x, y)
}

func probeStartup(scale float32, compact bool) {
	a := ui.New(ui.Options{Backend: "x11", Scale: scale, DisableLookWatch: true})
	p, err := series9.New(players.NewHost(a), series9.Options{Scale: scale})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Main.Hide()
	p.Main.SetTitle("Series 9 native startup probe")
	mode, width, height := "full", 700, 524
	if compact {
		mode, width, height = "compact", 346, 103
	}
	p.RestoreState(players.PresentationState{Mode: mode, Sizes: map[string]players.PresentationSize{
		"full": {Width: 450, Height: 400}, "compact": {Width: 346, Height: 103}}})
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {X: 170, Y: 210, Positioned: true, Width: width, Height: height}})
	start, previous := time.Now(), time.Now()
	for steps := 0; steps < 8 || time.Since(start) < 600*time.Millisecond; steps++ {
		a.PumpOnce()
		now := time.Now()
		p.Advance(now.Sub(previous))
		previous = now
		time.Sleep(20 * time.Millisecond)
	}
	insideX, insideY := 420, 120
	if compact {
		insideX, insideY = 180, 70
	}
	verify(p.Main, nil, scale, width, height, insideX, insideY, insideX+20, insideY+5, true)
	if compact {
		verifyLimits(p.Main, scale, 346, 103, 346, 103)
	} else {
		verifyLimits(p.Main, scale, 596, 468, 0, 0)
	}
	if x, y, known := p.Main.Position(); !known || x != 170 || y != 210 {
		panic(fmt.Sprintf("startup placement changed:%d,%d known%t", x, y, known))
	}
	if got := p.CaptureState(); got.Mode != mode || got.Sizes["full"] != (players.PresentationSize{Width: 450, Height: 400}) {
		panic(fmt.Sprintf("startup base state changed:%+v", got))
	}
	fmt.Printf("scale=%g startupMode=%s stateBeforePlacement=true actualNativeSize=true savedPosition=true preservedBase=true\n", scale, mode)
}
func nativeID(w *ui.Window) uint64 {
	parent := w.PortalParent()
	if !strings.HasPrefix(parent, "x11:") {
		panic("native X11 surface unavailable: " + parent)
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(parent, "x11:"), 16, 64)
	if err != nil {
		panic(err)
	}
	return id
}

func verify(w, owner *ui.Window, scale float32, width, height, insideX, insideY, extraX, extraY int, extraInside bool) {
	px := func(v int) int { return int(math.Round(float64(v) * float64(scale))) }
	id := nativeID(w)
	frame := platform.FrameOf(w.Surface()).Frame()
	wanted := make([]C.XRectangle, len(frame.Shape))
	extentX, extentY := 0, 0
	for i, r := range frame.Shape {
		wanted[i] = C.XRectangle{x: C.short(r.X), y: C.short(r.Y), width: C.ushort(r.W), height: C.ushort(r.H)}
		extentX, extentY = max(extentX, r.X+r.W), max(extentY, r.Y+r.H)
	}
	if len(wanted) == 0 {
		panic("public frame has no shape")
	}
	n := C.read_window(C.ulong(id), C.int(px(insideX)), C.int(px(insideY)), C.int(px(extraX)), C.int(px(extraY)), &wanted[0], C.int(len(wanted)))
	image := w.CaptureSurface()
	if n.valid == 0 || int(n.width) != px(width) || int(n.height) != px(height) || n.depth != 32 || n.mapped != 0 || w.Visible() || !w.ShapeActive() || !frame.Alpha {
		panic(fmt.Sprintf("native geometry/alpha mismatch scale=%g %dx%d native=%dx%d depth=%d mapped=%d alpha=%t shape=%t", scale, width, height, n.width, n.height, n.depth, n.mapped, frame.Alpha, w.ShapeActive()))
	}
	for _, region := range []struct {
		name string
		data C.NativeShape
	}{{"bounding", n.bounding}, {"input", n.input}} {
		s := region.data
		// Resizable silhouettes retain an invisible outer resize band. The
		// frame publishes that native region; the visual alpha remains cut.
		if s.count < 1 || (!w.Resizable() && (s.count <= 1 || s.corner != 0 || s.far_corner != 0)) || s.inside != 1 || (s.outside_old == 1) != extraInside || int(s.max_x) != extentX || int(s.max_y) != extentY {
			panic(fmt.Sprintf("native %s mismatch scale=%g %dx%d count=%d corners=%d/%d inside=%d extra=%d extent=%dx%d", region.name, scale, width, height, s.count, s.corner, s.far_corner, s.inside, s.outside_old, s.max_x, s.max_y))
		}
	}
	if n.bounding_matches != 1 || n.input_matches != 1 {
		panic(fmt.Sprintf("native regions differ from public Frame.Shape: bounding=%d input=%d", n.bounding_matches, n.input_matches))
	}
	visual := w.Shape().Raster(px(width), px(height))
	if visual.Clamped || len(visual.Rects) <= 1 || visual.Contains(0, 0) || visual.Contains(px(width)-1, px(height)-1) || !visual.Contains(px(insideX), px(insideY)) || visual.Contains(px(extraX), px(extraY)) != extraInside {
		panic(fmt.Sprintf("visual silhouette mismatch: count%d clamped%t corners%t/%t body%t extra%t", len(visual.Rects), visual.Clamped, visual.Contains(0, 0), visual.Contains(px(width)-1, px(height)-1), visual.Contains(px(insideX), px(insideY)), visual.Contains(px(extraX), px(extraY))))
	}
	if image == nil || image.NRGBAAt(0, 0).A != 0 || image.NRGBAAt(px(width)-1, px(height)-1).A != 0 || image.NRGBAAt(px(insideX), px(insideY)).A == 0 {
		panic(fmt.Sprintf("rendered transparent corners/body do not match native regions: corner%v far%v core%v", image.NRGBAAt(0, 0), image.NRGBAAt(px(width)-1, px(height)-1), image.NRGBAAt(px(insideX), px(insideY))))
	}
	policy := "main"
	if owner != nil {
		_, publicRoleAPI := w.Surface().(platform.RoleSurface)
		if !publicRoleAPI || !platform.RoleUtility.SkipsTaskbar() || n.utility != 1 || n.skip_taskbar != 1 || n.skip_pager != 1 || uint64(n.owner) != nativeID(owner) {
			panic(fmt.Sprintf("utility policy mismatch publicRoleAPI=%t nativeUtility=%d skipTaskbar=%d skipPager=%d nativeOwner=%x publicOwner=%x", publicRoleAPI, n.utility, n.skip_taskbar, n.skip_pager, n.owner, nativeID(owner)))
		}
		policy = "utility+skip-taskbar+skip-pager+current-owner"
	} else if n.skip_taskbar != 0 {
		panic("main window unexpectedly excluded from taskbar")
	}
	fmt.Printf("scale=%g size=%dx%d native=%dx%d visualRects=%d bounding=%d input=%d nativeMatchesFrame=true ARGB32=true transparentCorners=true bodyInput=true extraCoverageExpected=%t hidden=true compositor=%t policy=%s\n", scale, width, height, n.width, n.height, len(visual.Rects), n.bounding.count, n.input.count, extraInside, n.compositor == 1, policy)
}

func verifyLimits(w *ui.Window, scale float32, minW, minH, maxW, maxH int) {
	frame := platform.FrameOf(w.Surface()).Frame()
	wanted := make([]C.XRectangle, len(frame.Shape))
	for i, r := range frame.Shape {
		wanted[i] = C.XRectangle{x: C.short(r.X), y: C.short(r.Y), width: C.ushort(r.W), height: C.ushort(r.H)}
	}
	n := C.read_window(C.ulong(nativeID(w)), 0, 0, 0, 0, &wanted[0], C.int(len(wanted)))
	px := func(v int) int { return int(math.Round(float64(v) * float64(scale))) }
	if n.hint_flags&C.PMinSize == 0 || int(n.min_width) != px(minW) || int(n.min_height) != px(minH) {
		panic(fmt.Sprintf("native minimum hints changed: actual%d,%d expected%d,%d", n.min_width, n.min_height, px(minW), px(minH)))
	}
	if maxW == 0 && maxH == 0 {
		if n.hint_flags&C.PMaxSize != 0 {
			panic("full player retained MiniPlayer maximum")
		}
	} else if n.hint_flags&C.PMaxSize == 0 || int(n.max_width) != px(maxW) || int(n.max_height) != px(maxH) {
		panic(fmt.Sprintf("native maximum hints changed: actual%d,%d expected%d,%d", n.max_width, n.max_height, px(maxW), px(maxH)))
	}
	fmt.Printf("scale=%g nativeLimits=minimum%dx%d,maximum%dx%d\n", scale, n.min_width, n.min_height, n.max_width, n.max_height)
}
