package simulation

import (
	"testing"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
)

func testSimConfig() Config {
	return Config{
		Protocol: protocol.Config{
			ProtocolPeriod:       time.Second,
			ProbeTimeout:         200 * time.Millisecond,
			SuspicionTimeout:     5 * time.Second,
			IndirectProbeCount:   3,
			RetransmitMultiplier: 3,
			MaxUpdatesPerMessage: 8,
		},
		Network:       NetworkConfig{LatencyMin: time.Millisecond, LatencyMax: 10 * time.Millisecond},
		Nodes:         []protocol.NodeID{"a", "b", "c"},
		QueueCapacity: 64,
	}
}

func testSim(t *testing.T, seed uint64) *Simulator {
	t.Helper()
	s, err := New(testSimConfig(), seed)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBootOffsetsWithinFirstPeriod(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Run(time.Second); err != nil {
		t.Fatal(err)
	}
	booted := 0
	for _, entry := range s.Trace() {
		e := entry.Event
		if e.Kind != Boot {
			continue
		}
		if e.At < 0 || e.At >= time.Second {
			t.Errorf("%s booted at %v, outside [0, 1s)", e.Node, e.At)
		}
		booted++
	}
	if booted != 3 {
		t.Errorf("%d nodes booted, want 3", booted)
	}
}

func TestReplaySameSeed(t *testing.T) {
	run := func(seed uint64) [32]byte {
		s := testSim(t, seed)
		if err := s.Schedule(2*time.Second, Fault{Kind: Crash, Node: "c"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Run(5 * time.Second); err != nil {
			t.Fatal(err)
		}
		return s.Trace().Hash()
	}
	if run(42) != run(42) {
		t.Error("same seed produced different traces")
	}
	if run(42) == run(43) {
		t.Error("different seeds produced identical traces")
	}
}

func TestRunAdvancesNow(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Run(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	if s.Now() != 10*time.Second {
		t.Errorf("now %v, want 10s", s.Now())
	}
}

func TestCrashUnknownNodeFails(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Schedule(time.Second, Fault{Kind: Crash, Node: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(2 * time.Second); err == nil {
		t.Error("crash of unknown node accepted")
	}
}

func TestPartitionAndHealReachNetwork(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Schedule(time.Second, Fault{Kind: Partition, Groups: [][]protocol.NodeID{{"a", "b"}, {"c"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Schedule(2*time.Second, Fault{Kind: HealPartition}); err != nil {
		t.Fatal(err)
	}

	if err := s.Run(time.Second); err != nil {
		t.Fatal(err)
	}
	if s.net.reachable("a", "c") {
		t.Error("a -> c reachable during partition")
	}
	if err := s.Run(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if !s.net.reachable("a", "c") {
		t.Error("a -> c blocked after heal")
	}
}

// without faults every node probes each period and gets acked (the last may be in flight).
func TestDirectProbingWithoutFaults(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Run(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	pings := map[protocol.NodeID]int{}
	acks := map[protocol.NodeID]int{}
	for _, entry := range s.Trace() {
		e := entry.Event
		if e.Kind != Deliver {
			continue
		}
		switch e.Msg.Kind {
		case protocol.Ping:
			pings[e.From]++
		case protocol.Ack:
			acks[e.Node]++
		}
	}
	for _, id := range testSimConfig().Nodes {
		if pings[id] < 9 || acks[id] < pings[id]-1 {
			t.Errorf("%s: %d pings, %d acks; want >= 9 pings, all but the last acked", id, pings[id], acks[id])
		}
	}
}

// a and c cannot reach each other; a's probes of c succeed through b.
func TestIndirectProbeAroundBrokenLink(t *testing.T) {
	s := testSim(t, 1)
	s.net.blocked[link{"a", "c"}] = true
	s.net.blocked[link{"c", "a"}] = true
	if err := s.Run(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	forwarded := 0
	for _, entry := range s.Trace() {
		e := entry.Event
		if e.Kind == Deliver && e.Node == "a" && e.From == "b" && e.Msg.Kind == protocol.Ack && e.Msg.Target == "c" {
			forwarded++
		}
	}
	if forwarded == 0 {
		t.Error("no Ack about c forwarded to a by b")
	}
}

// c crashes: a and b suspect and then declare it Dead; nobody else is suspected.
func TestCrashedNodeDeclaredDead(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Schedule(2*time.Second, Fault{Kind: Crash, Node: "c"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(15 * time.Second); err != nil {
		t.Fatal(err)
	}
	for _, sn := range s.nodes {
		if sn.id == "c" {
			continue
		}
		for _, m := range sn.node.Members() {
			want := protocol.Alive
			if m.ID == "c" {
				want = protocol.Dead
			}
			if m.State != want {
				t.Errorf("%s sees %s as %d, want %d", sn.id, m.ID, m.State, want)
			}
		}
	}
}

// 5 nodes, c crashes: gossip spreads Dead to every live node, even those that never probed c.
func TestCrashDetectionSpreadsByGossip(t *testing.T) {
	config := testSimConfig()
	config.Nodes = []protocol.NodeID{"a", "b", "c", "d", "e"}
	config.QueueCapacity = 256
	s, err := New(config, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Schedule(2*time.Second, Fault{Kind: Crash, Node: "c"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(20 * time.Second); err != nil {
		t.Fatal(err)
	}
	for _, sn := range s.nodes {
		if sn.id == "c" {
			continue
		}
		for _, m := range sn.node.Members() {
			want := protocol.Alive
			if m.ID == "c" {
				want = protocol.Dead
			}
			if m.State != want {
				t.Errorf("%s sees %s as %d, want %d", sn.id, m.ID, m.State, want)
			}
		}
	}
}

// a cannot reach c and has no indirect probes: a suspects c, c refutes, nobody declares c Dead.
func TestFalseSuspicionRefuted(t *testing.T) {
	config := testSimConfig()
	config.Protocol.IndirectProbeCount = 0
	s, err := New(config, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.net.blocked[link{"a", "c"}] = true
	if err := s.Run(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	for _, sn := range s.nodes {
		for _, m := range sn.node.Members() {
			if m.State == protocol.Dead {
				t.Errorf("%s declared %s Dead", sn.id, m.ID)
			}
			if sn.id == "c" && m.ID == "c" && m.Incarnation == 0 {
				t.Error("c never refuted")
			}
		}
	}
}

// every live node records c as Alive -> Suspect -> Dead, in that order and in time order.
func TestCrashChangesInTrace(t *testing.T) {
	s := testSim(t, 1)
	if err := s.Schedule(2*time.Second, Fault{Kind: Crash, Node: "c"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(15 * time.Second); err != nil {
		t.Fatal(err)
	}
	seen := map[protocol.NodeID][]protocol.State{}
	last := map[protocol.NodeID]time.Duration{}
	for _, entry := range s.Trace() {
		for _, c := range entry.Changes {
			if c.Member != "c" {
				t.Errorf("%s: unexpected change %+v", entry.Event.Node, c)
				continue
			}
			node := entry.Event.Node
			if entry.Event.At < 2*time.Second || entry.Event.At < last[node] {
				t.Errorf("%s: change at %v out of order", node, entry.Event.At)
			}
			last[node] = entry.Event.At
			seen[node] = append(seen[node], c.To)
		}
	}
	for _, id := range []protocol.NodeID{"a", "b"} {
		if got := seen[id]; len(got) != 2 || got[0] != protocol.Suspect || got[1] != protocol.Dead {
			t.Errorf("%s recorded %v, want [Suspect Dead]", id, got)
		}
	}
}
