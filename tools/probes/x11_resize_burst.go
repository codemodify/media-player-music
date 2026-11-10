//go:build ignore

// Public native X11 resize-burst reproduction; no player/controller code.
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
	"os"
	"strconv"
	"strings"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		panic("native X11 DISPLAY required")
	}
	os.Setenv(style.ThemeEnv, "dark")
	a := ui.New(ui.Options{Backend: "x11", Scale: 1, DisableLookWatch: true})
	w, err := a.NewWindow(platform.WindowOptions{Title: "desktop-ui public resize burst probe", Width: 596, Height: 468, Decorations: platform.DecorationsNone})
	if err != nil {
		panic(err)
	}
	defer w.Close()
	w.Hide()
	w.SetContent(widgets.NewLabel("Hidden public resize probe"))
	shape := func(size paint2d.Point, _ float32) *platform.Shape {
		width, height := int(size.X+.5), int(size.Y+.5)
		mask := make([]byte, width*height)
		for at := range mask {
			mask[at] = 255
		}
		mask[0] = 0
		mask[len(mask)-1] = 0
		return platform.NewShapeMask(mask, width, height, width)
	}
	w.SetShapeFunc(shape)
	remove := w.OnResize(func(width, height int) { fmt.Printf("observed=%dx%d\n", width, height) })
	defer remove()
	pump := func() {
		start := time.Now()
		for steps := 0; steps < 8 || time.Since(start) < 400*time.Millisecond; steps++ {
			a.PumpOnce()
			time.Sleep(10 * time.Millisecond)
		}
	}
	pump()
	request := func(width, height int) {
		w.SetResizable(true)
		w.Surface().(platform.SizeLimitSurface).SetSizeLimits(platform.SizeLimits{MinWidth: width, MinHeight: height})
		w.SetSize(width, height)
		w.SetShapeFunc(shape)
	}
	for cycle := 0; cycle < 4; cycle++ {
		request(346, 468)
		request(346, 344)
		pump()
		check(w, 346, 344)
		request(596, 344)
		pump()
		check(w, 596, 344)
		request(596, 468)
		pump()
		check(w, 596, 468)
	}
	fmt.Println("resizeBurstSettled=true")
}
func check(w interface {
	PortalParent() string
	Size() (int, int)
	Surface() platform.Surface
}, width, height int) {
	id, err := strconv.ParseUint(strings.TrimPrefix(w.PortalParent(), "x11:"), 16, 64)
	if err != nil {
		panic(err)
	}
	frame := platform.FrameOf(w.Surface()).Frame()
	wanted := make([]C.XRectangle, len(frame.Shape))
	for i, r := range frame.Shape {
		wanted[i] = C.XRectangle{x: C.short(r.X), y: C.short(r.Y), width: C.ushort(r.W), height: C.ushort(r.H)}
	}
	if len(wanted) == 0 {
		panic("shape missing")
	}
	native := C.read_window(C.ulong(id), 0, 0, 0, 0, &wanted[0], C.int(len(wanted)))
	lw, lh := w.Size()
	sw, sh := w.Surface().Size()
	fmt.Printf("requested=%dx%d logical=%dx%d backing=%dx%d native=%dx%d\n", width, height, lw, lh, sw, sh, native.width, native.height)
	if lw != width || lh != height || int(native.width) != sw || int(native.height) != sh {
		panic("older resize answer replaced the final public request")
	}
}
