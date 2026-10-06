package winamp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type nativeTaskbarReport struct{ reports chan string }

func (r *nativeTaskbarReport) Report(text string) *dbus.Error {
	select {
	case r.reports <- text:
	default:
	}
	return nil
}

// Optional compositor verification, beyond xprop text. The test app stays on
// its private bus. This separately authorized diagnostic connection runs a
// short, read-only script and unloads it; no user window property is changed.
func assertKWinTaskbarPolicy(t *testing.T, p *Player) {
	t.Helper()
	address := os.Getenv("MEDIA_PLAYER_KWIN_AUDIT_BUS")
	if address == "" {
		return
	}
	conn, err := dbus.Dial(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		t.Fatal(err)
	}
	if err := conn.Hello(); err != nil {
		t.Fatal(err)
	}
	service := fmt.Sprintf("org.codemodify.MediaPlayerUtilityProbe.p%d", os.Getpid())
	const path dbus.ObjectPath = "/UtilityProbe"
	const iface = "org.codemodify.MediaPlayerUtilityProbe"
	report := &nativeTaskbarReport{reports: make(chan string, 1)}
	if err := conn.Export(report, path, iface); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.RequestName(service, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(t.TempDir(), "read-taskbar.js")
	script := fmt.Sprintf(`const rows = [];
for (const w of workspace.windowList()) {
  if (w.pid === %d) rows.push({caption:w.caption,skipTaskbar:w.skipTaskbar,skipPager:w.skipPager});
}
callDBus(%q, %q, %q, "Report", JSON.stringify(rows));
`, os.Getpid(), service, string(path), iface)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := fmt.Sprintf("media-player-utility-probe-%d", os.Getpid())
	scripting := conn.Object("org.kde.KWin", "/Scripting")
	var id int32
	if err := scripting.Call("org.kde.kwin.Scripting.loadScript", 0, scriptPath, plugin).Store(&id); err != nil {
		t.Fatal(err)
	}
	defer scripting.Call("org.kde.kwin.Scripting.unloadScript", 0, plugin)
	if err := conn.Object("org.kde.KWin", dbus.ObjectPath(fmt.Sprintf("/Scripting/Script%d", id))).Call("org.kde.kwin.Script.run", 0).Err; err != nil {
		t.Fatal(err)
	}
	var text string
	select {
	case text = <-report.reports:
	case <-time.After(time.Second):
		t.Fatal("KWin did not return its read-only taskbar state")
	}
	var rows []struct {
		Caption     string `json:"caption"`
		SkipTaskbar bool   `json:"skipTaskbar"`
		SkipPager   bool   `json:"skipPager"`
	}
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(p.Windows()) {
		t.Fatalf("KWin returned %d windows for the owned player, want %d: %s", len(rows), len(p.Windows()), text)
	}
	for _, named := range p.Windows() {
		found := false
		for _, row := range rows {
			if row.Caption != named.Window.Title() {
				continue
			}
			found = true
			wantSkip := named.Window != p.Main
			if row.SkipTaskbar != wantSkip || row.SkipPager != wantSkip {
				t.Fatalf("KWin policy for %s: skipTaskbar=%t skipPager=%t; want %t", named.Name, row.SkipTaskbar, row.SkipPager, wantSkip)
			}
		}
		if !found {
			t.Fatalf("KWin did not return the owned %s window", named.Name)
		}
	}
	t.Logf("KWin accepted Main-only taskbar policy: %s", text)
}
