// swimsim runs one deterministic simulation and prints its trace.
//
//	go run ./cmd/swimsim -nodes 5 -seed 1 -until 30s -crash n3@2s
//
// Flag defaults are for exploration, not experiments: experiment parameters belong in evaluation/configs/.
package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
	"github.com/julianpins/SWIM_IDP/simulation"
)

// Flag defaults; edit here.
const (
	defaultNodes = 5
	defaultSeed  = 1
	defaultUntil = 30 * time.Second
	defaultQueue = 4096
	defaultPeriod               = time.Second
	defaultProbeTimeout         = 700 * time.Millisecond
	defaultSuspicionTimeout     = 5 * time.Second
	defaultIndirectProbeCount   = 3
	defaultRetransmitMultiplier = 3
	defaultMaxUpdatesPerMessage = 8

	defaultLatencyMin = 100 * time.Millisecond
	defaultLatencyMax = 400 * time.Millisecond
	// 0.05 makes about 10% of probes of live nodes end in a false suspicion (with the defaults above).
	defaultLoss = 0.05
)

type crash struct {
	node protocol.NodeID
	at   time.Duration
}

// repeatable -crash id@time.
type crashFlag []crash

func (c *crashFlag) String() string { return fmt.Sprint(*c) }

func (c *crashFlag) Set(value string) error {
	id, at, ok := strings.Cut(value, "@")
	if !ok || id == "" {
		return errors.New("want id@time, e.g. n3@2s")
	}
	d, err := time.ParseDuration(at)
	if err != nil {
		return err
	}
	*c = append(*c, crash{node: protocol.NodeID(id), at: d})
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "swimsim:", err)
		os.Exit(1)
	}
}

func run() error {
	nodes := flag.Int("nodes", defaultNodes, "number of nodes, named n1..nN")
	seed := flag.Uint64("seed", defaultSeed, "random seed")
	until := flag.Duration("until", defaultUntil, "virtual time to run to")
	queue := flag.Int("queue", defaultQueue, "event queue capacity")
	changesOnly := flag.Bool("changes-only", false, "print only membership changes")
	jsonOutput := flag.Bool("json", false, "write the run and its trace as JSON (for scripts/trace_animation.py)")
	var crashList crashFlag
	flag.Var(&crashList, "crash", "crash a node at a virtual time, id@time (repeatable)")

	var config simulation.Config
	flag.DurationVar(&config.Protocol.ProtocolPeriod, "period", defaultPeriod, "protocol period")
	flag.DurationVar(&config.Protocol.ProbeTimeout, "probe-timeout", defaultProbeTimeout, "direct probe timeout")
	flag.DurationVar(&config.Protocol.SuspicionTimeout, "suspicion", defaultSuspicionTimeout, "suspicion timeout")
	flag.IntVar(&config.Protocol.IndirectProbeCount, "indirect", defaultIndirectProbeCount, "indirect probe count k")
	flag.IntVar(&config.Protocol.RetransmitMultiplier, "lambda", defaultRetransmitMultiplier, "retransmit multiplier λ")
	flag.IntVar(&config.Protocol.MaxUpdatesPerMessage, "max-updates", defaultMaxUpdatesPerMessage, "updates per message")
	flag.DurationVar(&config.Network.LatencyMin, "latency-min", defaultLatencyMin, "minimum one-way latency")
	flag.DurationVar(&config.Network.LatencyMax, "latency-max", defaultLatencyMax, "maximum one-way latency")
	flag.Float64Var(&config.Network.LossProbability, "loss", defaultLoss, "message loss probability")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: swimsim [flags]\ndefaults are for exploration, not experiments (see evaluation/configs/)")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *nodes <= 0 {
		return errors.New("-nodes must be positive")
	}
	for i := range *nodes {
		config.Nodes = append(config.Nodes, protocol.NodeID(fmt.Sprintf("n%d", i+1)))
	}
	config.QueueCapacity = *queue

	sim, err := simulation.New(config, *seed)
	if err != nil {
		return err
	}
	for _, c := range crashList {
		if err := sim.Schedule(c.at, simulation.Fault{Kind: simulation.Crash, Node: c.node}); err != nil {
			return err
		}
	}
	if err := sim.Run(*until); err != nil {
		return err
	}

	out := bufio.NewWriter(os.Stdout)
	trace := sim.Trace()
	if *jsonOutput {
		if err := writeJSON(out, config, *seed, *until, trace); err != nil {
			return err
		}
		return out.Flush()
	}
	if err := trace.Format(out, *changesOnly); err != nil {
		return err
	}
	changes := 0
	for _, entry := range trace {
		changes += len(entry.Changes)
	}
	fmt.Fprintf(out, "trace hash %x (%d events, %d changes)\n", trace.Hash(), len(trace), changes)
	return out.Flush()
}

// the run and its trace as one JSON document; durations are nanoseconds.
func writeJSON(w io.Writer, config simulation.Config, seed uint64, until time.Duration, trace simulation.Trace) error {
	hash := trace.Hash()
	return json.NewEncoder(w).Encode(struct {
		Config  simulation.Config
		Seed    uint64
		Until   time.Duration
		Hash    string
		Entries simulation.Trace
	}{config, seed, until, hex.EncodeToString(hash[:]), trace})
}
