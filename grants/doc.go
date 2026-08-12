// Package grants implements Season 1 of Decision 0007's execution-grant
// issuer/verifier: an in-process, HMAC-signed at+jwt profile behind the
// dependency-free contract4competios.OperationGrantIssuer/
// OperationGrantVerifier interfaces.
//
// Two trust directions exist, each with its own issuer, verifier, and
// symmetric secret:
//
//   - Chess-issued: the five source/launch purposes (manifest-closure-plan,
//     candidate-closure-validate-retain, artifact-disclosure-verify,
//     artifact-publish, contest-launch). Chess Raiders is the resource owner
//     and issues Competios narrow, revocable authority over its own
//     artifacts.
//   - Competios-issued: the two event purposes (contest-started-event,
//     contest-result-event). Competios issues the game a token scoped to one
//     contest start or result submission.
//
// An Issuer or Verifier configured for one Direction refuses every purpose
// outside that direction's permitted set, regardless of what a caller
// requests or what a presented token claims.
//
// This package never reimplements contract4competios.ValidateOperationGrant
// or validGrantPurposeScope -- both run on every issued and every verified
// grant. It owns only: claim assembly, HMAC signing, key selection by kid,
// JWT parsing, the fixed verification order, replay-store integration, and
// mapping failures onto the contract's stable errors.
//
// The cryptographic seam is the three-method KeyMaterial interface. HMACKey
// (Season 1) is the only implementation today; adding Ed25519 later is a
// second implementation of that interface -- no other type in this package
// needs to change, per the founder's "honest contract and transport"
// requirement recorded in Decision 0007's Season 1 amendment.
//
// Season 1 ships no token endpoint and no JWKS distribution: a service holds
// its Direction's secret and kid directly (env-sourced by its host) and
// calls NewIssuer/NewVerifier in-process. Both types depend on no HTTP
// transport, so adding a token endpoint or a JWKS document later is new
// transport in front of unchanged issuance and verification logic.
package grants
