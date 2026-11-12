package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/fstest"
)

// fakeStarter is a starter that starts nothing: what the page is told, and
// what it asked.
type fakeStarter struct {
	lock                sync.Mutex
	text                string
	problem             bool
	starts, unreachable int
}

func (thisRef *fakeStarter) Status() (string, bool) {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	return thisRef.text, thisRef.problem
}
func (thisRef *fakeStarter) Start() { thisRef.lock.Lock(); thisRef.starts++; thisRef.lock.Unlock() }
func (thisRef *fakeStarter) Unreachable() {
	thisRef.lock.Lock()
	thisRef.unreachable++
	thisRef.lock.Unlock()
}

// While media-player-musicd is away, GET /daemon tells the page what the
// web UI says of it (R115), a page's connection that cannot reach it asks
// the starter to start it again, and POST /daemon (the page's Retry, its
// own origin only) starts it now; all behind the session.
func TestTheDaemonAway(t *testing.T) {
	starter := &fakeStarter{text: "media-player-musicd not found next to this program or on PATH", problem: true}
	handler, err := New(Config{
		Daemon:  "ws://127.0.0.1:1/clients", // nothing there
		Page:    fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Log:     func(string, ...any) {},
		Listen:  "127.0.0.1:50131",
		Starter: starter,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := signInTo(t, handler, "127.0.0.1:50131")
	serve := func(method, path, origin string, signedIn bool) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, "http://127.0.0.1:50131"+path, nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		if signedIn {
			request.AddCookie(session)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	answer := serve(http.MethodGet, "/daemon", "", true)
	var status struct {
		Text    string `json:"text"`
		Problem bool   `json:"problem"`
	}
	if err := json.Unmarshal(answer.Body.Bytes(), &status); err != nil || answer.Code != http.StatusOK || status.Text != starter.text || !status.Problem {
		t.Fatalf("%d %q: %+v %v", answer.Code, answer.Body.String(), status, err)
	}
	if answer := serve(http.MethodGet, "/daemon", "", false); answer.Code == http.StatusOK {
		t.Fatalf("no session: %d", answer.Code)
	}
	if answer := serve(http.MethodGet, "/clients", "http://127.0.0.1:50131", true); answer.Code != http.StatusBadGateway || starter.unreachable != 1 {
		t.Fatalf("%d, unreachable told %d times", answer.Code, starter.unreachable)
	}
	if answer := serve(http.MethodPost, "/daemon", "https://evil.example", true); answer.Code != http.StatusForbidden || starter.starts != 0 {
		t.Fatalf("another site's Retry: %d, %d starts", answer.Code, starter.starts)
	}
	if answer := serve(http.MethodPost, "/daemon", "", false); answer.Code == http.StatusAccepted || starter.starts != 0 {
		t.Fatalf("no session's Retry: %d, %d starts", answer.Code, starter.starts)
	}
	if answer := serve(http.MethodPost, "/daemon", "http://127.0.0.1:50131", true); answer.Code != http.StatusAccepted || starter.starts != 1 {
		t.Fatalf("Retry: %d, %d starts", answer.Code, starter.starts)
	}
}

// Without a starter (an address given), the page is told nothing, and
// Retry starts nothing.
func TestNoStarter(t *testing.T) {
	handler, err := New(Config{Daemon: "ws://127.0.0.1:1/clients", Page: fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Log: func(string, ...any) {}, Listen: "127.0.0.1:50131"})
	if err != nil {
		t.Fatal(err)
	}
	session := signInTo(t, handler, "127.0.0.1:50131")
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:50131/daemon", nil)
	request.AddCookie(session)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "{\"text\":\"\",\"problem\":false}\n" {
		t.Fatalf("%d %q", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:50131/daemon", nil)
	request.Header.Set("Origin", "http://127.0.0.1:50131")
	request.AddCookie(session)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("%d", recorder.Code)
	}
}
