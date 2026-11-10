//go:build font_complex || font_emoji

package main

// The optional complex-text build: shaping, bidi and colour emoji. The tags
// are the player's own now; font_complex alone turns colour emoji off.
import _ "goframework.io/desktop-ui-text-complex"
