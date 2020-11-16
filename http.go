package goui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// HTTPServer handles HTTP requests for static content
type HTTPServer struct {
	token      []byte
	cookie     string
	mux        *http.ServeMux
	server     *httptest.Server
	end        chan bool
	connected  chan bool
	remote     interface{}
	dispatcher *Dispatcher
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

// NewHTTPServer creates a new HTTP server.
// Call Start() to run the server on a system-chosen port via
// the loopback-device and to launch the UI in the browser.
func NewHTTPServer(mux *http.ServeMux, remote interface{}) *HTTPServer {
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

	s := &HTTPServer{
		mux:        mux,
		cookie:     hex.EncodeToString(cookie),
		token:      token,
		end:        make(chan bool),
		connected:  make(chan bool),
		remote:     remote,
		dispatcher: NewDispatcher(remote),
	}

	ws := &websocket.Server{
		Handshake: func(config *websocket.Config, r *http.Request) error { return s.handshake(config, r) },
		Handler:   func(conn *websocket.Conn) { s.wshandler(conn) },
	}
	mux.Handle("/_socket", ws)
	mux.HandleFunc("/_rpc.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(GenerateJSCode(s.remote)))
	})
	return s
}

// ServeHTTP handles incoming HTTP requests
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//    println("Request ...", r.URL.String())
	if s.token != nil {
		v := r.URL.Query()
		// The first request must use the secret token and request "/"
		if r.URL.Path == "/" && v.Get("token") == hex.EncodeToString(s.token) {
			s.token = nil
			http.SetCookie(w, &http.Cookie{Name: "secret", Value: s.cookie})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		//        println("Wrong initial request: ", r.URL.String())
		// This is the wrong initial request.
		// Might be an attack. Stop the process.
		s.close()
		return
	}

	// Access control via cookie
	c, err := r.Cookie("secret")
	if c == nil || err != nil || c.Value != s.cookie {
		//        println("Missing cookie")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	// Now deliver content as requested
	s.mux.ServeHTTP(w, r)
}

func (s *HTTPServer) close() {
	s.server.CloseClientConnections()
	s.end <- true
	s.server.Close()
}

// Accept incoming websockets and allow for RPC
func (s *HTTPServer) handshake(config *websocket.Config, r *http.Request) error {
	if s.conn != nil {
		//        println("Double websocket connection")
		return errors.New("Unauthorized")
	}
	c, err := r.Cookie("secret")
	if c == nil || err != nil || c.Value != s.cookie {
		//        println("Illegal websocket connect")
		return errors.New("Unauthorized")
	}
	//    println("Websocket connect ok")
	return nil
}

func (s *HTTPServer) wshandler(conn *websocket.Conn) {
	s.conn = conn
	//    println("Websocket connected")
	s.connected <- true
	for {
		var msg string
		err := websocket.Message.Receive(conn, &msg)
		if err != nil {
			//            println("Websocket failed")
			s.close()
			return
		}
		println("Got data:", msg)
		result, err := s.dispatcher.Dispatch([]byte(msg))

		s.lock.Lock()
		if err != nil {
			err = websocket.Message.Send(conn, `{"err": "Internal error"}`)
		} else {
			println("Sending:", string(result))
			err = websocket.Message.Send(conn, string(result))
		}
		s.lock.Unlock()
		if err != nil {
			//            println("Websocket failed")
			s.close()
			return
		}
	}
}

// SendEvent sends an event to the browser
func (s *HTTPServer) SendEvent(name string, event interface{}) error {
	s.lock.Lock()
	if s.conn == nil {
		s.lock.Unlock()
		return errors.New("Not connected")
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

// SendCall invokes a function in the browser.
// SendCall is async, i.e. it does not wait for the browser to complete the function call
// and the result is not transmitted back to the server
func (s *HTTPServer) SendCall(fname string, args ...interface{}) error {
	s.lock.Lock()
	if s.conn == nil {
		s.lock.Unlock()
		return errors.New("Not connected")
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

// Start starts the web server and returns after the UI has
// either been opened or if the UI could not be started.
func (s *HTTPServer) Start() error {
	// Start accepting incoming HTTP requests
	s.server = httptest.NewServer(s)

	// Launch the browser
	u := s.server.URL + "/?token=" + hex.EncodeToString(s.token)
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
func (s *HTTPServer) Wait() {
	// Wait for the end
	<-s.end
}
