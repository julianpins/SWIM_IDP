package protocol

import (
	"math/rand/v2"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		ProtocolPeriod:       time.Second,
		ProbeTimeout:         200 * time.Millisecond,
		SuspicionTimeout:     5 * time.Second,
		IndirectProbeCount:   3,
		RetransmitMultiplier: 3,
		MaxUpdatesPerMessage: 8,
	}
}

func TestConfigValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalid := map[string]func(*Config){
		"zero period":       func(c *Config) { c.ProtocolPeriod = 0 },
		"zero timeout":      func(c *Config) { c.ProbeTimeout = 0 },
		"timeout >= period": func(c *Config) { c.ProbeTimeout = c.ProtocolPeriod },
		"zero suspicion":    func(c *Config) { c.SuspicionTimeout = 0 },
		"negative indirect": func(c *Config) { c.IndirectProbeCount = -1 },
		"zero multiplier":   func(c *Config) { c.RetransmitMultiplier = 0 },
		"zero max updates":  func(c *Config) { c.MaxUpdatesPerMessage = 0 },
	}
	for name, mutate := range invalid {
		c := validConfig()
		mutate(&c)
		if c.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNewInitialView(t *testing.T) {
	n, err := New("a", validConfig(), rand.New(rand.NewPCG(1, 0)), []NodeID{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(n.members) != 3 {
		t.Fatalf("%d members, want 3", len(n.members))
	}
	for id, m := range n.members {
		if m.State != Alive || m.Incarnation != 0 {
			t.Errorf("%s: state %v incarnation %d, want Alive 0", id, m.State, m.Incarnation)
		}
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 0))
	cases := map[string]func() error{
		"empty id":     func() error { _, err := New("", validConfig(), rng, nil); return err },
		"bad config":   func() error { _, err := New("a", Config{}, rng, nil); return err },
		"nil rng":      func() error { _, err := New("a", validConfig(), nil, nil); return err },
		"empty member": func() error { _, err := New("a", validConfig(), rng, []NodeID{""}); return err },
		"duplicate":    func() error { _, err := New("a", validConfig(), rng, []NodeID{"b", "b"}); return err },
	}
	for name, call := range cases {
		if call() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
