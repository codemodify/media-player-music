package plans

import "goframework.io/desktop-ui-skingen"

// Write uses the toolkit's authoring renderer and the public skin format.
// The toolkit emits artwork, responsive anchors, sampling and font preferences.
func Write(dir string, plan *Plan) error {
	return skingen.Write(dir, plan)
}
