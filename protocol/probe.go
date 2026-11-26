package protocol

import "slices"

type probe struct {
	id             uint64 // 0: no probe open
	target         NodeID
	acked          bool
	intermediaries []NodeID // set on ProbeTimeout
}

// a Ping sent on behalf of a PING-REQ, keyed by our own probe ID.
type relay struct {
	requester      NodeID
	requesterProbe uint64
	target         NodeID
}

// builds the probe order and starts the first period.
func (n *Node) startProbing() []Output {
	assert(n.probeOrder == nil, "Start: already started")
	for id := range n.members {
		if id != n.id {
			n.probeOrder = append(n.probeOrder, id)
		}
	}
	slices.Sort(n.probeOrder) // map order is random; sort before the seeded shuffle
	n.shuffle()
	return n.probeNext()
}

// one protocol period.
func (n *Node) probeNext() []Output {
	out := []Output{{Kind: SetTimer, After: n.cfg.ProtocolPeriod, Timer: Timer{Kind: PeriodEnd}}}
	target, ok := n.nextTarget()
	if !ok {
		n.probe = probe{}
		return out
	}
	n.probeID++
	n.probe = probe{id: n.probeID, target: target}
	return append(out,
		Output{Kind: Send, To: target, Msg: Message{Kind: Ping, Probe: n.probeID}},
		Output{Kind: SetTimer, After: n.cfg.ProbeTimeout,
			Timer: Timer{Kind: ProbeTimeout, Probe: n.probeID, Member: target}})
}

// round-robin over probeOrder.
func (n *Node) nextTarget() (NodeID, bool) {
	for range len(n.probeOrder) {
		if n.next == len(n.probeOrder) {
			n.shuffle()
			n.next = 0
		}
		id := n.probeOrder[n.next]
		n.next++
		if n.members[id].State != Dead {
			return id, true
		}
	}
	return "", false
}

// Fisher-Yates shuffle.
func (n *Node) shuffle() {
	for i := len(n.probeOrder) - 1; i > 0; i-- {
		j := n.rng.IntN(i + 1)
		n.probeOrder[i], n.probeOrder[j] = n.probeOrder[j], n.probeOrder[i]
	}
}

// answer ping with ack
func (n *Node) onPing(from NodeID, m Message) []Output {
	return []Output{{Kind: Send, To: from, Msg: Message{Kind: Ack, Probe: m.Probe}}}
}

func (n *Node) onAck(from NodeID, m Message) []Output {
	// intermediary: forward the target's Ack to the requester.
	if r, ok := n.relays[m.Probe]; ok && from == r.target {
		delete(n.relays, m.Probe)
		return []Output{{Kind: Send, To: r.requester,
			Msg: Message{Kind: Ack, Probe: r.requesterProbe, Target: r.target}}}
	}
	// requester: direct from the target, or forwarded by one of our intermediaries.
	if m.Probe != n.probe.id {
		return nil
	}
	direct := from == n.probe.target
	forwarded := m.Target == n.probe.target && slices.Contains(n.probe.intermediaries, from)
	if direct || forwarded {
		n.probe.acked = true
	}
	return nil
}

// unacked probe: ask up to k intermediaries to ping the target.
func (n *Node) onProbeTimeout(t Timer) []Output {
	if t.Probe != n.probe.id || n.probe.acked {
		return nil
	}
	n.probe.intermediaries = n.pickIntermediaries(n.probe.target, n.cfg.IndirectProbeCount)
	out := make([]Output, 0, len(n.probe.intermediaries))
	for _, id := range n.probe.intermediaries {
		out = append(out, Output{Kind: Send, To: id,
			Msg: Message{Kind: PingReq, Probe: n.probe.id, Target: n.probe.target}})
	}
	return out
}

// up to k random non-Dead members.
func (n *Node) pickIntermediaries(target NodeID, k int) []NodeID {
	candidates := make([]NodeID, 0, len(n.probeOrder))
	for _, id := range n.probeOrder { // deterministic order, unlike the map
		if id != target && n.members[id].State != Dead {
			candidates = append(candidates, id)
		}
	}
	k = min(k, len(candidates))
	for i := range k {
		j := i + n.rng.IntN(len(candidates)-i)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}
	return candidates[:k]
}

// ping the target with our own probe ID; forward its Ack to the requester.
func (n *Node) onPingReq(from NodeID, m Message) []Output {
	if m.Target == "" || m.Target == n.id || m.Target == from {
		return nil // malformed: the requester never picks itself or the target
	}
	n.probeID++
	n.relays[n.probeID] = relay{requester: from, requesterProbe: m.Probe, target: m.Target}
	return []Output{
		{Kind: Send, To: m.Target, Msg: Message{Kind: Ping, Probe: n.probeID}},
		{Kind: SetTimer, After: n.cfg.ProtocolPeriod, Timer: Timer{Kind: RelayTimeout, Probe: n.probeID}},
	}
}

// the requester's period is over; a later Ack is useless.
func (n *Node) onRelayTimeout(t Timer) []Output {
	delete(n.relays, t.Probe)
	return nil
}
