//go:build ignore

// Public toolkit-only proof that the stock menu bar bypasses its skin part.
// Run: GOCACHE=/home/user/.cache/go-build tools/testenv.sh go run ./tools/probes/menu_bar_skin.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"testing/fstest"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func main() {
	asset := paint2d.NewImage(8, 8)
	paint2d.NewContext(asset).DrawRect(paint2d.XYWH(0, 0, 8, 8), paint2d.Fill(paint2d.RGB(1, 0, 1)))
	var png bytes.Buffer
	if err := asset.WritePNG(&png); err != nil {
		panic(err)
	}
	fsys := fstest.MapFS{
		"skin.json": {Data: []byte(`{"skin":1,"label":"Bar probe","base":"dark","sheets":{"chrome":{"1x":"bar.png"}},"sprites":{"bar":{"at":[0,0,8,8]}},"parts":{"bar":{"states":{"normal":"bar"}}}}`)},
		"bar.png":   {Data: png.Bytes()},
	}
	const id = "public-menu-bar-skin-probe"
	if err := skins.RegisterSkinFS(fsys, id); err != nil {
		panic(err)
	}
	sk, ok := skins.LoadSkin(id)
	if !ok || sk.Parts["bar"] == nil {
		panic("public bar part did not load")
	}
	os.Setenv(style.ThemeEnv, id)
	for _, scale := range []float32{1, 1.75} {
		a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
		w, err := a.NewWindow(platform.WindowOptions{Title: "Bar probe", Width: 240, Height: 60, Headless: true, Decorations: platform.DecorationsNone})
		if err != nil {
			panic(err)
		}
		m := widgets.NewMenuBar(widgets.NewMenu("File", widgets.Item("Open", func() {})))
		w.SetContent(m)
		a.PumpOnce()
		control := paint2d.NewImage(int(240*scale), int(60*scale))
		if !skins.DrawSkinSprite(w.Look(), paint2d.NewContext(control), paint2d.XYWH(0, 0, float32(control.Width), float32(control.Height)), "bar", paint2d.Color{}) {
			panic("public bar sprite did not draw")
		}
		x, y := int(220*scale), int(30*scale)
		fmt.Printf("scale=%g loaded_bar=true sprite=%v actual_stock_MenuBar=%v at=%d,%d\n", scale, control.NRGBAAt(x, y), w.Capture().NRGBAAt(x, y), x, y)
		w.Close()
	}
}
