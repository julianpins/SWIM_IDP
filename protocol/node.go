package protocol

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

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
	// Send
	To    NodeID
	Msg   Message
	// Set Timer
	After time.Duration
	Timer Timer
}

type Node struct {
	id         NodeID
	cfg        Config
	rng        Rand
	members    map[NodeID]*Member
	probeOrder []NodeID
	next       int
	probe      probe
	probeID    uint64
	relays     map[uint64]relay
}

func (c Config) Validate() error {
	if c.ProtocolPeriod <= 0 {
		return errors.New("protocol: ProtocolPeriod must be positive")
	}
	if c.ProbeTimeout <= 0 || c.ProbeTimeout >= c.ProtocolPeriod {
		return errors.New("protocol: need 0 < ProbeTimeout < ProtocolPeriod")
	}
	if c.SuspicionTimeout <= 0 {
		return errors.New("protocol: SuspicionTimeout must be positive")
	}
	if c.IndirectProbeCount < 0 {
		return errors.New("protocol: IndirectProbeCount must not be negative")
	}
	return nil
}

// members is the initial view (all Alive, incarnation 0); may include id.
func New(id NodeID, cfg Config, rng Rand, members []NodeID) (*Node, error) {
	if id == "" {
		return nil, errors.New("protocol: empty node ID")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if rng == nil {
		return nil, errors.New("protocol: rng is nil")
	}
	n := &Node{id: id, cfg: cfg, rng: rng, members: map[NodeID]*Member{id: {ID: id}}, relays: map[uint64]relay{}}
	for _, m := range members {
		if m == "" {
			return nil, errors.New("protocol: empty member ID")
		}
		if _, dup := n.members[m]; dup && m != id {
			return nil, fmt.Errorf("protocol: duplicate member %q", m)
		}
		n.members[m] = &Member{ID: m}
	}
	return n, nil
}


func (n *Node) Members() []Member {
	out := make([]Member, 0, len(n.members))
	for _, m := range n.members {
		out = append(out, *m)
	}
	slices.SortFunc(out, func(a, b Member) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return out
}

// start probing with the initial view.
func (n *Node) Start() []Output { return n.startProbing() }

// join existing group through contact.
func (n *Node) Join(contact NodeID) []Output { return nil }

func (n *Node) Receive(from NodeID, m Message) []Output {
	assert(from != "" && from != n.id, "Receive: invalid sender")
	switch m.Kind {
	case Ping:
		return n.onPing(from, m)
	case Ack:
		return n.onAck(from, m)
	case PingReq:
		return n.onPingReq(from, m)
	}
	assert(false, "Receive: unknown message kind")
	return nil
}

func (n *Node) Fire(t Timer) []Output {
	switch t.Kind {
	case PeriodEnd:
		return n.probeNext()
	case ProbeTimeout:
		return n.onProbeTimeout(t)
	case RelayTimeout:
		return n.onRelayTimeout(t)
	case SuspicionTimeout:
		return nil // suspicion
	}
	assert(false, "Fire: unknown timer kind")
	return nil
}
