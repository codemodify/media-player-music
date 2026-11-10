//go:build ignore

// Half Life 2 public presentation with read-only native X11 SHAPE and owner checks.
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
 int bounding_matches, input_matches;
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
	"github.com/codemodify/media-player-music/internal/players/halflife2"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		panic("a native X11 DISPLAY is required")
	}
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.Setenv(style.ThemeEnv, players.HalfLife2Skin); err != nil {
		panic(err)
	}
	for _, scale := range []float32{1, 1.75} {
		probe(scale)
	}
}

func probe(scale float32) {
	a := ui.New(ui.Options{Backend: "x11", Scale: scale, DisableLookWatch: true})
	p, err := halflife2.New(players.NewHost(a), halflife2.Options{Scale: scale})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Main().Hide()
	p.Eq().Hide()
	p.List().Hide()
	p.Main().SetTitle("Half Life 2 native shape probe (main)")
	p.Eq().SetTitle("Half Life 2 native shape probe (equalizer)")
	p.List().SetTitle("Half Life 2 native shape probe (playlist)")
	pump := func() {
		until := time.Now().Add(250 * time.Millisecond)
		for time.Now().Before(until) {
			a.PumpOnce()
			p.Advance(0)
			time.Sleep(5 * time.Millisecond)
		}
	}
	pump()
	verify(p.Main(), nil, scale, 339, 300, 150, 150, 200, 120)
	verify(p.Eq(), p.Main(), scale, 430, 239, 210, 120, 350, 180)
	verify(p.List(), p.Main(), scale, 393, 226, 200, 100, 260, 160)
	p.List().SetSize(610, 430)
	pump()
	verify(p.List(), p.Main(), scale, 610, 430, 305, 215, 500, 350)
	for state := 0; state < 3; state++ {
		var id uint64
		p.Main().AccessibleTree().Walk(func(n *a11y.Node) bool {
			if strings.Contains(n.Name, "shutter") {
				id = n.ID
			}
			return true
		})
		if !p.Main().AccessibleAction(id, a11y.ActionDefault) {
			panic("shutter action unavailable")
		}
		pump()
		verify(p.Main(), nil, scale, 339, 300, 150, 150, 200, 120)
		verify(p.Eq(), p.Main(), scale, 430, 239, 210, 120, 350, 180)
		verify(p.List(), p.Main(), scale, 610, 430, 305, 215, 500, 350)
		fmt.Printf("scale=%g shutterState=%d currentUtilityOwners=true\n", scale, (state+1)%3)
	}
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

func verify(w, owner *ui.Window, scale float32, width, height, insideX, insideY, extraX, extraY int) {
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
		if s.count < 1 || (!w.Resizable() && (s.count <= 10 || s.corner != 0 || s.far_corner != 0)) || s.inside != 1 || s.outside_old != 1 || int(s.max_x) != extentX || int(s.max_y) != extentY {
			panic(fmt.Sprintf("native %s mismatch scale=%g %dx%d count=%d corners=%d/%d inside=%d extra=%d extent=%dx%d", region.name, scale, width, height, s.count, s.corner, s.far_corner, s.inside, s.outside_old, s.max_x, s.max_y))
		}
	}
	if n.bounding_matches != 1 || n.input_matches != 1 {
		panic(fmt.Sprintf("native regions differ from public Frame.Shape: bounding=%d input=%d", n.bounding_matches, n.input_matches))
	}
	visual := w.Shape().Raster(px(width), px(height))
	if visual.Clamped || len(visual.Rects) <= 10 || visual.Contains(0, 0) || visual.Contains(px(width)-1, px(height)-1) || !visual.Contains(px(insideX), px(insideY)) || !visual.Contains(px(extraX), px(extraY)) {
		panic("visual silhouette lost its original outline or resized body")
	}
	if image == nil || image.NRGBAAt(0, 0).A != 0 || image.NRGBAAt(px(width)-1, px(height)-1).A != 0 || image.NRGBAAt(px(insideX), px(insideY)).A != 255 {
		panic("rendered transparent corners/body do not match native regions")
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
	fmt.Printf("scale=%g size=%dx%d native=%dx%d visualRects=%d bounding=%d input=%d nativeMatchesFrame=true ARGB32=true transparentCorners=true bodyInput=true enlargedInput=true hidden=true compositor=%t policy=%s\n", scale, width, height, n.width, n.height, len(visual.Rects), n.bounding.count, n.input.count, n.compositor == 1, policy)
}
