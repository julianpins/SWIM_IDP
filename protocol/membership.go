package protocol

type State uint8

const (
	Alive State = iota
	Suspect
	Dead
)

type Member struct {
	ID          NodeID
	State       State
	Incarnation uint64
}
