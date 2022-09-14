package goui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	_ "embed"
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

//go:embed js/rpc.js
var rpcjs string

// GenerateJSCode creates a client-side javascript proxy for the object.
func GenerateJSCode(obj interface{}) string {
	// Generate JS stubs for all exported Go functions
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
	var api = strings.Join(apis, ",")

	tmpl := template.Must(template.New("_rpc.js").Parse(rpcjs))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, api); err != nil {
		panic(err)
	}
	return buf.String()
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
func (d *Dispatcher) Dispatch(inv *invocation) ([]byte, error) {
	println("Invoke", inv.Name)

	f, ok := d.funcs[inv.Name]
	if !ok {
		return nil, errors.New("unknown method")
	}
	vals := make([]reflect.Value, f.Type.NumIn())
	vals[0] = reflect.ValueOf(d.obj)
	if f.Type.NumIn() != len(inv.Message)+1 {
		return nil, errors.New("wrong parameter count")
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
			err := json.Unmarshal(inv.Message[i-1], v.Interface())
			if err != nil {
				return nil, err
			}
			vals[i] = v.Elem()
		case reflect.Ptr:
			v := reflect.New(t.Elem())
			err := json.Unmarshal(inv.Message[i-1], v.Interface())
			if err != nil {
				return nil, err
			}
			vals[i] = v
		default:
			return nil, errors.New("type not supported")
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
