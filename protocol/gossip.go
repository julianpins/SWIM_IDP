package protocol

import (
	"cmp"
	"math"
	"slices"
	"strings"
)

// updates waiting for dissemination.
type broadcast struct {
	update        Update
	transmissions int
}

// queues new updates for dissemination.
func (n *Node) enqueue(u Update) {
	for i := range n.gossip {
		if n.gossip[i].update.Member == u.Member {
			n.gossip[i] = broadcast{update: u}
			return
		}
	}
	n.gossip = append(n.gossip, broadcast{update: u})
}

// attaches up to MaxUpdatesPerMessage least-transmitted updates to every Send.
func (n *Node) piggyback(out []Output) []Output {
	for i := range out {
		if out[i].Kind != Send || len(n.gossip) == 0 {
			continue
		}
		slices.SortFunc(n.gossip, func(a, b broadcast) int {
			if c := cmp.Compare(a.transmissions, b.transmissions); c != 0 {
				return c
			}
			return strings.Compare(string(a.update.Member), string(b.update.Member))
		})
		k := min(n.cfg.MaxUpdatesPerMessage, len(n.gossip))
		updates := make([]Update, k) // own copy: messages never alias the buffer
		for j := range k {
			updates[j] = n.gossip[j].update
			n.gossip[j].transmissions++
		}
		out[i].Msg.Updates = updates
		limit := n.retransmitLimit()
		n.gossip = slices.DeleteFunc(n.gossip, func(b broadcast) bool { return b.transmissions >= limit })
	}
	return out
}

// ⌈λ·ln(n)⌉ with n = non-Dead members including self; at least 1.
func (n *Node) retransmitLimit() int {
	live := 0
	for _, m := range n.members {
		if m.State != Dead {
			live++
		}
	}
	limit := int(math.Ceil(float64(n.cfg.RetransmitMultiplier) * math.Log(float64(live))))
	return max(limit, 1)
}

// applies piggybacked updates before message itself is handled.
func (n *Node) receiveUpdates(updates []Update) []Output {
	var out []Output
	for _, u := range updates {
		if u.Member == n.id {
			n.refute(u)
			continue
		}
		out = append(out, n.update(u)...)
	}
	return out
}

// Suspect about self at the current incarnation
func (n *Node) refute(u Update) {
	self := n.members[n.id]
	if u.State != Suspect || u.Incarnation < self.Incarnation {
		return
	}
	self.Incarnation = u.Incarnation + 1
	n.enqueue(Update{Member: n.id, State: Alive, Incarnation: self.Incarnation})
}
