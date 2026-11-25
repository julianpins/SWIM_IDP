package simulation

// Entry records one processed event. Event already carries At and Node.
type Entry struct {
	Event Event
}

type Trace []Entry

// for comparing runs
func (t Trace) Hash() [32]byte { return [32]byte{} }
