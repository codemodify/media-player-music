// Command generate regenerates the application's skin packs using the
// toolkit's public skin authoring API.
package main

import (
	"flag"
	"log"
	"slices"
	"strings"

	"github.com/codemodify/media-player-music/internal/skins/plans"
)

func main() {
	out := flag.String("out", "packs", "directory in which to write the skin packs")
	only := flag.String("only", "", "write only the skins of these names (comma-separated)")
	flag.Parse()
	for _, p := range plans.Plans() {
		if *only != "" && !slices.Contains(strings.Split(*only, ","), p.Name) {
			continue
		}
		if err := plans.Write(*out, p); err != nil {
			log.Fatalf("generate %s: %v", p.Name, err)
		}
	}
}
