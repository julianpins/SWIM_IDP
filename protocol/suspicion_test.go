package protocol

import "testing"

func timers(out []Output, kind TimerKind) []Timer {
	var found []Timer
	for _, o := range out {
		if o.Kind == SetTimer && o.Timer.Kind == kind {
			found = append(found, o.Timer)
		}
	}
	return found
}

func TestUnackedProbeSuspectsTarget(t *testing.T) {
	n := probeNode(t, 1)
	target, _ := pingIn(t, n.Start())
	out := n.Fire(Timer{Kind: PeriodEnd})
	if m := n.members[target]; m.State != Suspect || m.Incarnation != 0 {
		t.Fatalf("target %+v, want Suspect 0", m)
	}
	want := Timer{Kind: SuspicionTimeout, Member: target, Incarnation: 0}
	if got := timers(out, SuspicionTimeout); len(got) != 1 || got[0] != want {
		t.Errorf("suspicion timers %+v, want [%+v]", got, want)
	}
	for _, o := range out {
		if o.Timer.Kind == SuspicionTimeout && o.After != n.cfg.SuspicionTimeout {
			t.Errorf("suspicion timer after %v, want %v", o.After, n.cfg.SuspicionTimeout)
		}
	}
	if len(timers(out, PeriodEnd)) != 1 {
		t.Error("next period not started")
	}
}

func TestAckedProbeNoSuspicion(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		n := probeNode(t, 1)
		target, id := pingIn(t, n.Start())
		if indirect {
			via := sends(n.Fire(Timer{Kind: ProbeTimeout, Probe: id, Member: target}), PingReq)[0].To
			n.Receive(via, Message{Kind: Ack, Probe: id, Target: target})
		} else {
			n.Receive(target, Message{Kind: Ack, Probe: id})
		}
		out := n.Fire(Timer{Kind: PeriodEnd})
		if n.members[target].State != Alive || len(timers(out, SuspicionTimeout)) != 0 {
			t.Errorf("indirect %v: acked target suspected", indirect)
		}
	}
}

func TestAlreadySuspectNoSecondTimer(t *testing.T) {
	n := probeNode(t, 1)
	target, _ := pingIn(t, n.Start())
	n.applyUpdate(Update{Member: target, State: Suspect})
	if out := n.Fire(Timer{Kind: PeriodEnd}); len(timers(out, SuspicionTimeout)) != 0 {
		t.Error("second suspicion timer for an already Suspect member")
	}
}

func TestIdlePeriodSuspectsNothing(t *testing.T) {
	n := probeNode(t, 1)
	for _, id := range []NodeID{"a", "b", "c"} {
		n.applyUpdate(Update{Member: id, State: Dead})
	}
	n.Start()
	if out := n.Fire(Timer{Kind: PeriodEnd}); len(timers(out, SuspicionTimeout)) != 0 {
		t.Error("idle period started a suspicion")
	}
}

func TestSuspicionTimeoutDeclaresDead(t *testing.T) {
	n := probeNode(t, 1)
	target, _ := pingIn(t, n.Start())
	timer := timers(n.Fire(Timer{Kind: PeriodEnd}), SuspicionTimeout)[0]
	for _, o := range n.Fire(timer) {
		if o.Kind == SetTimer {
			t.Errorf("Dead set a timer: %+v", o)
		}
	}
	if m := n.members[target]; m.State != Dead || m.Incarnation != 0 {
		t.Errorf("target %+v, want Dead 0", m)
	}
}

func TestStaleSuspicionTimeoutIgnored(t *testing.T) {
	cases := map[string]Update{
		"refuted":      {State: Alive, Incarnation: 1},
		"resuspected":  {State: Suspect, Incarnation: 1},
		"already dead": {State: Dead, Incarnation: 0},
	}
	for name, u := range cases {
		n := probeNode(t, 1)
		target, _ := pingIn(t, n.Start())
		timer := timers(n.Fire(Timer{Kind: PeriodEnd}), SuspicionTimeout)[0]
		u.Member = target
		n.applyUpdate(u)
		n.Fire(timer)
		if m := n.members[target]; m.State != u.State || m.Incarnation != u.Incarnation {
			t.Errorf("%s: stale timer changed %+v", name, m)
		}
	}
}
