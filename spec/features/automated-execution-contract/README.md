---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Automated execution contract

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-co/ext-competios/spec/features/automated-execution-contract?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-co/ext-competios/spec/features/automated-execution-contract?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-co/ext-competios/spec/features/automated-execution-contract?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-co/ext-competios/spec/features/automated-execution-contract?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

Expose a dependency-free, game-agnostic execution, lifecycle, evidence and token contract with conformance fixtures for Competios and game providers.

## Problem

The current public contract can launch scheduled human lineups and return a
start/result, but its `UserID`, lineup, side and required `StartsAt` shape cannot
describe frozen provider-owned participants or unattended execution. Adapter
IDs and durable command receipts protect domain transitions; they do not
authenticate a remote caller.

Copying a private request type into each game would let Chess-specific concepts
leak into Competios and make the next game invent another protocol. Sharing a
JWT implementation here would violate this repository's standard-library-only
boundary and couple every consumer to one security library.

**Mechanic:** this module defines the sealed request/receipt vocabulary and the
observable rules every provider must satisfy; each service implements the
transport and cryptography behind narrow ports. **Real-world analogy:** it is
the standard entry form and result envelope used by every venue, while each
venue still owns its locks, playing rules and equipment.

## Behavior

### Complete provider journey

Competios builds one immutable N-slot execution request, calculates its
canonical payload digest, obtains an exact-operation access token issued by the
selected game, and submits both to that provider. The provider verifies
authority and current eligibility before creating or replaying one execution
receipt. It later obtains a separate Competios-issued token for each allowed
start/result delivery. Competios accepts start before result and preserves the
same durable command acknowledgement on safe retry.

Taking no action creates no provider call or token. An expired transport token
is replaced while the same business command/body is retried. Wrong audience,
scope, provider, contest, command, digest, method or resource fails before an
execution receipt. A changed payload under one command or token ID conflicts;
unknown delivery remains retryable; cancellation closes authority even while a
token is unexpired.

### Generic execution vocabulary

The Competios automated execution contract v1 contains:

- stable provider, adapter, competition, contest, request and command IDs;
- ordered participant/version slots with generic artifact references;
- opaque, digest-bound provider configuration and requested public artifacts;
- not-before/deadline and callback resource facts, without a fabricated human
  schedule confirmation;
- one execution receipt with immutable provider instance and safe artifact
  links;
- typed queued/running/completed/failed/cancelled lifecycle events;
- ordered N-participant placements, ties, generic failure/adjudication facts;
  and
- a recorded-provenance envelope of source/artifact, provider/runtime/rules,
  limits, seed, transcript and execution digests.

Slot sides, Chess colours, pieces, Starlark, bids, player passwords and rating
math are absent. Chess Raiders constrains the generic request to two frozen bot
slots in its provider profile. A Bidding Tic-Tac-Toe fake uses the same types
with its own opaque configuration and result semantics.

### Token contract without a crypto implementation

The public module names five mutually exclusive credential purposes across four
trust domains but never parses or signs a JWT:

- a game-issued execution-launch access token for Competios;
- a separate game-issued source-validation access token for Competios;
- a Competios-issued event access token for the game;
- human platform/OIDC identity; and
- a GitHub App installation token.

Execution and event grant facts include exact issuer, subject, audience, token
type, scope, key ID, issued/not-before/expiry time, token ID, provider,
competition, contest, business command, canonical body digest, HTTP method and
resource. Implementations pin algorithms/issuer metadata and validate the token
before constructing a trusted grant value. The business service accepts only
that trusted value and still reauthorises stored state.

Token ID is short-lived transport replay identity. Command ID plus body digest
is the durable idempotency identity. Conformance proves that a new token can
retry the same command/body after an unknown outcome and that neither identity
can be reused for a different operation.

Source-validation grants additionally bind participant/version, repository node
ID, full commit, canonical path, expected manifest/artifact digests and byte
limit. The contract carries retained bytes/metadata, never a GitHub installation
token; validation and launch grants are not interchangeable.

### Greenfield cutover and compatibility

There are no production consumers or data requiring compatibility. Inventory
all callers, then replace the provisional scheduled-lineup wire shape with the
new discriminated request/profile model in one coordinated release. Preserve
participant-scheduled product behavior as an explicit profile; do not retain a
second legacy contract, dual decoder or migration adapter.

The Go module continues to depend only on the standard library. Stable JSON
fixtures, canonical digest vectors and `contract4competiostest` conformance are
the interoperability boundary. Crypto, HTTP handlers, repositories, workers
and game/domain logic remain in owning repositories.

## Acceptance Criteria

### AC: two-unrelated-providers-conform

**Given** a Chess Raiders two-bot provider, an independently implemented Bidding
Tic-Tac-Toe fake and a minimal three-slot synthetic provider<br>
**When** they consume the same generic request and emit lifecycle/result facts<br>
**Then** all pass one conformance suite, including tied N-slot ranks, without
adding game vocabulary to the public contract, while a deliberately invalid
provider proves the suite fails.

### AC: launch-grant-is-one-operation

**Given** a valid game-issued execution grant<br>
**When** any audience, scope, provider, competition, contest, command, body
digest, method, resource, time, token type or key fact differs<br>
**Then** the request fails before a provider receipt; the exact authorised
request creates or replays only one receipt.

### AC: event-grant-and-command-replay-are-distinct

**Given** a game received an unknown outcome while submitting a start or result<br>
**When** it obtains a fresh Competios event token and retries the identical
durable command/body<br>
**Then** the original acknowledgement is replayed, while token-ID or command-ID
reuse with changed content conflicts without a second lifecycle fact.

### AC: source-validation-and-launch-never-cross

**Given** one game-issued validation grant and one launch grant<br>
**When** either is sent to the other resource or source metadata/bytes differ
from participant/version/commit/path/digest/limit binding<br>
**Then** it fails before compilation, artifact persistence or match creation and
no GitHub credential/private source enters a safe error or log.

### AC: lifecycle-and-result-order-fail-closed

**Given** a queued execution with no accepted start<br>
**When** result-before-start, duplicate-conflicting result, wrong instance,
cancelled contest or out-of-order lifecycle evidence arrives<br>
**Then** conformance rejects it and preserves the last valid immutable state.

### AC: provenance-round-trips-without-game-fields

**Given** frozen artifacts and provider-supplied recorded provenance<br>
**When** the request, receipt, lifecycle and result fixtures round-trip through
JSON<br>
**Then** canonical IDs/digests/order/ties remain byte-stable and no private
source, credential or game-specific field appears in a public-safe projection.

### AC: full-consumer-cutover

**Given** every current Competios and Chess Raiders contract consumer is
inventoried<br>
**When** the new module release is adopted<br>
**Then** all callers and conformance tests use one discriminated execution
contract, the superseded wire shape is removed, and no dual path remains.

### AC: module-remains-portable

**Given** the completed public contract and conformance packages<br>
**When** dependency and build checks run<br>
**Then** the Go module has no non-standard-library dependency, secret, private
module import, network requirement or host/domain implementation.

## Open Questions

No contract-local product questions. Provider/token policy questions live in
the Competios `automated-competitions/provider-security-and-federated-identity`
Feature; Chess manifest/runtime questions live in the Chess Raiders Bots Cup
Feature.

---
*This document follows the https://specscore.md/feature-specification*
