package winamp

import (
	"bytes"
	"fmt"
	"testing"

	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

// Exercise the actual toolkit caption with a leading window-menu control as
// well as its normal trailing controls. A missing final letter or forced
// uppercase title must not pass merely because the native title is correct.
func TestMainCaptionKeepsExactBrandingAndFitsEverySkin(t *testing.T) {
	for _, skin := range Skins {
		for _, scale := range []float32{1, 1.75} {
			t.Run(fmt.Sprintf("%s@%g", skin, scale), func(t *testing.T) {
				a, p := open(t, skin, scale)
				prefs := platform.DefaultTitleBarPrefs("")
				prefs.Layout.Left = []platform.CaptionButton{platform.CaptionMenu}
				a.SetTitleBarPrefs(prefs)
				a.PumpOnce()
				caption := p.Main.Caption()
				r := widget.DeviceBounds(caption).Translate(p.Main.WindowRect().Min.Mul(-1))
				x0, y0, x1, y1 := r.IntBounds()
				pixels := func(title string) []byte {
					p.Main.SetTitle(title)
					a.PumpOnce()
					return p.Main.Capture().SubImage(x0, y0, x1, y1).Clone().Pix
				}
				full := pixels("Music Player")
				if bytes.Equal(full, pixels("MUSIC PLAYER")) {
					t.Fatal("the main caption forced the application title to uppercase")
				}
				if bytes.Equal(full, pixels("Music Playe")) {
					t.Fatal("the complete main title did not fit beside the caption controls")
				}
				p.Main.SetTitle("Music Player")
			})
		}
	}
}
