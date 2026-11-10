//go:build ignore

// Compare the current application packs with raw public manifest output.
// No source art or generated repository files are changed.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/codemodify/media-player-music/internal/skins/plans"
	"goframework.io/desktop-ui-skingen"
)

func main() {
	for _, p := range plans.Plans() {
		beforeBytes, err := os.ReadFile(filepath.Join("internal/skins/packs", p.Name, "skin.json"))
		must(err)
		afterBytes, err := skingen.Manifest(p)
		must(err)
		var before, after map[string]any
		must(json.Unmarshal(beforeBytes, &before))
		must(json.Unmarshal(afterBytes, &after))
		sameSampling := reflect.DeepEqual(sampling(before), sampling(after))
		sameDocument := reflect.DeepEqual(before, after)
		for _, doc := range []map[string]any{before, after} {
			if design, ok := doc["design"].(map[string]any); ok {
				delete(design, "pixelated")
				if len(design) == 0 {
					delete(doc, "design")
				}
			}
			for _, sheet := range doc["sheets"].(map[string]any) {
				delete(sheet.(map[string]any), "pixelated")
			}
		}
		fmt.Printf("%s sameDocument=%t sameEffectiveSampling=%t sameOtherFields=%t\n", p.Name, sameDocument, sameSampling, reflect.DeepEqual(before, after))
		if !sameSampling || !reflect.DeepEqual(before, after) {
			panic("unexpected manifest semantic change")
		}
	}
}

func sampling(doc map[string]any) map[string]bool {
	base := false
	if design, ok := doc["design"].(map[string]any); ok {
		base, _ = design["pixelated"].(bool)
	}
	out := map[string]bool{}
	for name, value := range doc["sheets"].(map[string]any) {
		v, ok := value.(map[string]any)["pixelated"].(bool)
		if !ok {
			v = base
		}
		out[name] = v
	}
	return out
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
