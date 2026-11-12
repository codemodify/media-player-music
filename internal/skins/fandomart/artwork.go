// Package fandomart embeds the Fandom skin's artwork: the project's own
// generated raster (its prompts and provenance are in
// ../sources/fandom/README.md until the history rewrite). Only the skin
// generator imports it.
package fandomart

import _ "embed"

//go:embed artwork.png
var artwork []byte

// Artwork returns a copy of the raster.
func Artwork() []byte { return append([]byte(nil), artwork...) }
