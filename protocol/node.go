package protocol

import "time"

type NodeID string

type Rand interface {
	IntN(n int) int
}

type Config struct {
	ProtocolPeriod     time.Duration
	ProbeTimeout       time.Duration
	SuspicionTimeout   time.Duration
	IndirectProbeCount int
}

type OutputKind uint8

const (
	Send OutputKind = iota
	SetTimer
)

// environment effect.
type Output struct {
	Kind  OutputKind
	To    NodeID        // Send
	Msg   Message       // Send
	After time.Duration // SetTimer
	Timer Timer         // SetTimer
}

type Node struct {
	id      	NodeID
	cfg     	Config
	rng     	Rand
	members 	map[NodeID]*Member
	probe_order	[]NodeID
	next    	int
}

func New(id NodeID, cfg Config, rng Rand) *Node         { return nil }
func (n *Node) Members() []Member                       { return nil }

func (n *Node) Join(seeds []NodeID) []Output            { return nil }
func (n *Node) Receive(from NodeID, m Message) []Output { return nil }
func (n *Node) Fire(t Timer) []Output                   { return nil }
