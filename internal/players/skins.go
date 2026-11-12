package players

import (
	"regexp"
	"strings"

	"goframework.io/desktop-ui-skins"
)

// The skins' IDs. Every skin's art is the app's own, drawn in Go
// (internal/skins/plans); the IDs are neutral, in Go as in the packs. The
// labels shown in the menus are the old ones (SkinLabel).
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
// The labels are the ones the skins were shown under before the redraw (the
// owner, 2026-11-13: the IDs are neutral, the names in the menus are not).
var panelSkins = []struct{ id, label string }{
	{SlateSkin, "Winamp"},
	{SilverSkin, "Winamp Silver"},
	{DossierSkin, "Winamp Classified"},
	{TextModeSkin, "DOSamp"},
	{BevelSkin, "Windows 98"},
	{AmberSkin, "Winamp Classic Amber"},
	{TallySkin, "Calculator"},
	{HeadUnitSkin, "Sony CDX-MP3"},
	{GraphingSkin, "TI-82"},
	{RadarSkin, "Radar Amp"},
	{ForgeSkin, "Quake 3 Arena"},
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

// SkinLabel respects user-supplied pack labels, except Fancy's fixed display
// name. It also names choices before registration, for saved settings and help.
func SkinLabel(id string) string {
	if id == DefaultSkin || id == Themed {
		return "Default (no skin)"
	}
	if id == FancySkin {
		return "Fancy"
	}
	if sk, ok := skins.LoadSkin(id); ok && sk.Label != "" {
		return formatAppearanceLabel(id, sk.Label)
	}
	switch id {
	case WingspanSkin:
		return formatAppearanceLabel(id, "Headspace")
	case PodSkin:
		return formatAppearanceLabel(id, "Xbox")
	case GaugeSkin:
		return formatAppearanceLabel(id, "Half Life 2")
	case FandomSkin:
		return formatAppearanceLabel(id, "Fandom")
	case AluminiumSkin:
		return formatAppearanceLabel(id, "iTunes")
	case CobaltSkin:
		return formatAppearanceLabel(id, "Series 9")
	case UtilitySkin, ConsoleSkin, SidebarSkin:
		return formatAppearanceLabel(id, "")
	}
	for _, skin := range panelSkins {
		if id == skin.id {
			return FormatSkinLabel(skin.label)
		}
	}
	return id
}

var mediaPlayerLabelPrefix = regexp.MustCompile(`(?i)^win\s+media\s+player\s*-\s*`)
var macOSLabelPrefix = regexp.MustCompile(`(?i)^mac\s+os\s+x\s*-\s*`)

// formatAppearanceLabel gives each family's skins their menu prefix ("Win
// Media Player - ", "Mac OS X - ", "VLC - ", "RealPlayer - ", "KDE - "),
// as the menus showed them before the redraw.
func formatAppearanceLabel(id, label string) string {
	if id == UtilitySkin || id == ConsoleSkin || id == SidebarSkin {
		prefix, fallback := "VLC", "Classic"
		switch id {
		case ConsoleSkin:
			prefix = "RealPlayer"
		case SidebarSkin:
			prefix, fallback = "KDE", "Amarok 1.4"
		}
		label = strings.Join(strings.Fields(label), " ")
		re := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(prefix) + `\s*-\s*`)
		for re.MatchString(label) {
			label = re.ReplaceAllString(label, "")
		}
		if label == "" {
			label = fallback
		}
		return prefix + " - " + label
	}
	if id == AluminiumSkin {
		label = strings.Join(strings.Fields(label), " ")
		for macOSLabelPrefix.MatchString(label) {
			label = macOSLabelPrefix.ReplaceAllString(label, "")
		}
		if label == "" {
			label = "iTunes"
		}
		return "Mac OS X - " + label
	}
	if id != WingspanSkin && id != PodSkin && id != GaugeSkin && id != FandomSkin && id != CobaltSkin {
		return FormatSkinLabel(label)
	}
	label = strings.Join(strings.Fields(label), " ")
	for mediaPlayerLabelPrefix.MatchString(label) {
		label = mediaPlayerLabelPrefix.ReplaceAllString(label, "")
	}
	if label == "" {
		switch id {
		case CobaltSkin:
			label = "Series 9"
		case FandomSkin:
			label = "Fandom"
		case GaugeSkin:
			label = "Half Life 2"
		case PodSkin:
			label = "Xbox"
		default:
			label = "Headspace"
		}
	}
	return "Win Media Player - " + label
}

var skinLabelPrefix = regexp.MustCompile(`(?i)^winamp\s*-\s*`)
var skinLabelBrand = regexp.MustCompile(`(?i)winamp`)

// FormatSkinLabel gives the compact panel's skins their menu prefix
// ("WinAmp - ") while preserving the supplied name's case and making
// already prefixed labels stable.
func FormatSkinLabel(label string) string {
	label = strings.Join(strings.Fields(label), " ")
	for skinLabelPrefix.MatchString(label) {
		label = skinLabelPrefix.ReplaceAllString(label, "")
	}
	label = strings.Join(strings.Fields(skinLabelBrand.ReplaceAllString(label, "")), " ")
	if label == "" {
		label = "classic"
	}
	return "WinAmp - " + label
}
