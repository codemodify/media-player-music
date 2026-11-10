// Package plans defines this application's skin artwork and responsive layouts.
// It uses uitoolkit's public authoring types; the toolkit renders the sheets
// and encodes the manifests without knowing anything about a music player.
// Winamp and Winamp Silver were recovered from uitoolkit
// v0.19.2-0.20260923035601-15b5246d1a5d. Classified imports preserved original
// bitmap artwork; its source archive and authors' notices live in sources.
package plans

import "goframework.io/desktop-ui-skingen"

type (
	Plan        = skingen.Plan
	Sheet       = skingen.Sheet
	Cell        = skingen.Cell
	PartBinding = skingen.PartBinding
	TextRole    = skingen.TextRole
	WindowSpec  = skingen.WindowSpec
	ShapeRect   = skingen.ShapeRect
	LayoutSpec  = skingen.LayoutSpec
	SlotSpec    = skingen.SlotSpec
)

var Hex = skingen.Hex

// Plans returns fresh plans for the skins shipped with the player.
func Plans() []*skingen.Plan {
	return append(append([]*skingen.Plan{Winamp(), WinampSilver(), WinampClassified()}, ImportedPlans()...), Headspace(), Xbox(), HalfLife2(), Fandom(), ITunes(), Series9(), VLC(), RealPlayer(), Amarok(), Fancy())
}
