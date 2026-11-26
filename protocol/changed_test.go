package protocol

import (
	"slices"
	"testing"
)

func changes(out []Output) []Change {
	var found []Change
	for _, o := range out {
		if o.Kind == Changed {
			found = append(found, o.Change)
		}
	}
	return found
}

func TestSuspectAndDeadEmitChanged(t *testing.T) {
	n := probeNode(t, 1)
	target, _ := pingIn(t, n.Start())
	out := n.Fire(Timer{Kind: PeriodEnd})
	want := []Change{{Member: target, Known: true, From: Alive, To: Suspect}}
	if got := changes(out); !slices.Equal(got, want) {
		t.Fatalf("suspect: got %+v, want %+v", got, want)
	}
	out = n.Fire(timers(out, SuspicionTimeout)[0])
	want = []Change{{Member: target, Known: true, From: Suspect, To: Dead}}
	if got := changes(out); !slices.Equal(got, want) {
		t.Errorf("dead: got %+v, want %+v", got, want)
	}
}

// others see the refutation as Suspect -> Alive; the refuting node itself emits nothing.
func TestRefutationChanged(t *testing.T) {
	n := probeNode(t, 1)
	n.update(Update{Member: "b", State: Suspect})
	out := n.Receive("a", Message{Kind: Ping, Probe: 1, Updates: []Update{{Member: "b", State: Alive, Incarnation: 1}}})
	want := []Change{{Member: "b", Known: true, From: Suspect, To: Alive, Incarnation: 1}}
	if got := changes(out); !slices.Equal(got, want) {
		t.Errorf("observer: got %+v, want %+v", got, want)
	}

	out = n.Receive("a", Message{Kind: Ping, Probe: 2, Updates: []Update{{Member: "self", State: Suspect}}})
	if got := changes(out); got != nil {
		t.Errorf("refuting node emitted %+v", got)
	}
}

func TestDuplicateAndStaleEmitNothing(t *testing.T) {
	n := probeNode(t, 1)
	n.update(Update{Member: "b", State: Alive, Incarnation: 2})
	for _, u := range []Update{
		{Member: "b", State: Alive, Incarnation: 2},   // duplicate
		{Member: "b", State: Suspect, Incarnation: 1}, // stale
	} {
		if got := changes(n.update(u)); got != nil {
			t.Errorf("%+v: emitted %+v", u, got)
		}
	}
}

func TestUnknownMemberChangedNotKnown(t *testing.T) {
	n := probeNode(t, 1)
	got := changes(n.update(Update{Member: "new", State: Suspect, Incarnation: 3}))
	if len(got) != 1 || got[0].Known || got[0].To != Suspect || got[0].Incarnation != 3 {
		t.Errorf("got %+v, want one Change with Known false, To Suspect, Incarnation 3", got)
	}
}
