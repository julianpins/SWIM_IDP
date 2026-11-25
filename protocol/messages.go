package protocol

type MessageKind uint8

const (
	Ping MessageKind = iota
	PingReq
	Ack
)

type Message struct {
	Kind    MessageKind
	Probe   uint64
	Target  NodeID
	Updates []Update
}

type Update struct {
	Member      NodeID
	State       State
	Incarnation uint64
}
