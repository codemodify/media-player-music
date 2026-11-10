package winamp_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players/winamp"
	"github.com/codemodify/media-player-music/internal/shell"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// Caption recreation can happen after a look/frame change. The ordinary shell
// timer must restore the drop policy without any direct call to Player.Advance.
func TestShellTimerRebindsPlaylistCaptionDrops(t *testing.T) {
	for _, skin := range []string{winamp.Skin, winamp.SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(style.ThemeEnv, skin)
			a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
			s, err := shell.New(a, shell.Options{Skin: skin, Headless: true, Scale: 1.75})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(s.Close)
			var list *ui.Window
			for _, named := range s.Windows() {
				if named.Name == "playlist" {
					list = named.Window
				}
			}
			if list == nil {
				t.Fatal("the shell did not open a playlist")
			}
			calls := 0
			var got []string
			s.Host.DropFiles = func(paths []string) { calls++; got = paths }
			s.Start()
			a.PumpOnce()
			if _, ok := list.Caption().Parent().(widget.DropTarget); !ok {
				t.Fatal("the first shell frame had no caption drop policy")
			}
			header := widgets.NewHeaderBar(nil, nil, nil)
			header.ShowTitle = true
			list.SetTitleBar(header)
			deadline := time.Now().Add(2 * time.Second)
			for {
				a.PumpOnce()
				if _, bound := header.Parent().(widget.DropTarget); bound {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the running shell timer did not bind the replacement caption")
				}
				time.Sleep(5 * time.Millisecond)
			}
			off := list.Surface().(*platform.Offscreen)
			off.SimulateDrop(widget.DeviceBounds(header).Center(), map[string][]byte{
				"text/uri-list": []byte("file:///home/user/Music/net\r\n"),
			})
			a.PumpOnce()
			if taken := off.DropTaken(); taken == nil || !*taken || calls != 1 || !reflect.DeepEqual(got, []string{"/home/user/Music/net"}) {
				t.Fatalf("the replacement caption did not deliver one folder drop: calls=%d paths=%q taken=%v", calls, got, taken)
			}
		})
	}
}
