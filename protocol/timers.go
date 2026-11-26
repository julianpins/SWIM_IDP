package protocol

type TimerKind uint8

const (
	ProbeTimeout TimerKind = iota
	SuspicionTimeout
	PeriodEnd
	RelayTimeout
)

type Timer struct {
	Kind        TimerKind
	Probe       uint64
	Member      NodeID
	Incarnation uint64
}
