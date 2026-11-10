//go:build ignore

// Public SNI property/lifecycle audit. Run with a private session bus:
// tools/testenv.sh go run tools/probes/status_icon_priority.go
package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"goframework.io/desktop-ui/platform"
	"goframework.io/graphics-paint2d"
	dbus "goframework.io/system-dbus"
)

func main() {
	red := paint2d.NewImage(32, 32)
	red.Clear(paint2d.RGB(1, 0, 0))
	item, err := platform.NewStatusItem(platform.StatusItemOptions{ID: "raster-icon-probe", Icon: platform.StatusIcon{Image: red}})
	if err != nil {
		panic(err)
	}
	defer item.Close()
	ctx := context.Background()
	conn, err := dbus.ConnectSessionBus(ctx)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	name := fmt.Sprintf("org.freedesktop.StatusNotifierItem-%d-1", os.Getpid())
	obj := conn.Object(name, "/StatusNotifierItem")
	read := func(stage string) {
		props, err := obj.GetAllProperties(ctx, "org.kde.StatusNotifierItem")
		if err != nil {
			panic(err)
		}
		var pix []struct {
			W, H int32
			Data []byte
		}
		if err := props["IconPixmap"].Store(&pix); err != nil {
			panic(err)
		}
		fmt.Printf("%s backend=%s shown=%t IconName=%q", stage, item.Backend(), item.Shown(), props["IconName"].Value())
		for _, p := range pix {
			fmt.Printf(" pixmap=%dx%d bytes=%d firstARGB=%v", p.W, p.H, len(p.Data), p.Data[:4])
		}
		fmt.Println()
	}
	read("Image-only initial")
	green := paint2d.NewImage(48, 48)
	green.Clear(paint2d.RGB(0, 1, 0))
	if err := item.SetIcon(platform.StatusIcon{Image: green}); err != nil {
		panic(err)
	}
	read("Image-only SetIcon")
	if err := item.SetIcon(platform.StatusIcon{Image: green, Name: "explicit-themed-icon"}); err != nil {
		panic(err)
	}
	read("Image plus explicit Name")
	dir, err := os.MkdirTemp("", "sni-picture-probe-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "icon.png")
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	im := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			im.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	if err := png.Encode(f, im); err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	if err := item.SetIcon(platform.StatusIcon{Path: path}); err != nil {
		panic(err)
	}
	read("Path-only SetIcon")
	if err := item.Close(); err != nil {
		panic(err)
	}
	owner, err := conn.NameHasOwner(ctx, name)
	if err != nil {
		panic(err)
	}
	fmt.Printf("after Close service-owned=%t\n", owner)
}
