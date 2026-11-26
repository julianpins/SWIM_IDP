package protocol

import (
	"slices"
	"testing"
)

// the Ack a node sends in answer to a Ping without updates.
func ackUpdates(t *testing.T, n *Node) []Update {
	t.Helper()
	acks := sends(n.Receive("a", Message{Kind: Ping, Probe: 1}), Ack)
	if len(acks) != 1 {
		t.Fatalf("got %d Acks, want 1", len(acks))
	}
	return acks[0].Msg.Updates
}

func TestEverySendCarriesUpdates(t *testing.T) {
	n := probeNode(t, 1)
	u := Update{Member: "b", State: Suspect}
	n.enqueue(u)
	out := n.Start()
	target, id := pingIn(t, out)
	if ping := sends(out, Ping)[0]; !slices.Equal(ping.Msg.Updates, []Update{u}) {
		t.Errorf("Ping carries %v", ping.Msg.Updates)
	}
	for _, req := range sends(n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}), PingReq) {
		if !slices.Equal(req.Msg.Updates, []Update{u}) {
			t.Errorf("PING-REQ carries %v", req.Msg.Updates)
		}
	}
	if got := ackUpdates(t, n); !slices.Equal(got, []Update{u}) {
		t.Errorf("Ack carries %v", got)
	}
}

func TestLeastTransmittedFirstTiesByID(t *testing.T) {
	n := probeNode(t, 1)
	n.cfg.MaxUpdatesPerMessage = 1
	n.enqueue(Update{Member: "b", State: Suspect})
	n.enqueue(Update{Member: "a", State: Suspect})
	var order []NodeID
	for range 4 {
		got := ackUpdates(t, n)
		if len(got) != 1 {
			t.Fatalf("message carries %d updates, want 1", len(got))
		}
		order = append(order, got[0].Member)
	}
	if want := []NodeID{"a", "b", "a", "b"}; !slices.Equal(order, want) {
		t.Errorf("order %v, want %v", order, want)
	}
}

func TestUpdateDroppedAfterLimit(t *testing.T) {
	n := probeNode(t, 1)
	limit := n.retransmitLimit()
	n.enqueue(Update{Member: "b", State: Suspect})
	for i := range limit {
		if len(ackUpdates(t, n)) != 1 {
			t.Fatalf("transmission %d: update missing", i+1)
		}
	}
	if got := ackUpdates(t, n); got != nil {
		t.Errorf("sent after %d transmissions: %v", limit, got)
	}
}

func TestRetransmitLimitCountsNonDead(t *testing.T) {
	n := probeNode(t, 1) // λ = 3, members self, a, b, c
	if got := n.retransmitLimit(); got != 5 {
		t.Errorf("4 live: got %d, want ⌈3·ln 4⌉ = 5", got)
	}
	n.applyUpdate(Update{Member: "a", State: Dead})
	n.applyUpdate(Update{Member: "b", State: Dead})
	if got := n.retransmitLimit(); got != 3 {
		t.Errorf("2 live: got %d, want ⌈3·ln 2⌉ = 3", got)
	}
	n.applyUpdate(Update{Member: "c", State: Dead})
	if got := n.retransmitLimit(); got != 1 {
		t.Errorf("1 live: got %d, want at least 1", got)
	}
}

func TestNewerUpdateReplacesOlder(t *testing.T) {
	n := probeNode(t, 1)
	n.enqueue(Update{Member: "b", State: Suspect})
	ackUpdates(t, n)
	newer := Update{Member: "b", State: Alive, Incarnation: 1}
	n.enqueue(newer)
	if len(n.gossip) != 1 || n.gossip[0] != (broadcast{update: newer}) {
		t.Errorf("buffer %+v, want only %+v with count 0", n.gossip, newer)
	}
}

func TestReceivedUpdatesAppliedAndForwarded(t *testing.T) {
	n := probeNode(t, 1)
	u := Update{Member: "b", State: Suspect}
	out := n.Receive("a", Message{Kind: Ping, Probe: 1, Updates: []Update{u}})
	if n.members["b"].State != Suspect {
		t.Error("piggybacked update not applied")
	}
	if len(timers(out, SuspicionTimeout)) != 1 {
		t.Error("gossiped Suspect started no suspicion timer")
	}
	if ack := sends(out, Ack); len(ack) != 1 || !slices.Equal(ack[0].Msg.Updates, []Update{u}) {
		t.Errorf("change not disseminated further: %+v", ack)
	}
	if out := n.Receive("a", Message{Kind: Ping, Probe: 2, Updates: []Update{u}}); len(timers(out, SuspicionTimeout)) != 0 {
		t.Error("duplicate update started a second timer")
	}
}

func TestRefuteSuspicionAboutSelf(t *testing.T) {
	n := probeNode(t, 1)
	n.Receive("a", Message{Kind: Ping, Probe: 1, Updates: []Update{{Member: "self", State: Suspect}}})
	if inc := n.members["self"].Incarnation; inc != 1 {
		t.Fatalf("incarnation %d, want 1", inc)
	}
	want := Update{Member: "self", State: Alive, Incarnation: 1}
	if got := ackUpdates(t, n); !slices.Contains(got, want) {
		t.Errorf("Ack carries %v, want %+v", got, want)
	}
}

func TestRefuteIgnoresOtherUpdatesAboutSelf(t *testing.T) {
	cases := map[string]Update{
		"stale suspect": {State: Suspect, Incarnation: 1},
		"alive":         {State: Alive, Incarnation: 9},
		"dead":          {State: Dead, Incarnation: 2},
	}
	for name, u := range cases {
		n := probeNode(t, 1)
		n.members["self"].Incarnation = 2
		u.Member = "self"
		n.refute(u)
		if inc := n.members["self"].Incarnation; inc != 2 || len(n.gossip) != 0 {
			t.Errorf("%s: incarnation %d, buffer %v", name, inc, n.gossip)
		}
		if n.members["self"].State != Alive {
			t.Errorf("%s: own state changed", name)
		}
	}
}

func TestMessagesDoNotAliasBuffer(t *testing.T) {
	n := probeNode(t, 1)
	n.enqueue(Update{Member: "b", State: Suspect})
	got := ackUpdates(t, n)
	got[0].State = Dead
	if n.gossip[0].update.State != Suspect {
		t.Error("message shares its Updates slice with the buffer")
	}
}
