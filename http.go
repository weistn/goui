package goui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Window opens a new browser window and handles the http
// communication between the Go process and the browser window.
type Window struct {
	initalPath string
	origin     string
	token      []byte
	cookie     string
	// Incoming http requests are first checked for cookies etc. and
	// then forwarded to this mux.
	mux        *http.ServeMux
	server     *httptest.Server
	end        chan bool
	connected  chan bool
	remote     interface{}
	dispatcher *Dispatcher
	model      ModelIface
	modelState ModelState
	lock       sync.Mutex
	conn       *websocket.Conn
}

// eventMessage is sent from server to client upon SendEvent
type eventMessage struct {
	Event interface{} `json:"ev"`
	Name  string      `json:"n"`
}

// callMessage is sent from server to client upon Call to invoke a JS
// function on the client.
type callMessage struct {
	Arguments []interface{} `json:"a"`
	Name      string        `json:"f"`
}

// NewWindow creates a new HTTP server.
// Call Start() to run the server on a system-chosen port via
// the loopback-device and to launch the UI in the browser.
// The browser will open the `initialPath`, e.g. "/".
// The functions of the `remote` interface can be called from JavaScript.
// The `model` is synced to the browser, i.e. all changes made in GO are synced to the browser.
func NewWindow(initialPath string, remote interface{}, model ModelIface) *Window {
	// Make sure the server emits the right Content-Type header
	mime.AddExtensionType(".css", "text/css")

	// Create a token that is passed in the URL to the browser
	token := make([]byte, 32)
	n, err := rand.Reader.Read(token)
	if n != len(token) || err != nil {
		panic("No randomness here")
	}
	// The browser will exchange the token with a cookie
	cookie := make([]byte, 32)
	n, err = rand.Reader.Read(cookie)
	if n != len(cookie) || err != nil {
		panic("No randomness here")
	}

	s := &Window{
		mux:        http.NewServeMux(),
		cookie:     hex.EncodeToString(cookie),
		token:      token,
		end:        make(chan bool),
		connected:  make(chan bool),
		remote:     remote,
		dispatcher: NewDispatcher(remote),
		model:      model,
		initalPath: initialPath,
	}

	ws := &websocket.Server{
		Handshake: func(config *websocket.Config, r *http.Request) error { return s.handshake(config, r) },
		Handler:   func(conn *websocket.Conn) { s.wshandler(conn) },
	}
	// WebSocket
	s.mux.Handle("/_socket", ws)
	// JavaScript code for RPC, events, model etc.
	s.mux.HandleFunc("/_rpc.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(GenerateJSCode(s.remote)))
	})
	return s
}

// ServeHTTP handles incoming HTTP requests, checks authentication via the auth cookie
// and checks for CSRF attacks via Referer and Origin header fields.
// Do not call this function from the application code.
func (s *Window) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	println("Request ...", r.URL.String(), r.RemoteAddr)
	// The initial page has not yet been loaded?
	if s.token != nil {
		v := r.URL.Query()
		// The first request must use the secret token and request "/"
		if r.URL.Path == "/" && v.Get("token") == hex.EncodeToString(s.token) {
			// s.token = nil
			http.SetCookie(w, &http.Cookie{Name: "secret", Value: s.cookie})
			// http.Redirect(w, r, "/foo", http.StatusSeeOther)
			// Redirect the browser to the initial page and serve an auth cookie.
			// On linux, xdg-open unfortunately requests the page itself and follows redirections.
			// Thus, HTTP redirect will not do as it redirects xdg-open instead of the browser.
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(fmt.Sprintf("<html><head><meta http-equiv=\"refresh\" content=\"0; url=%v%v\"></head><body></body></html>", s.server.URL, s.initalPath)))
			return
		} else if r.URL.Path == s.initalPath {
			// The browser hit the landing page. From now on, do not hand out the cookie any more.
			s.token = nil
		} else if r.URL.Path == "/favicon.ico" {
			// Ok ...
		} else {
			// println("Wrong initial request: ", r.URL.String())
			// This is the wrong request. Must not happen before the initial page has been requested.
			// Might be an attack. Stop the process.
			s.close()
			return
		}
	} else {
		// println("Normal request ...", r.Method)
		// println(r.RemoteAddr)

		// CSRF prevention. Referer or Origin must be present and correct
		referer := r.Header.Get("Referer")
		origin := r.Header.Get("Origin")
		if origin != s.origin && !strings.HasPrefix(referer, s.origin+"/") {
			println("Wrong Referer and Origin")
			w.WriteHeader(http.StatusUnauthorized)
		}
	}

	// Authentication via cookie
	c, err := r.Cookie("secret")
	if c == nil || err != nil || c.Value != s.cookie {
		println("Missing cookie")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Now deliver content as requested
	s.mux.ServeHTTP(w, r)
}

func (s *Window) close() {
	s.server.CloseClientConnections()
	s.end <- true
	s.server.Close()
}

func (s *Window) websocketConnected(conn *websocket.Conn) {
	s.conn = conn
	// if s.waitingForStart {
	s.connected <- true
	// }
}

func (s *Window) websocketDisconnected() {
	s.conn = nil

	go func() {
		select {
		case <-s.connected:
			// Ok, do nothing
			return
		case <-time.After(10 * time.Second):
			println("UI Timeout. No new websocket connection")
			s.close()
		}
	}()
}

// Accept incoming websockets and allow for RPC
func (s *Window) handshake(config *websocket.Config, r *http.Request) error {
	if s.conn != nil {
		println("Double websocket connection")
		return errors.New("Unauthorized")
	}
	c, err := r.Cookie("secret")
	if c == nil || err != nil || c.Value != s.cookie {
		println("Illegal websocket connect")
		return errors.New("Unauthorized")
	}
	// CSRF prevention
	origin := r.Header.Get("Origin")
	if origin != s.origin {
		return errors.New("Unauthorized")
	}

	println("Websocket connect ok")
	return nil
}

func (s *Window) wshandler(conn *websocket.Conn) {
	s.websocketConnected(conn)
	//    println("Websocket connected")
	s.SyncModel()
	for {
		var msg string
		err := websocket.Message.Receive(conn, &msg)
		if err != nil {
			println("Websocket failed while reading", err)
			println(fmt.Sprintf("ERR %v %v\n", err.Error(), err))
			s.websocketDisconnected()
			// s.close()
			return
		}
		println("Got data:", msg)
		var inv invocation
		err = json.Unmarshal([]byte(msg), &inv)
		if err != nil {
			println("Websocket got malformed request", err)
			continue
		}

		// The window has been closed?
		if inv.Name == "goui:gui_terminated" {
			s.close()
			return
		}

		result, err := s.dispatcher.Dispatch(&inv)

		s.SyncModel()

		s.lock.Lock()
		if err != nil {
			err = websocket.Message.Send(conn, `{"err": "Internal error"}`)
		} else {
			println("Sending:", string(result))
			err = websocket.Message.Send(conn, string(result))
		}
		s.lock.Unlock()
		if err != nil {
			println("Websocket failed while sending", err)
			s.websocketDisconnected()
			// s.close()
			return
		}
	}
}

// SendEvent sends an event to the browser
func (s *Window) SendEvent(name string, event interface{}) error {
	s.lock.Lock()
	if s.conn == nil {
		s.lock.Unlock()
		return errors.New("not connected")
	}
	e := &eventMessage{
		Event: event,
		Name:  name,
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	println("Sending", data)
	err = websocket.Message.Send(s.conn, string(data))
	s.lock.Unlock()
	if err != nil {
		//        println("Websocket failed")
		s.close()
		return err
	}
	return nil
}

// SyncModel synchronizes the client-side model with all changes
// applied to the server-side model.
func (s *Window) SyncModel() error {
	s.lock.Lock()
	if (s.model == nil || s.model.ModelState() == ModelSynced) && s.modelState == ModelSynced {
		s.lock.Unlock()
		return nil
	}
	if s.conn == nil {
		s.lock.Unlock()
		return errors.New("not connected")
	}
	data, err := MarshalDiff(s.model)
	if err != nil {
		return err
	}
	println("Sending Model", string(data))
	err = websocket.Message.Send(s.conn, string(data))
	s.lock.Unlock()
	if err != nil {
		s.close()
		return err
	}
	s.modelState = ModelSynced
	return nil
}

// Call invokes a function in the browser.
// Call is async, i.e. it does not wait for the browser to complete the function call
// and the result is not transmitted back to the server.
func (s *Window) Call(fname string, args ...interface{}) error {
	s.lock.Lock()
	if s.conn == nil {
		s.lock.Unlock()
		return errors.New("not connected")
	}
	c := &callMessage{
		Arguments: args,
		Name:      fname,
	}
	data, err := json.Marshal(c)
	if err != nil {
		s.lock.Unlock()
		return err
	}
	println("Sending", data)
	err = websocket.Message.Send(s.conn, string(data))
	s.lock.Unlock()
	if err != nil {
		//        println("Websocket failed")
		s.close()
		return err
	}
	return nil
}

func (s *Window) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// Start starts the web server and returns after the UI has
// either been opened or if the UI could not be started.
func (s *Window) Start() error {
	// Start accepting incoming HTTP requests
	s.server = httptest.NewServer(s)

	// Launch the browser
	u := s.server.URL + "/?token=" + hex.EncodeToString(s.token)
	s.origin = s.server.URL
	println("URL", s.server.URL)
	// cmd := exec.Command("open", u)
	cmd := LaunchBrowser(u)
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}

	// Wait for someone to connect in time.
	// Otherwise close
	select {
	case <-s.connected:
		//            println("UI has connected")
		// Ok, do nothing
	case <-time.After(15 * time.Second):
		// Terminate the process, because the UI did not come up
		s.server.Close()
		return errors.New("UI could not be started")
	case <-s.end:
		// The server has been closed before a timeout and before
		// a successfull connect happened
		return errors.New("UI could not be started")
	}
	return nil
}

// Wait blocks until the user closed the browser window.
func (s *Window) Wait() {
	// Wait for the end
	<-s.end
}
