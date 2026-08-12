package grants

import (
	"context"
	"time"
)

// ReplayStore records consumed token IDs (jti) so a Verifier can refuse a
// second presentation of the same signed, unexpired token. It is the token
// layer's ONLY replay concern -- per Decision 0007, durable command ID and
// payload digest remain the product idempotency boundary; a jti protects
// only transport replay of the token itself. A retry of the same durable
// command legitimately carries a FRESH token (a fresh jti), so ReplayStore
// must never be consulted for anything but the exact token ID a Verifier is
// currently checking.
//
// By the time Seen is called, the caller (Verifier) has already confirmed
// the token's signature and time window, so expiresAt is always in the
// future (within configured leeway) -- retention only needs to exceed the
// token lifetime (TokenLifetime), never the life of the deployment.
type ReplayStore interface {
	// Seen records jti as consumed if it has not been recorded before,
	// returning nil. If jti was already recorded, it returns an error
	// wrapping contract4competios.ErrTokenReplayConflict -- distinct from an
	// expiry failure, which a Verifier never even reaches this call for.
	Seen(ctx context.Context, jti string, expiresAt time.Time) error
}
