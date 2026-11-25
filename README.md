# SWIM_IDP

A reusable Go implementation of SWIM (Das, Gupta, Motivala 2002), evaluated through deterministic simulation and real-network benchmarks, and integrated into SeaweedFS for comparison against its heartbeat mechanism.

Protocol semantics and project decisions: [docs/protocol/protocol-specification.md](docs/protocol/protocol-specification.md). Repository layout and rules: [CLAUDE.md](CLAUDE.md).

## Status

`protocol/` and `simulation/` define the file layout; the files are still empty. The other folders are placeholders.

## Build and test

```sh
go test ./...
```
