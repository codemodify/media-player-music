//go:build ignore

package probes_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// Run independently of the player and its skin/widget adapters:
// tools/testenv.sh go test -v tools/probes/platform_primitives_02310_test.go
func TestPublicDigitAccelerators(t *testing.T) {
	for n := 0; n <= 9; n++ {
		r := rune('0' + n)
		key := platform.DigitKey(r)
		if key != platform.Key0+platform.Key(n) || platform.KeyChar(key) != r {
			t.Fatalf("digit %d is not a contiguous printable key: %d", n, key)
		}
		if got, ok := platform.DigitRune(key); !ok || got != r {
			t.Fatalf("digit %d did not round trip: %q %t", n, got, ok)
		}
		for _, prefix := range []string{"Ctrl+", "Ctrl+Shift+", "Alt+", "Cmd+"} {
			shortcut := prefix + string(r)
			got, mods, ok := widgets.ParseAccel(shortcut)
			if !ok || got != key {
				t.Fatalf("%s parsed as %d, %t", shortcut, got, ok)
			}
			calls := 0
			bar := widgets.NewMenuBar(widgets.NewMenu("Go", widgets.ItemAccel("Digit", shortcut, func() { calls++ })))
			if !bar.HandleAccelerator(key, mods|platform.ModCapsLock|platform.ModNumLock) || calls != 1 {
				t.Fatalf("%s did not activate once through MenuBar: calls=%d", shortcut, calls)
			}
			if bar.HandleAccelerator(key, mods^platform.ModAlt) || calls != 1 {
				t.Fatalf("%s also activated with the wrong modifier", shortcut)
			}
		}
	}
	for shortcut, want := range map[string]platform.Key{"Ctrl+S": platform.KeyS, "Ctrl+,": platform.KeyComma, "Ctrl+3": platform.Key3, "Ctrl+#": platform.KeyHash, "Space": platform.KeySpace, "Alt+F4": platform.KeyF4} {
		if key, _, ok := widgets.ParseAccel(shortcut); !ok || key != want {
			t.Fatalf("retained shortcut %q changed: %d, %t", shortcut, key, ok)
		}
	}
	if platform.DigitKey('a') != platform.KeyUnknown {
		t.Fatal("a letter became a digit")
	}
	if _, ok := platform.DigitRune(platform.KeyComma); ok {
		t.Fatal("comma became a digit")
	}
	t.Log("all ten digits, four modifier spellings, MenuBar actions, lock modifiers and retained shortcuts passed")
}

func equalPixels(a, b *paint2d.Image) bool {
	if a.Width != b.Width || a.Height != b.Height {
		return false
	}
	for y := 0; y < a.Height; y++ {
		for x := 0; x < a.Width; x++ {
			ar, ag, ab, aa := a.PremulAt(x, y)
			br, bg, bb, ba := b.PremulAt(x, y)
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}

func inkPixels(img *paint2d.Image) int {
	n := 0
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if _, _, _, a := img.PremulAt(x, y); a > 0 {
				n++
			}
		}
	}
	return n
}

func TestPublicCloudAndToolbarBadges(t *testing.T) {
	// Register the actual dependency's premiere art without copying it into
	// the user directory. This is an independent public rendering check.
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/codemodify/uitoolkit")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	style.AddSearchPath(strings.TrimSpace(string(raw)))
	lk := style.DarkLook()
	for _, scale := range []float32{1, 1.75, 2} {
		render := func(icon style.ToolIcon, set style.IconSetName) *paint2d.Image {
			img := paint2d.NewImage(int(48*scale), int(48*scale))
			ctx := paint2d.NewContext(img)
			ctx.Scale(scale, scale)
			style.DrawToolIcon(ctx, paint2d.XYWH(8, 8, 32, 32), icon, lk.Palette().Text, set)
			img.Touch()
			return img
		}
		for _, set := range []style.IconSetName{style.IconSetClassic, style.IconSetSharp, style.IconSetLucide, style.IconSetPhosphor, style.IconSetTabler, style.IconSetHeroicons, style.IconSetMaterialSymbols} {
			cloud := render(style.IconCloud, set)
			if inkPixels(cloud) == 0 || equalPixels(cloud, render(style.ToolIcon(9999), set)) {
				t.Fatalf("scale %g set %s cloud is empty or the missing-icon image", scale, set)
			}
		}
		shot := func(badge style.IconBadge) *paint2d.Image {
			bar := widgets.NewToolBar(&widgets.ToolItem{Text: "Cloud", Icon: style.IconCloud, Badge: badge})
			bar.SetLook(lk)
			bar.Arrange(paint2d.XYWH(0, 0, 160, 40))
			img := paint2d.NewImage(int(160*scale), int(40*scale))
			ctx := paint2d.NewContext(img)
			ctx.Scale(scale, scale)
			bar.Paint(ctx)
			img.Touch()
			return img
		}
		plain := shot(style.IconBadge{})
		for _, count := range []int{0, -1} {
			if !style.Badged(count).Empty() || !equalPixels(plain, shot(style.Badged(count))) {
				t.Fatalf("scale %g count %d changed the plain toolbar", scale, count)
			}
		}
		for _, badge := range []style.IconBadge{{Mark: style.BadgePlus}, {Mark: style.BadgeDot}, style.Badged(7), style.Badged(100)} {
			if equalPixels(plain, shot(badge)) {
				t.Fatalf("scale %g badge %v did not change actual toolbar pixels", scale, badge)
			}
		}
		if !equalPixels(shot(style.Badged(100)), shot(style.Badged(1000))) {
			t.Fatalf("scale %g counts over 99 did not share the documented 99+ rendering", scale)
		}
		t.Log(fmt.Sprintf("scale %g: all seven cloud sets and actual toolbar badge paints passed", scale))
	}
}
