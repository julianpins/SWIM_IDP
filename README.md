# SWIM_IDP

A reusable Go implementation of SWIM (Das, Gupta, Motivala 2002), evaluated through deterministic simulation and real-network benchmarks, and integrated into SeaweedFS for comparison against its heartbeat mechanism.

Protocol semantics and project decisions: [docs/protocol/protocol-specification.md](docs/protocol/protocol-specification.md). Repository layout and rules: [CLAUDE.md](CLAUDE.md).

## Status

`protocol/` implements SWIM+Inf.+Susp.: round-robin probing, indirect probing, suspicion, gossip and refutation. `simulation/` runs it deterministically. Join, restart, the real runtime and the SeaweedFS integration are not implemented yet.

## Build and test

```sh
go test ./...
```

## Run a simulation

```sh
go run ./cmd/swimsim -nodes 5 -seed 1 -until 30s -crash n3@2s -changes-only
```

`go run ./cmd/swimsim -help` lists all flags. The defaults are for exploration, not experiments.

To animate a run (up to 10 nodes) as a standalone HTML page, using only the Python standard library:

```sh
go run ./cmd/swimsim -nodes 6 -crash n3@2s -json | python3 scripts/trace_animation.py - -o trace.html
```

If `go` is the snap launcher, redirecting its output to a file (`> trace.json`) fails with `bad file descriptor`. Pipe it as shown above, or build a binary first with `go build -o bin/swimsim ./cmd/swimsim`.

By default, the animation stretches every flight to at least `--min-flight` (0.15 periods), so a Ping and its Ack can overlap on screen. To show the real flight times, raise the latency, pass `--min-flight 0`, and scale the timeouts so that `2 × latency-max < probe-timeout` and `probe-timeout + 4 × latency-max < period`:

```sh
go run ./cmd/swimsim -nodes 6 -crash n3@4s -until 40s \
  -period 2s -probe-timeout 700ms -suspicion 10s \
  -latency-min 200ms -latency-max 300ms -json \
| python3 scripts/trace_animation.py - -o trace.html --min-flight 0
```

These latencies are only for visualization, not for experiments.
