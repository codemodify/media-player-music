package skins

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/codemodify/media-player-music/internal/skins/panel"
	"github.com/codemodify/media-player-music/internal/skins/plans"
	"goframework.io/desktop-ui-skingen"
	uiskins "goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// Validate the generated pack with the runtime API, independently of player
// layout code. Growing a viewport and moving a footer need different anchors.
func TestGeneratedPlaylistViewportAndFooterRespondToResize(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, plan := range plans.Plans() {
		// Other presentation families have their own geometry/layout tests.
		if !slices.ContainsFunc(plan.Layouts, func(l plans.LayoutSpec) bool { return l.Name == panel.LayoutPlaylist }) {
			continue
		}
		plan.Name += "-generated-layout-probe"
		dir := t.TempDir()
		// The stock authoring API must preserve the plan's anchors, without
		// the application's manifest adapter repairing them after generation.
		if err := skingen.Write(dir, plan); err != nil {
			t.Fatal(err)
		}
		if err := uiskins.RegisterSkinFS(os.DirFS(filepath.Join(dir, plan.Name)), plan.Name); err != nil {
			t.Fatal(err)
		}
		pack, _ := style.LoadTheme(plan.Name)
		for _, scale := range []float32{1, 1.75} {
			t.Run(fmt.Sprintf("%s/%gx", plan.Name, scale), func(t *testing.T) {
				look := style.WithScale(pack.Look(), scale)
				size, ok := uiskins.SkinLayoutSize(look, panel.LayoutPlaylist)
				if !ok {
					t.Fatal("generated pack lost its playlist layout")
				}
				base := paint2d.XYWH(7, 9, size.X, size.Y)
				dx, dy := 120*scale, 90*scale
				grown := paint2d.XYWH(7, 9, size.X+dx, size.Y+dy)
				slot := func(name string, box paint2d.Rect) paint2d.Rect {
					t.Helper()
					r, ok := uiskins.SkinSlotRect(look, panel.LayoutPlaylist, name, box)
					if !ok {
						t.Fatalf("generated pack lost slot %q", name)
					}
					return r
				}
				rows, moreRows := slot("rows", base), slot("rows", grown)
				if moreRows.Min != rows.Min || moreRows.Dx() != rows.Dx()+dx || moreRows.Dy() != rows.Dy()+dy {
					t.Fatalf("resizing did not grow the playlist viewport: %v -> %v", rows, moreRows)
				}
				for _, name := range []string{"add", "mini.1", "opts"} {
					before, after := slot(name, base), slot(name, grown)
					want := before.Translate(paint2d.Pt(dx, dy))
					if name == "add" {
						want = before.Translate(paint2d.Pt(0, dy))
					}
					if after != want {
						t.Errorf("footer %s did not retain its size and edge distance: %v -> %v, want %v", name, before, after, want)
					}
				}
			})
		}
	}
}
