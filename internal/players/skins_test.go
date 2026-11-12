package players

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"goframework.io/desktop-ui-skins"
)

func TestSkinLabelsAreTheSkinsNeutralNamesWithoutChangingIDs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	skins.InvalidateSkinCache()
	t.Cleanup(skins.InvalidateSkinCache)
	want := []string{
		"Default (no skin)", "Slate", "Silver", "Dossier", "Text Mode", "Bevel",
		"Amber", "Tally", "Head Unit", "Graphing", "Radar", "Forge",
		"Wingspan", "Pod", "Gauge", "Fandom", "Aluminium", "Cobalt",
		"Utility", "Console", "Sidebar", "Fancy",
	}
	ids := AppearanceIDs()
	if len(ids) != len(want) {
		t.Fatalf("catalogue has %d choices, want %d", len(ids), len(want))
	}
	for i, id := range ids {
		if got := SkinLabel(id); got != want[i] {
			t.Errorf("%s label=%q, want %q", id, got, want[i])
		}
		if normalized, ok := NormalizeSkin(id); !ok || normalized != id {
			t.Errorf("the label changed saved/CLI identity %q", id)
		}
	}
	if SkinLabel(Themed) != want[0] || SkinLabel("unknown-choice") != "unknown-choice" {
		t.Fatal("the default label or an unrecognized identifier changed")
	}
}

func TestFormatSkinLabelTidiesWhiteSpaceAndIsIdempotent(t *testing.T) {
	for _, tc := range []struct{ supplied, want string }{
		{"Slate", "Slate"},
		{" sLaTe \t", "sLaTe"},
		{"\tMy\u00a0 Blue\t", "My Blue"},
		{"  ", "fallback"},
		{"", "fallback"},
	} {
		t.Run(tc.supplied, func(t *testing.T) {
			got := FormatSkinLabel(tc.supplied, "fallback")
			if got != tc.want || FormatSkinLabel(got, "fallback") != got {
				t.Fatalf("format %q=%q; want stable %q", tc.supplied, got, tc.want)
			}
		})
	}
}

func TestSkinLabelUsesTheUserOverride(t *testing.T) {
	for _, tc := range []struct{ id, label, want string }{
		{WinampSilverSkin, "  My   Silver ", "My Silver"},
		{HeadspaceSkin, "My Wings", "My Wings"},
		{XboxSkin, " ", "Pod"},
		{ITunesSkin, "Brushed", "Brushed"},
		{VLCSkin, "Custom", "Custom"},
		{AmarokSkin, "", "Sidebar"},
		{FancySkin, "Custom Fancy", "Fancy"},
	} {
		t.Run(tc.id+"/"+tc.label, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			dir := skins.UserSkinDir(tc.id)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"skin": 1, "label": tc.label, "base": "dark"})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), data, 0600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if got := SkinLabel(tc.id); got != tc.want {
				t.Fatalf("user-supplied label: %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWMPCatalogueKeepsTheWinampFamilySeparate(t *testing.T) {
	for _, id := range []string{HeadspaceSkin, XboxSkin, HalfLife2Skin, FandomSkin, ITunesSkin, Series9Skin, VLCSkin, RealPlayerSkin, AmarokSkin, FancySkin} {
		if IsPanelSkin(id) || !IsSkinned(id) {
			t.Fatalf("%s was not recognized as its own optional family", id)
		}
	}
	if IsSkinned(DefaultSkin) || IsSkinned("unknown") {
		t.Fatal("non-skinned choices became optional families")
	}
	for _, id := range PanelSkins() {
		if !IsPanelSkin(id) || !IsSkinned(id) {
			t.Fatalf("%s lost its Winamp family", id)
		}
	}
	// Callers may reorder choices without changing the shared catalogue.
	ids := SkinnedIDs()
	ids[0] = "changed"
	if next := SkinnedIDs(); next[0] != WinampSkin || !reflect.DeepEqual(next[len(next)-10:], []string{HeadspaceSkin, XboxSkin, HalfLife2Skin, FandomSkin, ITunesSkin, Series9Skin, VLCSkin, RealPlayerSkin, AmarokSkin, FancySkin}) {
		t.Fatalf("mutable choice list changed catalogue order: %v", next)
	}
}

func TestEveryImportedAppearanceRetainsItsSavedSession(t *testing.T) {
	want := []string{"default", "slate", "silver", "dossier", "textmode", "bevel", "amber", "tally", "headunit", "graphing", "radar", "forge", "wingspan", "pod", "gauge", "fandom", "aluminium", "cobalt", "utility", "console", "sidebar", "fancy"}
	if got := AppearanceIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu/keyboard order=%v, want %v", got, want)
	}
	for _, id := range want {
		t.Run(id, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			saved := Settings{Skin: id,
				Panels:        &SavedPanels{Equaliser: false, Playlist: true},
				Windows:       map[string]map[string]WindowPlacement{WinampSkin: {"playlist": {X: -120, Y: 230, Positioned: true, Dock: &WindowDock{Parent: "main", Side: "bottom", Along: 7}}}},
				Presentations: map[string]PresentationState{ITunesSkin: {Mode: "mini", Sizes: map[string]PresentationSize{"full": {Width: 900, Height: 600}, "mini": {Width: 380, Height: 68}}}},
			}
			if err := saved.Save(); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings(); !reflect.DeepEqual(got, saved) {
				t.Fatalf("%s lost its saved arrangement: %#v", id, got)
			}
			if canonical, ok := NormalizeSkin(id); !ok || canonical != id || IsPanelSkin(id) != slices.Contains(PanelSkins(), id) || IsSkinned(id) != (id != DefaultSkin) {
				t.Fatalf("%s was not recognized in its presentation family", id)
			}
		})
	}
}
