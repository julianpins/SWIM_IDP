package protocol

type TimerKind uint8

const (
	ProbeTimeout TimerKind = iota
	SuspicionTimeout
	PeriodEnd // end of protocol period -> next probe
)

type Timer struct {
	Kind   TimerKind
	Probe  uint64
	Member NodeID
}
