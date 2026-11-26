package protocol

import (
	"math/rand/v2"
	"testing"
)

func testNode(t *testing.T) *Node {
	t.Helper()
	n, err := New("self", validConfig(), rand.New(rand.NewPCG(1, 0)), []NodeID{"self", "b"})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// spec "Updates and incarnation", with existing incarnation 5.
func TestOverridesPrecedenceTable(t *testing.T) {
	const j = 5
	lower, equal, higher := uint64(j-1), uint64(j), uint64(j+1)
	cases := []struct {
		existing State
		incoming State
		inc      uint64
		want     bool
	}{
		{Alive, Alive, lower, false},
		{Alive, Alive, equal, false},
		{Alive, Alive, higher, true},
		{Alive, Suspect, lower, false},
		{Alive, Suspect, equal, true},
		{Alive, Suspect, higher, true},
		{Alive, Dead, lower, true},
		{Alive, Dead, equal, true},
		{Alive, Dead, higher, true},

		{Suspect, Alive, lower, false},
		{Suspect, Alive, equal, false},
		{Suspect, Alive, higher, true},
		{Suspect, Suspect, lower, false},
		{Suspect, Suspect, equal, false},
		{Suspect, Suspect, higher, true},
		{Suspect, Dead, lower, true},
		{Suspect, Dead, equal, true},
		{Suspect, Dead, higher, true},

		{Dead, Alive, lower, false},
		{Dead, Alive, equal, false},
		{Dead, Alive, higher, false},
		{Dead, Suspect, lower, false},
		{Dead, Suspect, equal, false},
		{Dead, Suspect, higher, false},
		{Dead, Dead, lower, false},
		{Dead, Dead, equal, false},
		{Dead, Dead, higher, false},
	}
	for _, c := range cases {
		m := Member{ID: "b", State: c.existing, Incarnation: j}
		u := Update{Member: "b", State: c.incoming, Incarnation: c.inc}
		if got := overrides(u, m); got != c.want {
			t.Errorf("existing %d(%d), incoming %d(%d): got %v, want %v",
				c.existing, j, c.incoming, c.inc, got, c.want)
		}
	}
}

func TestApplyUpdateUnknownMemberStored(t *testing.T) {
	for _, state := range []State{Alive, Suspect, Dead} {
		n := testNode(t)
		previous, changed := n.applyUpdate(Update{Member: "new", State: state, Incarnation: 2})
		if !changed || previous != state {
			t.Errorf("state %d: previous %d changed %v, want %d true", state, previous, changed, state)
		}
		m := n.members["new"]
		if m == nil || m.State != state || m.Incarnation != 2 {
			t.Errorf("state %d: stored %+v", state, m)
		}
	}
}

func TestApplyUpdateChangesEntry(t *testing.T) {
	n := testNode(t)
	previous, changed := n.applyUpdate(Update{Member: "b", State: Suspect, Incarnation: 0})
	if !changed || previous != Alive {
		t.Fatalf("previous %d changed %v, want Alive true", previous, changed)
	}
	if m := n.members["b"]; m.State != Suspect || m.Incarnation != 0 {
		t.Errorf("stored %+v, want Suspect 0", m)
	}
}

func TestApplyUpdateDuplicateUnchanged(t *testing.T) {
	n := testNode(t)
	u := Update{Member: "b", State: Suspect, Incarnation: 0}
	n.applyUpdate(u)
	if _, changed := n.applyUpdate(u); changed {
		t.Error("duplicate update reported a change")
	}
}

func TestApplyUpdateStaleUnchanged(t *testing.T) {
	n := testNode(t)
	n.applyUpdate(Update{Member: "b", State: Alive, Incarnation: 3})
	previous, changed := n.applyUpdate(Update{Member: "b", State: Suspect, Incarnation: 2})
	if changed || previous != Alive {
		t.Errorf("previous %d changed %v, want Alive false", previous, changed)
	}
	if m := n.members["b"]; m.State != Alive || m.Incarnation != 3 {
		t.Errorf("stale update modified entry: %+v", m)
	}
}

func TestApplyUpdateDeadIsFinal(t *testing.T) {
	n := testNode(t)
	n.applyUpdate(Update{Member: "b", State: Dead, Incarnation: 0})
	for _, state := range []State{Alive, Suspect, Dead} {
		if _, changed := n.applyUpdate(Update{Member: "b", State: state, Incarnation: 100}); changed {
			t.Errorf("state %d overrode Dead", state)
		}
	}
	if m := n.members["b"]; m.State != Dead || m.Incarnation != 0 {
		t.Errorf("tombstone modified: %+v", m)
	}
}

// a Dead stored for an unknown member blocks a later stale Alive.
func TestApplyUpdateUnknownDeadBlocksAlive(t *testing.T) {
	n := testNode(t)
	n.applyUpdate(Update{Member: "new", State: Dead, Incarnation: 0})
	if _, changed := n.applyUpdate(Update{Member: "new", State: Alive, Incarnation: 0}); changed {
		t.Error("stale Alive revived a dead member")
	}
}

func TestApplyUpdateAboutSelfPanics(t *testing.T) {
	n := testNode(t)
	defer func() {
		if recover() == nil {
			t.Error("update about self did not panic")
		}
	}()
	n.applyUpdate(Update{Member: "self", State: Suspect, Incarnation: 0})
}
