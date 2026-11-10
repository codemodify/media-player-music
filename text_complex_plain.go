//go:build font_complex && !font_emoji

package main

import textcomplex "goframework.io/desktop-ui-text-complex"

func init() { textcomplex.SetColourEmoji(false) }
