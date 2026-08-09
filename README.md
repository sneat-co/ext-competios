# ext-competios

The public contract for **Competios** — the Sneat competitions and tournament
engine. Types and interfaces only: what a game must implement to be launched as
a contest, and what it must report back when the contest ends.

The engine itself is private (`sneat-co/competios`). This repository is the seam
a game integrates against.

## What's here

| Package | Purpose |
|---|---|
| [`backend/contract4competios`](backend/contract4competios) | The contract: `LaunchRequest`, `OrderedResult`, `ContestStart`, the `GameLauncher` / `ResultSink` / `ContestStartSink` interfaces, capabilities, projections and drafts. |
| [`backend/contract4competiostest`](backend/contract4competiostest) | The conformance harness an implementor runs against their own adapter. |

```
go get github.com/sneat-co/ext-competios/backend
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
originate here and flow inward.
