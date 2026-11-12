package panel

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"

	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// An independently authored pack can put each optional backdrop at its own
// location, including a pinned slot. The real strip must paint those pixels
// there; the player's built-in readout coordinates cannot decide their place.
func TestStripPaintsExternalBackdropSlotsAtTheirAuthoredLocations(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			_, p := open(t, Skin, scale)
			const custom = "winamp-external-backdrops"
			dir := skins.UserSkinDir(custom)
			source := os.DirFS(filepath.Join("..", "..", "skins", "packs", Skin))
			if err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				to := filepath.Join(dir, filepath.FromSlash(path))
				if entry.IsDir() {
					return os.MkdirAll(to, 0o700)
				}
				data, err := fs.ReadFile(source, path)
				if err != nil {
					return err
				}
				return os.WriteFile(to, data, 0o600)
			}); err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(dir, skins.SkinFile)
			raw, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			doc["sheets"].(map[string]any)["test.backdrop"] = map[string]any{"1x": "art/test-backdrop.png", "pixelated": true}
			doc["sprites"].(map[string]any)["test.backdrop"] = map[string]any{"sheet": "test.backdrop", "at": []int{0, 0, 5, 3}}
			slots := doc["layouts"].(map[string]any)[layoutStrip].(map[string]any)["slots"].(map[string]any)
			names := []string{"backdrop.display", "backdrop.title", "backdrop.rate", "backdrop.freq", "backdrop.labels"}
			for i, name := range names {
				slots[name] = map[string]any{"at": []int{6 + 9*i, 52, 5, 3}, "art": "test.backdrop", "fromRight": i == 4}
			}
			raw, err = json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			im := image.NewNRGBA(image.Rect(0, 0, 5, 3))
			for y := 0; y < 3; y++ {
				for x := 0; x < 5; x++ {
					im.SetNRGBA(x, y, color.NRGBA{R: 0x09, G: 0x82, B: 0xee, A: 255})
				}
			}
			im.SetNRGBA(2, 1, color.NRGBA{R: 0xf1, G: 0x0b, B: 0xb9, A: 255})
			f, err := os.Create(filepath.Join(dir, "art", "test-backdrop.png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, im)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("write backdrop: %v %v", err, closeErr)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			p.SetSkin(custom)
			for _, extra := range []float32{0, 40 * scale} {
				b := p.strip.LocalBounds()
				b.Max.X += extra
				p.strip.SetBounds(b)
				shot := paint2d.NewImage(int(math.Ceil(float64(b.Dx()))), int(math.Ceil(float64(b.Dy()))))
				p.strip.Paint(paint2d.NewContext(shot))
				for i, name := range names {
					x := float32(6+9*i) * scale
					if i == 4 {
						x = b.Dx() - float32(6+9*i+5)*scale
					}
					at := paint2d.Pt(x+2.5*scale, 53.5*scale)
					r, g, bl, a := shot.PremulAt(int(at.X), int(at.Y))
					if [4]uint8{r, g, bl, a} != [4]uint8{0xf1, 0x0b, 0xb9, 255} {
						t.Errorf("%s extra=%g: authored source pixel at %v is #%02x%02x%02x/%d", name, extra, at, r, g, bl, a)
					}
				}
			}
		})
	}
}

// This compares actual rendered texel patterns with the original 1x art,
// rather than merely accepting any color already found somewhere on a sheet.
// It protects tiny glyph gaps, segmented digits, source display textures and
// baked EQ labels while the surrounding chrome may use independent 2x art.
func TestProtectedReadoutAndEQSpritesKeepOriginalPixels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, id := range Skins {
		if id == SkinSilver {
			continue
		}
		pack, ok := style.LoadTheme(id)
		if !ok {
			t.Fatalf("missing %s theme", id)
		}
		images := map[string]image.Image{}
		for _, scale := range []float32{1.75, 2} {
			t.Run(fmt.Sprintf("%s@%gx", id, scale), func(t *testing.T) {
				lk := style.WithScale(pack.Look(), scale)
				sk, _ := skins.LoadSkin(id)
				names := []string{"eq.face", "backdrop.display", "backdrop.title", "backdrop.rate", "backdrop.freq", "backdrop.labels", "led.0", "led.1", "led.8", "led.colon", "state.stop", "lamp.stereo", "font.41"}
				info, _, _, ok := textGlyph(lk, '1', "info.")
				if !ok {
					t.Fatal("missing info digit")
				}
				names = append(names, info)
				for _, name := range names {
					sp := sk.Sprites[name]
					if sp == nil || sp.Sheet == nil || !sp.Sheet.Pixelated {
						t.Fatalf("%s must keep its source-pixel sheet", name)
					}
					path := filepath.Join("..", "..", "skins", "packs", id, sp.Sheet.Files[1])
					source := images[path]
					if source == nil {
						f, err := os.Open(path)
						if err != nil {
							t.Fatal(err)
						}
						source, err = png.Decode(f)
						f.Close()
						if err != nil {
							t.Fatal(err)
						}
						images[path] = source
					}
					w, h, _ := skins.SkinSpriteSize(lk, name)
					shot := paint2d.NewImage(int(math.Ceil(float64(w*scale)))+4, int(math.Ceil(float64(h*scale)))+4)
					ctx := paint2d.NewContext(shot)
					ctx.Translate(.125, .375)
					if !skins.DrawSkinSprite(lk, ctx, paint2d.XYWH(.25, .5, w*scale, h*scale), name, paint2d.White) {
						t.Fatalf("could not draw %s", name)
					}
					for y := 0; y < shot.Height; y++ {
						for x := 0; x < shot.Width; x++ {
							tx := int(math.Floor((float64(x)+.5-.375)/float64(scale) + 1e-6))
							ty := int(math.Floor((float64(y)+.5-.875)/float64(scale) + 1e-6))
							want := color.RGBA{}
							if tx >= 0 && ty >= 0 && tx < int(w) && ty < int(h) {
								want = color.RGBAModel.Convert(source.At(int(sp.X)+tx, int(sp.Y)+ty)).(color.RGBA)
								if sp.Tint {
									want.R, want.G, want.B = want.A, want.A, want.A
								}
							}
							r, g, b, a := shot.PremulAt(x, y)
							if [4]uint8{r, g, b, a} != [4]uint8{want.R, want.G, want.B, want.A} {
								t.Fatalf("%s source texel (%d,%d) changed at device pixel (%d,%d): #%02x%02x%02x/%d want #%02x%02x%02x/%d", name, tx, ty, x, y, r, g, b, a, want.R, want.G, want.B, want.A)
							}
						}
					}
				}
			})
		}
	}
}
