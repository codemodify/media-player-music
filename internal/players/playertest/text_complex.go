//go:build font_complex || font_emoji

package playertest

// The font_complex and font_emoji tests select complex text by the toolkit
// module that provides it; the tags are the player's own switches now.
import _ "goframework.io/desktop-ui-text-complex"
