package tusd

// StoreComposer represents a composable data store. It consists of the core
// data store and optional extensions. Please consult the package's overview
// for a more detailed introduction in how to use this structure.
type StoreComposer struct {
	Core DataStore

	UsesTerminater     bool
	Terminater         TerminaterDataStore
	UsesFinisher       bool
	Finisher           FinisherDataStore
	UsesLocker         bool
	Locker             LockerDataStore
	UsesGetReader      bool
	GetReader          GetReaderDataStore
	UsesConcater       bool
	Concater           ConcaterDataStore
	UsesLengthDeferrer bool
	LengthDeferrer     LengthDeferrerDataStore
}

// NewStoreComposer creates a new and empty store composer.
func NewStoreComposer() *StoreComposer {
	return &StoreComposer{}
}

// newStoreComposerFromDataStore creates a new store composer and attempts to
// extract the extensions for the provided store. This is intended to be used
// for transitioning from data stores to composers.
func newStoreComposerFromDataStore(store DataStore) *StoreComposer {
	composer := NewStoreComposer()
	composer.UseCore(store)

	if mod, ok := store.(TerminaterDataStore); ok {
		composer.UseTerminater(mod)
	}
	if mod, ok := store.(FinisherDataStore); ok {
		composer.UseFinisher(mod)
	}
	if mod, ok := store.(LockerDataStore); ok {
		composer.UseLocker(mod)
	}
	if mod, ok := store.(GetReaderDataStore); ok {
		composer.UseGetReader(mod)
	}
	if mod, ok := store.(ConcaterDataStore); ok {
		composer.UseConcater(mod)
	}
	if mod, ok := store.(LengthDeferrerDataStore); ok {
		composer.UseLengthDeferrer(mod)
	}

	return composer
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
	str += ` Finisher: `
	if store.UsesFinisher {
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
	str += ` GetReader: `
	if store.UsesGetReader {
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
func (store *StoreComposer) UseFinisher(ext FinisherDataStore) {
	store.UsesFinisher = ext != nil
	store.Finisher = ext
}
func (store *StoreComposer) UseLocker(ext LockerDataStore) {
	store.UsesLocker = ext != nil
	store.Locker = ext
}
func (store *StoreComposer) UseGetReader(ext GetReaderDataStore) {
	store.UsesGetReader = ext != nil
	store.GetReader = ext
}
func (store *StoreComposer) UseConcater(ext ConcaterDataStore) {
	store.UsesConcater = ext != nil
	store.Concater = ext
}

func (store *StoreComposer) UseLengthDeferrer(ext LengthDeferrerDataStore) {
	store.UsesLengthDeferrer = ext != nil
	store.LengthDeferrer = ext
}
