package simulation

import "github.com/julianpins/SWIM_IDP/protocol"

type FaultKind uint8

const (
	Crash FaultKind = iota
	Restart
	Partition
	HealPartition
)

type Fault struct {
	Kind   FaultKind
	Node   protocol.NodeID
	Groups [][]protocol.NodeID
}
