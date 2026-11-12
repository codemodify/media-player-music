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

// The menus show each skin under the label it had before the redraw (the
// owner, 2026-11-13), whatever its new ID.
func TestSkinLabelsAreTheOldLabelsWithoutChangingIDs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	skins.InvalidateSkinCache()
	t.Cleanup(skins.InvalidateSkinCache)
	want := map[string]string{
		DefaultSkin:   "Default (no skin)",
		SlateSkin:     "WinAmp - classic",
		SilverSkin:    "WinAmp - Silver",
		DossierSkin:   "WinAmp - Classified",
		TextModeSkin:  "WinAmp - DOSamp",
		BevelSkin:     "WinAmp - Windows 98",
		AmberSkin:     "WinAmp - Classic Amber",
		TallySkin:     "WinAmp - Calculator",
		HeadUnitSkin:  "WinAmp - Sony CDX-MP3",
		GraphingSkin:  "WinAmp - TI-82",
		RadarSkin:     "WinAmp - Radar Amp",
		ForgeSkin:     "WinAmp - Quake 3 Arena",
		WingspanSkin:  "Win Media Player - Headspace",
		PodSkin:       "Win Media Player - Xbox",
		GaugeSkin:     "Win Media Player - Half Life 2",
		FandomSkin:    "Win Media Player - Fandom",
		AluminiumSkin: "Mac OS X - iTunes",
		CobaltSkin:    "Win Media Player - Series 9",
		UtilitySkin:   "VLC - Classic",
		ConsoleSkin:   "RealPlayer - Classic",
		SidebarSkin:   "KDE - Amarok 1.4",
		FancySkin:     "Fancy",
	}
	ids := AppearanceIDs()
	if len(ids) != len(want) {
		t.Fatalf("catalogue has %d choices, want %d", len(ids), len(want))
	}
	for _, id := range ids {
		if got := SkinLabel(id); got != want[id] {
			t.Errorf("%s label=%q, want %q", id, got, want[id])
		}
		if normalized, ok := NormalizeSkin(id); !ok || normalized != id {
			t.Errorf("the label changed saved/CLI identity %q", id)
		}
	}
	// and the same before the packs are registered (no pack of that ID)
	for _, skin := range panelSkins {
		if got := FormatSkinLabel(skin.label); got != want[skin.id] {
			t.Errorf("%s's catalogue label=%q, want %q", skin.id, got, want[skin.id])
		}
	}
	if SkinLabel(Themed) != want[DefaultSkin] || SkinLabel("unknown-choice") != "unknown-choice" {
		t.Fatal("the default label or an unrecognized identifier changed")
	}
}

func TestFormatSkinLabelNormalizesOverridesAndIsIdempotent(t *testing.T) {
	for _, tc := range []struct{ supplied, want string }{
		{"Winamp", "WinAmp - classic"},
		{" wInAmP \t", "WinAmp - classic"},
		{"Winamp Classic Amber", "WinAmp - Classic Amber"},
		{"Night WINAMP Slate Winamp", "WinAmp - Night Slate"},
		{"\tWinamp\n My\u00a0 Blue\t", "WinAmp - My Blue"},
		{"WinampCustom", "WinAmp - Custom"},
		{"WinAmp - Silver", "WinAmp - Silver"},
		{"WINAMP- WINamp - Custom", "WinAmp - Custom"},
		{"WinAmp - ", "WinAmp - classic"},
		{"Sony CDX-MP3", "WinAmp - Sony CDX-MP3"},
	} {
		t.Run(tc.supplied, func(t *testing.T) {
			got := FormatSkinLabel(tc.supplied)
			if got != tc.want || FormatSkinLabel(got) != got {
				t.Fatalf("format %q=%q; want stable %q", tc.supplied, got, tc.want)
			}
		})
	}
}

func TestSkinLabelFormatsTheUserOverrideBeforeCatalogueFallback(t *testing.T) {
	for _, tc := range []struct{ id, label, want string }{
		{SilverSkin, "WINAMP   My Silver", "WinAmp - My Silver"},
		{WingspanSkin, "Headspace", "Win Media Player - Headspace"},
		{WingspanSkin, " Win Media Player - My Headspace ", "Win Media Player - My Headspace"},
		{PodSkin, "Xbox", "Win Media Player - Xbox"},
		{PodSkin, "  wIN Media Player - Win Media Player - My Xbox ", "Win Media Player - My Xbox"},
		{GaugeSkin, "Half Life 2", "Win Media Player - Half Life 2"},
		{GaugeSkin, " Win Media Player - My Half-Life 2 ", "Win Media Player - My Half-Life 2"},
		{FandomSkin, "Fandom", "Win Media Player - Fandom"},
		{FandomSkin, " Win Media Player - My Fandom ", "Win Media Player - My Fandom"},
		{AluminiumSkin, "iTunes", "Mac OS X - iTunes"},
		{AluminiumSkin, "  mAC OS x - Mac OS X - My iTunes ", "Mac OS X - My iTunes"},
		{AluminiumSkin, " Mac OS X - ", "Mac OS X - iTunes"},
		{CobaltSkin, "Series 9", "Win Media Player - Series 9"},
		{CobaltSkin, " WIN Media Player - Win Media Player - My Series 9 ", "Win Media Player - My Series 9"},
		{CobaltSkin, "Win Media Player - ", "Win Media Player - Series 9"},
		{UtilitySkin, "Classic", "VLC - Classic"},
		{UtilitySkin, "vlc - VLC - Custom", "VLC - Custom"},
		{UtilitySkin, "VLC - ", "VLC - Classic"},
		{ConsoleSkin, "RealPlayer - Custom", "RealPlayer - Custom"},
		{ConsoleSkin, "RealPlayer - ", "RealPlayer - Classic"},
		{SidebarSkin, "Amarok 1.4", "KDE - Amarok 1.4"},
		{SidebarSkin, "KDE - KDE - Custom", "KDE - Custom"},
		{SidebarSkin, "KDE - ", "KDE - Amarok 1.4"},
		{FancySkin, "Fancy", "Fancy"},
		{FancySkin, "WinAmp - Fancy", "Fancy"},
		{FancySkin, "Win Media Player - Custom Fancy", "Fancy"},
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
				t.Fatalf("user-supplied label lost precedence or prefix: %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSkinnedCatalogueKeepsThePanelFamilySeparate(t *testing.T) {
	for _, id := range []string{WingspanSkin, PodSkin, GaugeSkin, FandomSkin, AluminiumSkin, CobaltSkin, UtilitySkin, ConsoleSkin, SidebarSkin, FancySkin} {
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
	if next := SkinnedIDs(); next[0] != SlateSkin || !reflect.DeepEqual(next[len(next)-10:], []string{WingspanSkin, PodSkin, GaugeSkin, FandomSkin, AluminiumSkin, CobaltSkin, UtilitySkin, ConsoleSkin, SidebarSkin, FancySkin}) {
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
				Windows:       map[string]map[string]WindowPlacement{SlateSkin: {"playlist": {X: -120, Y: 230, Positioned: true, Dock: &WindowDock{Parent: "main", Side: "bottom", Along: 7}}}},
				Presentations: map[string]PresentationState{AluminiumSkin: {Mode: "mini", Sizes: map[string]PresentationSize{"full": {Width: 900, Height: 600}, "mini": {Width: 380, Height: 68}}}},
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
