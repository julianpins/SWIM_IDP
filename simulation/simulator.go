package simulation

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
)

type Config struct {
	Protocol      protocol.Config
	Network       NetworkConfig
	Nodes         []protocol.NodeID
	QueueCapacity int
}

// random streams derived from the seed.
const (
	streamNetwork   = 0
	streamSimulator = 1
	streamNodes     = 2
)

type simNode struct {
	id      protocol.NodeID
	node    *protocol.Node
	crashed bool
}

type Simulator struct {
	sched *scheduler
	net   *network
	nodes []simNode
	index map[protocol.NodeID]int
	trace Trace
}

// every node starts with the full member list at a random offset in [0, ProtocolPeriod).
func New(config Config, seed uint64) (*Simulator, error) {
	sched, err := newScheduler(config.QueueCapacity)
	if err != nil {
		return nil, err
	}
	net, err := newNetwork(config.Network, rand.New(rand.NewPCG(seed, streamNetwork)))
	if err != nil {
		return nil, err
	}
	s := &Simulator{sched: sched, net: net, index: make(map[protocol.NodeID]int)}

	offsets := rand.New(rand.NewPCG(seed, streamSimulator))
	for i, id := range config.Nodes {
		random := rand.New(rand.NewPCG(seed, streamNodes+uint64(i)))
		node, err := protocol.New(id, config.Protocol, random, config.Nodes)
		if err != nil {
			return nil, err
		}
		s.nodes = append(s.nodes, simNode{id: id, node: node})
		s.index[id] = i

		offset := time.Duration(offsets.Int64N(int64(config.Protocol.ProtocolPeriod)))
		if err := sched.push(Event{At: offset, Kind: Boot, Node: id}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// adds a fault at virtual time at.
func (s *Simulator) Schedule(at time.Duration, f Fault) error {
	return s.sched.push(Event{At: at, Kind: Inject, Fault: f})
}

// executes the next event; false when the queue is empty.
func (s *Simulator) Step() (bool, error) {
	e, ok := s.sched.pop()
	if !ok {
		return false, nil
	}
	s.trace = append(s.trace, Entry{Event: e})
	if e.Kind == Inject {
		return true, s.applyFault(e.Fault)
	}

	i := s.index[e.Node]
	n := &s.nodes[i]
	if n.crashed {
		return true, nil
	}

	var out []protocol.Output
	switch e.Kind {
	case Deliver:
		if !s.net.reachable(e.From, e.Node) {
			return true, nil // link cut while in flight
		}
		out = n.node.Receive(e.From, e.Msg)
	case Expire:
		out = n.node.Fire(e.Timer)
	case Boot:
		out = n.node.Start()
	}
	return true, s.apply(i, out)
}

// executes all events up to and including until, then sets now = until.
func (s *Simulator) Run(until time.Duration) error {
	for {
		e, ok := s.sched.peek()
		if !ok || e.At > until {
			break
		}
		if _, err := s.Step(); err != nil {
			return err
		}
	}
	s.sched.advance(until)
	return nil
}

func (s *Simulator) Now() time.Duration { return s.sched.now }

func (s *Simulator) Trace() Trace { return s.trace }

// turns a node's outputs into events, in order.
func (s *Simulator) apply(i int, out []protocol.Output) error {
	from := s.nodes[i].id
	for _, o := range out {
		var e Event
		switch o.Kind {
		case protocol.Send:
			delay, ok := s.net.route(from, o.To)
			if !ok {
				continue
			}
			e = Event{At: s.sched.now + delay, Kind: Deliver, Node: o.To, From: from, Msg: o.Msg}
		case protocol.SetTimer:
			e = Event{At: s.sched.now + o.After, Kind: Expire, Node: from, Timer: o.Timer}
		}
		if err := s.sched.push(e); err != nil {
			return err
		}
	}
	return nil
}

func (s *Simulator) applyFault(f Fault) error {
	switch f.Kind {
	case Crash:
		i, ok := s.index[f.Node]
		if !ok {
			return fmt.Errorf("crash: unknown node %q", f.Node)
		}
		s.nodes[i].crashed = true
	case Partition:
		s.net.partition(f.Groups)
	case HealPartition:
		s.net.heal()
	default:
		return fmt.Errorf("fault kind %d not implemented", f.Kind)
	}
	return nil
}
