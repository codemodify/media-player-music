package winamp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbus "goframework.io/system-dbus"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := dbus.Connect(ctx, address) // authenticated, and Hello said
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	service := fmt.Sprintf("org.codemodify.MediaPlayerUtilityProbe.p%d", os.Getpid())
	const path dbus.ObjectPath = "/UtilityProbe"
	const iface = "org.codemodify.MediaPlayerUtilityProbe"
	report := &nativeTaskbarReport{reports: make(chan string, 1)}
	if err := conn.Export(path, dbus.Interface{Name: iface, Methods: dbus.MethodTable(report)}); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.RequestName(ctx, service, dbus.NameDoNotQueue); err != nil {
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
	if err := scripting.Call(ctx, "org.kde.kwin.Scripting.loadScript", scriptPath, plugin).Store(&id); err != nil {
		t.Fatal(err)
	}
	defer func() {
		unload, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		scripting.Call(unload, "org.kde.kwin.Scripting.unloadScript", plugin)
	}()
	if err := conn.Object("org.kde.KWin", dbus.ObjectPath(fmt.Sprintf("/Scripting/Script%d", id))).Call(ctx, "org.kde.kwin.Script.run").Err; err != nil {
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
