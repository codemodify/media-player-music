package winamp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// Observed/model values must repaint the optional source track sequence,
// with signed balance controlling the thumb and its magnitude the backdrop.
// Colored frame fixtures make the final rendered choice observable without
// relying on a particular imported skin's gradient or artwork.
func TestModelRefreshPaintsTheCorrespondingSliderTrackFrames(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, Skin, scale)
			id := fmt.Sprintf("winamp-test-track-frames-%d", int(scale*100))
			installTrackFrameFixture(t, id)
			p.SetSkin(id)
			a.PumpOnce()
			check := func(label string, w *ui.Window, f *players.Fader, family, frame int) {
				t.Helper()
				im := w.Capture()
				r := widget.DeviceBounds(f).Translate(w.WindowRect().Min.Mul(-1))
				want := trackFrameColor(family, frame)
				wr, wg, wb := uint8(want.R*255+.5), uint8(want.G*255+.5), uint8(want.B*255+.5)
				count := 0
				for y := max(0, int(r.Min.Y)); y < min(im.Height, int(r.Max.Y)); y++ {
					for x := max(0, int(r.Min.X)); x < min(im.Width, int(r.Max.X)); x++ {
						r, g, b, a := im.PremulAt(x, y)
						if r == wr && g == wg && b == wb && a == 255 {
							count++
						}
					}
				}
				if count < 4 {
					t.Fatalf("%s did not paint frame %d color %v: %d pixels", label, frame, want, count)
				}
			}
			for _, v := range []struct {
				volume, pan, gain                  float32
				volumeFrame, balanceFrame, eqFrame int
			}{
				{0, 0, -12, 0, 0, 0}, {1, -1, 12, 27, 27, 27}, {.5, 1, 0, 14, 27, 14}, {.25, -.6, -6, 7, 16, 7}, {.25, .6, -6, 7, 16, 7},
			} {
				p.Host.SetVolume(v.volume)
				p.Host.SetPan(v.pan)
				p.Equaliser.SetPreamp(v.gain)
				p.Equaliser.Set(0, v.gain)
				p.Refresh()
				a.PumpOnce()
				check("Volume", p.Main, p.strip.volume, 0, v.volumeFrame)
				check("Balance", p.Main, p.strip.balance, 1, v.balanceFrame)
				check("Preamp", p.Eq, p.eqPane.preamp, 2, v.eqFrame)
				check("First EQ band", p.Eq, p.eqPane.bands[0], 2, v.eqFrame)
			}
		})
	}
}

func trackFrameColor(family, frame int) paint2d.Color {
	return paint2d.RGB(float32(35+frame*6)/255, float32(80+family*45)/255, 57.0/255)
}

func installTrackFrameFixture(t *testing.T, id string) {
	t.Helper()
	files := fstest.MapFS{}
	source := os.DirFS(filepath.Join("..", "..", "skins", "packs", Skin))
	if err := fs.WalkDir(source, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		files[path] = &fstest.MapFile{Data: b}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(files[skins.SkinFile].Data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["label"] = "Observed slider frames"
	sheets := doc["sheets"].(map[string]any)
	sprites := doc["sprites"].(map[string]any)
	sheets["testFrames"] = map[string]any{"1x": "track-frames.png"}
	im := paint2d.NewImage(68, 28*(13+13+63))
	y := 0
	for family, spec := range []struct {
		prefix string
		w, h   int
	}{{"volume", 68, 13}, {"balance", 38, 13}, {"eq", 14, 63}} {
		for frame := 0; frame < 28; frame++ {
			for yy := y; yy < y+spec.h; yy++ {
				for x := 0; x < spec.w; x++ {
					im.SetColor(x, yy, trackFrameColor(family, frame))
				}
			}
			sprites[fmt.Sprintf("%s.frame.%d", spec.prefix, frame)] = map[string]any{"sheet": "testFrames", "at": []int{0, y, spec.w, spec.h}}
			y += spec.h
		}
	}
	var png bytes.Buffer
	if err := im.WritePNG(&png); err != nil {
		t.Fatal(err)
	}
	files["track-frames.png"] = &fstest.MapFile{Data: png.Bytes()}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	files[skins.SkinFile] = &fstest.MapFile{Data: b}
	if err := skins.RegisterSkinFS(files, id); err != nil {
		t.Fatal(err)
	}
}
