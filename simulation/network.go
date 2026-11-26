package simulation

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
	"github.com/julianpins/SWIM_IDP/protocol"
)

type NetworkConfig struct {
	LatencyMin      time.Duration
	LatencyMax      time.Duration
	LossProbability float64
}

//connection between two nodes
type link struct{ from, to protocol.NodeID }

type network struct {
	config  NetworkConfig
	random  *rand.Rand
	blocked map[link]bool
}

func newNetwork(config NetworkConfig, random *rand.Rand) (*network, error) {
	if config.LatencyMin < 0 || config.LatencyMax < config.LatencyMin {
		return nil, fmt.Errorf("network: need 0 <= LatencyMin <= LatencyMax, got %v..%v",
			config.LatencyMin, config.LatencyMax)
	}
	if config.LossProbability < 0 || config.LossProbability > 1 {
		return nil, fmt.Errorf("network: LossProbability must be in [0, 1], got %v",
			config.LossProbability)
	}
	if random == nil {
		return nil, errors.New("network: random is nil")
	}
	return &network{config: config, random: random, blocked: make(map[link]bool)}, nil
}

//decides message delay and if message is dropped (respects partitions).
func (n *network) route(from, to protocol.NodeID) (delay time.Duration, ok bool) {
	assert(from != "" && to != "", "route: empty node ID")
	assert(from != to, "route: node sends to itself")

	lost := n.random.Float64() < n.config.LossProbability
	delay = n.config.LatencyMin + time.Duration(n.random.Int64N(int64(n.config.LatencyMax-n.config.LatencyMin) + 1))
	assert(delay >= n.config.LatencyMin && delay <= n.config.LatencyMax, "route: delay out of range")

	if lost || !n.reachable(from, to) {
		return 0, false
	}
	return delay, true
}

// returns whether from can currentyl reach to (not blocked by partition).
func (n *network) reachable(from, to protocol.NodeID) bool {
	return !n.blocked[link{from, to}]
}

// replaces the current partition: link between nodes in different groups is blocked 
// Links within a group, and links to nodes in no group stay open.
func (n *network) partition(groups [][]protocol.NodeID) {
	assert(len(groups) >= 2, "partition: needs at least two groups")
	n.heal()
	for i, group := range groups {
		for j, other := range groups {
			if i == j {
				continue
			}
			for _, from := range group {
				for _, to := range other {
					n.blocked[link{from, to}] = true
				}
			}
		}
	}
}

// heal unblocks all links.
func (n *network) heal() {
	clear(n.blocked)
}
