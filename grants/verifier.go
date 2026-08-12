package grants

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

var (
	// ErrWrongTokenType is returned when a token's JOSE "typ" header is not
	// exactly "at+jwt".
	ErrWrongTokenType = errors.New("grants: typ header is not at+jwt")

	// ErrUnknownKey is returned when a token's "kid" header does not resolve
	// to a key this Verifier's Direction trusts, or when the resolved key's
	// algorithm does not match the token's signing algorithm (an algorithm-
	// confusion guard).
	ErrUnknownKey = errors.New("grants: kid does not resolve to a trusted key for this direction")

	// ErrTokenExpired wraps a golang-jwt expiry/not-yet-valid failure. It is
	// deliberately distinct from contract4competios.ErrTokenReplayConflict:
	// the two mean different things operationally (see Decision 0007) and
	// must be distinguishable in logs.
	ErrTokenExpired = errors.New("grants: token is outside its valid time window")
)

// Verifier checks at+jwt operation grants for exactly the purposes
// configured in its Direction, in the fixed order Decision 0007 requires:
// parse header -> check typ -> resolve kid to a trusted key -> verify
// signature -> check iss/aud -> time window -> decode claims into
// OperationGrant -> ValidateOperationGrant -> replay check. It fails closed
// at the first failing step; only steps 1-4 run inside golang-jwt's Keyfunc
// (which the library invokes before verifying the signature), so an
// untrusted-key or wrong-typ token is refused before any HMAC comparison
// runs against it.
type Verifier struct {
	direction Direction
}

// NewVerifier validates direction before returning a usable Verifier. A nil
// return always pairs with a non-nil error.
func NewVerifier(direction Direction) (*Verifier, error) {
	if err := direction.validateForVerify(); err != nil {
		return nil, err
	}
	return &Verifier{direction: direction}, nil
}

// VerifyOperationGrant implements contract4competios.OperationGrantVerifier.
// It never returns a grant the caller can trust the payload/route facts of
// without ALSO comparing the returned VerifiedOperationGrant against the
// exact OperationRouteBinding and payload digests of the request actually
// received -- see RouteBinding and the per-purpose
// contract4competios.ValidateXGrantForRequest helpers, which this package
// does not re-derive.
func (v *Verifier) VerifyOperationGrant(ctx context.Context, token contract4competios.EncodedAccessToken) (contract4competios.VerifiedOperationGrant, error) {
	if v == nil || token == "" {
		return contract4competios.VerifiedOperationGrant{}, contract4competios.ErrInvalidGrant
	}

	var (
		resolvedKey KeyMaterial
		keyFuncErr  error
	)
	keyFunc := func(t *jwt.Token) (any, error) {
		// Step 2: check typ.
		typHeader, _ := t.Header["typ"].(string)
		if typHeader != tokenTypeHeader {
			keyFuncErr = ErrWrongTokenType
			return nil, keyFuncErr
		}
		// Step 3: resolve kid to a trusted key for this direction.
		kid, _ := t.Header["kid"].(string)
		key := v.direction.keyByID(kid)
		if key == nil || key.Method().Alg() != t.Method.Alg() {
			keyFuncErr = ErrUnknownKey
			return nil, keyFuncErr
		}
		resolvedKey = key
		return key.Key(), nil
	}

	var claims accessTokenClaims
	parser := jwt.NewParser(
		jwt.WithIssuer(v.direction.Issuer),
		jwt.WithAudience(v.direction.Audience),
		jwt.WithLeeway(v.direction.Leeway),
		jwt.WithExpirationRequired(),
		// Share the Direction's clock with the issuer side: Direction.now()
		// defaults to time.Now().UTC() when Now is nil (production), and a
		// deterministic override otherwise (tests). Verification must judge
		// the time window against the SAME notion of "now" issuance used.
		jwt.WithTimeFunc(v.direction.now),
	)
	// Step 1 (parse header) happens inside ParseWithClaims before keyFunc
	// runs. Steps 2-3 run inside keyFunc above. Step 4 (verify signature)
	// runs immediately after keyFunc returns a key, before any claim is
	// trusted. Steps 5-6 (iss/aud, time window) run as part of the library's
	// registered-claim validation, which only executes once the signature
	// has already checked out.
	parsedToken, err := parser.ParseWithClaims(string(token), &claims, keyFunc)
	if err != nil {
		if keyFuncErr != nil {
			return contract4competios.VerifiedOperationGrant{}, fmt.Errorf("%w: %w", contract4competios.ErrInvalidGrant, keyFuncErr)
		}
		return contract4competios.VerifiedOperationGrant{}, classifyParseError(err)
	}
	if !parsedToken.Valid || resolvedKey == nil {
		return contract4competios.VerifiedOperationGrant{}, contract4competios.ErrInvalidGrant
	}

	// Step 7: decode claims into OperationGrant.
	grant, err := grantFromClaims(claims, resolvedKey.KeyID())
	if err != nil {
		return contract4competios.VerifiedOperationGrant{}, fmt.Errorf("%w: %w", contract4competios.ErrInvalidGrant, err)
	}

	// Step 8: run the contract's own field-shape and purpose/scope validation.
	if err := contract4competios.ValidateOperationGrant(grant); err != nil {
		return contract4competios.VerifiedOperationGrant{}, err
	}

	// Direction-level purpose enforcement: a verifier configured for one
	// direction refuses every other direction's purpose, even a
	// structurally valid, correctly signed, unexpired one.
	if !v.direction.permits(grant.Purpose) {
		return contract4competios.VerifiedOperationGrant{}, ErrPurposeNotPermitted
	}

	// Replay: distinct from expiry, checked only after every other gate has
	// already passed. contract4competiostest.CheckOperationGrantVerifier
	// asserts errors.Is(err, contract4competios.ErrTokenReplayConflict) here.
	if err := v.direction.Replay.Seen(ctx, grant.TokenID, grant.ExpiresAt); err != nil {
		return contract4competios.VerifiedOperationGrant{}, err
	}

	return contract4competios.VerifiedOperationGrant{Claims: grant}, nil
}

var _ contract4competios.OperationGrantVerifier = (*Verifier)(nil)

// classifyParseError maps a golang-jwt parse/validation failure onto this
// package's own vocabulary. Time-window failures keep an identifiable,
// distinct sentinel (ErrTokenExpired) from replay conflicts; every other
// parse failure (malformed token, bad signature, issuer/audience mismatch)
// collapses to the generic contract4competios.ErrInvalidGrant, still wrapping
// the underlying golang-jwt error for diagnostics.
func classifyParseError(err error) error {
	if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) || errors.Is(err, jwt.ErrTokenUsedBeforeIssued) {
		return fmt.Errorf("%w: %w: %v", contract4competios.ErrInvalidGrant, ErrTokenExpired, err)
	}
	return fmt.Errorf("%w: %v", contract4competios.ErrInvalidGrant, err)
}
