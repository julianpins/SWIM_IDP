package protocol

import (
	"slices"
	"testing"
)

func sends(out []Output, kind MessageKind) []Output {
	var found []Output
	for _, o := range out {
		if o.Kind == Send && o.Msg.Kind == kind {
			found = append(found, o)
		}
	}
	return found
}

func TestProbeTimeoutSendsPingReqs(t *testing.T) {
	n := probeNode(t, 1)
	n.cfg.IndirectProbeCount = 1
	target, id := pingIn(t, n.Start())
	reqs := sends(n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}), PingReq)
	if len(reqs) != 1 {
		t.Fatalf("got %d PING-REQs, want 1", len(reqs))
	}
	r := reqs[0]
	if r.To == "self" || r.To == target || r.Msg.Probe != id || r.Msg.Target != target {
		t.Errorf("bad PING-REQ %+v for target %s probe %d", r, target, id)
	}
	if !slices.Equal(n.probe.intermediaries, []NodeID{r.To}) {
		t.Errorf("intermediaries %v, want [%s]", n.probe.intermediaries, r.To)
	}
}

// fewer candidates than k: all are used; Dead members are never picked.
func TestPingReqsUseAllLiveCandidates(t *testing.T) {
	n := probeNode(t, 1) // k = 3, peers a, b, c
	n.applyUpdate(Update{Member: "c", State: Dead})
	target, id := pingIn(t, n.Start())
	reqs := sends(n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}), PingReq)
	if len(reqs) != 1 || reqs[0].To == "c" || reqs[0].To == target {
		t.Errorf("got %+v, want one PING-REQ to the other live peer", reqs)
	}
}

func TestProbeTimeoutAfterAckOrStaleSendsNothing(t *testing.T) {
	n := probeNode(t, 1)
	target, id := pingIn(t, n.Start())
	if out := n.Fire(Timer{Kind: ProbeTimeout, Probe: id + 1, Member: target}); out != nil {
		t.Errorf("stale timeout sent %+v", out)
	}
	n.Receive(target, Message{Kind: Ack, Probe: id})
	if out := n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}); out != nil {
		t.Errorf("timeout after Ack sent %+v", out)
	}
}

func TestIntermediaryRelaysAndForwards(t *testing.T) {
	n := probeNode(t, 1)
	out := n.Receive("a", Message{Kind: PingReq, Probe: 5, Target: "b"})
	pings := sends(out, Ping)
	if len(pings) != 1 || pings[0].To != "b" {
		t.Fatalf("got %+v, want one Ping to b", out)
	}
	own := pings[0].Msg.Probe
	if own == 5 || own == 0 {
		t.Errorf("relayed Ping uses probe ID %d, want a fresh own ID", own)
	}
	if len(out) != 2 || out[1].Timer != (Timer{Kind: RelayTimeout, Probe: own}) || out[1].After != n.cfg.ProtocolPeriod {
		t.Errorf("got %+v, want RelayTimeout after ProtocolPeriod", out)
	}

	if out := n.Receive("c", Message{Kind: Ack, Probe: own}); out != nil {
		t.Errorf("Ack from wrong sender forwarded: %+v", out)
	}
	acks := sends(n.Receive("b", Message{Kind: Ack, Probe: own}), Ack)
	if len(acks) != 1 || acks[0].To != "a" || acks[0].Msg.Probe != 5 || acks[0].Msg.Target != "b" {
		t.Errorf("got %+v, want Ack{5, b} to a", acks)
	}
	if out := n.Receive("b", Message{Kind: Ack, Probe: own}); out != nil {
		t.Errorf("duplicate Ack forwarded: %+v", out)
	}
}

func TestRelayTimeoutDropsEntry(t *testing.T) {
	n := probeNode(t, 1)
	own := sends(n.Receive("a", Message{Kind: PingReq, Probe: 5, Target: "b"}), Ping)[0].Msg.Probe
	n.Fire(Timer{Kind: RelayTimeout, Probe: own})
	if out := n.Receive("b", Message{Kind: Ack, Probe: own}); out != nil {
		t.Errorf("Ack after RelayTimeout forwarded: %+v", out)
	}
}

func TestMalformedPingReqIgnored(t *testing.T) {
	n := probeNode(t, 1)
	for _, target := range []NodeID{"", "self", "a"} {
		if out := n.Receive("a", Message{Kind: PingReq, Probe: 5, Target: target}); out != nil {
			t.Errorf("target %q: got %+v", target, out)
		}
	}
}

// requester with k = 1: one intermediary, one other peer that was not asked.
func TestRequesterAcceptsOnlyForwardedAckFromIntermediary(t *testing.T) {
	start := func() (n *Node, target, intermediary, other NodeID, id uint64) {
		n = probeNode(t, 1)
		n.cfg.IndirectProbeCount = 1
		target, id = pingIn(t, n.Start())
		intermediary = sends(n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}), PingReq)[0].To
		for _, p := range []NodeID{"a", "b", "c"} {
			if p != target && p != intermediary {
				other = p
			}
		}
		return
	}

	n, target, intermediary, other, id := start()
	n.Receive(other, Message{Kind: Ack, Probe: id, Target: target})
	n.Receive(intermediary, Message{Kind: Ack, Probe: id, Target: other})
	n.Receive(intermediary, Message{Kind: Ack, Probe: id + 1, Target: target})
	if n.probe.acked {
		t.Fatal("accepted a forwarded Ack from a non-intermediary, for a wrong target or a stale probe")
	}
	n.Receive(intermediary, Message{Kind: Ack, Probe: id, Target: target})
	if !n.probe.acked {
		t.Error("forwarded Ack from the intermediary not accepted")
	}
}
