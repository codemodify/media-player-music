// Package skins bundles the player-owned skin packs and makes them available
// to uitoolkit. The application owns the artwork and layout; the toolkit owns
// interpretation, scaling, controls, and rendering.
package skins

import (
	"embed"
	"fmt"
	"io/fs"
	"sync"

	"github.com/codemodify/uitoolkit/style"
)

//go:generate go run ./cmd/generate -out packs

//go:embed packs/*/skin.json packs/*/art/*.png
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
				err = style.RegisterSkinFS(pack, entry.Name())
			}
			if err != nil {
				registration.err = fmt.Errorf("register bundled skin %s: %w", entry.Name(), err)
				return
			}
		}
	})
	return registration.err
}
