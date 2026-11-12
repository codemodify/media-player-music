package panel

import (
	"goframework.io/desktop-ui/platform"
	"testing"
)

func TestSatelliteUtilityPolicyIsDeclaredBeforeFirstFrame(t *testing.T) {
	_, p := open(t, Skin, 1.75)
	for _, window := range p.Windows()[1:] {
		surface := window.Window.Surface().(*platform.Offscreen)
		if surface.WindowRole() != platform.RoleUtility || !surface.SkipsTaskbar() || surface.Owner() != p.Main().Surface() {
			t.Fatalf("%s did not declare utility role, owner and taskbar policy", window.Name)
		}
	}
	main := p.Main().Surface().(*platform.Offscreen)
	if main.WindowRole() != platform.RoleNormal || main.SkipsTaskbar() || main.Owner() != nil {
		t.Fatal("Main inherited satellite utility policy")
	}
	p.ShowEqualiser(false)
	p.ShowPlaylist(false)
	p.ShowEqualiser(true)
	p.ShowPlaylist(true)
	for _, window := range p.Windows()[1:] {
		surface := window.Window.Surface().(*platform.Offscreen)
		if !window.Window.Visible() || !surface.SkipsTaskbar() || surface.Owner() != p.Main().Surface() {
			t.Fatalf("%s lost declared policy on hide/show", window.Name)
		}
	}
}
