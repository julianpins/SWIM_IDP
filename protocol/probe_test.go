package protocol

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func probeNode(t *testing.T, seed uint64) *Node {
	t.Helper()
	n, err := New("self", validConfig(), rand.New(rand.NewPCG(seed, 0)), []NodeID{"self", "a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// returns the target and probe ID of the Ping in out; fails if there is none.
func pingIn(t *testing.T, out []Output) (NodeID, uint64) {
	t.Helper()
	for _, o := range out {
		if o.Kind == Send && o.Msg.Kind == Ping {
			return o.To, o.Msg.Probe
		}
	}
	t.Fatalf("no Ping in %+v", out)
	return "", 0
}

func TestStartEmitsPeriodPingAndTimeout(t *testing.T) {
	n := probeNode(t, 1)
	out := n.Start()
	if len(out) != 3 {
		t.Fatalf("got %d outputs, want 3: %+v", len(out), out)
	}
	target, id := pingIn(t, out)
	if id == 0 {
		t.Error("probe ID 0 is reserved for no probe")
	}
	if out[0].Kind != SetTimer || out[0].Timer.Kind != PeriodEnd || out[0].After != n.cfg.ProtocolPeriod {
		t.Errorf("first output %+v, want PeriodEnd after ProtocolPeriod", out[0])
	}
	want := Output{Kind: SetTimer, After: n.cfg.ProbeTimeout, Timer: Timer{Kind: ProbeTimeout, Probe: id, Member: target}}
	if out[2].Kind != want.Kind || out[2].After != want.After || out[2].Timer != want.Timer {
		t.Errorf("timeout %+v, want %+v", out[2], want)
	}
}

func TestStartTwicePanics(t *testing.T) {
	n := probeNode(t, 1)
	n.Start()
	defer func() {
		if recover() == nil {
			t.Error("second Start did not panic")
		}
	}()
	n.Start()
}

func TestEachTraversalProbesEveryMemberOnce(t *testing.T) {
	n := probeNode(t, 1)
	out := n.Start()
	for round := range 5 {
		var seen []NodeID
		for range 3 {
			target, _ := pingIn(t, out)
			seen = append(seen, target)
			out = n.Fire(Timer{Kind: PeriodEnd})
		}
		slices.Sort(seen)
		if !slices.Equal(seen, []NodeID{"a", "b", "c"}) {
			t.Errorf("round %d probed %v", round, seen)
		}
	}
}

func TestProbeIDsIncrease(t *testing.T) {
	n := probeNode(t, 1)
	_, previous := pingIn(t, n.Start())
	for range 5 {
		_, id := pingIn(t, n.Fire(Timer{Kind: PeriodEnd}))
		if id <= previous {
			t.Errorf("probe ID %d after %d", id, previous)
		}
		previous = id
	}
}

func TestSameSeedSameProbeOrder(t *testing.T) {
	targets := func(seed uint64) []NodeID {
		n := probeNode(t, seed)
		target, _ := pingIn(t, n.Start())
		out := []NodeID{target}
		for range 8 {
			target, _ = pingIn(t, n.Fire(Timer{Kind: PeriodEnd}))
			out = append(out, target)
		}
		return out
	}
	if a, b := targets(7), targets(7); !slices.Equal(a, b) {
		t.Errorf("same seed, different orders: %v vs %v", a, b)
	}
}

func TestDeadMembersSkipped(t *testing.T) {
	n := probeNode(t, 1)
	n.applyUpdate(Update{Member: "b", State: Dead})
	out := n.Start()
	for range 9 {
		if target, _ := pingIn(t, out); target == "b" {
			t.Fatal("probed a Dead member")
		}
		out = n.Fire(Timer{Kind: PeriodEnd})
	}
}

func TestAllPeersDeadStillTicks(t *testing.T) {
	n := probeNode(t, 1)
	for _, id := range []NodeID{"a", "b", "c"} {
		n.applyUpdate(Update{Member: id, State: Dead})
	}
	for _, out := range [][]Output{n.Start(), n.Fire(Timer{Kind: PeriodEnd})} {
		if len(out) != 1 || out[0].Timer.Kind != PeriodEnd {
			t.Errorf("got %+v, want only PeriodEnd", out)
		}
	}
}

func TestPingAnsweredWithAck(t *testing.T) {
	n := probeNode(t, 1) // not started: Pings are answered before Start
	out := n.Receive("a", Message{Kind: Ping, Probe: 42})
	want := Output{Kind: Send, To: "a", Msg: Message{Kind: Ack, Probe: 42}}
	if len(out) != 1 || out[0].Kind != want.Kind || out[0].To != want.To ||
		out[0].Msg.Kind != want.Msg.Kind || out[0].Msg.Probe != want.Msg.Probe {
		t.Errorf("got %+v, want %+v", out, want)
	}
}

func TestPingFromUnknownSenderNotAdded(t *testing.T) {
	n := probeNode(t, 1)
	n.Receive("stranger", Message{Kind: Ping, Probe: 1})
	if _, known := n.members["stranger"]; known {
		t.Error("Ping sender was added to the view")
	}
}

func TestAckCompletesMatchingProbe(t *testing.T) {
	n := probeNode(t, 1)
	target, id := pingIn(t, n.Start())
	n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}) // late ACK still counts
	n.Receive(target, Message{Kind: Ack, Probe: id})
	if !n.probe.acked {
		t.Error("matching Ack did not complete the probe")
	}
}

func TestAckIgnoredWhenUnrelated(t *testing.T) {
	n := probeNode(t, 1)
	target, id := pingIn(t, n.Start())
	other := NodeID("a")
	if target == other {
		other = "b"
	}
	n.Receive(target, Message{Kind: Ack, Probe: id + 1}) // wrong probe
	n.Receive(other, Message{Kind: Ack, Probe: id})      // wrong sender
	if n.probe.acked {
		t.Error("unrelated Ack completed the probe")
	}
}

func TestAckFromPreviousPeriodIgnored(t *testing.T) {
	n := probeNode(t, 1)
	oldTarget, oldID := pingIn(t, n.Start())
	n.Fire(Timer{Kind: PeriodEnd})
	n.Receive(oldTarget, Message{Kind: Ack, Probe: oldID})
	if n.probe.acked {
		t.Error("Ack for the previous probe completed the current one")
	}
}

func TestAckBeforeStartIgnored(t *testing.T) {
	n := probeNode(t, 1)
	n.Receive("a", Message{Kind: Ack, Probe: 0})
	if n.probe.acked {
		t.Error("Ack before Start completed a probe")
	}
}

func TestMembersSortedCopy(t *testing.T) {
	n := probeNode(t, 1)
	members := n.Members()
	ids := make([]NodeID, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	if !slices.Equal(ids, []NodeID{"a", "b", "c", "self"}) {
		t.Errorf("got %v", ids)
	}
	members[0].State = Dead
	if n.members["a"].State != Alive {
		t.Error("Members returned a reference into the view")
	}
}
