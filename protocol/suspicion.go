package protocol

// applies u; reports and disseminates every change and starts a suspicion timer on a new Suspect.
func (n *Node) update(u Update) []Output {
	_, known := n.members[u.Member]
	previous, changed := n.applyUpdate(u)
	if !changed {
		return nil
	}
	n.enqueue(u)
	out := []Output{{Kind: Changed, Change: Change{Member: u.Member, Known: known,
		From: previous, To: u.State, Incarnation: u.Incarnation}}}
	if u.State != Suspect {
		return out
	}
	return append(out, Output{Kind: SetTimer, After: n.cfg.SuspicionTimeout,
		Timer: Timer{Kind: SuspicionTimeout, Member: u.Member, Incarnation: u.Incarnation}})
}

// unacked probe (direct and indirect) suspects the target, then the next period starts.
func (n *Node) onPeriodEnd() []Output {
	var out []Output
	if n.probe.id != 0 && !n.probe.acked {
		target := n.members[n.probe.target]
		out = n.update(Update{Member: target.ID, State: Suspect, Incarnation: target.Incarnation})
	}
	return append(out, n.probeNext()...)
}

// still Suspect at the same incarnation: declare Dead. otherwise stale.
func (n *Node) onSuspicionTimeout(t Timer) []Output {
	m := n.members[t.Member]
	if m == nil || m.State != Suspect || m.Incarnation != t.Incarnation {
		return nil
	}
	return n.update(Update{Member: m.ID, State: Dead, Incarnation: m.Incarnation})
}
