//go:build ignore

// Public GPU cleanup lifetime/routing probe; isolated X11 display only.
// It opens only its own small toolkit windows, never the player.
package main

/*
#cgo pkg-config: egl glesv2
#include <EGL/egl.h>
#include <GLES2/gl2.h>
#include <stdint.h>
static uintptr_t current_context(void) { return (uintptr_t)eglGetCurrentContext(); }
static const char *current_renderer(void) { return (const char *)glGetString(GL_RENDERER); }
*/
import "C"

import (
	"fmt"
	"os"
	"time"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d-gpu-gles"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		panic("isolated native DISPLAY required")
	}
	if len(os.Args) > 1 && os.Args[1] == "--during-run" {
		duringRun()
		return
	}
	surfaceClose := len(os.Args) > 1 && os.Args[1] == "--surface-close"
	closeWindow := func(w *ui.Window) {
		if surfaceClose {
			// Independently exercise the live per-surface registration on
			// real backend teardown, then let Window.Close clear its state.
			// This is a diagnostic path, not an application workaround.
			_ = w.Surface().Close()
		}
		w.Close()
		w.Close() // A repeated application close must not notify twice.
	}
	if surfaceClose {
		fmt.Println("diagnostic path: Surface.Close followed by Window.Close")
	}
	a := ui.New(ui.Options{Backend: "x11", Scale: 1, Look: style.DarkLook(), DisableLookWatch: true})
	open := func(title string) *ui.Window {
		w, err := a.NewWindow(platform.WindowOptions{Title: title, Width: 160, Height: 90, Decorations: platform.DecorationsNone})
		if err != nil {
			panic(err)
		}
		w.SetContent(widgets.NewLabel(title))
		return w
	}
	one, two := open("GPU closing control"), open("GPU closing routing")
	postRun := open("GPU closing after Run")
	defer one.Close()
	defer two.Close()
	defer postRun.Close()
	until := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(until) {
		a.PumpOnce()
		time.Sleep(2 * time.Millisecond)
	}
	if !one.UsesGPU() || !two.UsesGPU() || !postRun.UsesGPU() {
		fmt.Printf("native GPU unavailable: window1=%t window2=%t postRun=%t; cleanup callback test skipped\n", one.UsesGPU(), two.UsesGPU(), postRun.UsesGPU())
		return
	}
	device := one.PaintDevice().(*gles.Device)
	_, wantContext, _ := device.EGLHandles()
	_, secondContext, _ := two.PaintDevice().(*gles.Device).EGLHandles()
	_, postContext, _ := postRun.PaintDevice().(*gles.Device).EGLHandles()
	if wantContext == 0 || secondContext == 0 || postContext == 0 {
		panic("GPU control has no live EGL context")
	}
	first, second := 0, 0
	contextMatches, secondMatches, postMatches := false, false, false
	one.OnPaintDeviceClosing(func() {
		first++
		contextMatches = uintptr(C.current_context()) == wantContext
	})
	two.OnPaintDeviceClosing(func() {
		second++
		secondMatches = uintptr(C.current_context()) == secondContext
	})
	// Deliberately leave the OTHER window's context current. No Run is used:
	// teardown must bind its own device and notify pumping applications.
	if err := two.PaintDevice().(*gles.Device).MakeCurrent(); err != nil {
		panic(err)
	}
	fmt.Printf("native GPU/EGL devices active; renderer=%s\n", C.GoString(C.current_renderer()))
	closeWindow(one)
	fmt.Printf("single Application/pump control: callback=%d want=1 ownContextCurrent=%t\n", first, contextMatches)
	afterRun := 0
	postRun.OnPaintDeviceClosing(func() {
		afterRun++
		postMatches = uintptr(C.current_context()) == postContext
	})
	a.Post(a.Quit)
	if err := a.Run(); err != nil {
		panic(err)
	}
	closeWindow(postRun)
	fmt.Printf("single Application after Run returns: callback=%d want=1 ownContextCurrent=%t\n", afterRun, postMatches)
	// Constructing an unrelated headless Application must not take away the
	// first application's live GPU window's cleanup notification.
	_ = ui.New(ui.Options{Headless: true, Scale: 1, Look: style.DarkLook(), DisableLookWatch: true})
	closeWindow(two)
	fmt.Printf("existing GPU window after second Application.New: callback=%d want=1 ownContextCurrent=%t\n", second, secondMatches)
	// Report every lifecycle control before asserting, so an early teardown
	// failure cannot conceal the independent post-Run and routing results.
	if first != 1 || !contextMatches || afterRun != 1 || !postMatches || second != 1 || !secondMatches {
		panic("GPU cleanup positive controls failed")
	}
}

// Isolate the constructor's routing effect while the original run loop stays
// active. Unlike pump-only cleanup, this was supported by v0.23.6, allowing a
// version-pair control without manually installing any platform hook.
func duringRun() {
	a := ui.New(ui.Options{Backend: "x11", Scale: 1, Look: style.DarkLook(), DisableLookWatch: true})
	open := func(title string) *ui.Window {
		w, err := a.NewWindow(platform.WindowOptions{Title: title, Width: 160, Height: 90, Decorations: platform.DecorationsNone})
		if err != nil {
			panic(err)
		}
		w.SetContent(widgets.NewLabel(title))
		return w
	}
	one, two := open("GPU running control"), open("GPU running routing")
	defer one.Close()
	defer two.Close()
	until := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(until) {
		a.PumpOnce()
		time.Sleep(2 * time.Millisecond)
	}
	if !one.UsesGPU() || !two.UsesGPU() {
		fmt.Printf("native GPU unavailable: window1=%t window2=%t; running routing test skipped\n", one.UsesGPU(), two.UsesGPU())
		return
	}
	first, second := 0, 0
	_, firstContext, _ := one.PaintDevice().(*gles.Device).EGLHandles()
	_, secondContext, _ := two.PaintDevice().(*gles.Device).EGLHandles()
	if firstContext == 0 || secondContext == 0 {
		panic("running GPU control has no live EGL context")
	}
	firstMatches, secondMatches := false, false
	if err := two.PaintDevice().(*gles.Device).MakeCurrent(); err != nil {
		panic(err)
	}
	fmt.Printf("native GPU/EGL devices active; renderer=%s\n", C.GoString(C.current_renderer()))
	one.OnPaintDeviceClosing(func() {
		first++
		firstMatches = uintptr(C.current_context()) == firstContext
	})
	two.OnPaintDeviceClosing(func() {
		second++
		secondMatches = uintptr(C.current_context()) == secondContext
	})
	a.Post(func() {
		one.Close()
		one.Close()
		_ = ui.New(ui.Options{Headless: true, Scale: 1, Look: style.DarkLook(), DisableLookWatch: true})
		two.Close()
		two.Close()
		a.Quit()
	})
	if err := a.Run(); err != nil {
		panic(err)
	}
	fmt.Printf("while first Application.Run remains active: beforeSecondNew=%d afterSecondNew=%d want=1,1 ownContextsCurrent=%t,%t\n", first, second, firstMatches, secondMatches)
	if first != 1 || second != 1 || !firstMatches || !secondMatches {
		panic("GPU cleanup during Run did not notify each owning window exactly once")
	}
}
