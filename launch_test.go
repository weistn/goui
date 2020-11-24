package goui

import (
	"errors"
	"net/http"
	"testing"
)

type MyRemote struct {
	model *MyModel
}

func (r *MyRemote) DoMe() {
	println("DoMe has been called")
	r.model.Age = 101
	r.model.List[0].Name = "Changed"
	r.model.List[0].ModelDirty()
	r.model.ModelDirty()
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
	m := &MyModel{}
	m.Age = 42
	m.Details = &DetailsModel{Name: "Joe"}
	m.Embed.Name = "Embedded"
	m.Embed.More.Name = "Sub-Embedded"
	m.List = make([]*DetailsModel, 2)
	m.List[0] = &DetailsModel{Name: "Elem 1"}
	m.List[1] = &DetailsModel{Name: "Elem 2"}
	remote := &MyRemote{model: m}

	mux := http.NewServeMux()
	// Serve content
	mux.Handle("/", http.FileServer(http.Dir("./test_data")))

	s := NewHTTPServer(mux, remote, m)
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
