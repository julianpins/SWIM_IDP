package simulation

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
)

func testNetwork(t *testing.T, config NetworkConfig, seed uint64) *network {
	t.Helper()
	n, err := newNetwork(config, rand.New(rand.NewPCG(seed, 0)))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

var testConfig = NetworkConfig{
	LatencyMin:      5 * time.Millisecond,
	LatencyMax:      50 * time.Millisecond,
	LossProbability: 0.1,
}

func TestNewNetworkRejectsInvalidConfig(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 0))
	invalid := map[string]NetworkConfig{
		"negative min":   {LatencyMin: -1, LatencyMax: 0},
		"max below min":  {LatencyMin: 10, LatencyMax: 5},
		"negative loss":  {LossProbability: -0.1},
		"loss above one": {LossProbability: 1.1},
	}
	for name, config := range invalid {
		if _, err := newNetwork(config, random); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := newNetwork(testConfig, nil); err == nil {
		t.Error("nil random: accepted")
	}
}

func TestRouteDelayWithinBounds(t *testing.T) {
	n := testNetwork(t, NetworkConfig{LatencyMin: 5, LatencyMax: 50}, 1)
	for i := 0; i < 10_000; i++ {
		delay, ok := n.route("a", "b")
		if !ok {
			t.Fatal("message lost with LossProbability 0")
		}
		if delay < 5 || delay > 50 {
			t.Fatalf("delay %v outside [5, 50]", delay)
		}
	}
}

func TestRouteFixedLatency(t *testing.T) {
	n := testNetwork(t, NetworkConfig{LatencyMin: 7, LatencyMax: 7}, 1)
	for i := 0; i < 100; i++ {
		if delay, _ := n.route("a", "b"); delay != 7 {
			t.Fatalf("delay %v, want 7", delay)
		}
	}
}

func TestRouteLossAlways(t *testing.T) {
	n := testNetwork(t, NetworkConfig{LossProbability: 1}, 1)
	for i := 0; i < 1000; i++ {
		if _, ok := n.route("a", "b"); ok {
			t.Fatal("message delivered with LossProbability 1")
		}
	}
}

func TestRouteDeterministic(t *testing.T) {
	first := testNetwork(t, testConfig, 42)
	second := testNetwork(t, testConfig, 42)
	for i := 0; i < 1000; i++ {
		d1, ok1 := first.route("a", "b")
		d2, ok2 := second.route("a", "b")
		if d1 != d2 || ok1 != ok2 {
			t.Fatalf("draw %d differs: (%v, %v) vs (%v, %v)", i, d1, ok1, d2, ok2)
		}
	}
}

func TestPartitionBlocksCrossGroupLinks(t *testing.T) {
	n := testNetwork(t, testConfig, 1)
	n.partition([][]protocol.NodeID{{"a", "b"}, {"c"}})

	blocked := []link{{"a", "c"}, {"c", "a"}, {"b", "c"}, {"c", "b"}}
	for _, l := range blocked {
		if n.reachable(l.from, l.to) {
			t.Errorf("%s -> %s reachable across partition", l.from, l.to)
		}
	}
	open := []link{{"a", "b"}, {"b", "a"}, {"a", "d"}, {"d", "c"}}
	for _, l := range open {
		if !n.reachable(l.from, l.to) {
			t.Errorf("%s -> %s blocked, want reachable", l.from, l.to)
		}
	}
}

func TestPartitionReplacesPrevious(t *testing.T) {
	n := testNetwork(t, testConfig, 1)
	n.partition([][]protocol.NodeID{{"a"}, {"b"}})
	n.partition([][]protocol.NodeID{{"a", "b"}, {"c"}})
	if !n.reachable("a", "b") {
		t.Error("a -> b still blocked by previous partition")
	}
}

func TestHealUnblocksAll(t *testing.T) {
	n := testNetwork(t, testConfig, 1)
	n.partition([][]protocol.NodeID{{"a"}, {"b"}})
	n.heal()
	if !n.reachable("a", "b") || !n.reachable("b", "a") {
		t.Error("link still blocked after heal")
	}
}

// A partition must not shift later random draws: after healing, delays and
// losses match a run that never had a partition.
func TestPartitionDoesNotShiftDraws(t *testing.T) {
	plain := testNetwork(t, testConfig, 7)
	parted := testNetwork(t, testConfig, 7)

	parted.partition([][]protocol.NodeID{{"a"}, {"b"}})
	for i := 0; i < 100; i++ {
		plain.route("a", "b")
		if _, ok := parted.route("a", "b"); ok {
			t.Fatal("message crossed partition")
		}
	}
	parted.heal()

	for i := 0; i < 1000; i++ {
		d1, ok1 := plain.route("a", "b")
		d2, ok2 := parted.route("a", "b")
		if d1 != d2 || ok1 != ok2 {
			t.Fatalf("draw %d after heal differs: (%v, %v) vs (%v, %v)", i, d1, ok1, d2, ok2)
		}
	}
}

func TestRouteAssertsOnSelfSend(t *testing.T) {
	n := testNetwork(t, testConfig, 1)
	defer func() {
		if recover() == nil {
			t.Error("route to self did not panic")
		}
	}()
	n.route("a", "a")
}
