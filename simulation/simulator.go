package simulation

import (
	"time"
	"github.com/julianpins/SWIM_IDP/protocol"
)

type Config struct {
	Protocol protocol.Config
	Network  NetworkConfig
	Nodes    []protocol.NodeID
}

type Simulator struct {
	sched   scheduler
	net     network
	ids     []protocol.NodeID
	nodes   map[protocol.NodeID]*protocol.Node
	crashed map[protocol.NodeID]bool
	trace   Trace
}


func New(cfg Config, seed uint64) *Simulator            { return nil }

//add fault to queue
func (s *Simulator) Schedule(at time.Duration, f Fault) {}

//pop events
func (s *Simulator) Step() bool                         { return false } // one event

func (s *Simulator) Run(until time.Duration)            {}

func (s *Simulator) Trace() Trace                       { return nil }

// turns node's outputs into events
func (s *Simulator) apply(from protocol.NodeID, out []protocol.Output) {}
