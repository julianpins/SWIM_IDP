package simulation

import (
	"math/rand/v2"
	"time"
	"github.com/julianpins/SWIM_IDP/protocol"
)

type NetworkConfig struct {
	MinLatency time.Duration
	MaxLatency time.Duration
	LossRate   float64
}

//partition map
type link struct{ from, to protocol.NodeID }

type network struct {
	cfg     NetworkConfig
	rng     *rand.Rand
	blocked map[link]bool
}

func (n *network) route(from, to protocol.NodeID) (delay time.Duration, ok bool) {
	return 0, false
}

func (n *network) reachable(from, to protocol.NodeID) bool { return false }

func (n *network) partition(groups [][]protocol.NodeID) {}

func (n *network) heal() {}
