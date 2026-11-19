# SWIM: Paper Notes

Das, A., Gupta, I., Motivala, A. (2002). *SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol.* DSN 2002, pp. 303–312. [IEEE](https://ieeexplore.ieee.org/document/1028914) · [PDF](https://s3.amazonaws.com/systemsandpapers/papers/swim.pdf)

This paper is the primary reference for protocol semantics. `protocol-specification.md` states what this project implements. This file lists only what the paper says beyond that.

## Section map

| Section | Topic |
|---|---|
| 3.1 | Basic failure detection: PING, PING-REQ, ACK, protocol period `T'`, `k` intermediaries |
| 4.1 | Infection-style dissemination by piggybacking |
| 4.2 | Suspicion, incarnation numbers, update precedence |
| 4.3 | Round-robin probe selection |
| 5 | Prototype evaluation |

## Details from the paper not spelled out in the spec

- The probe timeout comes from an estimated RTT. The protocol period must be at least 3× that RTT estimate.
- Messages carry a sequence number that identifies the initiator's protocol period. This is how ACKs are matched to probes.
- Piggybacking adds no extra packets. Expected dissemination latency grows logarithmically with group size.
- Round-robin bound: with at most `n_i` members, two selections of the same target are at most `2·n_i − 1` periods apart (time-bounded completeness).
- Properties: constant expected message load per member, constant expected time until some non-faulty member first detects a failure, weakly consistent views.

## Evaluated variants (Section 5)

| Variant | Dissemination | Suspicion |
|---|---|---|
| SWIM:Basic | Multicast | No |
| SWIM+Inf. | Infection-style | No |
| SWIM+Inf.+Susp. | Infection-style | Yes (this project's baseline) |

Prototype settings, which are experimental values and not defaults:
- protocol period 2 s
- K = 1
- max dissemination count `3·⌈log(N+1)⌉`
- suspicion timeout `⌈log(N+1)⌉` periods

Findings: per-member overhead stays roughly constant as N grows, and median dissemination takes a few periods. Suspicion makes the protocol much more robust to packet loss, at the cost of slower failure detection.

## Not covered by the paper (project-level decisions)

- AZ-aware probing, and the latency and loss between AZs.
- Storage-specific recovery, and how SWIM interacts with SeaweedFS heartbeats.
- Restart and identity semantics beyond incarnation numbers.

Later extension: Lifeguard (*SWIM-ing with the Lifeguards*), which deals with false positives caused by local overload. Keep it separate from the baseline.
