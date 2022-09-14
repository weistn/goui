# goui

goui is a library for attaching a HTML/JS UI to a native go application.
goui uses the operating system's default web browser to display the HTML/JS UI
and uses Go to implement the application logic.

A goui app behaves like a native app, except that is opens a new browser tab instead of opening a new main window upon launch.
This approach is more resource friendly compared to electron and friends, because it does not launch a separate browser and goui apps are small, because they do not need to ship a browser engine.

goui offers multiple mechanisms to communicate between the Go part
and the browser part of the application.
It is possible to **call Go functions** from the browser and to **call Javascript functions**
from Go.
Furthermore, Go can send **events** to the browser.
Finally, goui can **sync a data structure** with the browser.
Any changes applied to the Go data structure are automatically synced with the browser,
where the same data is available as JSON.
On the browser side, a UI framework such as Vue can be used to render a UI based on this JSON data.

## Technical Details and Security

The Go part of the application communicates with the browser via a web socket on the loopback device.
The initial URL contains a random token.
The first request to the Go side exchanges this token with a cookie.
All further communication between Go and browser are then authorized using this cookie.

## Lifecycle

When the browser window or tab is closed, the Go application terminates.
When the Go application is terminated first, the HTML/JS UI becomes disabled.
