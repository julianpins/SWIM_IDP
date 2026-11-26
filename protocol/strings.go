package protocol

import "fmt"

// String methods for logs and traces; %#v (Trace.Hash) does not use them.
// MarshalText makes JSON use the names instead of numbers.

func (s State) MarshalText() ([]byte, error)       { return []byte(s.String()), nil }
func (k MessageKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }
func (k TimerKind) MarshalText() ([]byte, error)   { return []byte(k.String()), nil }

func (s State) String() string {
	switch s {
	case Alive:
		return "Alive"
	case Suspect:
		return "Suspect"
	case Dead:
		return "Dead"
	}
	return fmt.Sprintf("State(%d)", uint8(s))
}

func (k MessageKind) String() string {
	switch k {
	case Ping:
		return "Ping"
	case PingReq:
		return "PingReq"
	case Ack:
		return "Ack"
	}
	return fmt.Sprintf("MessageKind(%d)", uint8(k))
}

func (k TimerKind) String() string {
	switch k {
	case ProbeTimeout:
		return "ProbeTimeout"
	case SuspicionTimeout:
		return "SuspicionTimeout"
	case PeriodEnd:
		return "PeriodEnd"
	case RelayTimeout:
		return "RelayTimeout"
	}
	return fmt.Sprintf("TimerKind(%d)", uint8(k))
}
