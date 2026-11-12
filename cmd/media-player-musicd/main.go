// Command media-player-musicd is media-player-music's daemon: it owns the
// music every renderer shows, the desktop, terminal and web UIs (the
// queue, the track playing and where it is, the volume, the equaliser and
// ReplayGain), plays it on goframework's audio-playback, and serves it
// over goframework's application protocol: a Unix socket for the programs
// on this machine, a WebSocket on the loopback interface for the web UI's
// pages. A track chosen in one renderer plays in all of them. Only the
// user's own programs come in: through the socket, which only the user may
// reach (LocalSockets), and over the WebSocket, which every user of the
// machine reaches, with a token: the one in media-player-musicd.token
// beside the socket, which only the user may read (made when missing,
// goframework's transport.TokenFile; the renderers and
// media-player-music-wui present it), or one of Config.Tokens.
//
// Its folders are the user's (goframework.io/vfs-source-disk's UserPlaces
// for "media-player-music"): media-player-musicd.json in the config folder
// configures it (Config), the environment's MEDIA_PLAYER_MUSICD_* override
// that; the session (the queue, where it was, the sound's settings) is
// kept in the state folder's session.json. It runs as a user service too
// (app-as-service).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/service"
	"github.com/codemodify/media-player-music/musicapi"
	app "goframework.io/app-as-service-w-plugins"
	"goframework.io/app-spec/kit/clienthost"
	"goframework.io/app-spec/protocol/transport"
	config "goframework.io/config-json"
	source "goframework.io/config-source-file"
	logconfig "goframework.io/logging-config"
	httpserver "goframework.io/networking-http-server"
	"goframework.io/system-audio-spec/audiotest"
	"goframework.io/vfs"
	disk "goframework.io/vfs-source-disk"
	statefile "goframework.io/vfs-statefile"
)

// Version is media-player-musicd's version; a release may set it with
// -ldflags "-X main.Version=...".
var Version = "0.0.1"

// Config is media-player-musicd.json.
type Config struct {
	// Listen are where renderers connect; by default the Unix socket
	// media-player-musicd.sock in the runtime folder and
	// ws://127.0.0.1:50130/clients.
	Listen []string `json:"listen,omitempty"`
	// Tokens are more tokens a ws:// or wss:// client may present, each
	// with the name it comes in as ({"<a secret>": "laptop"}), beside the
	// one in media-player-musicd.token. The socket's clients need none.
	Tokens map[string]string `json:"tokens,omitempty"`
	// Origins are the web UI's pages' origins, which may open the
	// WebSocket; by default those of WUI's address and names
	// (httpserver.OriginsOf), the rule media-player-music-wui keeps too.
	Origins []string `json:"origins,omitempty"`
	// WUI is where media-player-music-wui is reached: its listen and
	// hosts, as media-player-music-wui.json gives them. The default
	// origins are made from it.
	WUI WUI `json:"wui"`
	// Sound is where the music is heard: "" (the system's sound), or
	// "memory", a sound system in memory that nothing hears (the
	// application's tests: goframework's audiotest).
	Sound string `json:"sound,omitempty"`
}

// WUI is where media-player-music-wui is reached.
type WUI struct {
	// Listen is its address: "127.0.0.1:50131" by default.
	Listen string `json:"listen,omitempty"`
	// Hosts are more names it is reached by ("music.lan:50131").
	Hosts []string `json:"hosts,omitempty"`
}

// origins are the origins of the web UI's pages, given where it is reached:
// an error for a name that is not a Host value (host[:port]).
func origins(wui WUI) (list []string, err error) {
	defer func() { // OriginsOf panics on such a name
		if problem := recover(); problem != nil {
			list, err = nil, fmt.Errorf("the web UI's names (listen %q, hosts %q): %v", wui.Listen, wui.Hosts, problem)
		}
	}()
	return httpserver.OriginsOf(wui.Listen, wui.Hosts...), nil
}

func main() {
	// --version answers before the configuration is read (goframework R105).
	app.AnswerVersion("media-player-musicd", Version)
	places, err := disk.UserPlaces(musicapi.Name)
	if err != nil {
		fail(err)
	}
	settings := Config{ // the defaults
		Listen: []string{SocketAddress(places.Runtime), "ws://127.0.0.1:50130/clients"},
		WUI:    WUI{Listen: "127.0.0.1:50131"},
	}
	if _, err := config.Load(&settings,
		source.Open(filepath.Join(places.Config, "media-player-musicd.json")),
		source.Environment("MEDIA_PLAYER_MUSICD"),
	); err != nil {
		fail(err)
	}
	if len(settings.Origins) == 0 {
		if settings.Origins, err = origins(settings.WUI); err != nil {
			fail(err)
		}
	}
	var engine playback.Engine
	switch settings.Sound {
	case "":
		// opened at the first track: nothing is opened until then
		engine = playback.NewBuiltin(playback.BuiltinOptions{})
	case "memory":
		engine = playback.NewBuiltin(playback.BuiltinOptions{System: audiotest.New(audiotest.Defaults()...)})
	default:
		fail(fmt.Errorf("sound %q: give \"\" for the system's or \"memory\"", settings.Sound))
	}
	// one daemon a user: two would play over each other
	lock, err := disk.Single(context.Background(), musicapi.Name)
	if errors.Is(err, vfs.ErrLocked) {
		fail(errors.New("another media-player-musicd runs for this user"))
	} else if err != nil {
		fail(err)
	}
	defer lock.Unlock()

	var clients *app.Clients
	var player *service.Service
	var keeper *statefile.Saver
	app.Run(app.App[Config]{
		Name:    "media-player-musicd",
		Version: Version,
		Config:  settings,
		Logging: logconfig.Config{Level: logconfig.Info},
		Start: func(ctx context.Context, a *app.Context[Config]) error {
			for _, folder := range []string{places.State, places.Runtime} {
				if err := disk.MkdirAll(folder, 0o700); err != nil {
					return err
				}
			}
			// the session, kept in session.json in the state folder: a file
			// that does not read is moved aside
			state, err := disk.Create(places.State, 0o700)
			if err != nil {
				return err
			}
			saved, err := statefile.Load[service.Saved](ctx, state, "session.json")
			if errors.Is(err, statefile.ErrDamaged) {
				a.Log.Warningf("%v: starting anew", err)
			} else if err != nil {
				return err
			}
			// written a second after a change; made before the player, which
			// tells it of changes from its first moment
			keeper = statefile.NewSaver(state, "session.json", func() service.Saved { return player.Saved() },
				statefile.Delay(time.Second), statefile.Indent("\t"),
				statefile.OnError(func(err error) { a.Log.Errorf("saving: %v", err) }))
			// each change sets the newest states, under the service's lock
			// (R13): a renderer never shows an older state after a newer one
			player = service.New(service.Options{
				Engine: engine, Saved: saved,
				Publish: func(event string, value any) {
					if clients == nil {
						return
					}
					if _, err := clients.SetState(event, value); err != nil {
						a.Log.Errorf("telling the renderers: %v", err)
					}
				},
				Changed: keeper.Changed,
			})
			config := a.Config()
			// the socket's clients come in without a token: only the user
			// reaches it; a WebSocket's present the token the user's programs
			// share (goframework R112)
			token, err := transport.TokenFile(filepath.Join(places.Runtime, musicapi.TokenFile))
			if err != nil {
				return err
			}
			tokens := map[string]string{token: "owner"}
			for each, name := range config.Tokens {
				tokens[each] = name
			}
			clients = app.NewClients(app.ClientsConfig{
				Listen:  config.Listen,
				Origins: config.Origins,
				// media-player-music-wui, on this machine, says where each
				// browser is (R47)
				Proxies:      clienthost.Proxies{Trusted: []string{clienthost.Loopback}},
				LocalSockets: true,
				Tokens:       tokens,
				Connected:    func(*app.Client) { player.Connected(+1) },
				Disconnected: func(*app.Client, error) { player.Connected(-1) },
			}, app.Implement[musicapi.Player](player, musicapi.Version))
			player.Publish() // the state as the daemon starts, before any renderer
			if err := clients.Start(ctx, app.Info{Name: a.Name, Version: a.Version}, a.LogPipe()); err != nil {
				return err
			}
			a.Log.Infof("serving the player at %v", config.Listen)
			return nil
		},
		Stop: func(ctx context.Context, a *app.Context[Config]) error {
			clients.Stop(ctx)
			err := keeper.Close(ctx) // what changed last, written
			return errors.Join(err, player.Close())
		},
	}, nil)
}

// SocketAddress is the daemon's Unix socket in the runtime folder, where
// the renderers on this machine dial it by default.
func SocketAddress(runtime string) string {
	return "unix://" + filepath.ToSlash(filepath.Join(runtime, musicapi.SocketName))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "media-player-musicd:", err)
	os.Exit(1)
}
