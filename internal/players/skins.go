package players

import (
	"strings"

	"goframework.io/desktop-ui-skins"
)

// The skins' IDs. Every skin's art is the app's own, drawn in Go
// (internal/skins/plans); the names are neutral. The Go names are the
// presentations' (the face packages they wear), not the skins'.
const (
	WinampSkin           = "slate"
	WinampSilverSkin     = "silver"
	WinampClassifiedSkin = "dossier"
	DOSampSkin           = "textmode"
	Windows98Skin        = "bevel"
	WinampAmberSkin      = "amber"
	CalculatorSkin       = "tally"
	SonyCDXMP3Skin       = "headunit"
	TI82Skin             = "graphing"
	RadarAmpSkin         = "radar"
	Quake3ArenaSkin      = "forge"
	HeadspaceSkin        = "wingspan"
	XboxSkin             = "pod"
	HalfLife2Skin        = "gauge"
	FandomSkin           = "fandom"
	ITunesSkin           = "aluminium"
	Series9Skin          = "cobalt"
	VLCSkin              = "utility"
	RealPlayerSkin       = "console"
	AmarokSkin           = "sidebar"
	FancySkin            = "fancy"
)

// panelSkins is the compact panel's skin catalogue. Every entry uses the
// same panel controls and window family, with its own artwork and layout.
var panelSkins = []struct{ id, label string }{
	{WinampSkin, "Slate"},
	{WinampSilverSkin, "Silver"},
	{WinampClassifiedSkin, "Dossier"},
	{DOSampSkin, "Text Mode"},
	{Windows98Skin, "Bevel"},
	{WinampAmberSkin, "Amber"},
	{CalculatorSkin, "Tally"},
	{SonyCDXMP3Skin, "Head Unit"},
	{TI82Skin, "Graphing"},
	{RadarAmpSkin, "Radar"},
	{Quake3ArenaSkin, "Forge"},
}

// familyLabels names the other presentations' skins before their packs are
// registered.
var familyLabels = map[string]string{
	HeadspaceSkin: "Wingspan", XboxSkin: "Pod", HalfLife2Skin: "Gauge", FandomSkin: "Fandom",
	ITunesSkin: "Aluminium", Series9Skin: "Cobalt", VLCSkin: "Utility", RealPlayerSkin: "Console",
	AmarokSkin: "Sidebar", FancySkin: "Fancy",
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
	return append(PanelSkins(), HeadspaceSkin, XboxSkin, HalfLife2Skin, FandomSkin, ITunesSkin, Series9Skin, VLCSkin, RealPlayerSkin, AmarokSkin, FancySkin)
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
	return IsPanelSkin(id) || id == HeadspaceSkin || id == XboxSkin || id == HalfLife2Skin || id == FandomSkin || id == ITunesSkin || id == Series9Skin || id == VLCSkin || id == RealPlayerSkin || id == AmarokSkin || id == FancySkin
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
