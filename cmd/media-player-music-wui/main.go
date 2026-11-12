// Command media-player-music-wui is media-player-music's web UI: it serves
// the page, a remote control of the player media-player-musicd owns (the
// same one the desktop and terminal UIs show: the track playing, the
// queue, the library, the sound's settings), on the loopback interface by
// default, and passes the page's connection on to the daemon. The music is
// heard where the daemon runs, not in the browser.
//
// media-player-music-wui.json in the user's config folder for
// "media-player-music" configures it (Config), the environment's
// MEDIA_PLAYER_MUSIC_WUI_* override that. media-player-musicd's own
// configuration names where the web UI is reached (its wui), for the
// origins it admits: the two agree by default.
//
// The page is for the person given its link: a launch link, good once, for
// ten minutes, which the browser exchanges for a session
// (goframework.io/networking-http-server's Launch). media-player-music-wui
// prints one at start when its output is a terminal; while it runs,
//
//	media-player-music-wui link   prints a new one
//	media-player-music-wui open   opens a new one in the default browser
//
// (the links are signed with the key in media-player-music-wui.key, in the
// user's runtime folder, which only the user may read). It reaches
// media-player-musicd over its WebSocket with the token in
// media-player-musicd.token beside it.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/cmd/media-player-music-wui/internal/frontend"
	"github.com/codemodify/media-player-music/cmd/media-player-music-wui/internal/server"
	"github.com/codemodify/media-player-music/musicapi"
	app "goframework.io/app-as-service"
	"goframework.io/app-spec/protocol/transport"
	config "goframework.io/config-json"
	source "goframework.io/config-source-file"
	logconfig "goframework.io/logging-config"
	httpserver "goframework.io/networking-http-server"
	open "goframework.io/system-open"
	disk "goframework.io/vfs-source-disk"
)

// The defaults: where the page is served, and media-player-musicd's
// clients' WebSocket (its Config.Listen's and Config.WUI's defaults).
const (
	DefaultListen = "127.0.0.1:50131"
	DefaultDaemon = "ws://127.0.0.1:50130/clients"
)

// KeyFile holds the key the launch links are signed with, in the runtime
// folder.
const KeyFile = "media-player-music-wui.key"

// Version is media-player-music-wui's version; a release may set it with
// -ldflags "-X main.Version=...".
var Version = "0.0.1"

// Config is media-player-music-wui.json.
type Config struct {
	// Listen is where the page is served: "127.0.0.1:50131" by default.
	// media-player-musicd's wui.listen names it too.
	Listen string `json:"listen,omitempty"`
	// Daemon is media-player-musicd's clients' WebSocket:
	// "ws://127.0.0.1:50130/clients" by default.
	Daemon string `json:"daemon,omitempty"`
	// Hosts are more names the page is reached by ("music.lan:50131"),
	// beyond Listen's and, on the loopback interface, localhost's: any
	// other is refused, as a site pointed at this machine would be.
	// media-player-musicd's wui.hosts names them too.
	Hosts []string `json:"hosts,omitempty"`
	// Token is the token media-player-musicd admits this server by, for a
	// daemon whose media-player-musicd.token is not in this user's runtime
	// folder (one on another machine, one of its tokens); by default that
	// file's.
	Token string `json:"token,omitempty"`
}

func main() {
	// --version answers before the configuration is read (goframework R105).
	app.AnswerVersion("media-player-music-wui", Version)
	places, err := disk.UserPlaces(musicapi.Name)
	if err != nil {
		fail(err)
	}
	settings := Config{Listen: DefaultListen, Daemon: DefaultDaemon} // the defaults
	if _, err := config.Load(&settings,
		source.Open(filepath.Join(places.Config, "media-player-music-wui.json")),
		source.Environment("MEDIA_PLAYER_MUSIC_WUI"),
	); err != nil {
		fail(err)
	}
	token := func() (string, error) {
		return transport.TokenFile(filepath.Join(places.Runtime, musicapi.TokenFile))
	}
	if settings.Token != "" {
		token = func() (string, error) { return settings.Token, nil }
	}
	if len(os.Args) == 2 && (os.Args[1] == "link" || os.Args[1] == "open") {
		if err := giveLink(os.Stdout, os.Args[1], places, settings); err != nil {
			fail(err)
		}
		return
	}

	var listener net.Listener
	var web *httpserver.Server
	app.Run(app.App[Config]{
		Name:    "media-player-music-wui",
		Version: Version,
		Config:  settings,
		Logging: logconfig.Config{Level: logconfig.Info},
		Start: func(ctx context.Context, a *app.Context[Config]) error {
			settings := a.Config()
			key, err := launchKey(places)
			if err != nil {
				return err
			}
			handler, err := server.New(server.Config{Daemon: settings.Daemon, Page: frontend.Files, Log: a.Log.Warningf,
				Listen: settings.Listen, Hosts: settings.Hosts, Key: key, Token: token})
			if err != nil {
				return err
			}
			if listener, err = net.Listen("tcp", settings.Listen); err != nil {
				return err
			}
			web = &httpserver.Server{HTTP: &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}}
			a.Log.Infof("serving the page at http://%s/, media-player-musicd at %s", listener.Addr(), settings.Daemon)
			// the link is printed where only the person sees it: a terminal,
			// not a log or a service's journal
			if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
				link, err := handler.Link(pageAddress(listener.Addr().String()))
				if err != nil {
					return err
				}
				fmt.Printf("media-player-music-wui: open %s (good once, for ten minutes; media-player-music-wui link gives another)\n", link)
			} else {
				a.Log.Infof("for the page's link, run media-player-music-wui link (or media-player-music-wui open)")
			}
			return nil
		},
		Run: func(ctx context.Context, a *app.Context[Config]) error {
			return web.Serve(listener) // nil once Stop began
		},
		Stop: func(ctx context.Context, a *app.Context[Config]) error {
			return web.Shutdown(ctx)
		},
	})
}

// launchKey is the key the links are signed with:
// media-player-music-wui.key's, made when missing, only the user may read
// it.
func launchKey(places disk.Places) ([]byte, error) {
	text, err := transport.TokenFile(filepath.Join(places.Runtime, KeyFile))
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

// giveLink prints a new launch link to out ("link"), or opens it in the
// default browser ("open") through a page in the runtime folder that sends
// the browser on, so that the link is never on a command line other users
// of the machine may read (xdg-open's, the browser's).
func giveLink(out io.Writer, how string, places disk.Places, settings Config) error {
	key, err := launchKey(places)
	if err != nil {
		return err
	}
	link, err := httpserver.LaunchLink(key, pageAddress(settings.Listen))
	if err != nil {
		return err
	}
	if how == "link" {
		_, err := fmt.Fprintln(out, link)
		return err
	}
	page, err := writeOpenPage(places.Runtime, link)
	if err != nil {
		return err
	}
	return open.File(page)
}

// writeOpenPage writes media-player-music-wui-open.html in folder, only
// the user may read it, a page refreshing to link at once, and returns its
// path.
func writeOpenPage(folder, link string) (string, error) {
	page := filepath.Join(folder, "media-player-music-wui-open.html")
	escaped := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;").Replace(link)
	content := "<!doctype html><meta charset=utf-8><meta name=referrer content=no-referrer><meta http-equiv=refresh content=\"0;url=" + escaped + "\"><title>media-player-music</title><a href=\"" + escaped + "\">media-player-music</a>\n"
	os.Remove(page)
	file, err := os.OpenFile(page, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return "", err
	}
	return page, file.Close()
}

// pageAddress is the page's address for a link, from where the server
// listens: an address of every interface (0.0.0.0, ::, none) is reached at
// 127.0.0.1.
func pageAddress(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen + "/"
	}
	if ip := net.ParseIP(host); host == "" || ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "media-player-music-wui:", err)
	os.Exit(1)
}
