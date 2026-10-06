package players

import (
	"regexp"
	"strings"

	"github.com/codemodify/uitoolkit/style"
)

const (
	WinampSkin           = "winamp"
	WinampSilverSkin     = "winamp-silver"
	WinampClassifiedSkin = "winamp-classified"
	DOSampSkin           = "dosamp"
	Windows98Skin        = "windows-98"
	WinampAmberSkin      = "winamp-amber"
	CalculatorSkin       = "calculator"
	SonyCDXMP3Skin       = "sony-cdx-mp3"
	TI82Skin             = "ti-82"
	RadarAmpSkin         = "radar-amp"
	Quake3ArenaSkin      = "quake-3-arena"
	HeadspaceSkin        = "headspace"
	XboxSkin             = "xbox"
	HalfLife2Skin        = "half-life-2"
	FandomSkin           = "fandom"
	ITunesSkin           = "itunes"
	Series9Skin          = "series-9"
	VLCSkin              = "vlc"
	RealPlayerSkin       = "realplayer"
	AmarokSkin           = "amarok"
)

// panelSkins is the application-owned skin catalogue. Every entry uses the
// same Winamp controls and window family, with its own artwork and layout.
var panelSkins = []struct{ id, label string }{
	{WinampSkin, "Winamp"},
	{WinampSilverSkin, "Winamp Silver"},
	{WinampClassifiedSkin, "Winamp Classified"},
	{DOSampSkin, "DOSamp"},
	{Windows98Skin, "Windows 98"},
	{WinampAmberSkin, "Winamp Classic Amber"},
	{CalculatorSkin, "Calculator"},
	{SonyCDXMP3Skin, "Sony CDX-MP3"},
	{TI82Skin, "TI-82"},
	{RadarAmpSkin, "Radar Amp"},
	{Quake3ArenaSkin, "Quake 3 Arena"},
}

// PanelSkins returns the Winamp-family skins in menu and keyboard order.
func PanelSkins() []string {
	ids := make([]string, len(panelSkins))
	for i, skin := range panelSkins {
		ids[i] = skin.id
	}
	return ids
}

// SkinnedIDs returns all optional skins, including separate window families.
func SkinnedIDs() []string {
	return append(PanelSkins(), HeadspaceSkin, XboxSkin, HalfLife2Skin, FandomSkin, ITunesSkin, Series9Skin, VLCSkin, RealPlayerSkin, AmarokSkin)
}

// AppearanceIDs includes the ordinary UI followed by the optional skins.
func AppearanceIDs() []string { return append([]string{DefaultSkin}, SkinnedIDs()...) }

// IsPanelSkin identifies an application choice in the Winamp family.
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
	return IsPanelSkin(id) || id == HeadspaceSkin || id == XboxSkin || id == HalfLife2Skin || id == FandomSkin || id == ITunesSkin || id == Series9Skin || id == VLCSkin || id == RealPlayerSkin || id == AmarokSkin
}

// SkinChoices is the same catalogue expressed as command-line choices.
func SkinChoices() string { return strings.Join(AppearanceIDs(), ", ") }

// SkinLabel respects user-supplied pack labels and also names choices before
// registration, when loading saved settings or displaying command-line help.
func SkinLabel(id string) string {
	if id == DefaultSkin || id == Themed {
		return "Default (no skin)"
	}
	if sk, ok := style.LoadSkin(id); ok && sk.Label != "" {
		return formatAppearanceLabel(id, sk.Label)
	}
	if id == HeadspaceSkin {
		return formatAppearanceLabel(id, "Headspace")
	}
	if id == XboxSkin {
		return formatAppearanceLabel(id, "Xbox")
	}
	if id == HalfLife2Skin {
		return formatAppearanceLabel(id, "Half Life 2")
	}
	if id == FandomSkin {
		return formatAppearanceLabel(id, "Fandom")
	}
	if id == ITunesSkin {
		return formatAppearanceLabel(id, "iTunes")
	}
	if id == Series9Skin {
		return formatAppearanceLabel(id, "Series 9")
	}
	if id == VLCSkin || id == RealPlayerSkin || id == AmarokSkin {
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

func formatAppearanceLabel(id, label string) string {
	if id == VLCSkin || id == RealPlayerSkin || id == AmarokSkin {
		prefix, fallback := "VLC", "Classic"
		switch id {
		case RealPlayerSkin:
			prefix = "RealPlayer"
		case AmarokSkin:
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
	if id == ITunesSkin {
		label = strings.Join(strings.Fields(label), " ")
		for macOSLabelPrefix.MatchString(label) {
			label = macOSLabelPrefix.ReplaceAllString(label, "")
		}
		if label == "" {
			label = "iTunes"
		}
		return "Mac OS X - " + label
	}
	if id != HeadspaceSkin && id != XboxSkin && id != HalfLife2Skin && id != FandomSkin && id != Series9Skin {
		return FormatSkinLabel(label)
	}
	label = strings.Join(strings.Fields(label), " ")
	for mediaPlayerLabelPrefix.MatchString(label) {
		label = mediaPlayerLabelPrefix.ReplaceAllString(label, "")
	}
	if label == "" {
		switch id {
		case Series9Skin:
			label = "Series 9"
		case FandomSkin:
			label = "Fandom"
		case HalfLife2Skin:
			label = "Half Life 2"
		case XboxSkin:
			label = "Xbox"
		default:
			label = "Headspace"
		}
	}
	return "Win Media Player - " + label
}

var skinLabelPrefix = regexp.MustCompile(`(?i)^winamp\s*-\s*`)
var skinLabelBrand = regexp.MustCompile(`(?i)winamp`)

// FormatSkinLabel gives optional skins their application menu prefix while
// preserving the supplied name's case and making already branded labels stable.
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
