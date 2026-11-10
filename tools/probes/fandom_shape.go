//go:build ignore

// Fandom reference presentation with read-only native X11 SHAPE and owner checks.
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
	"github.com/codemodify/media-player-music/internal/players/fandom"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	uiskins "goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		panic("a native X11 DISPLAY is required")
	}
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.Setenv(style.ThemeEnv, players.FandomSkin); err != nil {
		panic(err)
	}
	for _, scale := range []float32{1, 1.75} {
		probe(scale)
	}
}

func probe(scale float32) {
	a := ui.New(ui.Options{Backend: "x11", Scale: scale, DisableLookWatch: true})
	p, err := fandom.New(players.NewHost(a), fandom.Options{Scale: scale})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Main().Hide()
	p.List().Hide()
	p.Main().SetTitle("Fandom native shape probe (main)")
	p.List().SetTitle("Fandom native shape probe (playlist)")
	pump := func() {
		until := time.Now().Add(250 * time.Millisecond)
		for time.Now().Before(until) {
			a.PumpOnce()
			p.Advance(0)
			time.Sleep(5 * time.Millisecond)
		}
	}
	pump()
	for _, visible := range []bool{true, false, true} {
		p.ShowSettingsPanel(visible)
		pump()
		verify(p.Main(), nil, scale, 490, 330, 300, 110, 61, 110, visible)
		verify(p.List(), p.Main(), scale, 190, 246, 95, 130, 110, 200, true)
		px := func(v int) int { return int(math.Round(float64(v) * float64(scale))) }
		frame := platform.FrameOf(p.Main().Surface()).Frame()
		wanted := make([]C.XRectangle, len(frame.Shape))
		for i, r := range frame.Shape {
			wanted[i] = C.XRectangle{x: C.short(r.X), y: C.short(r.Y), width: C.ushort(r.W), height: C.ushort(r.H)}
		}
		n := C.read_window(C.ulong(nativeID(p.Main())), C.int(px(300)), C.int(px(110)), C.int(px(126)), C.int(px(110)), &wanted[0], C.int(len(wanted)))
		if n.bounding.outside_old != 0 || n.input.outside_old != 0 || p.Main().CaptureSurface().NRGBAAt(px(126), px(110)).A != 0 {
			panic("empty settings/chassis gap is painted or accepts native input")
		}
		fmt.Printf("scale=%g settingsVisible=%t nativeWingCoverage=true gapInput=false currentUtilityOwner=true\n", scale, visible)
	}
	p.List().SetSize(310, 356)
	pump()
	verify(p.List(), p.Main(), scale, 310, 356, 155, 178, 270, 300, true)
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
		if s.count < 1 || (!w.Resizable() && (s.count <= 10 || s.corner != 0 || s.far_corner != 0)) || s.inside != 1 || (s.outside_old == 1) != extraInside || int(s.max_x) != extentX || int(s.max_y) != extentY {
			panic(fmt.Sprintf("native %s mismatch scale=%g %dx%d count=%d corners=%d/%d inside=%d extra=%d extent=%dx%d", region.name, scale, width, height, s.count, s.corner, s.far_corner, s.inside, s.outside_old, s.max_x, s.max_y))
		}
	}
	if n.bounding_matches != 1 || n.input_matches != 1 {
		panic(fmt.Sprintf("native regions differ from public Frame.Shape: bounding=%d input=%d", n.bounding_matches, n.input_matches))
	}
	visual := w.Shape().Raster(px(width), px(height))
	if visual.Clamped || len(visual.Rects) <= 10 || visual.Contains(0, 0) || visual.Contains(px(width)-1, px(height)-1) || !visual.Contains(px(insideX), px(insideY)) || visual.Contains(px(extraX), px(extraY)) != extraInside {
		panic("visual silhouette lost its original outline or resized body")
	}
	if image == nil || image.NRGBAAt(0, 0).A != 0 || image.NRGBAAt(px(width)-1, px(height)-1).A != 0 || image.NRGBAAt(px(insideX), px(insideY)).A == 0 {
		panic(fmt.Sprintf("rendered transparent corners/body do not match native regions: corner%v far%v core%v", image.NRGBAAt(0, 0), image.NRGBAAt(px(width)-1, px(height)-1), image.NRGBAAt(px(insideX), px(insideY))))
	}
	if owner == nil {
		verifySoftAlpha(w, image, extraInside)
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

// Match partially covered source edges after actual native rendering. A binary
// window extent must not flatten or apply the source artwork alpha twice.
func verifySoftAlpha(w *ui.Window, capture *paint2d.Image, settingsVisible bool) {
	pw, ph := w.PixelSize()
	raw := paint2d.NewImage(pw, ph)
	ctx := paint2d.NewContext(raw)
	ctx.Clear(paint2d.Transparent)
	box := paint2d.XYWH(0, 0, float32(pw), float32(ph))
	slots := []string{"shape.main"}
	if settingsVisible {
		slots = append(slots, "shape.settings")
	}
	for _, slot := range slots {
		r, ok := uiskins.SkinSlotRect(w.Look(), fandom.LayoutMain, slot, box)
		if !ok || !uiskins.DrawSkinSlot(w.Look(), ctx, r, fandom.LayoutMain, slot, 0) {
			panic("native soft-alpha fixture art missing")
		}
	}
	var overlays []paint2d.Rect
	for _, slot := range []string{"header", "settings.title", "metadata", "status", "status.eq", "analyser", "eq.balance.label"} {
		if r, ok := uiskins.SkinSlotRect(w.Look(), fandom.LayoutMain, slot, box); ok {
			overlays = append(overlays, r)
		}
	}
	checked := 0
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			alpha := raw.Pix[(y*pw+x)*4+3]
			if alpha == 0 || alpha == 255 {
				continue
			}
			at := paint2d.Pt(float32(x)+.5, float32(y)+.5)
			covered := false
			for _, r := range overlays {
				if r.Contains(at) {
					covered = true
					break
				}
			}
			for _, c := range w.Content().Children() {
				if c.Visible() && widget.DeviceBounds(c).Contains(at) {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			got := capture.NRGBAAt(x, y).A
			diff := int(got) - int(alpha)
			if diff < -2 || diff > 2 {
				panic(fmt.Sprintf("native source alpha not preserved at %d,%d: actual%d original%d", x, y, got, alpha))
			}
			checked++
		}
	}
	if checked < 10 {
		panic("native soft-alpha fixture has no uncovered fractional edges")
	}
	fmt.Printf("nativeSoftAlphaPreserved=true sampledEdges=%d\n", checked)
}
