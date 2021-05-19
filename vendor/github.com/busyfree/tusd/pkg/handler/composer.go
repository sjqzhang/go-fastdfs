package handler

// StoreComposer represents a composable data store. It consists of the core
// data store and optional extensions. Please consult the package's overview
// for a more detailed introduction in how to use this structure.
type StoreComposer struct {
	Core DataStore

	UsesTerminater     bool
	Terminater         TerminaterDataStore
	UsesLocker         bool
	Locker             Locker
	UsesConcater       bool
	Concater           ConcaterDataStore
	UsesLengthDeferrer bool
	LengthDeferrer     LengthDeferrerDataStore
}

// NewStoreComposer creates a new and empty store composer.
func NewStoreComposer() *StoreComposer {
	return &StoreComposer{}
}

// Capabilities returns a string representing the provided extensions in a
// human-readable format meant for debugging.
func (store *StoreComposer) Capabilities() string {
	str := "Core: "

	if store.Core != nil {
		str += "✓"
	} else {
		str += "✗"
	}

	str += ` Terminater: `
	if store.UsesTerminater {
		str += "✓"
	} else {
		str += "✗"
	}
	str += ` Locker: `
	if store.UsesLocker {
		str += "✓"
	} else {
		str += "✗"
	}
	str += ` Concater: `
	if store.UsesConcater {
		str += "✓"
	} else {
		str += "✗"
	}
	str += ` LengthDeferrer: `
	if store.UsesLengthDeferrer {
		str += "✓"
	} else {
		str += "✗"
	}

	return str
}

// UseCore will set the used core data store. If the argument is nil, the
// property will be unset.
func (store *StoreComposer) UseCore(core DataStore) {
	store.Core = core
}

func (store *StoreComposer) UseTerminater(ext TerminaterDataStore) {
	store.UsesTerminater = ext != nil
	store.Terminater = ext
}

func (store *StoreComposer) UseLocker(ext Locker) {
	store.UsesLocker = ext != nil
	store.Locker = ext
}

func (store *StoreComposer) UseConcater(ext ConcaterDataStore) {
	store.UsesConcater = ext != nil
	store.Concater = ext
}

func (store *StoreComposer) UseLengthDeferrer(ext LengthDeferrerDataStore) {
	store.UsesLengthDeferrer = ext != nil
	store.LengthDeferrer = ext
}
