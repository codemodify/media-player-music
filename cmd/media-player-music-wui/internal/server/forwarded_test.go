package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	plugin "goframework.io/app-spec/protocol/transport/websocket"
	ws "goframework.io/networking-websocket"
)

// The daemon is told each browser's address (R47): the WUI passes the
// page's WebSocket on with the Forwarded header for the connection it
// took, the browser's, with its port, whatever the browser sent in
// Forwarded or X-Forwarded-For. The daemon here is app-spec's WebSocket
// handler, telling the test what reached it.
func TestTheDaemonIsToldTheBrowsersAddress(t *testing.T) {
	told := make(chan []string, 1)
	page := httptest.NewUnstartedServer(nil)
	address := page.Listener.Addr().String()
	daemon := httptest.NewServer(plugin.Handler(nil, nil, func(connection net.Conn, request *http.Request) {
		told <- request.Header["Forwarded"]
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	browser := ""
	conn, err := ws.Dial(ctx, "ws://"+address+"/clients", ws.Subprotocols(plugin.Subprotocol),
		ws.Header(http.Header{"Forwarded": {"for=192.0.2.66"}, "X-Forwarded-For": {"192.0.2.66"}, "Cookie": sessionHeader(signInTo(t, handler, address))["Cookie"]}),
		ws.DialWith(func(ctx context.Context) (net.Conn, error) {
			connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
			if err == nil {
				browser = connection.LocalAddr().String()
			}
			return connection, err
		}))
	if err != nil {
		t.Fatal(err)
	}
	conn.CloseNow()
	want := []string{`for="` + browser + `";proto=http;host="` + address + `"`}
	select {
	case got := <-told:
		if !slices.Equal(got, want) {
			t.Errorf("the daemon was told %q, want %q", got, want)
		}
	case <-ctx.Done():
		t.Fatal("nothing reached the daemon")
	}
}
