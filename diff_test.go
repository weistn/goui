package goui

import (
	"testing"
)

type MyModel struct {
	Model
	Age      int
	Details  *DetailsModel
	Details2 *DetailsModel
	Embed    EmbedModel
	List     []*DetailsModel
}

type DetailsModel struct {
	Model
	Name string
}

type EmbedModel struct {
	Model
	Name string
	More DetailsModel
}

func TestDiff(t *testing.T) {
	m := &MyModel{}
	m.Age = 42
	m.Details = &DetailsModel{Name: "Joe"}
	m.Embed.Name = "Embedded"
	m.Embed.More.Name = "Sub-Embedded"
	m.List = make([]*DetailsModel, 2)
	m.List[0] = &DetailsModel{Name: "Elem 1"}
	m.List[1] = &DetailsModel{Name: "Elem 2"}

	data, err := MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.Age = 30
	m.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.Details.Name = "Jack"
	m.Details.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.Details2 = m.Details
	m.Details = nil
	m.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.Embed.More.Name = "Subby"
	m.Embed.More.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.List[0].Name = "Changed 0"
	m.List[0].ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.List[1].Name = "Changed 1"
	m.List[1].ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	lst := make([]*DetailsModel, 5)
	lst[0] = &DetailsModel{Name: "Ins 0"}
	lst[2] = m.List[0]
	lst[3] = &DetailsModel{Name: "Ins 1"}
	lst[4] = &DetailsModel{Name: "Ins 2"}
	m.List = lst
	m.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.List = m.List[1:]
	m.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))

	m.List = m.List[1:]
	m.ModelDirty()

	data, err = MarshalDiff(m)
	if err != nil {
		t.Fail()
	}
	println(string(data))
}
