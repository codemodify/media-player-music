// Package skins bundles the player-owned skin packs and makes them available
// to uitoolkit. The application owns the artwork and layout; the toolkit owns
// interpretation, scaling, controls, and rendering.
package skins

import (
	"embed"
	"fmt"
	"io/fs"
	"sync"

	uiskins "goframework.io/desktop-ui-skins"
)

//go:generate go run ./cmd/generate -out packs

// Only the redrawn packs are built in. The old-named folders beside them in
// packs/ (the originals' art, and Slate's and Silver's under their old
// names) stay on disk, not embedded, until the owner's history rewrite.
//
//go:embed packs/slate packs/silver packs/dossier packs/textmode packs/bevel packs/amber packs/tally packs/headunit packs/graphing packs/radar packs/forge packs/wingspan packs/pod packs/gauge packs/fandom packs/aluminium packs/cobalt packs/utility packs/console packs/sidebar packs/fancy
var bundled embed.FS

var registration struct {
	sync.Once
	err error
}

// Register makes the bundled skins available through the toolkit's normal
// theme and skin APIs. Call it before creating the application or loading a
// skin. Repeated calls reuse the same registered packs. The person's own
// skins retain precedence; their toolkit configuration is never written.
//
// The toolkit retains the embedded filesystem and reads its artwork directly,
// so registration needs no extracted files or shutdown cleanup.
func Register() error {
	registration.Do(func() {
		entries, err := bundled.ReadDir("packs")
		if err != nil {
			registration.err = fmt.Errorf("read bundled skins: %w", err)
			return
		}
		for _, entry := range entries {
			pack, err := fs.Sub(bundled, "packs/"+entry.Name())
			if err == nil {
				err = uiskins.RegisterSkinFS(pack, entry.Name())
			}
			if err != nil {
				registration.err = fmt.Errorf("register bundled skin %s: %w", entry.Name(), err)
				return
			}
		}
	})
	return registration.err
}
