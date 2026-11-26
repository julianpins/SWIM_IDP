# CLAUDE.md

Implement SWIM (Das, Gupta, Motivala 2002) as a reusable Go protocol, evaluated through deterministic simulation and real-network benchmarks, with the goal of integrating it into SeaweedFS and comparing it against SeaweedFS's heartbeat mechanism.

## Read first

Before changing protocol behavior, read `docs/protocol/protocol-specification.md`. It defines the implementation, invariants, required tests and open decisions. `docs/protocol/paper-notes.md` maps paper sections and records relevant experimental settings.

The paper defines original SWIM semantics. The specification records project decisions and deviations. If something is unclear or undecided, raise it before choosing and document the decision in the specification.

## Stack

- Go; start with one module unless separate modules become necessary.
- SeaweedFS (Go), pinned in `seaweedfs/upstream/`.
- UDP transport for real deployments; simulated transport for DST.
- YAML experiment configs. Python evaluation (`uv`, pandas, NumPy, matplotlib) with type hints.
- LaTeX report, PowerPoint presentation, GitHub Actions CI.

## Layout

- `protocol/`: reusable SWIM core — probing, suspicion, gossip, incarnation numbers and membership state transitions.
- `simulation/`: deterministic DST — virtual clock, event scheduler, simulated network, fault injection, scenarios and runner.
- `cmd/`: executables (`swimsim` simulator CLI, later the runtime node); wiring only.
- `runtime/`: real execution environment — UDP transport, real timers, process lifecycle and metrics.
- `seaweedfs/`: `upstream/` is never edited in place; changes go into documented patches or a fork. `integration/` contains adapters between SWIM and SeaweedFS.
- `evaluation/`: `configs/` (experiment parameters), `experiments/` (orchestration only), `data/raw/` (immutable, large files not committed), `data/processed/`, `analysis/` and `plots/`. Final figures go in `report/figures/`.
- `docs/`, `report/`, `presentation/`, `scripts/`.

## Architecture rules

- `protocol/` must be independent of the simulator, runtime and SeaweedFS.
- The same protocol implementation must run in simulation and real deployments. Never create a simulation-only copy.
- The protocol is a pure state machine: the environment feeds it messages and expired timers, and it returns sends and timer requests as outputs. Randomness is injected (`protocol.Rand`). Protocol logic must never access system time, real timers, UDP sockets or global randomness directly.
- Represent protocol deadlines explicitly through timer events. The simulator uses virtual time; the real runtime uses wall-clock time and actual timers.
- SeaweedFS integration belongs in `seaweedfs/integration/` and must remain isolated from the protocol core.
- The baseline is SWIM+Inf.+Susp. with round-robin probing. Extensions (e.g. Lifeguard, AZ-aware behavior) must be separate, named and configurable. In the baseline, AZ is metadata only.
- SWIM membership is advisory. Suspicions and Dead declarations must not trigger storage reassignment, replica removal or authoritative membership changes.
- Leave SeaweedFS's existing heartbeat intact as the comparison baseline.
- Use small Go interfaces, idiomatic Go, `gofmt`, explicit errors and `context.Context` for lifecycle. Avoid global mutable state, unnecessary concurrency and speculative abstractions.

## Simulation

- Use a virtual clock and discrete-event scheduler. Never use real sleeping, timers or wall-clock time in simulation.
- Schedule message delivery, protocol timeouts, suspicion expiration, crashes and restarts as events.
- Process events in deterministic timestamp order with a deterministic tie-breaking rule.
- Use seeded randomness for all stochastic behavior.
- Identical initial state, configuration, seed and event ordering must produce identical traces.
- Support timer cancellation or invalidation without relying on wall-clock behavior.
- The simulator executes the actual protocol core; simulation infrastructure must not implement protocol behavior.
## Testing

- Run `go test ./...` from the relevant module directory.
- Test protocol state transitions independently of the simulator.
- New features need unit tests and, where applicable, simulation and real-network integration tests.
- Simulation tests verify replayability by comparing traces for identical seeds and configurations.
- Keep fast unit tests separate from long integration tests and experiments.
- State assumptions behind safety and liveness properties.
- Never claim tests passed without running them.

## Coding Style

Follow [TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) when writing Go code.

Key principles:
- Prefer explicit, readable code over clever abstractions or excessive conciseness.
- Use descriptive names and make units explicit (e.g. `timeout_ms`, `latency_ns`).
- Keep functions small, focused and easy to reason about.
- Make assumptions, invariants and preconditions explicit. Validate inputs at component boundaries.
- Avoid hidden control flow, implicit state changes and unnecessary mutable state.
- Handle errors explicitly; never silently ignore unexpected failures.
- Use fixed limits and bounded resource usage where practical, especially for queues, buffers and message sizes.
- Prefer deterministic behavior and explicit dependencies over implicit global state.
- Add assertions and tests for important invariants and edge cases.
- Document non-obvious design decisions and explain why, not just what.

Apply these principles idiomatically in Go. Do not mechanically copy TigerBeetle's Zig-specific conventions or introduce unnecessary complexity.

## Experiments and data

- Experimental parameters belong in `evaluation/configs/`; never hardcode them in protocol or analysis code.
- Use one result schema for heartbeat, SWIM and its variants. Every result records protocol version, Git commit, config, seed, cluster size and network conditions.
- Measure suspicion time, failure-declaration time and membership-convergence time separately.
- Run multiple seeds per configuration. Keep simulated and real-network results separate.
- Never edit raw results, fabricate data or silently drop runs. Document failed runs, missing data and limitations. Derived data, plots and tables must be reproducible from raw data and scripts.

## Working style

- Read the relevant documentation and README before changing a component.
- Work in the component that owns the functionality.
- Make small, incremental changes with tests alongside them. Avoid large refactorings; never edit generated files or build artifacts.
- Before substantial architectural changes, explain alternatives and trade-offs. Ask when requirements are ambiguous or conflict with existing decisions.
- Document deviations, assumptions and implementation decisions in the relevant documentation.

## Reference implementation

My Viewstamped Replication implementation: `https://github.com/julianpins/Deterministic-Simulation-Testing-for-VSR/blob/main/main.c`

Use it as a reference for codestyle / conciseness, deterministic simulation, event scheduling, reproducibility, traces and invariant-based testing. Do not copy its architecture blindly; SWIM must also support a real runtime and SeaweedFS integration.