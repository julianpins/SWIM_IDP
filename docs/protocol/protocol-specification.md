# SWIM Protocol Specification

Implement **SWIM+Inf.+Susp.** (paper Sections 3–4) with round-robin probe selection (Section 4.3). No Lifeguard or other extensions in the baseline. Architecture, simulation and determinism rules live in `CLAUDE.md`.

## Membership

Each entry holds: member ID, network address, Availability Zone, state and incarnation number.

| State | Meaning | Entered when |
|---|---|---|
| Alive | Considered responsive | Join, or a valid Alive update arrives (clears Suspect) |
| Suspect | Failed one probe round, not yet confirmed | Direct and indirect probes both fail |
| Dead | Declared failed | Suspicion timeout expires, or a Confirm arrives |

Suspect members stay eligible for probing. Dead members leave the active list, but their Confirm update may still need to be disseminated.

## Updates and incarnation

An update carries a member ID, a type (Alive, Suspect or Confirm) and an incarnation number.

- The incarnation starts at 0 on join. Only the member itself increments it.
- When a member receives Suspect about itself at its current incarnation, it increments and disseminates Alive.

Precedence (Section 4.2). Ignore any update that doesn't satisfy it:

| Incoming | Existing | Apply when |
|---|---|---|
| Alive(i) | Alive(j) | i > j |
| Alive(i) | Suspect(j) | i > j |
| Suspect(i) | Suspect(j) | i > j |
| Suspect(i) | Alive(j) | i >= j |
| Confirm(i) | Alive/Suspect(j) | always |

## Failure detection

Configurable parameters: `ProtocolPeriod`, `ProbeTimeout`, `IndirectProbeCount`, `SuspicionTimeout`. No universal defaults. Document whatever defaults are chosen.

Each protocol round:
1. Pick the target by round-robin. Send PING and wait up to `ProbeTimeout` for an ACK.
2. If no ACK arrives, send PING-REQ(target, probe ID) to `IndirectProbeCount` intermediaries. Each one PINGs the target and forwards any ACK back.
3. If no valid ACK arrives by the end of the round, mark the target Suspect, disseminate Suspect and start a local suspicion timer.
4. If the timer expires without a valid Alive, mark the target Dead and disseminate Confirm.

ACKs are bound to their probe, so a late or unrelated ACK never completes a different probe. The protocol must not rely on synchronized clocks. Several members can suspect the same target independently; resolve this through the precedence rules alone.

## Dissemination

- Piggyback updates on PING, PING-REQ and ACK only. Never send separate gossip packets.
- Keep a buffer with a local transmission count per update, and send the least-transmitted updates first.
- Cap the number of updates per message. Drop an update after `λ·log(n)` local transmissions, where `n` is the current group size.
- λ and the message-size limit are configurable.

## Probe target selection

Keep an ordered member list and traverse it sequentially. Insert new members at uniformly random positions, and shuffle the list after each full traversal. Keep selection behind its own interface so alternative strategies can be evaluated.

## Dynamic membership

- **Initial group:** every member starts with the full member list (all Alive, incarnation 0). See Decisions.
- **Join:** the joiner contacts one known member, which announces it. Alive is then disseminated. The contact is a static coordinator, as in the paper's prototype (Section 3.2).
- **Leave:** a voluntary leave is announced before shutdown, and is represented separately from crash detection.
- **Restart:** a restarted process is a new member with a new ID, incarnation 0, joining through the coordinator. See Decisions.

## Availability Zones

AZ is metadata only. Expose it to the simulator and the evaluation code. The baseline must not change probe selection, precedence, dissemination or suspicion based on AZ. AZ-aware behavior is a separate experimental extension.

## Invariants

- A failed direct probe alone never leads to Dead. A failed indirect round leads to Suspect, not Dead.
- Only the member itself increments its incarnation. Stale updates never override newer ones, and Confirm overrides Alive and Suspect.
- Late ACKs never complete unrelated probes.
- Every update eventually stops being retransmitted.
- Views may diverge temporarily. Correctness never depends on synchronized clocks.

## Required tests

- **Failure detection:** direct ACK; direct fails but indirect succeeds; both fail.
- **Suspicion:** Alive→Suspect, Suspect→Alive, Suspect→Dead on timeout, self-refutation by incarnation increment.
- **Updates:** precedence table, duplicate and outdated updates, concurrent suspicions, Confirm overriding.
- **Dissemination:** piggybacking on all three message types, prioritization, retransmission limit, eventual delivery under loss.
- **Network/faults:** loss, delay, reordering, duplication, partitions, crash and restart.

## Decisions

- **Initial group (project decision).** Members start with the full member list instead of joining one by one. The paper does not say how its experimental groups were formed. Starting complete keeps join time out of detection-time measurements.
- **Start offset (simulation).** Each member starts its first protocol period at a random offset in `[0, ProtocolPeriod)`, drawn from a seeded stream. Starting all members at once would synchronize every round, which real clusters do not do. The paper does not assume synchronized clocks (Section 3.1).
- **Restart identity (deviation: paper is silent).** The paper initializes incarnation to 0 on join and requires a confirmed member to drop out and rejoin (Section 4.2), but says nothing about restarts or identifiers. A restarted process therefore joins as a new member with a new ID (`<id>#<restart count>`) and incarnation 0. Reusing the old ID would let a still-circulating Confirm, which overrides Alive at any incarnation, remove the new process.
- **Updates about unknown members (paper is silent).** Any update about an unknown member is stored with its state and incarnation. Suspect or Confirm can arrive before a joiner's Alive because of reordering; storing keeps that information, and a stored Dead blocks a later stale Alive.
- **Dead entries are kept (tombstones).** A Dead member leaves the probe order but stays in the membership table, so a late Alive at an old incarnation cannot re-add it. The paper keeps recently failed members in a separate list (Section 4.1); expiring tombstones is left for later. The table grows by one entry per ID ever seen, which is bounded by nodes plus restarts.
- **Probe IDs.** One increasing counter per node, starting at 1 (0 means no probe open). An ACK completes the open probe only if both its probe ID and its sender match. It still counts after `ProbeTimeout`, until the period ends.
- **Indirect probing (paper is silent on these details).**
  - Intermediaries are up to `IndirectProbeCount` random non-Dead members other than the node itself and the target. If fewer are available, all of them are used.
  - An intermediary pings the target with its own probe ID from the same counter, so relayed Pings never collide with its own probe. It forwards the target's Ack to the requester as `Ack{requester's probe ID, Target: target}`.
  - The requester accepts a forwarded Ack only from one of the intermediaries it chose for that probe.
  - An intermediary drops its relay entry after one `ProtocolPeriod` (`RelayTimeout`). By then the requester's period has ended, and a later Ack would be useless. This keeps the relay table bounded.
- **Suspicion.**
  - A target is suspected at `PeriodEnd` if neither a direct nor a forwarded Ack arrived (paper step 3).
  - Every change to Suspect starts a timer of the fixed `Config.SuspicionTimeout`. The formula is still open.
  - The timer carries the suspected incarnation, and it declares Dead only if the member is still Suspect at that incarnation. A stale timer from before a refutation can therefore never kill a re-suspected member.
- **Gossip.**
  - **Retransmission limit.** Each update is sent at most `⌈RetransmitMultiplier · ln(n)⌉` times, and at least once. `n` is the number of non-Dead members, including self. The paper leaves λ and the log base open; the base only rescales λ.
  - **Message size.** Each message carries at most `MaxUpdatesPerMessage` updates. A byte limit follows once serialization exists.
  - **Order.** The least-transmitted updates go first, with ties broken by member ID.
  - **Buffer.** It holds at most one update per member. A newer update replaces the older one and resets its count. Dead updates are disseminated like any other.
  - **Refutation.** Suspect about self at the current incarnation or higher sets the incarnation to that value + 1 and enqueues Alive. Alive about self and stale suspicions are ignored. Dead about self is ignored until join exists; the paper requires a rejoin.
- **Observability.**
  - Every change to a member's entry in a node's view is emitted as a `Changed` output: member, previous and new state, incarnation, and whether the member was known before. It carries no timestamp; the environment adds one.
  - The initial member list emits nothing.
  - A refutation emits nothing at the refuting node. It becomes visible as Suspect → Alive in the views of the nodes that suspected it, which is all the detection metrics need.
  - The simulator stores the changes with the trace entry of the event that caused them, so they are part of the trace hash.
- **Ping from an unknown sender.** It is answered, but the sender is not added to the view. Members are added only through updates; the paper learns about members only through join announcements and dissemination.
- **Coordinator.** Rejoins contact the first member of the configured node list. If that member is crashed, the first live one is used instead (our addition; the paper uses a single static coordinator).

## Open decisions

Document and justify each of these before or during implementation:
- default period and timeout
- indirect probe count
- suspicion timeout formula
- values for `RetransmitMultiplier` and `MaxUpdatesPerMessage`; maximum message size in bytes
- serialization format
- leave representation
- membership scope across AZs

Never introduce behavior that neither the paper nor this document specifies without saying so.
