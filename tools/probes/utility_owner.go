//go:build ignore

// Public native surface utility-policy and owner-recreation reproduction.
package main

import (
	"fmt"
	"github.com/codemodify/uitoolkit/platform"
	"os/exec"
	"strings"
	"time"
)

func main() {
	b := platform.Select("x11", false)
	parent, e := b.NewSurface(platform.WindowOptions{Title: "owner probe", Width: 300, Height: 200, Scale: 1.75})
	if e != nil {
		panic(e)
	}
	defer parent.Close()
	child, e := b.NewSurface(platform.WindowOptions{Title: "utility probe", Width: 200, Height: 120, Scale: 1.75, Role: platform.RoleUtility, Owner: parent})
	if e != nil {
		panic(e)
	}
	defer child.Close()
	id := func(s platform.Surface) string { return s.(platform.PortalParenter).PortalParent() }
	pump := func() {
		until := time.Now().Add(250 * time.Millisecond)
		for time.Now().Before(until) {
			parent.Poll()
			child.Poll()
			time.Sleep(5 * time.Millisecond)
		}
	}
	parent.Present(nil)
	child.Present(nil)
	pump()
	props := func(stage string) {
		data, e := exec.Command("xprop", "-id", "0x"+strings.TrimPrefix(id(child), "x11:"), "_NET_WM_WINDOW_TYPE", "_NET_WM_STATE", "WM_TRANSIENT_FOR").CombinedOutput()
		fmt.Printf("%s parent=%s child=%s error=%v\n%s", stage, id(parent), id(child), e, data)
	}
	props("initial")
	child.(platform.WindowGeometry).Hide()
	pump()
	child.(platform.WindowGeometry).Show()
	pump()
	props("same-ID hide/show")
	f := parent.(platform.WindowFrame)
	frame := f.Frame()
	frame.Alpha = true
	f.SetFrame(frame)
	pump()
	props("parent-only visual recreation")
	child.(platform.OwnedSurface).SetOwner(parent)
	pump()
	props("explicit same-owner refresh")
	f = child.(platform.WindowFrame)
	frame = f.Frame()
	frame.Alpha = true
	f.SetFrame(frame)
	pump()
	props("child visual recreation")
}
