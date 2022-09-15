package main

import (
	"embed"
	"net/http"

	"github.com/weistn/goui"
)

// All methods on this type can be invoked from the browser.
type WindowAPI struct {
}

// The window is a global variable for convenience.
var window *goui.Window

// Embed index.html directly in the Go executable.
//go:embed index.html
var fs embed.FS

// If the browser says Hello, send back a greeting.
func (api *WindowAPI) Hello(msg string) string {
	println("Hello:", msg)
	return "Hello browser"
}

func main() {
	// Configure a new window
	var api = &WindowAPI{}
	window = goui.NewWindow("/", api, nil)
	// Make index.html available to the window
	window.Handle("/", http.FileServer(http.FS(fs)))
	// Open the window
	err := window.Start()
	if err != nil {
		panic(err)
	}
	// Wait till the window has been closed
	window.Wait()
}
