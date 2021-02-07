package goui

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// invocation is the message sent from client to server to invoke a function
type invocation struct {
	Name    string            `json:"n"`
	Message []json.RawMessage `json:"v"`
	ID      int               `json:"id"`
}

// resultMessage is the message sent from the server to the client in response
// to an invocation
type resultMessage struct {
	Value      interface{}   `json:"v,omitempty"`
	ArrayValue []interface{} `json:"a,omitempty"`
	Error      interface{}   `json:"e,omitempty"`
	ID         int           `json:"id"`
}

// Dispatcher can invoke functions on an object.
// A function call is passed in as a JSON message, the
// function is called and the result is returned as a JSON message.
type Dispatcher struct {
	funcs map[string]reflect.Method
	obj   interface{}
}

// GenerateJSCode creates a client-side javascript proxy for the object.
func GenerateJSCode(obj interface{}) string {
	var lines []string
	lines = append(lines, "window.go = (function() {")
	lines = append(lines, "var go = {")
	var apis []string
	t := reflect.TypeOf(obj)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		var api = strconv.Quote(m.Name) + ": async function("
		var params []string
		for i := 1; i < m.Type.NumIn(); i++ {
			//            it := m.Type.In(i)
			paramname := fmt.Sprintf("p%v", i)
			params = append(params, paramname)
		}
		api += strings.Join(params, ",")
		api += ") {\n"
		api += "return new Promise((ff, rej) => {"
		api += "  try {\n"
		api += fmt.Sprintf("    send({\"n\": %v, \"v\": [%v]}, ff, rej);\n", strconv.Quote(m.Name), strings.Join(params, ","))
		api += "  } catch(e) {\n"
		api += "    rej('RPC error');\n"
		api += "  }"
		api += "})}\n"
		apis = append(apis, api)
	}
	apis = append(apis, `
		data: null,
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
            if (initPromise) {
                return initPromise;
            }
            initPromise = new Promise((ff, rej) => {
                initFf = ff;
                initRej = rej;
            });
            connection = new WebSocket('ws://' + window.location.host + "/_socket");
            connection.onopen = function () {
                console.log('Welcome');
            };
    
            // Log errors
            connection.onerror = function (error) {
				console.log('WebSocket Error ' + error);
				if (initRej) {
					initRej();
				}
				document.body.innerHTML = "Application terminated<br>Close browser tab."
            };
    
            // Log messages from the server
            connection.onmessage = function (e) {
                console.log('Server: ' + e.data);
				var msg = JSON.parse(e.data);
				if (msg.m !== undefined) {
					applyDiff(window.go, "data", undefined, false, msg.m)
					if (!gotModel) {
						// We are ready, once the initial model has been retrieved.
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
        }
    `)
	lines = append(lines, strings.Join(apis, ",\n"))
	lines = append(lines, "};")
	lines = append(lines, `
        var initPromise = null;
        var initFf = null;
        var initRej = null;
        var pending = { };
        var counter = 0;
        var listeners = { };
        var connection = null;
		var gotModel = false;

        function send(msg, ff, rej) {
            counter++;
            msg.id = counter;
            pending[counter] = {ff: ff, rej: rej};
            connection.send(JSON.stringify(msg));
        }
	`)
	lines = append(lines, `
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
	`)
	lines = append(lines, "return go; })();")

	return strings.Join(lines, "\n")
}

// NewDispatcher creates a new dispatcher for the object
func NewDispatcher(obj interface{}) *Dispatcher {
	d := &Dispatcher{
		funcs: make(map[string]reflect.Method),
		obj:   obj,
	}

	t := reflect.TypeOf(obj)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		d.funcs[m.Name] = m
	}

	return d
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

// Dispatch decodes the JSON msg and invokes a function on the object.
// It returns a JSON encoded return message.
func (d *Dispatcher) Dispatch(msg []byte) ([]byte, error) {
	var inv invocation
	err := json.Unmarshal([]byte(msg), &inv)
	if err != nil {
		return nil, err
	}

	f, ok := d.funcs[inv.Name]
	if !ok {
		return nil, errors.New("Unknown method")
	}
	vals := make([]reflect.Value, f.Type.NumIn())
	vals[0] = reflect.ValueOf(d.obj)
	if f.Type.NumIn() != len(inv.Message)+1 {
		return nil, errors.New("Wrong parameter count")
	}
	for i := 1; i < f.Type.NumIn(); i++ {
		t := f.Type.In(i)
		switch t.Kind() {
		case reflect.Int:
			var v int
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Int8:
			var v int8
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Int16:
			var v int16
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Int32:
			var v int32
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Int64:
			var v int64
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Uint:
			var v uint
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Uint8:
			var v uint8
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Uint16:
			var v uint16
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Uint32:
			var v uint32
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Uint64:
			var v uint64
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Float32:
			var v float32
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Float64:
			var v float64
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Bool:
			var v bool
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.String:
			var v string
			json.Unmarshal(inv.Message[i-1], &v)
			vals[i] = reflect.ValueOf(v)
		case reflect.Map, reflect.Struct, reflect.Slice, reflect.Array:
			v := reflect.New(t)
			err = json.Unmarshal(inv.Message[i-1], v.Interface())
			if err != nil {
				return nil, err
			}
			vals[i] = v.Elem()
		case reflect.Ptr:
			v := reflect.New(t.Elem())
			err = json.Unmarshal(inv.Message[i-1], v.Interface())
			if err != nil {
				return nil, err
			}
			vals[i] = v
		default:
			return nil, errors.New("Type not supported")
		}
	}
	rets := f.Func.Call(vals)

	returnsErr := false
	if len(rets) > 0 {
		t := f.Type.Out(len(rets) - 1)
		if t.Kind() == reflect.Interface && t.Implements(errorInterface) {
			returnsErr = true
		}
	}
	returnsArr := len(rets) > 2 || (!returnsErr && len(rets) > 1)

	result := &resultMessage{
		ID: inv.ID,
	}
	// Did the function return a non-nil error?
	if returnsErr {
		err := (rets[len(rets)-1].Interface())
		if err != nil {
			result.Error = err.(error).Error()
		}
	}
	// In case of an error, do not marshal the other parameters
	if result.Error == nil {
		if returnsArr {
			if returnsErr {
				result.ArrayValue = make([]interface{}, len(rets)-1)
			} else {
				result.ArrayValue = make([]interface{}, len(rets))
			}
		}
		for i, r := range rets {
			if returnsErr && i+1 == len(rets) {
				// Do nothing by intention
			} else if i == 0 && !returnsArr {
				result.Value = r.Interface()
			} else {
				result.ArrayValue[i] = r.Interface()
			}
		}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return data, nil
}
