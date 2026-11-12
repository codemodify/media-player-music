package server

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	plugin "goframework.io/app-spec/protocol/transport/websocket"
)

// signInTo follows a new launch link of web, reached at address, through
// its handler, and returns the session cookie.
func signInTo(t *testing.T, web *Server, address string) *http.Cookie {
	t.Helper()
	link, err := web.Link("http://" + address + "/")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	web.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, link, nil))
	for _, cookie := range recorder.Result().Cookies() {
		if strings.HasPrefix(cookie.Name, "media-player-music-wui-") {
			return cookie
		}
	}
	t.Fatalf("signing in: %d, no session cookie", recorder.Code)
	return nil
}

// sessionHeader is the Cookie header of a session, for a WebSocket's
// handshake.
func sessionHeader(session *http.Cookie) http.Header {
	return http.Header{"Cookie": {session.Name + "=" + session.Value}}
}

// upgrade asks the server at address for its WebSocket at /clients as a
// browser would, with origin and the session cookie (nil none), and
// returns the status: 101 when it reached the daemon and the daemon took
// it.
func upgrade(t *testing.T, address, origin string, session *http.Cookie) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/clients", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	// a page's own token in the subprotocol, which the server replaces
	request.Header.Set("Sec-WebSocket-Protocol", plugin.Subprotocol+", "+plugin.TokenSubprotocol("the-pages-guess"))
	request.Header.Set("Origin", origin)
	request.Header.Set("Authorization", "Bearer the-browsers-guess")
	if session != nil {
		request.AddCookie(session)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	return response.StatusCode
}

// Every path needs a session: without one, 401, and the daemon hears
// nothing. A launch link is good once, for a cookie named after the port,
// HttpOnly and SameSite=Strict. The page's WebSocket goes on to the daemon
// with the daemon's token (Config.Token) in place of any the browser sent,
// and without the session cookie; the daemon here admits that token alone,
// as media-player-musicd does. POST /logout ends the session, and another
// site's page cannot.
func TestTheSession(t *testing.T) {
	page := httptest.NewUnstartedServer(nil)
	address := page.Listener.Addr().String()
	_, port, _ := net.SplitHostPort(address)
	var reached atomic.Int32
	seen := make(chan http.Header, 4)
	accept := plugin.Handler([]string{"http://" + address},
		func(_ *http.Request, token, _ string) bool { return token == "the-daemons-token" },
		func(connection net.Conn, _ *http.Request) { connection.Close() })
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		seen <- r.Header.Clone()
		accept.ServeHTTP(w, r)
	}))
	defer daemon.Close()
	web, err := New(Config{
		Daemon: "ws://" + daemon.Listener.Addr().String() + "/clients",
		Page:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}, "app.js": {Data: []byte("export {}")}},
		Log:    func(string, ...any) {},
		Listen: address,
		Key:    []byte("0123456789abcdef0123456789abcdef"),
		Token:  func() (string, error) { return "the-daemons-token", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	page.Config.Handler = web
	page.Start()
	defer page.Close()

	for _, path := range []string{"/", "/app.js", "/favicon.ico", "/nothing"} {
		response, err := http.Get("http://" + address + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized || strings.Contains(string(body), "doctype") || strings.Contains(string(body), "export") {
			t.Errorf("%s without a session: %d", path, response.StatusCode)
		}
	}
	if got := upgrade(t, address, "http://"+address, nil); got != http.StatusUnauthorized || reached.Load() != 0 {
		t.Errorf("the WebSocket without a session: %d, the daemon reached %d times", got, reached.Load())
	}

	link, err := web.Link("http://" + address + "/")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	var session *http.Cookie
	for _, cookie := range response.Cookies() {
		session = cookie
	}
	if response.StatusCode != http.StatusOK || session == nil || session.Name != "media-player-music-wui-"+port || !session.HttpOnly || session.SameSite != http.SameSiteStrictMode {
		t.Fatalf("the link: %d %+v", response.StatusCode, session)
	}
	if again, err := http.Get(link); err != nil || again.StatusCode != http.StatusUnauthorized {
		t.Errorf("the link again: %v %v", again.StatusCode, err)
	}
	other, _ := New(Config{Daemon: "ws://127.0.0.1:1/clients", Page: fstest.MapFS{"index.html": {}}, Log: func(string, ...any) {}, Listen: address})
	if foreign, _ := other.Link("http://" + address + "/"); foreign != "" {
		if response, err := http.Get(foreign); err != nil || response.StatusCode != http.StatusUnauthorized {
			t.Errorf("a link signed with another key: %v %v", response.StatusCode, err)
		}
	}
	request, _ := http.NewRequest(http.MethodGet, "http://"+address+"/", nil)
	request.AddCookie(session)
	if response, err := http.DefaultClient.Do(request); err != nil || response.StatusCode != http.StatusOK {
		t.Errorf("the page with the session: %v %v", response.StatusCode, err)
	}

	if got := upgrade(t, address, "http://evil.example", session); got != http.StatusForbidden || reached.Load() != 0 {
		t.Errorf("another site's page with the session: %d, the daemon reached %d times", got, reached.Load())
	}
	if got := upgrade(t, address, "http://"+address, session); got != http.StatusSwitchingProtocols {
		t.Fatalf("the WebSocket with the session: %d", got)
	}
	header := <-seen
	if header.Get("Authorization") != "Bearer the-daemons-token" || header.Get("Cookie") != "" {
		t.Errorf("the daemon saw Authorization %q, Cookie %q", header.Get("Authorization"), header.Get("Cookie"))
	}

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	logout := func(origin string) int {
		request, _ := http.NewRequest(http.MethodPost, "http://"+address+"/logout", nil)
		request.Header.Set("Origin", origin)
		request.AddCookie(session)
		response, err := noFollow.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if got := logout("http://evil.example"); got != http.StatusForbidden {
		t.Errorf("another site's logout: %d", got)
	}
	if got := upgrade(t, address, "http://"+address, session); got != http.StatusSwitchingProtocols {
		t.Errorf("the WebSocket after another site's logout: %d", got)
	}
	if got := logout("http://" + address); got != http.StatusSeeOther {
		t.Errorf("logout: %d", got)
	}
	if got := upgrade(t, address, "http://"+address, session); got != http.StatusUnauthorized {
		t.Errorf("the WebSocket after logout: %d", got)
	}
}

// Without the daemon's token (none in Config.Token, or a wrong one), the
// daemon refuses the page's WebSocket (401), whatever the browser
// presented itself: its Authorization never reaches the daemon.
func TestTheDaemonWantsItsToken(t *testing.T) {
	for name, token := range map[string]string{"no token": "", "a wrong token": "guessed"} {
		page := httptest.NewUnstartedServer(nil)
		address := page.Listener.Addr().String()
		accept := plugin.Handler([]string{"http://" + address},
			func(_ *http.Request, token, _ string) bool { return token == "the-daemons-token" },
			func(connection net.Conn, _ *http.Request) { connection.Close() })
		daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.Header.Get("Authorization"), "the-browsers-guess") {
				t.Errorf("%s: the browser's Authorization reached the daemon", name)
			}
			accept.ServeHTTP(w, r)
		}))
		web, err := New(Config{
			Daemon: "ws://" + daemon.Listener.Addr().String() + "/clients",
			Page:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
			Log:    func(string, ...any) {},
			Listen: address,
			Token:  func() (string, error) { return token, nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		page.Config.Handler = web
		page.Start()
		if got := upgrade(t, address, "http://"+address, signInTo(t, web, address)); got != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", name, got)
		}
		page.Close()
		daemon.Close()
	}
}
