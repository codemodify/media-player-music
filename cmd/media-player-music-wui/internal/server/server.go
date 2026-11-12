// Package server is the web UI's HTTP side: it serves the page and passes
// the page's WebSocket on to media-player-musicd (one port to reach, the
// daemon's own staying on the loopback interface), as office-calculator's
// web UI does.
//
// It answers only to its own names (Config.Listen's and Config.Hosts): a
// page of another site whose name was pointed at this machine (DNS
// rebinding) is refused, 421 (httpserver.Hosts); and it passes on only its
// own page's WebSocket: one opened by a page of any other origin is
// refused, 403 (httpserver.Origins, given httpserver.OriginsOf of the same
// names). The page's Origin goes on to the daemon, which checks it again
// against the origins it lists (its Config.Origins, made by OriginsOf from
// the same address and names). Without these, any web page open in the
// person's browser could drive the player through it. The daemon is told
// each browser's address (httpserver.Forward: the Forwarded header, what
// the browser sent replaced), which it takes from this machine alone.
//
// Every request needs a session (httpserver.Launch, goframework R112): the
// person opens a link carrying a launch token (Link), good once, for ten
// minutes, which the browser exchanges for a session cookie (HttpOnly,
// SameSite=Strict, named after the port); POST /logout ends it, and the
// page's WebSocket with it. The page's WebSocket goes on to the daemon
// with the token the daemon admits WebSocket clients by (Config.Token),
// never the session cookie.
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	httpserver "goframework.io/networking-http-server"
)

// Config is what the server needs.
type Config struct {
	// Daemon is media-player-musicd's clients' WebSocket:
	// "ws://127.0.0.1:50130/clients".
	Daemon string
	// Page is the page's files (frontend.Files).
	Page fs.FS
	// Log hears what goes wrong.
	Log func(format string, v ...any)
	// Listen is where the server is reached, "127.0.0.1:50131": on the
	// loopback interface, its names are localhost, 127.0.0.1 and [::1] at
	// that port (httpserver.LoopbackNames).
	Listen string
	// Hosts are more names it answers to ("music.lan:50131"), for a server
	// reached through a proxy or by another name.
	Hosts []string
	// Key signs the launch links (32 bytes or more), so that another
	// program holding it (media-player-music-wui link) makes them too;
	// empty: a key of the server's own.
	Key []byte
	// Token is the token the daemon admits WebSocket clients by
	// (media-player-musicd.token), read at each connection passed on; nil:
	// none presented.
	Token func() (string, error)
	// Daemon, when given, starts media-player-musicd when it does not
	// answer (goframework R115's ensure; the web UI's starter): the page
	// shows its Status while the daemon is away (GET /daemon, JSON
	// {"text", "problem"}) and asks Start again with its Retry (POST
	// /daemon); Unreachable is told each time a page's connection cannot
	// reach the daemon.
	Starter Starter
}

// Starter starts media-player-musicd for the web UI.
type Starter interface {
	// Status is what the page shows while the daemon is away:
	// "Starting media-player-musicd…", or why it could not be started
	// (problem); "" when there is nothing to say.
	Status() (text string, problem bool)
	// Start starts it now, unless a start is under way.
	Start()
	// Unreachable hears that a page's connection could not reach it: it
	// starts it again, at most so often.
	Unreachable()
}

// Server is the web UI's handler.
type Server struct {
	handler http.Handler
	launch  *httpserver.Launch
}

// ServeHTTP serves a request.
func (thisRef *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	thisRef.handler.ServeHTTP(w, r)
}

// Link returns the page's address base ("http://127.0.0.1:50131/") with a
// new launch token, good once, for ten minutes.
func (thisRef *Server) Link(base string) (string, error) { return thisRef.launch.Link(base) }

// New returns the server. A name in Hosts that is not a Host value
// (host[:port]) is an error, and so is a server with no names: one
// listening off the loopback interface with no Hosts.
func New(config Config) (server *Server, err error) {
	target, err := url.Parse(config.Daemon)
	if err != nil || (target.Scheme != "ws" && target.Scheme != "wss") {
		return nil, fmt.Errorf("the daemon's address %q is not a ws:// or wss:// URL", config.Daemon)
	}
	if _, err := fs.Stat(config.Page, "index.html"); err != nil {
		return nil, err
	}
	page, err := httpserver.Page(config.Page, httpserver.PageSettings{}) // app.js keeps its name: none is hashed
	if err != nil {
		return nil, err
	}
	defer func() { // Hosts and OriginsOf panic on a name that is not a Host value
		if problem := recover(); problem != nil {
			server, err = nil, fmt.Errorf("the server's names (listen %q, hosts %q): %v", config.Listen, config.Hosts, problem)
		}
	}()
	names := append(httpserver.LoopbackNames(config.Listen), config.Hosts...)
	if len(names) == 0 { // Hosts would refuse every request
		return nil, fmt.Errorf("the server's address %q is not on the loopback interface: give the names it is reached by in hosts", config.Listen)
	}
	if config.Token == nil {
		config.Token = func() (string, error) { return "", nil }
	}
	// the cookie named after the port: cookies are not kept to a port, so
	// two media-player-music-wui (two users') on one machine keep apart
	_, port, _ := net.SplitHostPort(config.Listen)
	launch, err := httpserver.NewLaunch(httpserver.LaunchSettings{Cookie: "media-player-music-wui-" + port, Key: config.Key})
	if err != nil {
		return nil, err
	}
	origins := httpserver.Origins(httpserver.OriginsOf(config.Listen, config.Hosts...)...)
	mux := httpserver.NewMux(httpserver.Hosts(names...), httpserver.SecurityHeaders(""), httpserver.Recover(nil), launch.Require)
	mux.Handle("GET /", page)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(favicon))
	})
	var unreachable func()
	if config.Starter != nil {
		unreachable = config.Starter.Unreachable
	}
	mux.Handle("GET /clients", origins(proxy(target, config.Token, config.Log, unreachable)))
	mux.HandleFunc("GET /daemon", func(w http.ResponseWriter, _ *http.Request) {
		var status struct {
			Text    string `json:"text"`
			Problem bool   `json:"problem"`
		}
		if config.Starter != nil {
			status.Text, status.Problem = config.Starter.Status()
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(status)
	})
	mux.Handle("POST /daemon", origins(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if config.Starter == nil {
			http.Error(w, "this web UI starts no daemon", http.StatusNotFound)
			return
		}
		config.Starter.Start()
		w.WriteHeader(http.StatusAccepted)
	})))
	mux.Handle("POST /logout", origins(launch.Logout()))
	return &Server{handler: mux, launch: launch}, nil
}

// proxy passes the page's WebSocket on to the daemon with the page's Origin
// as the browser sent it, so that the daemon's own check (its Origins: the
// same origins by default) refuses what Origins let through by mistake,
// 403, which comes back to the browser as it is; with the browser's
// address in Forwarded; and with the daemon's token (token) in
// Authorization, in place of anything the browser sent there, the session
// cookie staying here (R112).
func proxy(target *url.URL, token func() (string, error), log func(string, ...any), unreachable func()) http.Handler {
	upstream := *target
	upstream.Scheme = map[string]string{"ws": "http", "wss": "https"}[target.Scheme]
	forward := httpserver.Forward(httpserver.Proxies{})
	return &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.Out.URL.Scheme, request.Out.URL.Host, request.Out.URL.Path = upstream.Scheme, upstream.Host, upstream.Path
			request.Out.URL.RawPath = ""
			request.Out.Host = upstream.Host
			forward(request.Out, request.In)
			request.Out.Header.Del("Cookie")
			request.Out.Header.Del("Authorization")
			if token, err := token(); err != nil {
				log("the daemon's token: %v", err)
			} else if token != "" {
				request.Out.Header.Set("Authorization", "Bearer "+token)
			}
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			log("passing a page's connection on to media-player-musicd: %v", err)
			http.Error(w, "media-player-musicd cannot be reached", http.StatusBadGateway)
			if unreachable != nil {
				unreachable()
			}
		},
	}
}

// favicon is a note on a round disc.
const favicon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="15" fill="#0f766e"/><path d="M13 8v11.2a3.6 3.6 0 1 0 2 3.2V12.5l7-1.6v6.3a3.6 3.6 0 1 0 2 3.2V6z" fill="#ecfeff"/></svg>`
