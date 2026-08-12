package grants

import (
	"context"
	"fmt"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// checkPurposeSeparation is this package's own conformance checker for
// Decision 0007's purpose-separation requirement: a verifier configured for
// exactly `permitted` must accept a token for each of its own permitted
// purposes and refuse a token for every OTHER purpose -- even a token that
// is genuinely valid (correctly signed, unexpired, unreplayed) for its OWN
// issuing direction. It returns one error per violation, in the same
// "collect every failure" shape as contract4competiostest's own Check*
// helpers, so a caller can report everything wrong in one run rather than
// stopping at the first mismatch.
func checkPurposeSeparation(
	ctx context.Context,
	tokensByPurpose map[contract4competios.GrantPurpose]contract4competios.EncodedAccessToken,
	permitted []contract4competios.GrantPurpose,
	verifier contract4competios.OperationGrantVerifier,
) []error {
	permittedSet := make(map[contract4competios.GrantPurpose]bool, len(permitted))
	for _, p := range permitted {
		permittedSet[p] = true
	}
	var violations []error
	for purpose, token := range tokensByPurpose {
		_, err := verifier.VerifyOperationGrant(ctx, token)
		switch {
		case permittedSet[purpose] && err != nil:
			violations = append(violations, fmt.Errorf("permitted purpose %s: verify unexpectedly failed: %w", purpose, err))
		case !permittedSet[purpose] && err == nil:
			violations = append(violations, fmt.Errorf("non-permitted purpose %s: verify unexpectedly succeeded", purpose))
		}
	}
	return violations
}

// allPurposeTokens issues one genuine, correctly signed token per purpose --
// five from a chess-issued direction, two from a competios-issued direction,
// exactly Decision 0007's two-direction split -- and returns them keyed by
// purpose alongside the two keys used, so a caller can build verifiers scoped
// however a test needs.
func allPurposeTokens(t *testing.T) (tokens map[contract4competios.GrantPurpose]contract4competios.EncodedAccessToken, chessKey, eventKey KeyMaterial) {
	t.Helper()
	chessKey = testHMACKey(t, "conformance-chess-key")
	eventKey = testHMACKey(t, "conformance-event-key")
	chessIssuer, err := NewIssuer(chessDirection(t, chessKey, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewIssuer(chess): %v", err)
	}
	eventIssuer, err := NewIssuer(eventDirection(t, eventKey, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewIssuer(event): %v", err)
	}
	ctx := context.Background()
	tokens = make(map[contract4competios.GrantPurpose]contract4competios.EncodedAccessToken)
	for _, purpose := range chessPurposes() {
		issued, err := chessIssuer.IssueOperationGrant(ctx, requestForPurpose(purpose))
		if err != nil {
			t.Fatalf("issue(%s): %v", purpose, err)
		}
		tokens[purpose] = issued.AccessToken
	}
	for _, purpose := range eventPurposes() {
		issued, err := eventIssuer.IssueOperationGrant(ctx, requestForPurpose(purpose))
		if err != nil {
			t.Fatalf("issue(%s): %v", purpose, err)
		}
		tokens[purpose] = issued.AccessToken
	}
	return tokens, chessKey, eventKey
}

// TestPurposeSeparationConformance is the negative cross-purpose test
// Decision 0007 requires: a token issued for one purpose, presented to a
// verifier scoped to a DIFFERENT set of purposes, must fail -- for every
// purpose against every other direction's verifier, not just one example
// pair.
func TestPurposeSeparationConformance(t *testing.T) {
	tokens, chessKey, eventKey := allPurposeTokens(t)
	chessVerifier, err := NewVerifier(chessDirection(t, chessKey, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier(chess): %v", err)
	}
	eventVerifier, err := NewVerifier(eventDirection(t, eventKey, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier(event): %v", err)
	}
	ctx := context.Background()
	if violations := checkPurposeSeparation(ctx, tokens, chessPurposes(), chessVerifier); len(violations) != 0 {
		t.Fatalf("chess-direction verifier purpose separation violations: %v", violations)
	}
	if violations := checkPurposeSeparation(ctx, tokens, eventPurposes(), eventVerifier); len(violations) != 0 {
		t.Fatalf("event-direction verifier purpose separation violations: %v", violations)
	}
}

// brokenVerifier is a DELIBERATELY incomplete OperationGrantVerifier: it
// trusts both directions' keys but never enforces which purposes it was
// supposed to be scoped to -- the class of bug a real deployment could
// introduce by accidentally merging the chess and event trust sets instead
// of keeping two separate Verifier instances. A correct implementation must
// never behave like this; grants.Verifier does not.
type brokenVerifier struct {
	chess, event *Verifier
}

func (b brokenVerifier) VerifyOperationGrant(ctx context.Context, token contract4competios.EncodedAccessToken) (contract4competios.VerifiedOperationGrant, error) {
	if verified, err := b.chess.VerifyOperationGrant(ctx, token); err == nil {
		return verified, nil
	}
	return b.event.VerifyOperationGrant(ctx, token)
}

// TestPurposeSeparationConformanceCatchesABrokenVerifier proves
// checkPurposeSeparation is not vacuously green: run against brokenVerifier
// (which accepts every purpose from either direction), it must report
// violations. This is the "one deliberately-broken verifier implementation
// proving the conformance suite can fail" the brief requires.
func TestPurposeSeparationConformanceCatchesABrokenVerifier(t *testing.T) {
	tokens, chessKey, eventKey := allPurposeTokens(t)
	chessVerifier, err := NewVerifier(chessDirection(t, chessKey, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier(chess): %v", err)
	}
	eventVerifier, err := NewVerifier(eventDirection(t, eventKey, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier(event): %v", err)
	}
	broken := brokenVerifier{chess: chessVerifier, event: eventVerifier}

	// Scoped to only the chess purposes, brokenVerifier still happily
	// accepts every event-direction token too -- exactly the defect
	// checkPurposeSeparation exists to catch.
	violations := checkPurposeSeparation(context.Background(), tokens, chessPurposes(), broken)
	if len(violations) == 0 {
		t.Fatalf("expected checkPurposeSeparation to report violations against a deliberately-broken verifier, got none")
	}
}
