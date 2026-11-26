package simulation

import (
	"crypto/sha256"
	"testing"

	"github.com/julianpins/SWIM_IDP/protocol"
)

func testTrace() Trace {
	return Trace{
		{Event: Event{At: 10, Kind: Deliver, Node: "b", From: "a", Msg: protocol.Message{
			Kind:    protocol.Ping,
			Probe:   1,
			Updates: []protocol.Update{{Member: "c", State: protocol.Suspect, Incarnation: 3}},
		}}},
		{Event: Event{At: 20, Kind: Expire, Node: "a", Timer: protocol.Timer{Kind: protocol.ProbeTimeout, Probe: 1}}},
		{Event: Event{At: 30, Kind: Inject, Fault: Fault{
			Kind:   Partition,
			Groups: [][]protocol.NodeID{{"a", "b"}, {"c"}},
		}}},
	}
}

func TestHashIdenticalTraces(t *testing.T) {
	t1 := testTrace()
	t2 := testTrace()
	if t1.Hash() != t2.Hash() {
		t.Error("identical traces hash differently")
	}
}

func TestHashEmptyTrace(t *testing.T) {
	if Trace(nil).Hash() != sha256.Sum256(nil) {
		t.Error("empty trace is not the hash of no data")
	}
}

func TestHashDetectsFieldChange(t *testing.T) {
	base := testTrace().Hash()
	changes := map[string]func(Trace){
		"At":          func(tr Trace) { tr[1].Event.At = 21 },
		"incarnation": func(tr Trace) { tr[0].Event.Msg.Updates[0].Incarnation = 4 },
		"group node":  func(tr Trace) { tr[2].Event.Fault.Groups[1][0] = "d" },
	}
	for name, change := range changes {
		tr := testTrace()
		change(tr)
		if tr.Hash() == base {
			t.Errorf("%s: change not detected", name)
		}
	}
}

func TestHashDetectsReorder(t *testing.T) {
	tr := testTrace()
	tr[0], tr[1] = tr[1], tr[0]
	if tr.Hash() == testTrace().Hash() {
		t.Error("swapped entries hash the same")
	}
}

// quoting keeps entry boundaries unambiguous.
func TestHashNodeIDWithNewline(t *testing.T) {
	one := Trace{{Event: Event{Node: "a\nb"}}}
	two := Trace{{Event: Event{Node: "a"}}, {Event: Event{Node: "b"}}}
	if one.Hash() == two.Hash() {
		t.Error("newline in node ID collides with two entries")
	}
}
