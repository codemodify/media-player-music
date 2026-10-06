package plans

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/codemodify/uitoolkit/skingen"
)

// Write uses the toolkit's authoring renderer and the public skin format.
// v0.23.4 supports the plans' responsive anchors directly. This adapter
// preserves font preferences and explicit nonpixelated sheet overrides,
// which the authoring API still cannot emit.
func Write(dir string, plan *Plan) error {
	if err := skingen.Write(dir, plan); err != nil {
		return err
	}
	path := filepath.Join(dir, plan.Name, skingen.SkinManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	// The public skin format accepts font preferences but skingen.Plan does
	// not expose them. Carry original playlist typography with open fallbacks.
	if fonts := importedFontPrefs(plan.Name); len(fonts) != 0 {
		doc["fonts"] = map[string]any{"ui": fonts}
	}
	// skingen emits only true sheet overrides, while a pixelated sheet also
	// makes the design default true. State false explicitly for a mixed pack.
	for _, sheet := range plan.Sheets {
		if !sheet.Pixelated {
			doc["sheets"].(map[string]any)[sheet.Name].(map[string]any)["pixelated"] = false
		}
	}
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if bytes.Equal(data, encoded) {
		return nil
	}
	return os.WriteFile(path, encoded, 0o644)
}
