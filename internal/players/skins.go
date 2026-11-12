package players

import (
	"strings"

	"goframework.io/desktop-ui-skins"
)

// The skins' IDs. Every skin's art is the app's own, drawn in Go
// (internal/skins/plans); the names are neutral, in Go as in the packs.
const (
	SlateSkin     = "slate"
	SilverSkin    = "silver"
	DossierSkin   = "dossier"
	TextModeSkin  = "textmode"
	BevelSkin     = "bevel"
	AmberSkin     = "amber"
	TallySkin     = "tally"
	HeadUnitSkin  = "headunit"
	GraphingSkin  = "graphing"
	RadarSkin     = "radar"
	ForgeSkin     = "forge"
	WingspanSkin  = "wingspan"
	PodSkin       = "pod"
	GaugeSkin     = "gauge"
	FandomSkin    = "fandom"
	AluminiumSkin = "aluminium"
	CobaltSkin    = "cobalt"
	UtilitySkin   = "utility"
	ConsoleSkin   = "console"
	SidebarSkin   = "sidebar"
	FancySkin     = "fancy"
)

// panelSkins is the compact panel's skin catalogue. Every entry uses the
// same panel controls and window family, with its own artwork and layout.
var panelSkins = []struct{ id, label string }{
	{SlateSkin, "Slate"},
	{SilverSkin, "Silver"},
	{DossierSkin, "Dossier"},
	{TextModeSkin, "Text Mode"},
	{BevelSkin, "Bevel"},
	{AmberSkin, "Amber"},
	{TallySkin, "Tally"},
	{HeadUnitSkin, "Head Unit"},
	{GraphingSkin, "Graphing"},
	{RadarSkin, "Radar"},
	{ForgeSkin, "Forge"},
}

// familyLabels names the other presentations' skins before their packs are
// registered.
var familyLabels = map[string]string{
	WingspanSkin: "Wingspan", PodSkin: "Pod", GaugeSkin: "Gauge", FandomSkin: "Fandom",
	AluminiumSkin: "Aluminium", CobaltSkin: "Cobalt", UtilitySkin: "Utility", ConsoleSkin: "Console",
	SidebarSkin: "Sidebar", FancySkin: "Fancy",
}

// PanelSkins returns the compact panel's skins in menu and keyboard order.
func PanelSkins() []string {
	ids := make([]string, len(panelSkins))
	for i, skin := range panelSkins {
		ids[i] = skin.id
	}
	return ids
}

// SkinnedIDs returns all optional skins, including separate window families.
func SkinnedIDs() []string {
	return append(PanelSkins(), WingspanSkin, PodSkin, GaugeSkin, FandomSkin, AluminiumSkin, CobaltSkin, UtilitySkin, ConsoleSkin, SidebarSkin, FancySkin)
}

// AppearanceIDs includes the ordinary UI followed by the optional skins.
func AppearanceIDs() []string { return append([]string{DefaultSkin}, SkinnedIDs()...) }

// IsPanelSkin identifies an application choice in the compact panel family.
func IsPanelSkin(id string) bool {
	for _, skin := range panelSkins {
		if skin.id == id {
			return true
		}
	}
	return false
}

// IsSkinned identifies an optional appearance in any presentation family.
func IsSkinned(id string) bool {
	return IsPanelSkin(id) || id == WingspanSkin || id == PodSkin || id == GaugeSkin || id == FandomSkin || id == AluminiumSkin || id == CobaltSkin || id == UtilitySkin || id == ConsoleSkin || id == SidebarSkin || id == FancySkin
}

// SkinChoices is the same catalogue expressed as command-line choices.
func SkinChoices() string { return strings.Join(AppearanceIDs(), ", ") }

// SkinLabel is a skin's name in the menus: its pack's label (a person's own
// pack of that ID keeps its own), or the catalogue's before registration.
// Fancy's is fixed.
func SkinLabel(id string) string {
	if id == DefaultSkin || id == Themed {
		return "Default (no skin)"
	}
	if id == FancySkin {
		return "Fancy"
	}
	fallback, known := familyLabels[id]
	for _, skin := range panelSkins {
		if id == skin.id {
			fallback, known = skin.label, true
		}
	}
	if !known {
		return id
	}
	// A pack without a label is listed under its ID by the loader; the
	// catalogue's name reads better.
	if sk, ok := skins.LoadSkin(id); ok && sk.Label != id {
		return FormatSkinLabel(sk.Label, fallback)
	}
	return fallback
}

// FormatSkinLabel tidies a supplied label's white space, and gives the
// fallback for an empty one.
func FormatSkinLabel(label, fallback string) string {
	label = strings.Join(strings.Fields(label), " ")
	if label == "" {
		return fallback
	}
	return label
}
