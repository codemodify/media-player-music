//go:build font_complex && !font_emoji

package playertest

import textcomplex "goframework.io/desktop-ui-text-complex"

// font_complex alone shapes text without colour emoji, as it did before
// one module provided both.
func init() { textcomplex.SetColourEmoji(false) }
