package skins_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

// Exercise the public toolkit API with an unrelated in-memory skin. No app
// registration, extraction, custom painters or production skin plans are used.
func TestPublicSkinRegistrationPaintsAndReloadsItsOwnArtwork(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	style.InvalidateSkinCache()
	t.Cleanup(style.InvalidateSkinCache)
	const id = "skin-registration-public-probe"
	red := color.NRGBA{R: 240, G: 20, B: 30, A: 255}
	blue := color.NRGBA{R: 20, G: 40, B: 230, A: 255}
	green := color.NRGBA{R: 30, G: 210, B: 50, A: 255}
	register := func(label string, fill color.NRGBA) {
		t.Helper()
		if err := style.RegisterSkinFS(registrationFixture(t, label, fill), id); err != nil {
			t.Fatal(err)
		}
		if sk, ok := style.LoadSkin(id); !ok || sk.Label != label {
			t.Fatalf("registered skin lookup = %+v, %v, want %q", sk, ok, label)
		}
		if pack, ok := style.LoadTheme(id); !ok || pack.Label != label {
			t.Fatalf("registered theme lookup = %+v, %v, want %q", pack, ok, label)
		}
	}
	register("Red", red)
	checkPaint := func(fill color.NRGBA) {
		t.Helper()
		// Build a new look after replacing or overriding a registered skin;
		// v0.23.3 memoizes the old skin on a look that has already painted.
		pack, _ := style.LoadTheme(id)
		look := pack.Look()
		for _, scale := range []float32{1, 1.75} {
			img := paintengine2d.NewImage(24, 24)
			if !style.DrawSkinSprite(style.WithScale(look, scale), paintengine2d.NewContext(img), paintengine2d.XYWH(0, 0, 4*scale, 4*scale), "face", paintengine2d.Color{}) {
				t.Fatalf("registered artwork did not paint at %gx", scale)
			}
			if got := img.NRGBAAt(2, 2); got != fill {
				t.Errorf("%gx artwork = %+v, want %+v", scale, got, fill)
			}
		}
	}
	checkPaint(red)
	// The replacement must use its own sheets even though the old
	// registration populated caches under the same skin ID and file names.
	register("Blue", blue)
	checkPaint(blue)
	if got := style.SkinSourceFile(id); got != "" {
		t.Errorf("in-memory source file = %q", got)
	}

	// A user's override must win even when an app registers the name again.
	userDir := style.UserSkinDir(id)
	if err := os.MkdirAll(filepath.Join(userDir, "art"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, entry := range registrationFixture(t, "User", green) {
		if err := os.WriteFile(filepath.Join(userDir, filepath.FromSlash(path)), entry.Data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := style.RegisterSkinFS(registrationFixture(t, "Blue again", blue), id); err != nil {
		t.Fatal(err)
	}
	if sk, ok := style.LoadSkin(id); !ok || sk.Label != "User" {
		t.Fatalf("registration replaced the user's override: %+v, %v", sk, ok)
	}
	checkPaint(green)
	if err := os.RemoveAll(userDir); err != nil {
		t.Fatal(err)
	}
	style.InvalidateSkinCache()
	checkPaint(blue)
}

func registrationFixture(t *testing.T, label string, fill color.NRGBA) fstest.MapFS {
	t.Helper()
	encode := func(size int) []byte {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				img.SetNRGBA(x, y, fill)
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	return fstest.MapFS{
		style.SkinFile: {Data: []byte(`{"skin":1,"label":"` + label + `","base":"dark",` +
			`"sheets":{"sheet":{"1x":"art/sheet.png","2x":"art/sheet@2x.png"}},` +
			`"sprites":{"face":{"sheet":"sheet","at":[0,0,4,4]}}}`)},
		"art/sheet.png":    {Data: encode(4)},
		"art/sheet@2x.png": {Data: encode(8)},
	}
}
