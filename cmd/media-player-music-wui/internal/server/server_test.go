package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/codemodify/media-player-music/cmd/media-player-music-wui/internal/frontend"
	httpserver "goframework.io/networking-http-server"
)

// The server answers to its own names only, and passes on only its own
// page's WebSocket: a page of another site, open in the same browser, or
// one whose name was pointed at this machine, cannot drive the daemon.
func TestOnlyItsOwnPage(t *testing.T) {
	handler, err := New(Config{
		Daemon: "ws://127.0.0.1:1/clients", // nothing there: a request let through is a 502
		Page:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Log:    func(string, ...any) {},
		Listen: "127.0.0.1:50131",
		Hosts:  []string{"music.lan:50131"},
	})
	if err != nil {
		t.Fatal(err)
	}
	session := signInTo(t, handler, "127.0.0.1:50131")
	for _, c := range []struct {
		name, host, path, origin string
		want                     int
	}{
		{"the page", "127.0.0.1:50131", "/", "", http.StatusOK},
		{"the page by localhost", "localhost:50131", "/", "", http.StatusOK},
		{"the page by a name given", "music.lan:50131", "/", "", http.StatusOK},
		{"a rebound name", "evil.example:50131", "/", "", http.StatusMisdirectedRequest},
		{"a rebound name's WebSocket", "evil.example:50131", "/clients", "http://evil.example:50131", http.StatusMisdirectedRequest},
		{"another site's page", "127.0.0.1:50131", "/clients", "https://evil.example", http.StatusForbidden},
		{"another port's page", "127.0.0.1:50131", "/clients", "http://127.0.0.1:8080", http.StatusForbidden},
		{"another site's page by its own name", "127.0.0.1:50131", "/clients", "http://evil.example", http.StatusForbidden},
		{"a sandboxed page", "127.0.0.1:50131", "/clients", "null", http.StatusForbidden},
		{"its own page", "127.0.0.1:50131", "/clients", "http://127.0.0.1:50131", http.StatusBadGateway},
		{"its own page by localhost", "127.0.0.1:50131", "/clients", "http://localhost:50131", http.StatusBadGateway},
		{"its own page by a name given", "music.lan:50131", "/clients", "http://music.lan:50131", http.StatusBadGateway},
		{"a program, no Origin", "127.0.0.1:50131", "/clients", "", http.StatusBadGateway},
	} {
		request := httptest.NewRequest(http.MethodGet, "http://"+c.host+c.path, nil)
		if c.origin != "" {
			request.Header.Set("Origin", c.origin)
		}
		request.AddCookie(session)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != c.want {
			t.Errorf("%s: %d, want %d", c.name, recorder.Code, c.want)
		}
	}
}

// The page is sent with its content's ETag and revalidated each time
// (no-cache), so a browser gets a 304 while the page is the same and the
// next build's page at once; every answer carries PagePolicy; a name the
// server does not have is told so (R38).
func TestThePage(t *testing.T) {
	handler, err := New(Config{
		Daemon: "ws://127.0.0.1:1/clients",
		Page:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}, "app.js": {Data: []byte("export {}")}},
		Log:    func(string, ...any) {},
		Listen: "127.0.0.1:50131",
	})
	if err != nil {
		t.Fatal(err)
	}
	session := signInTo(t, handler, "127.0.0.1:50131")
	get := func(host, path, etag string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
		if etag != "" {
			request.Header.Set("If-None-Match", etag)
		}
		request.AddCookie(session)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	for _, path := range []string{"/", "/app.js"} {
		first := get("127.0.0.1:50131", path, "")
		etag := first.Header().Get("Etag")
		if first.Code != http.StatusOK || etag == "" || first.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: %d, ETag %q, Cache-Control %q; want 200, an ETag, no-cache", path, first.Code, etag, first.Header().Get("Cache-Control"))
		}
		if got := first.Header().Get("Content-Security-Policy"); got != httpserver.PagePolicy {
			t.Errorf("%s: Content-Security-Policy %q, want PagePolicy", path, got)
		}
		if again := get("127.0.0.1:50131", path, etag); again.Code != http.StatusNotModified {
			t.Errorf("%s revalidated: %d, want 304", path, again.Code)
		}
	}
	if got := get("127.0.0.1:50131", "/favicon.ico", ""); got.Code != http.StatusOK || got.Header().Get("Content-Type") != "image/svg+xml" {
		t.Errorf("/favicon.ico: %d %q", got.Code, got.Header().Get("Content-Type"))
	}
	if got := get("127.0.0.1:50131", "/nothing", ""); got.Code != http.StatusNotFound {
		t.Errorf("/nothing: %d, want 404", got.Code)
	}
	if got := get("evil.example:50131", "/", ""); strings.TrimSpace(got.Body.String()) != "Misdirected Request" {
		t.Errorf("a rebound name: %d %q, want 421 Misdirected Request", got.Code, got.Body.String())
	}
}

// A name that is not a Host value, or a server with no names, is an error,
// not a panic or a server refusing everything; the built page serves.
func TestNamesThatAreNot(t *testing.T) {
	if _, err := New(Config{Daemon: "ws://127.0.0.1:1/clients", Page: frontend.Files, Log: func(string, ...any) {}, Listen: "127.0.0.1:50131"}); err != nil {
		t.Errorf("the built page: %v", err)
	}
	page := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}
	for _, config := range []Config{
		{Listen: "127.0.0.1:50131", Hosts: []string{"http://music.lan:50131"}},
		{Listen: "192.0.2.1:50131"},
	} {
		config.Daemon, config.Page, config.Log = "ws://127.0.0.1:1/clients", page, func(string, ...any) {}
		if _, err := New(config); err == nil {
			t.Errorf("listen %q, hosts %q: no error", config.Listen, config.Hosts)
		}
	}
}

// The built page (frontend.Files) is served with its security headers to
// a session: index.html, whose only script is app.js (the policy runs no
// inline script), and app.js as JavaScript.
func TestTheBuiltPage(t *testing.T) {
	handler, err := New(Config{Daemon: "ws://127.0.0.1:1/clients", Page: frontend.Files, Log: func(string, ...any) {}, Listen: "127.0.0.1:50131"})
	if err != nil {
		t.Fatal(err)
	}
	session := signInTo(t, handler, "127.0.0.1:50131")
	get := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:50131"+path, nil)
		request.AddCookie(session)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	page := get("/")
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, `<script type="module" src="/app.js"></script>`) {
		t.Fatalf("/: %d %.200q", page.Code, body)
	}
	if strings.Count(body, "<script") != 1 {
		t.Errorf("/ has %d scripts, want app.js alone", strings.Count(body, "<script"))
	}
	for name, want := range map[string]string{
		"Content-Security-Policy":      httpserver.PagePolicy,
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := page.Header().Get(name); got != want {
			t.Errorf("/: %s %q, want %q", name, got, want)
		}
	}
	script := get("/app.js")
	if script.Code != http.StatusOK || !strings.Contains(script.Header().Get("Content-Type"), "javascript") || script.Body.Len() == 0 {
		t.Errorf("/app.js: %d %q, %d bytes", script.Code, script.Header().Get("Content-Type"), script.Body.Len())
	}
}
