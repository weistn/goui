package goui

// ModelState describes the synchronization state of a Model object
type ModelState int

const (
	// ModelNew means that the Model object has not yet been synched
	ModelNew ModelState = 0
	// ModelDirty means that fields of the Model object have been modified
	// and need synchronization.
	// In addition, some child Models may be dirty as well.
	ModelDirty ModelState = 1
	// ModelChildDirty means that a child (or indirect child) Model is dirty.
	ModelChildDirty ModelState = 2
	// ModelSynced means that the Model and all of its children is synched.
	ModelSynced ModelState = 3
)

// ModelIface is implemented by Model.
type ModelIface interface {
	ModelDirty()
	ModelChildDirty()
	ModelSynced()
	ModelState() ModelState
	ModelTestSync(parent ModelIface, field *Field) ModelState
	ModelSwapIndex(index int) int
}

// Model implements synchronization between the GO Model and the JavaScript Model in the browser.
// To use Model, build a model like this:
//
// type MyModel struct {
//     Model
//     Value int
//     OtherValue string
// }
//
// It is possible to build a hierarchy of models.
// A model can contain another child model or point to a child model.
// The graphs of models must form a tree at all times.
//
// type RootModel struct {
//     Model
//     M1 *MyModel
//     M2 MyModel
// }
type Model struct {
	state  ModelState
	field  *Field
	parent ModelIface
	index  int
}

// ModelDirty marks the object as requiring synchronization.
// The parent Models are marked with ModelChildDirty.
func (m *Model) ModelDirty() {
	if m.state == ModelSynced {
		m.state = ModelDirty
		if m.parent != nil {
			m.parent.ModelChildDirty()
		}
	}
}

// ModelChildDirty marks that a child model requires synchronization.
func (m *Model) ModelChildDirty() {
	if m.state == ModelSynced {
		m.state = ModelChildDirty
		if m.parent != nil {
			m.parent.ModelChildDirty()
		}
	}
}

// ModelSynced marks the object as being successfully synched.
// This does not affect parent or child models.
func (m *Model) ModelSynced() {
	m.state = ModelSynced
}

// ModelState returns the synchronization state of the Model object.
func (m *Model) ModelState() ModelState {
	if m == nil {
		return ModelSynced
	}
	return m.state
}

// ModelTestSync returns the resulting synchronization state of the Model object.
// If a model object is moved to another parent of another field of the same parent,
// then the model object will be resynchronized.
func (m *Model) ModelTestSync(parent ModelIface, field *Field) ModelState {
	if m.state == ModelSynced || m.state == ModelChildDirty || m.state == ModelDirty {
		if m.parent != parent || m.field != field {
			m.state = ModelNew
			m.parent = parent
			m.field = field
		}
	} else if m.state == ModelNew {
		m.parent = parent
		m.field = field
	}
	return m.state
}

// ModelSwapIndex returns the current index of the model object inside its
// containing array. This index is replace with the new index passed
// as parameter to this function.
func (m *Model) ModelSwapIndex(index int) int {
	i := m.index
	m.index = index
	return i
}
