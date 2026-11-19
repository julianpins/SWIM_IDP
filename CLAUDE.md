# CLAUDE.md

SWIM failure detector (Das, Gupta, Motivala 2002) integrated with SeaweedFS and evaluated against SeaweedFS's heartbeat baseline, using deterministic simulation and real-network benchmarks.

## Read first

Before changing protocol behavior, read `docs/protocol/protocol-specification.md`. It covers what to implement, the invariants, required tests and open decisions. `docs/protocol/paper-notes.md` maps paper sections and lists the paper's experimental settings. The paper decides protocol semantics. The spec records project decisions. If both leave something open, raise it before deciding, then record the decision in the spec.

## Stack

- Go workspace (`go.work`) with independent modules `swim/` and `simulation/`.
- SeaweedFS (Go), pinned in `seaweedfs/upstream/`.
- UDP transport for real deployments, simulated transport for DST.
- YAML experiment configs. Python evaluation (`uv`, pandas, NumPy, matplotlib) with type hints.
- LaTeX report, PowerPoint presentation, GitHub Actions CI.

## Layout

- `swim/`: protocol module. `internal/protocol` (probing, suspicion, gossip, incarnation), `internal/membership` (membership table; all state transitions go here), `internal/transport` (UDP, serialization), `pkg/swim` (minimal, stable public API), `test/`.
- `simulation/`: `engine` (virtual clock, scheduler), `network` (latency, jitter, loss, duplication, reordering, partitions), `faults` (crash, restart, overload), `scenarios` (parameterized, kept apart from engine logic), `runner`.
- `seaweedfs/`: `upstream/` is never edited in place (changes go into documented patches or a fork). `integration/` holds the adapters from SWIM events to SeaweedFS.
- `evaluation/`: `configs/` (all experiment parameters), `experiments/` (orchestration only, no protocol logic), `data/raw/` (immutable, large files not committed), `data/processed/`, `analysis/`, `plots/`. Final figures go in `report/figures/`.
- Also `research/` (literature notes, Markdown with citations), `docs/`, `report/`, `presentation/`, `scripts/`.

## Architecture rules

- `swim/` must not import SeaweedFS. Integration logic belongs in `seaweedfs/integration/`.
- Inject transport, clock, scheduling and randomness. Protocol code never uses UDP sockets or system time directly.
- One SWIM implementation runs on both the real network and the simulator. Never write a simulated copy.
- The baseline is SWIM+Inf.+Susp. with round-robin probing. Extensions (Lifeguard, AZ-aware behavior) are separate, named and configurable. In the baseline, AZ is metadata only.
- SWIM membership is advisory. Suspicions and Dead declarations must not trigger storage reassignment, replica removal or authoritative membership changes.
- Leave SeaweedFS's existing heartbeat intact as the comparison baseline.
- Use small Go interfaces at component boundaries, idiomatic Go, `gofmt`, explicit errors and `context.Context` for lifecycle. No global mutable state, unneeded concurrency or speculative abstractions.
- When a public interface changes, check every consuming module and update `docs/` and the affected READMEs.

## Simulation

Simulation is deterministic: virtual clock, seeded randomness, no real timers and no uncontrolled goroutine scheduling. The same initial state, config, seed and event ordering must produce the same trace.

## Testing

- Run `go test ./...` from the module directory.
- New protocol features need unit tests, plus integration tests where applicable. Test SWIM without SeaweedFS, on both real and simulated transports.
- Simulation tests verify replay by comparing traces for identical seeds and configs.
- Keep fast unit tests separate from long integration tests and experiments.
- Never claim tests passed without running them.

## Experiments and data

- Experimental parameters belong in `evaluation/configs/`. Never hardcode them in code or analysis scripts.
- Use one result schema for heartbeat, SWIM and its variants. Every result records protocol version, Git commit, config, seed, cluster size and network conditions.
- Measure suspicion time, failure-declaration time and membership-convergence time separately.
- Run multiple seeds per configuration. Keep simulated and real-network results separate.
- Never edit raw results, fabricate data or silently drop runs. Document failed runs, missing data and limitations. Derived data, plots and tables must be reproducible from raw data plus scripts.

## Working style

- Read the module's README and the relevant docs first, and work in the component that owns the functionality.
- Make small, incremental changes with tests alongside them. No large refactorings, and never edit generated files or build artifacts.
- Before substantial architectural changes, lay out the alternatives and trade-offs. Ask when a requirement is ambiguous or conflicts with an existing decision.
- Document deviations, assumptions and implementation decisions in the relevant doc.
