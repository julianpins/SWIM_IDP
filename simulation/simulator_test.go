package simulation

import (
	"testing"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
)

func testSimConfig() Config {
	return Config{
		Protocol: protocol.Config{
			ProtocolPeriod:     time.Second,
			ProbeTimeout:       200 * time.Millisecond,
			SuspicionTimeout:   5 * time.Second,
			IndirectProbeCount: 3,
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
