# ext-competios

The public contract for **Competios** — the Sneat competitions and tournament
engine. Types and interfaces only: what a game must implement to be launched as
a contest, and what it must report back when the contest ends.

The engine itself is private (`sneat-co/competios`). This repository is the seam
a game integrates against.

## What's here

| Package | Purpose |
|---|---|
| [`backend/contract4competios`](backend/contract4competios) | The contract: discriminated scheduled/provider-executed requests, immutable N-slot execution receipts and lifecycle evidence, scoped-operation token ports, and staged source-plan/retention/publication/disclosure facts, plus capabilities, projections and drafts. |
| [`backend/contract4competiostest`](backend/contract4competiostest) | Positive and adversarial execution, event-delivery, grant and source-artifact conformance harnesses an implementor runs against its own adapter. |
| [`grants`](grants) | Season 1 (Decision 0007) implementation of `contract4competios`'s `OperationGrantIssuer`/`OperationGrantVerifier` ports: HMAC-signed `at+jwt` tokens, per-direction purpose separation, and in-memory/dalgo-backed replay stores. Its own module, depending on `backend` plus `golang-jwt/jwt` and `dalgo` — the contract module itself stays dependency-free. |

```
go get github.com/sneat-co/ext-competios/backend
go get github.com/sneat-co/ext-competios/grants
```

## Why it is public, and why it has no dependencies

A contract that cannot be imported without credentials is not a contract — it is
a private type declaration that happens to be named like one. Every Sneat
extension is meant to import `*-contract` libraries freely and let the
application wire the implementation; that pattern only works if the contract is
reachable.

So this module holds **one invariant, enforced in CI: it depends on nothing but
the Go standard library.** No `require` block, no `go.sum`. A dependency arriving
here would reintroduce, one level down, exactly the credential requirement this
module exists to remove — so the build fails rather than quietly acquiring one.

## Relationship to `sneat-co/competios`

The private engine imports this module; never the reverse. Contract changes
originate here and flow inward. Provider and event operations receive only
verified grant facts. The narrow issuer/verifier ports carry an opaque encoded
access token without parsing or serializing bearer material.
OAuth/JWT/JWKS/HTTP implementations remain in the owning services.
