//go:build ignore

// Public stock resizable-mask window plus read-only native SHAPE verification.
package main

/*
#cgo pkg-config: x11 xext
#include <X11/Xlib.h>
#include <X11/extensions/shape.h>
#include <stdlib.h>
static int errors;
static int error_count() { return errors; }
static int note_error(Display *d, XErrorEvent *e) { errors++; return 0; }
static int shape_count(unsigned long id, int kind) {
  Display *d=XOpenDisplay(NULL); if(!d)return -1;
  XSetErrorHandler(note_error);
  int n=0, order=0;
  XRectangle *r=XShapeGetRectangles(d,id,kind,&n,&order);
  if(r)XFree(r);
  XSync(d,False); XCloseDisplay(d); return n;
}
*/
import "C"

import (
	"fmt"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widgets"
	"math"
	"strconv"
	"strings"
	"time"
)

func main() {
	for _, scale := range []float32{1, 1.75} {
		a := app.New(app.Options{Backend: "x11", Scale: scale, Look: style.DarkLook(), DisableLookWatch: true})
		w, e := a.NewWindow(platform.WindowOptions{Title: "shape audit", Width: 300, Height: 200, Decorations: platform.DecorationsClient})
		if e != nil {
			panic(e)
		}
		w.SetContent(widgets.NewLabel("shape"))
		w.SetShapeFunc(func(size paintengine2d.Point, _ float32) *platform.Shape {
			width, height := int(size.X), int(size.Y)
			mask := make([]uint8, width*height)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					if math.Pow((float64(x)+.5-float64(width)/2)/(float64(width)/2-10), 2)+math.Pow((float64(y)+.5-float64(height)/2)/(float64(height)/2-10), 2) <= 1 {
						mask[y*width+x] = 255
					}
				}
			}
			return platform.NewShapeMask(mask, width, height, width)
		})
		pump := func() {
			until := time.Now().Add(250 * time.Millisecond)
			for time.Now().Before(until) {
				a.PumpOnce()
				time.Sleep(5 * time.Millisecond)
			}
		}
		for _, size := range [][2]int{{300, 200}, {450, 320}} {
			w.SetSize(size[0], size[1])
			pump()
			frame := platform.FrameOf(w.Surface()).Frame()
			backtrack := false
			for i := 1; i < len(frame.Shape); i++ {
				if frame.Shape[i].Y < frame.Shape[i-1].Y {
					backtrack = true
				}
			}
			id, e := strconv.ParseUint(strings.TrimPrefix(w.PortalParent(), "x11:"), 16, 64)
			if e != nil {
				panic(e)
			}
			fmt.Printf("scale=%g size=%v frameRects=%d backtrack=%t nativeBoundingRects=%d nativeInputRects=%d queryErrors=%d\n", scale, size, len(frame.Shape), backtrack, int(C.shape_count(C.ulong(id), C.ShapeBounding)), int(C.shape_count(C.ulong(id), C.ShapeInput)), int(C.error_count()))
		}
		w.Close()
	}
}
