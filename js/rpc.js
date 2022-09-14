window.go = (function() {
    var initPromise = null;
    var initFf = null;
    var initRej = null;
    var pending = { };
    // Message counter
    var counter = 0;
    var listeners = { };
    var connection = null;
    var gotModel = false;
    var queue = [];
    var reconnectCount = 0;

    addEventListener("beforeunload", beforeUnload);

    // Signal the application process (if possible) that the UI is going away.
    // The application process will terminate when the UI is gone.
    function beforeUnload(ev) {
        if (connection) {
            connection.send(JSON.stringify({n: "goui:gui_terminated"}))
        }
    }

    function send(msg, ff, rej) {
        counter++;
        msg.id = counter;
        pending[counter] = {ff: ff, rej: rej};
        if (connection) {
            connection.send(JSON.stringify(msg));
        } else {
            println("Queue")
            queue.push(JSON.stringify(msg));
        }
    }

    function applyDiff(parent, prop, index, ins, diff) {
        var value
        if (diff === null) {
            value = null
        } else if (typeof(diff) === "object") {
            if (diff._a !== undefined) {
                // Modify an array
                var arr = parent[prop]
                var cloned = null
                // Chop the array when necessary
                if (arr.length != diff._l) {
                    arr.splice(diff._l, arr.length - diff._l)
                }
                var pos = arr.length
                var insertCount = 0
                for (let i = diff._a.length - 1; i >= 0; i--) {
                    var e = diff._a[i]
                    if (typeof(e) === "number") {
                        pos -= e
                    } else if (e._d !== undefined) {
                        if (cloned === null) {
                            cloned = [...arr]
                        }
                        pos -= e._d
                        arr.splice(pos, e._d)
                    } else if (e._i !== undefined) {
                        insertCount = e._i
                    } else if (e._c !== undefined) {
                        if (cloned === null) {
                            cloned = [...arr]
                        }
                        arr.splice(pos, 0, ...(clone.slice(e._c, e._c + e._l)))
                    } else if (e._t !== undefined) {
                        if (cloned === null) {
                            cloned = [...arr]
                        }
                        arr.splice(pos, 0, ...(clone.slice(e._c, e._c + e._l)))
                        applyDiff(arr, undefined, pos, false, e._v)
                    } else {
                        if (insertCount > 0) {
                            applyDiff(arr, undefined, pos, true, e)
                            insertCount--
                        } else {
                            pos--
                            applyDiff(arr, undefined, pos, false, e)
                        }
                    }
                }
                return
            } else if (diff._id !== undefined) {
                // The value is an object literal
                value = diff
            } else {
                // Modify an object
                for (let key of Object.keys(diff)) {
                    if (index === undefined) {
                        applyDiff(parent[prop], key, undefined, false, diff[key])
                    } else {
                        applyDiff(parent[index], key, undefined, false, diff[key])
                    }
                }
                return
            }
        } else if (Array.isArray(diff)) {
            // The value is an array literal
            value = diff
        } else {
            // The value is a primitive literal
            value = diff
        }
    
        // Set the property or list element
        if (index === undefined) {
            // Set property
            parent[prop] = diff
        } else {
            // Insert or replace an array element.
            // Use splice here to ensure vue.js compatibility
            if (ins) {
                parent.splice(index, 0, diff)
            } else {
                parent.splice(index, 1, diff)
            }
        }
    }

    function serverIsGone() {
        if (initRej) {
            initRej();
        }

        // Emit an event
        var ev = {preventDefault: false};
        var l = listeners["goui:process_terminated"];
        if (l) {
            for (var i = 0; i < l.length; i++) {
                l[i](ev);
            }
        }
        // By default, remove the API since the application terminated
        if (!ev.preventDefault) {
            document.body.innerHTML = "Application terminated<br>Close browser tab.";
        }
    }

    // Some browsers close websockets when the computer enters power safe mode or upon inactivity.
    // In this case reconnect tries several times to force a new websocket connection.
    // In case this fails repeatedly, the application process is no longer responding and severIsGone() is invoked.
    async function reconnect() {
        console.log("Trying to reconnect", reconnectCount);
        reconnectCount++;
        // After a maximum amount of retries, give up
        if (reconnectCount == 10) {
            serverIsGone();
            return;
        }

        await api.connect();
        if (connection) {
            return
        }

        // Try again
        window.setTimeout(reconnect, 5000);
    }

    var api = {
        data: null,
        // The event "goui:process_terminated" is emitted by goui when the application process terminates.
        // The default is to remove the UI, but event listeners can keep the UI visible if desired.
        // All other events are emitted by the application process.
        addEventListener : function(name, cb) {
            if (!listeners[name]) {
                listeners[name] = [ ];
            }
            listeners[name].push(cb);
        },
        removeEventListener : function(name, cb) {
            var arr = listeners[name];
            if (!arr) {
                return;
            }
            for (var i = 0; i < arr.length; i++) {
                if (arr[i] == cb) {
                    arr.splice(i, 1);
                    return;
                }
            }
        },
        connect: async function() {
            //if (initPromise) {
            //    return initPromise;
            //}
            initPromise = new Promise((ff, rej) => {
                initFf = ff;
                initRej = rej;
            });

            connection = new WebSocket('ws://' + window.location.host + "/_socket");

            connection.onopen = function () {
                console.log('Welcome');
                reconnectCount = 0;
                // Send queued data
                while (queue && queue.length > 0) {
                    connection.send(queue[0]);
                    queue.shift();
                }
            };

            connection.onerror = function (error) {
                console.log('WebSocket Error ' + error);
                connection = null;
                if (initRej) {
                    initRej();
                }
                document.body.innerHTML = "Application terminated<br>Close browser tab."
            };

            connection.onclose = function (ev) {
                console.log('WebSocket close ' + ev.code);
                connection = null;

                if (ev.code == 1006) {
                    window.setTimeout(reconnect, 1000);
                }
            };

            connection.onmessage = function (e) {
                console.log('Server: ' + e.data);
                var msg = JSON.parse(e.data);
                if (msg.m !== undefined) {
                    applyDiff(window.go, "data", undefined, false, msg.m)
                    if (!gotModel) {
                        // We are ready, because the initial model has been retrieved.
                        gotModel = true
                        initFf();
                        initFf = null
                        initRej = null
                    }
                } else if (msg.n !== undefined) {
                    var arr = listeners[msg.n]
                    if (arr) {
                        for (var i = 0; i < arr.length; i++) {
                            arr[i](msg.ev);
                        }
                    }
                } else if (msg.f !== undefined) {
                    if (!window[msg.f]) {
                        console.log("Server is calling unknown function", msg.f);
                        return;
                    }
                    window[msg.f].apply(null, msg.a);
                } else {
                    var p = pending[msg.id];
                    if (!p) {
                        console.log("Unexpected answer");
                    }
                    delete pending[msg.id];
                    if (msg.e !== undefined) {
                        p.rej(msg.e);
                    } else if (msg.a !== undefined) {
                        p.ff(msg.a);
                    } else {
                        p.ff(msg.v);
                    }
                }
            };
            return initPromise;
        },
        {{ . }}
    };

    return api;
})();
