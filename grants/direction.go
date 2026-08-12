package grants

import (
	"errors"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// MaxVerifierLeeway is the largest clock-skew allowance a Direction may
// configure for verification. Decision 0007's Season 1 amendment calls for a
// "small leeway" bound at 30 seconds; a Direction asking for more fails
// closed at construction rather than silently widening the token's five
// -minute window.
const MaxVerifierLeeway = 30 * time.Second

var (
	// ErrDirectionMisconfigured is returned by NewIssuer/NewVerifier when a
	// Direction is missing a required field or asks for an out-of-bounds
	// leeway. It never depends on request or token content.
	ErrDirectionMisconfigured = errors.New("grants: direction is misconfigured")

	// ErrPurposeNotPermitted is returned when a request or a verified token
	// names a purpose outside the Direction's configured set. An Issuer or
	// Verifier configured for one direction refuses every other direction's
	// purpose, per Decision 0007.
	ErrPurposeNotPermitted = errors.New("grants: purpose is not permitted for this direction")
)

// Direction is the shared configuration behind one issuer/verifier pair for
// one trust direction (Chess-issued or Competios-issued, per Decision 0007).
// Both the Issuer and the Verifier built from the same Direction refuse any
// purpose outside Purposes, regardless of what a caller requests or a token
// claims.
type Direction struct {
	// Name identifies this direction in logs only (e.g. "chess-issued",
	// "competios-issued"). It never appears in a token.
	Name string

	// Issuer, Subject, and Audience are the fixed iss/sub/aud this direction
	// issues and expects. Subject names the fixed recipient identity (e.g.
	// "svc:competios" for the chess-issued direction) -- OperationGrant
	// carries one Subject per token, but every token from one Direction
	// shares the same recipient, so it is direction-level configuration, not
	// per-request.
	Issuer   string
	Subject  string
	Audience string

	// Purposes is the exact, closed set of contract4competios.GrantPurpose
	// values this direction may issue or accept. Per Decision 0007, an
	// issuer/verifier configured for one direction must refuse every other
	// direction's purpose.
	Purposes []contract4competios.GrantPurpose

	// Key is the active KeyMaterial: the Issuer signs with it, and the
	// Verifier trusts it (in addition to any Trusted keys, e.g. during
	// rotation -- Key does not need to be repeated in Trusted).
	Key KeyMaterial

	// Trusted holds additional keys a Verifier accepts by kid without
	// signing new tokens with them -- the overlap window a rotation needs so
	// a new key can be trusted before it is used to sign, and an old key can
	// stop signing before it stops being trusted. Season 1 deployments may
	// leave this nil; a single Key is a fully valid direction.
	Trusted []KeyMaterial

	// Replay is required for a Verifier (NewVerifier fails closed without
	// one) and unused by an Issuer.
	Replay ReplayStore

	// Now overrides the clock. Nil means time.Now().UTC(); tests set this
	// deterministically.
	Now func() time.Time

	// Leeway is the verifier-side allowance for iat/nbf/exp comparisons. Zero
	// means no leeway; a value above MaxVerifierLeeway fails NewVerifier
	// closed rather than silently widening the tolerance.
	Leeway time.Duration
}

func (d Direction) permits(purpose contract4competios.GrantPurpose) bool {
	for _, p := range d.Purposes {
		if p == purpose {
			return true
		}
	}
	return false
}

func (d Direction) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

func (d Direction) trustedKeys() []KeyMaterial {
	keys := make([]KeyMaterial, 0, len(d.Trusted)+1)
	if d.Key != nil {
		keys = append(keys, d.Key)
	}
	keys = append(keys, d.Trusted...)
	return keys
}

func (d Direction) keyByID(kid string) KeyMaterial {
	if kid == "" {
		return nil
	}
	for _, k := range d.trustedKeys() {
		if k != nil && k.KeyID() == kid {
			return k
		}
	}
	return nil
}

func (d Direction) validatePurposes() error {
	if len(d.Purposes) == 0 {
		return ErrDirectionMisconfigured
	}
	seen := make(map[contract4competios.GrantPurpose]bool, len(d.Purposes))
	for _, p := range d.Purposes {
		if contract4competios.GrantScopeForPurpose(p) == "" || seen[p] {
			return ErrDirectionMisconfigured
		}
		seen[p] = true
	}
	return nil
}

func (d Direction) validateForIssue() error {
	if d.Issuer == "" || d.Subject == "" || d.Audience == "" || d.Key == nil {
		return ErrDirectionMisconfigured
	}
	return d.validatePurposes()
}

func (d Direction) validateForVerify() error {
	if d.Issuer == "" || d.Audience == "" || d.Replay == nil {
		return ErrDirectionMisconfigured
	}
	if len(d.trustedKeys()) == 0 {
		return ErrDirectionMisconfigured
	}
	if d.Leeway < 0 || d.Leeway > MaxVerifierLeeway {
		return ErrDirectionMisconfigured
	}
	return d.validatePurposes()
}
