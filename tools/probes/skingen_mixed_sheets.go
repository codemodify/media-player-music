//go:build ignore

// Standalone UIToolkit mixed-sheet generator reproduction.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui-skins"
	"goframework.io/graphics-paint2d"
)

func main() {
	dir, err := os.MkdirTemp("", "uitoolkit-skingen-mixed-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	plan := &skingen.Plan{Name: "mixed-sheet-probe", Label: "Mixed sheet probe", Base: "dark"}
	for _, spec := range []struct {
		name      string
		pixelated bool
	}{{"pixel", true}, {"smooth", false}} {
		plan.Sheets = append(plan.Sheets, &skingen.Sheet{Name: spec.name, W: 12, H: 12, Pixelated: spec.pixelated, Cells: []skingen.Cell{{Name: spec.name + ".face", W: 12, H: 12, Draw: func(ctx *paint2d.Context, w, h float32) {
			ctx.DrawRect(paint2d.XYWH(0, 0, w, h), paint2d.Fill(paint2d.RGB(1, 0, 0)))
		}}}})
	}
	if err := skingen.Write(dir, plan); err != nil {
		panic(err)
	}
	fsys := os.DirFS(filepath.Join(dir, plan.Name))
	raw, err := os.ReadFile(filepath.Join(dir, plan.Name, skins.SkinFile))
	if err != nil {
		panic(err)
	}
	var doc struct {
		Design map[string]any            `json:"design"`
		Sheets map[string]map[string]any `json:"sheets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		panic(err)
	}
	sk, err := skins.LoadSkinFS(fsys, plan.Name)
	if err != nil {
		panic(err)
	}
	fmt.Printf("generated design.pixelated=%v\n", doc.Design["pixelated"])
	for _, sheet := range plan.Sheets {
		value, present := doc.Sheets[sheet.Name]["pixelated"]
		fmt.Printf("sheet=%s requestedPixelated=%t manifestPixelatedPresent=%t manifestValue=%v decodedPixelated=%t\n", sheet.Name, sheet.Pixelated, present, value, sk.Sheets[sheet.Name].Pixelated)
	}
}
