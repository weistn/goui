package goui

import (
    "testing"
)

type point struct {
    X int `json:"x"`
    Y int `json:"y"`
}

// Demo is
type Demo struct {
}

func TestReflect(t *testing.T) {
    d := &Demo{}
    disp := NewDispatcher(d)

    j := `
        {"n": "Foo2", "v": [42, "huhu", {"x": 12, "y": 24}, {"hudel": 13, "dudel": 16}, {"x": 123, "y": 456}, [4,5,6], [10,11,12]]}
    `
    _, err := disp.Dispatch([]byte(j))
    if err != nil {
        t.Fatal(err)
    }

    j = `{"n": "Foo1", "v": []}`
    _, err = disp.Dispatch([]byte(j))
    if err != nil {
        t.Fatal(err)
    }
}

// Foo1 is
func (d *Demo) Foo1() {
    println("I have been called")
}

// Foo2 is
func (d *Demo) Foo2(x int, s string, p *point, m map[string]int, p2 point, sl []int, a [3]int) (string, float32) {
    println("I have been called with number", x, s, p.X, p.Y, p2.X, p2.Y)
    for k, v := range m {
        println(k, v)
    }
    for _, i := range sl {
        println(i)
    }
    for _, i := range a {
        println(i)
    }
    return "The end", 3.14
}
