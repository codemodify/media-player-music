package players

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/codemodify/uitoolkit/style"
)

func TestSkinLabelsUseTheRequestedBrandingWithoutChangingIDs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	style.InvalidateSkinCache()
	t.Cleanup(style.InvalidateSkinCache)
	want := []string{
		"Default (no skin)", "WinAmp - classic", "WinAmp - Silver", "WinAmp - Classified", "WinAmp - DOSamp", "WinAmp - Windows 98",
		"WinAmp - Classic Amber", "WinAmp - Calculator", "WinAmp - Sony CDX-MP3", "WinAmp - TI-82", "WinAmp - Radar Amp", "WinAmp - Quake 3 Arena",
		"Win Media Player - Headspace", "Win Media Player - Xbox", "Win Media Player - Half Life 2", "Win Media Player - Fandom", "Mac OS X - iTunes", "Win Media Player - Series 9",
		"VLC - Classic", "RealPlayer - Classic", "KDE - Amarok 1.4",
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
			t.Errorf("display branding changed saved/CLI identity %q", id)
		}
	}
	if SkinLabel(Themed) != want[0] || SkinLabel("unknown-choice") != "unknown-choice" {
		t.Fatal("branding changed the default label or an unrecognized identifier")
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
		{WinampSilverSkin, "WINAMP   My Silver", "WinAmp - My Silver"},
		{HeadspaceSkin, "Headspace", "Win Media Player - Headspace"},
		{HeadspaceSkin, " Win Media Player - My Headspace ", "Win Media Player - My Headspace"},
		{XboxSkin, "Xbox", "Win Media Player - Xbox"},
		{XboxSkin, "  wIN Media Player - Win Media Player - My Xbox ", "Win Media Player - My Xbox"},
		{HalfLife2Skin, "Half Life 2", "Win Media Player - Half Life 2"},
		{HalfLife2Skin, " Win Media Player - My Half-Life 2 ", "Win Media Player - My Half-Life 2"},
		{FandomSkin, "Fandom", "Win Media Player - Fandom"},
		{FandomSkin, " Win Media Player - My Fandom ", "Win Media Player - My Fandom"},
		{ITunesSkin, "iTunes", "Mac OS X - iTunes"},
		{ITunesSkin, "  mAC OS x - Mac OS X - My iTunes ", "Mac OS X - My iTunes"},
		{ITunesSkin, " Mac OS X - ", "Mac OS X - iTunes"},
		{Series9Skin, "Series 9", "Win Media Player - Series 9"},
		{Series9Skin, " WIN Media Player - Win Media Player - My Series 9 ", "Win Media Player - My Series 9"},
		{Series9Skin, "Win Media Player - ", "Win Media Player - Series 9"},
		{VLCSkin, "Classic", "VLC - Classic"},
		{VLCSkin, "vlc - VLC - Custom", "VLC - Custom"},
		{VLCSkin, "VLC - ", "VLC - Classic"},
		{RealPlayerSkin, "RealPlayer - Custom", "RealPlayer - Custom"},
		{RealPlayerSkin, "RealPlayer - ", "RealPlayer - Classic"},
		{AmarokSkin, "Amarok 1.4", "KDE - Amarok 1.4"},
		{AmarokSkin, "KDE - KDE - Custom", "KDE - Custom"},
		{AmarokSkin, "KDE - ", "KDE - Amarok 1.4"},
	} {
		t.Run(tc.id+"/"+tc.label, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			dir := style.UserSkinDir(tc.id)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"skin": 1, "label": tc.label, "base": "dark"})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, style.SkinFile), data, 0600); err != nil {
				t.Fatal(err)
			}
			style.InvalidateSkinCache()
			t.Cleanup(style.InvalidateSkinCache)
			if got := SkinLabel(tc.id); got != tc.want {
				t.Fatalf("user-supplied label lost precedence or branding: %q", got)
			}
		})
	}
}

func TestWMPCatalogueKeepsTheWinampFamilySeparate(t *testing.T) {
	for _, id := range []string{HeadspaceSkin, XboxSkin, HalfLife2Skin, FandomSkin, ITunesSkin, Series9Skin, VLCSkin, RealPlayerSkin, AmarokSkin} {
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
	if next := SkinnedIDs(); next[0] != WinampSkin || !reflect.DeepEqual(next[len(next)-9:], []string{HeadspaceSkin, XboxSkin, HalfLife2Skin, FandomSkin, ITunesSkin, Series9Skin, VLCSkin, RealPlayerSkin, AmarokSkin}) {
		t.Fatalf("mutable choice list changed catalogue order: %v", next)
	}
}

func TestEveryImportedAppearanceRetainsItsSavedSession(t *testing.T) {
	want := []string{"default", "winamp", "winamp-silver", "winamp-classified", "dosamp", "windows-98", "winamp-amber", "calculator", "sony-cdx-mp3", "ti-82", "radar-amp", "quake-3-arena", "headspace", "xbox", "half-life-2", "fandom", "itunes", "series-9", "vlc", "realplayer", "amarok"}
	if got := AppearanceIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu/keyboard order=%v, want %v", got, want)
	}
	for _, id := range want {
		t.Run(id, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			saved := Settings{Skin: id,
				Playlist:      &SavedPlaylist{Tracks: []Track{{Path: "/music/track.flac", Title: "Saved track", Length: time.Minute}}, Index: 0, Position: 9 * time.Second},
				Panels:        &SavedPanels{Equaliser: false, Playlist: true},
				Windows:       map[string]map[string]WindowPlacement{WinampSkin: {"playlist": {X: -120, Y: 230, Positioned: true, Dock: &WindowDock{Parent: "main", Side: "bottom", Along: 7}}}},
				Presentations: map[string]PresentationState{ITunesSkin: {Mode: "mini", Sizes: map[string]PresentationSize{"full": {Width: 900, Height: 600}, "mini": {Width: 380, Height: 68}}}},
			}
			if err := saved.Save(); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings(); !reflect.DeepEqual(got, saved) {
				t.Fatalf("%s lost saved queue or arrangement: %#v", id, got)
			}
			if canonical, ok := NormalizeSkin(id); !ok || canonical != id || IsPanelSkin(id) != slices.Contains(PanelSkins(), id) || IsSkinned(id) != (id != DefaultSkin) {
				t.Fatalf("%s was not recognized in its presentation family", id)
			}
		})
	}
}
