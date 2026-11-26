package simulation

import (
	"crypto/sha256"
	"fmt"
)

// one processed event; Event already carries At and Node.
type Entry struct {
	Event Event
}

type Trace []Entry

// sha256 over the Go-syntax form of every entry.
func (t Trace) Hash() [32]byte {
	h := sha256.New()
	for _, e := range t {
		fmt.Fprintf(h, "%#v\n", e)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
