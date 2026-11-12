// Package plans defines this application's skin artwork and responsive layouts.
// It uses uitoolkit's public authoring types; the toolkit renders the sheets
// and encodes the manifests without knowing anything about a music player.
// Every skin's art is the app's own: Slate and Silver (winamp.go,
// winamp_silver.go), Fancy and Fandom as they were, and the rest redrawn in
// Go over geometry tables (redraw.go). The generator reads no file under
// internal/skins/sources (TestGeneratorReadsNoOriginals).
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
	return []*skingen.Plan{
		Winamp(), WinampSilver(), Dossier(), Textmode(), Bevel(), Amber(), Tally(), Headunit(), Graphing(), Radar(), Forge(),
		Wingspan(), Pod(), Gauge(), Fandom(), Aluminium(), Cobalt(), Utility(), Console(), Sidebar(), Fancy(),
	}
}
