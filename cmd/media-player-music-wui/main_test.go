package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/codemodify/media-player-music/cmd/media-player-music-wui/internal/server"
	httpserver "goframework.io/networking-http-server"
	disk "goframework.io/vfs-source-disk"
)

// pageAddress: where a link points for each listen address.
func TestPageAddress(t *testing.T) {
	for listen, want := range map[string]string{
		"127.0.0.1:50131":  "http://127.0.0.1:50131/",
		"0.0.0.0:50131":    "http://127.0.0.1:50131/",
		":50131":           "http://127.0.0.1:50131/",
		"[::]:50131":       "http://127.0.0.1:50131/",
		"[::1]:50131":      "http://[::1]:50131/",
		"localhost:50131":  "http://localhost:50131/",
		"192.0.2.10:50131": "http://192.0.2.10:50131/",
	} {
		if got := pageAddress(listen); got != want {
			t.Errorf("%s: %s, want %s", listen, got, want)
		}
	}
}

// The key in media-player-music-wui.key is made once, only the user may
// read it, and a link another process makes with it
// (media-player-music-wui link) signs a browser in to the server's Launch,
// once.
func TestTheLinkCommandsKey(t *testing.T) {
	places := disk.Places{Runtime: filepath.Join(t.TempDir(), "run")}
	server, err := launchKey(places)
	if err != nil {
		t.Fatal(err)
	}
	command, err := launchKey(places)
	if err != nil || string(command) != string(server) {
		t.Fatalf("the key read again: %v", err)
	}
	if info, err := os.Stat(filepath.Join(places.Runtime, KeyFile)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the key file: %v %v", info, err)
	}
	launch, err := httpserver.NewLaunch(httpserver.LaunchSettings{Cookie: "media-player-music-wui-50131", Key: server})
	if err != nil {
		t.Fatal(err)
	}
	link, err := httpserver.LaunchLink(command, pageAddress("127.0.0.1:50131"))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		recorder := httptest.NewRecorder()
		launch.Require(http.NotFoundHandler()).ServeHTTP(recorder, httptest.NewRequest("GET", link, nil))
		if recorder.Code != want || want == http.StatusOK && len(recorder.Result().Cookies()) != 1 {
			t.Errorf("the command's link, use %d: %d, want %d", i+1, recorder.Code, want)
		}
	}
}

// open's page is the user's alone, refreshes to the link, escaped, and
// replaces the one before.
func TestTheOpenPage(t *testing.T) {
	folder := t.TempDir()
	for _, link := range []string{"http://127.0.0.1:50131/?launch=first", `http://127.0.0.1:50131/?launch=a&b="<c>"`} {
		page, err := writeOpenPage(folder, link)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(page)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("the page: %v %v", info, err)
		}
		data, _ := os.ReadFile(page)
		escaped := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;").Replace(link)
		if !strings.Contains(string(data), `content="0;url=`+escaped+`"`) || strings.Contains(string(data), "<c>") {
			t.Errorf("the page for %s: %s", link, data)
		}
	}
}

// link prints a launch link to the page's address, which a running server
// holding the same key (its own, read from media-player-music-wui.key)
// signs a browser in with, once (a link made before the server started is
// refused: Launch's rule); nothing else is printed.
func TestTheLinkCommand(t *testing.T) {
	places := disk.Places{Runtime: filepath.Join(t.TempDir(), "run")}
	settings := Config{Listen: DefaultListen, Daemon: DefaultDaemon}
	key, err := launchKey(places)
	if err != nil {
		t.Fatal(err)
	}
	web, err := server.New(server.Config{Daemon: DefaultDaemon, Page: fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Log: func(string, ...any) {}, Listen: settings.Listen, Key: key})
	if err != nil {
		t.Fatal(err)
	}
	// the command, run while the server runs
	var out bytes.Buffer
	if err := giveLink(&out, "link", places, settings); err != nil {
		t.Fatal(err)
	}
	link := strings.TrimSuffix(out.String(), "\n")
	if !strings.HasPrefix(link, "http://127.0.0.1:50131/?") || strings.Contains(link, "\n") {
		t.Fatalf("link printed %q", out.String())
	}
	recorder := httptest.NewRecorder()
	web.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, link, nil))
	if cookies := recorder.Result().Cookies(); recorder.Code == http.StatusUnauthorized || len(cookies) != 1 || cookies[0].Name != "media-player-music-wui-50131" {
		t.Errorf("the printed link: %d, cookies %v; want a session", recorder.Code, cookies)
	}
	recorder = httptest.NewRecorder()
	web.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, link, nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("the printed link again: %d, want 401", recorder.Code)
	}
}

// The defaults agree with media-player-musicd's: where it listens for the
// web UI's pages, and where it says the web UI is (its origins' source).
func TestTheDefaultsAgreeWithTheDaemon(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "media-player-musicd", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{fmt.Sprintf("%q", DefaultDaemon), fmt.Sprintf("WUI{Listen: %q}", DefaultListen)} {
		if !bytes.Contains(source, []byte(want)) {
			t.Errorf("media-player-musicd's main.go has no %s", want)
		}
	}
}
