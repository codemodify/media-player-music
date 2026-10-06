//go:build ignore

// Public native Wayland utility/group capability audit; isolated compositor only.
package main

import (
	"fmt"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"time"
)

func main() {
	a := app.New(app.Options{Backend: "wayland", Look: style.DarkLook(), DisableLookWatch: true})
	main, e := a.NewWindow(platform.WindowOptions{Title: "Wayland group probe", Width: 300, Height: 120})
	if e != nil {
		panic(e)
	}
	defer main.Close()
	child, e := a.NewWindow(platform.WindowOptions{Title: "Wayland utility probe", Width: 300, Height: 150, Role: platform.RoleUtility, Owner: main.Surface()})
	if e != nil {
		panic(e)
	}
	defer child.Close()
	until := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(until) {
		a.PumpOnce()
		time.Sleep(5 * time.Millisecond)
	}
	d := rack.NewDesk(10)
	m, c := d.Add("main", main), d.Add("child", child)
	d.Attach(c, m, rack.SideBottom)
	x, y, known := child.Position()
	fmt.Printf("childSurface=%T places=%t move=%t position=(%d,%d,%t) setOwner=%t setSkipTaskbar=%t ownerCap=%t skipCap=%t logicalRack=%+v\n", child.Surface(), d.Places(), child.Move(400, 200), x, y, known, child.SetOwner(main), child.SetSkipTaskbar(true), child.FrameCaps().Has(platform.FrameOwner), child.FrameCaps().Has(platform.FrameSkipTaskbar), d.Rack.Boxes())
}
