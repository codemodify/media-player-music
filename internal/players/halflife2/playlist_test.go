package halflife2

import (
	"fmt"
	"image/color"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func TestPlaylistReferenceHeadersMetadataAndStatusFitAfterResize(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist(players.Track{Path: "alpha.flac", Title: "Alpha", Artist: "Artist", Album: "Album", Length: 187 * time.Second})
			p.Refresh()
			for _, size := range [][2]int{{393, 226}, {610, 430}} {
				p.List.SetSize(size[0], size[1])
				a.PumpOnce()
				pane := p.listPane
				dropdown := widget.DeviceBounds(pane.buttons["playlist.dropdown"])
				header := widget.DeviceBounds(pane.buttons["playlist.header"])
				footer := widget.DeviceBounds(pane.footer)
				list := pane.list
				rows := list.geometry(list.Look()).Rows.Translate(widget.DeviceOrigin(list))
				// Slot layout rounds to whole device pixels; row geometry
				// preserves fractional source coordinates at noninteger scale.
				near := func(a, b float32) bool { return math.Abs(float64(a-b)) <= 1 }
				if !near(dropdown.Min.Y, 46*scale) || !near(header.Min.Y, dropdown.Max.Y) || !near(rows.Min.Y, header.Max.Y) ||
					!near(rows.Max.Y, footer.Min.Y) || !near(footer.Max.Y, float32(size[1]-37)*scale) {
					t.Fatalf("%dx%d playlist chrome overlaps or leaves its source viewport: dropdown=%v header=%v rows=%v footer=%v", size[0], size[1], dropdown, header, rows, footer)
				}
				capture := p.List.CaptureSurface()
				bg := capture.NRGBAAt(int(dropdown.Min.X+scale), int(dropdown.Min.Y+scale))
				if bg.R < 200 || bg.R > 225 || bg.G != bg.R || bg.B != bg.R {
					t.Fatalf("Now Playing reference background is not light gray: %v", bg)
				}
				bg = capture.NRGBAAt(int(header.Max.X-scale), int(header.Min.Y+scale))
				if bg.R < 245 || bg.G < 245 || bg.B < 245 {
					t.Fatalf("column reference background is not white: %v", bg)
				}
				// Every advertised column must paint its actual track metadata,
				// including album and file type rather than a repeated row label.
				row := list.AccessibleItems()[0].Bounds
				for column, cell := range playlistColumnRects(row, list.Look()) {
					if countPlaylistWhite(capture, cell) < 3 {
						t.Fatalf("%dx%d metadata column %d was not painted in its cell %v", size[0], size[1], column, cell)
					}
				}
				if selected, total := pane.footer.text(); selected != "Selected: 3:07" || total != "Total Time: 3:07" {
					t.Fatalf("source status labels: %q / %q", selected, total)
				}
			}
		})
	}
}

func countPlaylistWhite(img interface{ NRGBAAt(int, int) color.NRGBA }, r paintengine2d.Rect) int {
	n := 0
	for y := int(math.Ceil(float64(r.Min.Y))); y < int(r.Max.Y); y++ {
		for x := int(math.Ceil(float64(r.Min.X))); x < int(r.Max.X); x++ {
			c := img.NRGBAAt(x, y)
			if c.A > 240 && c.R > 200 && c.G > 150 && c.B > 150 {
				n++
			}
		}
	}
	return n
}

func TestPlaylistColumnsAndStatusUseKnownMetadataAndBrowsedSelection(t *testing.T) {
	a, p := open(t, 1.75)
	known := players.Track{Path: "song.Mp3", Title: "Title", Artist: "Artist", Album: "Album", Length: 187 * time.Second}
	if got := playlistColumns(known); !reflect.DeepEqual(got, [5]string{"Title", "Artist", "Album", "MP3", "3:07"}) {
		t.Fatalf("track metadata changed: %q", got)
	}
	unknown := players.Track{Title: "Waiting for metadata"}
	if got := playlistColumns(unknown); !reflect.DeepEqual(got, [5]string{"Waiting for metadata", "", "", "", "--:--"}) {
		t.Fatalf("unknown metadata was invented: %q", got)
	}
	p.Transport.List = players.NewPlaylist(known, unknown)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	p.Refresh()
	a.PumpOnce()
	click(p.List, platform.ButtonLeft, p.listPane.list.AccessibleItems()[1].Bounds.Center())
	a.PumpOnce()
	if p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
		t.Fatal("browsing a status duration started or moved playback")
	}
	selected, total := p.listPane.footer.text()
	if selected != "Selected: --:--" || total != "Total Time: 3:07+" {
		t.Fatalf("incomplete durations presented as complete: %q / %q", selected, total)
	}
	var node a11y.Node
	p.listPane.footer.Describe(&node)
	if node.Role != a11y.RoleStatusBar || node.Name != "Selected: --:--; Total Time: 3:07+" {
		t.Fatalf("status accessibility does not describe the browsed row: %#v", node)
	}
	p.Transport.List.Tracks[1].Length = 130 * time.Second
	p.Refresh()
	a.PumpOnce()
	if selected, total = p.listPane.footer.text(); selected != "Selected: 2:10" || total != "Total Time: 5:17" {
		t.Fatalf("decoder metadata did not update source status: %q / %q", selected, total)
	}
	p.Transport.List = players.NewPlaylist(unknown)
	p.Refresh()
	if _, total = p.listPane.footer.text(); total != "Total Time: --:--" {
		t.Fatal("an entirely unknown queue advertises a zero duration")
	}
}

func TestPlaylistDropdownAndHeadersOpenUsableQueueMenus(t *testing.T) {
	a, p := open(t, 1.75)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	loads := 0
	p.Host.LoadFiles = func(_ widget.Component, appendToList bool) {
		if !appendToList {
			t.Error("Add replaced the playlist")
		}
		loads++
	}
	for _, slot := range []string{"playlist.dropdown", "playlist.header"} {
		from := p.listPane.buttons[slot]
		if slot == "playlist.dropdown" {
			click(p.List, platform.ButtonLeft, widget.DeviceBounds(from).Center())
			a.PumpOnce()
		} else if !p.List.AccessibleAction(from.ID(), a11y.ActionDefault) {
			t.Fatal("column headers do not expose their queue-menu action")
		}
		menu, ok := p.List.Popup().(*widgets.PopupMenu)
		if !ok {
			t.Fatalf("%s opened %T instead of the queue menu", slot, p.List.Popup())
		}
		found := false
		for i, item := range menu.AccessibleItems() {
			if item.Name == "Add music files…" {
				found = p.List.AccessibleAction(widget.ItemID(menu, i), a11y.ActionDefault)
				break
			}
		}
		if !found || p.List.Popup() != nil {
			t.Fatal("queue menu Add action is inaccessible or stayed open")
		}
		if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
			t.Fatal("opening playlist chrome changed playback")
		}
	}
	if loads != 2 {
		t.Fatalf("new controls dispatched %d Add actions, want 2", loads)
	}
}
