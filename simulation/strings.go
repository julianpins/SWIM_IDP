package simulation

import "fmt"

// String methods for printing; %#v (Trace.Hash) does not use them.
// MarshalText makes JSON use the names instead of numbers.

func (k EventKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }
func (k FaultKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

func (k EventKind) String() string {
	switch k {
	case Deliver:
		return "Deliver"
	case Expire:
		return "Expire"
	case Inject:
		return "Inject"
	case Boot:
		return "Boot"
	}
	return fmt.Sprintf("EventKind(%d)", uint8(k))
}

func (k FaultKind) String() string {
	switch k {
	case Crash:
		return "Crash"
	case Restart:
		return "Restart"
	case Partition:
		return "Partition"
	case HealPartition:
		return "HealPartition"
	}
	return fmt.Sprintf("FaultKind(%d)", uint8(k))
}
