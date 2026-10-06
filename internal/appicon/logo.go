package appicon

import _ "embed"

// logoPNG is an exact copy of the project-root logo.png. Keeping it next to
// the package makes skin generation independent of the working directory.
//
//go:embed logo.png
var logoPNG []byte

// LogoPNG returns the application's original speaker logo. The returned copy
// can be decoded or passed to Images without changing the embedded source.
func LogoPNG() []byte { return append([]byte(nil), logoPNG...) }
