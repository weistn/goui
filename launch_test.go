package goui

import (
	"errors"
	"net/http"
	"testing"
)

type MyRemote struct {
}

func (r *MyRemote) Foo() {
	println("Foo has been called")
}

func (r *MyRemote) FooErr(i int) (int, error) {
	if i > 0 {
		return 2 * i, nil
	}
	return 0, errors.New("Value is negative")
}

func (r *MyRemote) FooArr(i int, j int) (int, int) {
	return 2 * i, 2 * j
}

func (r *MyRemote) FooArrErr(i int, j int) (int, int, error) {
	if i > 0 {
		return 2 * i, 2 * j, nil
	}
	return 0, 0, errors.New("Value is negative")
}

func (r *MyRemote) Names(age map[string]int) (string, int) {
	result := 0
	concat := ""
	for n, a := range age {
		result += a
		concat += n + " "
	}
	return concat, result
}

func TestLaunch(t *testing.T) {
	remote := &MyRemote{}
	mux := http.NewServeMux()
	// Serve content
	mux.Handle("/", http.FileServer(http.Dir("./test_data")))

	s := NewHTTPServer(mux, remote)
	err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SendEvent("greet", "Guten Tag"); err != nil {
		t.Fatal(err)
	}
	if err := s.SendCall("sayHello", "Joe Doe"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
}
