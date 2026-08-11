---
format: https://specscore.md/plan-specification
status: Approved
---
# Plan: Automated execution contract

**Status:** Executing
**Source Feature:** automated-execution-contract
**Date:** 2026-08-11
**Owner:** alex
**Supersedes:** —
**Parent:** competios:chess-raiders-bots-cup-platform

## Summary

Replace the provisional scheduled-lineup seam with one dependency-free,
game-agnostic contract for participant-scheduled and provider-executed contests,
exact-operation service grants, lifecycle/result delivery and recorded
provenance. Publish conformance before Competios and Chess Raiders consume it.
This is the external-contract child of the
[Competios cross-repository master](https://github.com/sneat-co/competios/blob/main/spec/plans/chess-raiders-bots-cup-platform.md).

## End-to-end journey

| Stage | Caller action or null action | Observable good result at both services | Isolation proof |
|---|---|---|---|
| No request | Competios never claims an executable contest. | No token, provider call, receipt or game instance exists. | Fake issuer/provider observation count remains zero. |
| Authorise and launch | Competios hashes one request, obtains the game-issued exact-operation token and calls the named provider. | The game creates/replays one receipt; wrong provider/contest/command/body/method/resource creates nothing. | Grant vectors plus good/bad provider conformance. |
| Start | The game obtains one Competios event token and reports the bound instance start. | Competios accepts start once; another contest/instance or result-before-start fails without state drift. | Cross-contest and lifecycle-order fixtures. |
| Unknown outcome | A response is lost or a token expires. | A fresh token retries the same durable command/body and receives the original acknowledgement; changed content conflicts. | Replay matrix distinguishes token ID from command ID. |
| Finish/fail | The provider returns N-slot ordered/tied result and evidence, or cancellation/failure races delivery. | Competios sees one immutable generic terminal fact; late invalid evidence is rejected; no game field/private source leaks. | Chess, Bidding Tic-Tac-Toe, three-slot and adversarial round trips. |

## Approach

Start with the complete no-action/request/retry/start/result journey and an
inventory of every existing consumer. Freeze JSON/digest vectors and threat
model before Go types. Build generic execution and trusted-grant types in
parallel, then make a Chess-shaped provider, a deliberately different Bidding
Tic-Tac-Toe fake and bad implementations prove the conformance boundary.

This repository owns vocabulary and tests only. OAuth/JWT/JWKS, HTTP, storage,
workers and domain rules live in their service repositories. Since there are no
production users, publish one coordinated clean cutover: no legacy decoder,
dual interface or migration package. Provider-first release precedes exact
consumer bumps.

## Tasks

### Task 1: Freeze journey, consumer inventory and wire invariants

**Id:** task-1
**Verifies:** automated-execution-contract#ac:full-consumer-cutover, automated-execution-contract#ac:two-unrelated-providers-conform
**Depends-On:** —
**Status:** in_progress

Inventory every `LaunchRequest`, `GameLauncher`, start/result and conformance
consumer in Competios, Chess Raiders and host composition. Record canonical
JSON, time/order/digest rules and null-action, unknown-outcome, cancellation,
conflict and adjudication sequences. Prove the proposed vocabulary represents
both Chess Raiders and a Bidding Tic-Tac-Toe fake before changing types.

### Task 2: Define the generic execution and evidence contract

**Id:** task-2
**Verifies:** automated-execution-contract#ac:provenance-round-trips-without-game-fields, automated-execution-contract#ac:lifecycle-and-result-order-fail-closed
**Depends-On:** 1
**Status:** planning

Add discriminated scheduling/execution profiles, N-slot immutable participant
versions, request/receipt/lifecycle/result and recorded-provenance types with
stable errors and validators. Preserve human scheduling behavior through its
explicit profile, while removing user-lineup/side assumptions from the generic
provider-executed path. Freeze positive and malformed JSON fixtures.

### Task 3: Define trusted launch and event grant semantics

**Id:** task-3
**Verifies:** automated-execution-contract#ac:launch-grant-is-one-operation, automated-execution-contract#ac:event-grant-and-command-replay-are-distinct, automated-execution-contract#ac:source-validation-and-launch-never-cross
**Depends-On:** 1
**Status:** planning

Add dependency-free trusted-grant facts and issuer/verifier/client ports for
game-issued launch/source-validation and Competios-issued event authority. Bind provider,
competition, contest, command, body digest, method and resource; separate token
replay from durable command replay. Publish time/skew/rotation/retry vectors but
leave cryptographic and HTTP implementations outside this module.

### Task 4: Expand positive and adversarial conformance

**Id:** task-4
**Verifies:** automated-execution-contract#ac:two-unrelated-providers-conform, automated-execution-contract#ac:launch-grant-is-one-operation, automated-execution-contract#ac:event-grant-and-command-replay-are-distinct, automated-execution-contract#ac:source-validation-and-launch-never-cross, automated-execution-contract#ac:lifecycle-and-result-order-fail-closed
**Depends-On:** 2, 3
**Status:** planning

Run the same suite against Chess-shaped, independently implemented Bidding
Tic-Tac-Toe and minimal three-slot tied-rank fakes. Add
deliberately bad providers/verifiers for wrong slot count, audience, scope,
operation binding, payload conflict, result order and premature result. Verify
identical retries return the original receipt and no rejection leaks a private
participant fact.

### Task 5: Prove portability and deterministic wire behavior

**Id:** task-5
**Verifies:** automated-execution-contract#ac:provenance-round-trips-without-game-fields, automated-execution-contract#ac:module-remains-portable
**Depends-On:** 2, 3, 4
**Status:** planning

Add compile-time/API, JSON round-trip, canonical digest and fuzz/property tests.
Keep `backend/go.mod` free of `require` entries and prove tests need neither
network nor credentials. Reject private imports, logging of bearer material and
game-specific field names in the public execution types.

### Task 6: Cut every provider and consumer to the release candidate

**Id:** task-6
**Verifies:** automated-execution-contract#ac:full-consumer-cutover, automated-execution-contract#ac:event-grant-and-command-replay-are-distinct
**Depends-On:** 4, 5
**Status:** planning

Coordinate exact prerelease consumers in Competios and Chess Raiders, update
their released contract imports and run their real conformance/e2e paths.
Publish the host cutover requirements and accept the dedicated Sneat Go child
plan's receipt; this contract task authors no host file. Remove the superseded
scheduled-lineup wire and all local contract copies in the same wave. Search the
fleet to prove no old caller remains; do not add a compatibility adapter to
satisfy an incomplete cutover.

### Task 7: Publish provider-first and verify the remote graph

**Id:** task-7
**Verifies:** automated-execution-contract#ac:full-consumer-cutover, automated-execution-contract#ac:module-remains-portable
**Depends-On:** 6
**Status:** planning

Land and tag the public Go module first, bump each consumer to the exact release,
run full contract/Competios/Chess verification and observe post-merge CI. Prove
remote target receipt in dependency order, then terminally clean every WB branch
and worktree; a local-only graph is not complete.

## Open Questions

No plan-local questions. Contract and token choices remain in the source Feature
and the owning Competios/Chess Raiders Features.

---
*This document follows the https://specscore.md/plan-specification*
