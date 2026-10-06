package players

import (
	"log"
	"os"
)

// TraceFileDrop provides opt-in diagnostics for the native transfer and the
// application's asynchronous folder loader. Ordinary runs produce no trace.
func TraceFileDrop(format string, args ...any) {
	if os.Getenv("MUSIC_PLAYER_DROP_DEBUG") == "1" {
		log.Printf("file-drop: "+format, args...)
	}
}
