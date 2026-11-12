package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"goframework.io/app-spec/protocol/transport"
	plugin "goframework.io/app-spec/protocol/transport/websocket"
	httpserver "goframework.io/networking-http-server"
	ws "goframework.io/networking-websocket"
)

// The page's WebSocket goes on to the daemon with the page's Origin (R25):
// the page of this server's origin connects, and the daemon sees that
// origin; a page of another site is refused (403); a program, which sends
// no Origin, goes on as one. The daemon here is app-spec's WebSocket
// handler, the check media-player-musicd's clients' listener makes, given
// the origins media-player-musicd lists by default: httpserver.OriginsOf of
// this server's address (the rule the server keeps too, R38).
func TestOnlyThePagesOriginReachesTheDaemon(t *testing.T) {
	origins := make(chan string, 4)
	page := httptest.NewUnstartedServer(nil) // its address first: the server and the daemon are told it
	address := page.Listener.Addr().String()
	_, port, _ := net.SplitHostPort(address)
	own := "http://" + address
	daemon := httptest.NewServer(plugin.Handler(httpserver.OriginsOf(address), nil, func(connection net.Conn, _ *http.Request) {
		origins <- transport.OriginOf(connection)
		connection.Close()
	}))
	defer daemon.Close()
	handler, err := New(Config{
		Daemon: "ws://" + daemon.Listener.Addr().String() + "/clients",
		Page:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Log:    func(string, ...any) {},
		Listen: address,
	})
	if err != nil {
		t.Fatal(err)
	}
	page.Config.Handler = handler
	page.Start()
	defer page.Close()
	session := signInTo(t, handler, address)

	for _, c := range []struct {
		name, origin string
		want         int // 101 connected, or the refusal
	}{
		{"its own page", own, http.StatusSwitchingProtocols},
		{"its own page by localhost", "http://localhost:" + port, http.StatusSwitchingProtocols},
		{"another site's page", "http://evil.example", http.StatusForbidden},
		{"another site at this port", "http://evil.example:" + port, http.StatusForbidden},
		{"another port's page", "http://127.0.0.1:1", http.StatusForbidden},
		{"a sandboxed page", "null", http.StatusForbidden},
		{"a program, no Origin", "", http.StatusSwitchingProtocols},
	} {
		header := sessionHeader(session)
		if c.origin != "" {
			header.Set("Origin", c.origin)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := ws.Dial(ctx, "ws://"+address+"/clients", ws.Header(header), ws.Subprotocols(plugin.Subprotocol))
		cancel()
		var refusal *ws.HandshakeError
		switch {
		case err == nil:
			conn.CloseNow()
			if c.want != http.StatusSwitchingProtocols {
				t.Errorf("%s: connected, want %d", c.name, c.want)
				continue
			}
			if got := <-origins; !strings.EqualFold(got, c.origin) {
				t.Errorf("%s: the daemon saw the origin %q, want %q", c.name, got, c.origin)
			}
		case errors.As(err, &refusal):
			if refusal.Status != c.want {
				t.Errorf("%s: %d, want %d", c.name, refusal.Status, c.want)
			}
		default:
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// What the server's Origins lets through by mistake, the daemon still refuses: a name
// given in Hosts that the daemon does not list as an origin.
func TestTheDaemonChecksTheOriginToo(t *testing.T) {
	page := httptest.NewUnstartedServer(nil)
	address := page.Listener.Addr().String()
	_, port, _ := net.SplitHostPort(address)
	daemon := httptest.NewServer(plugin.Handler([]string{"http://" + address}, nil, func(connection net.Conn, _ *http.Request) { connection.Close() }))
	defer daemon.Close()
	handler, err := New(Config{
		Daemon: "ws://" + daemon.Listener.Addr().String() + "/clients",
		Page:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Log:    func(string, ...any) {},
		Listen: address,
		Hosts:  []string{"music.lan:" + port},
	})
	if err != nil {
		t.Fatal(err)
	}
	page.Config.Handler = handler
	page.Start()
	defer page.Close()
	header := sessionHeader(signInTo(t, handler, address))
	header.Set("Origin", "http://music.lan:"+port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = ws.Dial(ctx, "ws://"+address+"/clients", ws.Header(header), ws.Subprotocols(plugin.Subprotocol))
	var refusal *ws.HandshakeError
	if !errors.As(err, &refusal) || refusal.Status != http.StatusForbidden {
		t.Errorf("a page of a name the daemon does not list: %v, want a 403", err)
	}
}
