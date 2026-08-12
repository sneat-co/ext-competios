package grants

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func issueAndVerify(t *testing.T, direction Direction, purpose contract4competios.GrantPurpose) (contract4competios.IssuedOperationAccessToken, contract4competios.VerifiedOperationGrant) {
	t.Helper()
	issuer, err := NewIssuer(direction)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	verifier, err := NewVerifier(direction)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	ctx := context.Background()
	issued, err := issuer.IssueOperationGrant(ctx, requestForPurpose(purpose))
	if err != nil {
		t.Fatalf("issue(%s): %v", purpose, err)
	}
	verified, err := verifier.VerifyOperationGrant(ctx, issued.AccessToken)
	if err != nil {
		t.Fatalf("verify(%s): %v", purpose, err)
	}
	return issued, verified
}

func TestRoundTripEveryPurpose(t *testing.T) {
	key := testHMACKey(t, "kid-a")
	for _, purpose := range allPurposes() {
		purpose := purpose
		t.Run(string(purpose), func(t *testing.T) {
			var direction Direction
			replay := NewMemoryReplayStore(nil)
			if containsPurpose(chessPurposes(), purpose) {
				direction = chessDirection(t, key, replay)
			} else {
				direction = eventDirection(t, key, replay)
			}
			request := requestForPurpose(purpose)
			_, verified := issueAndVerify(t, direction, purpose)
			if verified.Claims.Purpose != purpose {
				t.Fatalf("Claims.Purpose = %s, want %s", verified.Claims.Purpose, purpose)
			}
			if verified.Claims.Scope != contract4competios.GrantScopeForPurpose(purpose) {
				t.Fatalf("Claims.Scope = %s, want %s", verified.Claims.Scope, contract4competios.GrantScopeForPurpose(purpose))
			}
			if verified.Claims.KeyID != key.KeyID() {
				t.Fatalf("Claims.KeyID = %s, want %s", verified.Claims.KeyID, key.KeyID())
			}
			if contract4competios.ValidateOperationGrant(verified.Claims) != nil {
				t.Fatalf("ValidateOperationGrant rejected a freshly verified grant: %+v", verified.Claims)
			}
			if verified.Claims.RequestedOperation() != request {
				t.Fatalf("RequestedOperation() = %+v, want %+v", verified.Claims.RequestedOperation(), request)
			}
		})
	}
}

func containsPurpose(purposes []contract4competios.GrantPurpose, target contract4competios.GrantPurpose) bool {
	for _, p := range purposes {
		if p == target {
			return true
		}
	}
	return false
}

func TestVerifierRejectsSecondPresentationOfSameToken(t *testing.T) {
	key := testHMACKey(t, "kid-a")
	// The replay store's own clock must agree with the direction's clock --
	// otherwise MemoryReplayStore's expired-entry sweep (keyed off ITS
	// clock) evicts the just-recorded jti before the second call ever sees
	// it. testClock keeps both readings pinned to the same instant.
	direction := chessDirection(t, key, NewMemoryReplayStore(testClock))
	verifier, err := NewVerifier(direction)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	issuer, err := NewIssuer(direction)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	ctx := context.Background()
	issued, err := issuer.IssueOperationGrant(ctx, requestForPurpose(contract4competios.GrantPurposeContestLaunch))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.VerifyOperationGrant(ctx, issued.AccessToken); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	_, err = verifier.VerifyOperationGrant(ctx, issued.AccessToken)
	if !errors.Is(err, contract4competios.ErrTokenReplayConflict) {
		t.Fatalf("second verify err = %v, want ErrTokenReplayConflict", err)
	}
}

func TestVerifierDistinguishesReplayFromExpiry(t *testing.T) {
	key := testHMACKey(t, "kid-a")

	// clock is shared, mutable "now" for BOTH the issuer and the verifier
	// (Verifier.VerifyOperationGrant judges the time window against
	// Direction.now(), same as the issuer stamps iat/nbf/exp with it) --
	// tests advance it explicitly instead of sleeping.
	now := testFixtureTime
	clock := func() time.Time { return now }

	t.Run("expired token distinct from replay", func(t *testing.T) {
		now = testFixtureTime
		replay := NewMemoryReplayStore(clock)
		direction := chessDirection(t, key, replay)
		direction.Now = clock
		issuer, err := NewIssuer(direction)
		if err != nil {
			t.Fatalf("NewIssuer: %v", err)
		}
		verifier, err := NewVerifier(direction)
		if err != nil {
			t.Fatalf("NewVerifier: %v", err)
		}
		ctx := context.Background()
		issued, err := issuer.IssueOperationGrant(ctx, requestForPurpose(contract4competios.GrantPurposeContestLaunch))
		if err != nil {
			t.Fatalf("issue: %v", err)
		}

		now = testFixtureTime.Add(TokenLifetime + time.Minute) // past the 5-minute lifetime
		_, expiredErr := verifier.VerifyOperationGrant(ctx, issued.AccessToken)
		if !errors.Is(expiredErr, ErrTokenExpired) {
			t.Fatalf("expiredErr = %v, want ErrTokenExpired", expiredErr)
		}
		if errors.Is(expiredErr, contract4competios.ErrTokenReplayConflict) {
			t.Fatalf("expiry error must not also be classified as replay")
		}
	})

	t.Run("replay distinct from expiry", func(t *testing.T) {
		now = testFixtureTime
		replay := NewMemoryReplayStore(clock)
		direction := chessDirection(t, key, replay)
		direction.Now = clock
		issuer, err := NewIssuer(direction)
		if err != nil {
			t.Fatalf("NewIssuer: %v", err)
		}
		verifier, err := NewVerifier(direction)
		if err != nil {
			t.Fatalf("NewVerifier: %v", err)
		}
		ctx := context.Background()
		issued, err := issuer.IssueOperationGrant(ctx, requestForPurpose(contract4competios.GrantPurposeContestLaunch))
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		now = testFixtureTime.Add(time.Minute) // still well within the 5-minute window

		if _, err := verifier.VerifyOperationGrant(ctx, issued.AccessToken); err != nil {
			t.Fatalf("first verify: %v", err)
		}
		_, replayErr := verifier.VerifyOperationGrant(ctx, issued.AccessToken)
		if !errors.Is(replayErr, contract4competios.ErrTokenReplayConflict) {
			t.Fatalf("replay err = %v, want ErrTokenReplayConflict", replayErr)
		}
		if errors.Is(replayErr, ErrTokenExpired) {
			t.Fatalf("replay error must not also be classified as expiry")
		}
	})
}

func TestVerifierRejectsWrongTokenType(t *testing.T) {
	key := testHMACKey(t, "kid-a")
	direction := chessDirection(t, key, NewMemoryReplayStore(nil))
	grant := requestForPurpose(contract4competios.GrantPurposeContestLaunch)
	claims := grantForSigning(t, direction, key, grant, testFixtureTime)
	token := jwt.NewWithClaims(key.Method(), claims)
	token.Header["kid"] = key.KeyID()
	token.Header["typ"] = "JWT" // wrong: not "at+jwt"
	signed, err := token.SignedString(key.Key())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	verifier, err := NewVerifier(direction)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	_, err = verifier.VerifyOperationGrant(context.Background(), contract4competios.EncodedAccessToken(signed))
	if !errors.Is(err, ErrWrongTokenType) {
		t.Fatalf("err = %v, want ErrWrongTokenType", err)
	}
}

func TestVerifierRejectsUnknownKeyID(t *testing.T) {
	key := testHMACKey(t, "kid-a")
	other := testHMACKey(t, "kid-unrelated")
	direction := chessDirection(t, key, NewMemoryReplayStore(nil))
	issuer, err := NewIssuer(chessDirection(t, other, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	issued, err := issuer.IssueOperationGrant(context.Background(), requestForPurpose(contract4competios.GrantPurposeContestLaunch))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verifier, err := NewVerifier(direction) // trusts only "kid-a", not "kid-unrelated"
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	_, err = verifier.VerifyOperationGrant(context.Background(), issued.AccessToken)
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("err = %v, want ErrUnknownKey", err)
	}
}

func TestVerifierAcceptsRotatedTrustedKey(t *testing.T) {
	active := testHMACKey(t, "kid-active")
	rotatedOut := testHMACKey(t, "kid-old")
	issuerDirection := chessDirection(t, rotatedOut, NewMemoryReplayStore(nil))
	issuer, err := NewIssuer(issuerDirection)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	issued, err := issuer.IssueOperationGrant(context.Background(), requestForPurpose(contract4competios.GrantPurposeContestLaunch))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verifierDirection := chessDirection(t, active, NewMemoryReplayStore(nil))
	verifierDirection.Trusted = []KeyMaterial{rotatedOut} // rotation overlap: old key still trusted
	verifier, err := NewVerifier(verifierDirection)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	verified, err := verifier.VerifyOperationGrant(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatalf("verify with rotated-out key: %v", err)
	}
	if verified.Claims.KeyID != "kid-old" {
		t.Fatalf("Claims.KeyID = %s, want kid-old", verified.Claims.KeyID)
	}
}

func TestVerifierRejectsIssuerMismatch(t *testing.T) {
	key := testHMACKey(t, "kid-a")
	issuerDirection := chessDirection(t, key, NewMemoryReplayStore(nil))
	issuerDirection.Issuer = "https://different-issuer.example"
	issuer, err := NewIssuer(issuerDirection)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	issued, err := issuer.IssueOperationGrant(context.Background(), requestForPurpose(contract4competios.GrantPurposeContestLaunch))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verifier, err := NewVerifier(chessDirection(t, key, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if _, err := verifier.VerifyOperationGrant(context.Background(), issued.AccessToken); err == nil {
		t.Fatalf("expected an issuer-mismatch error, got nil")
	}
}

func TestVerifierRejectsTamperedSignature(t *testing.T) {
	key := testHMACKey(t, "kid-a")
	direction := chessDirection(t, key, NewMemoryReplayStore(nil))
	issuer, err := NewIssuer(direction)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	issued, err := issuer.IssueOperationGrant(context.Background(), requestForPurpose(contract4competios.GrantPurposeContestLaunch))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	tampered := string(issued.AccessToken)
	// Flip the last character of the signature segment.
	last := tampered[len(tampered)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	tampered = tampered[:len(tampered)-1] + string(replacement)
	verifier, err := NewVerifier(direction)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if _, err := verifier.VerifyOperationGrant(context.Background(), contract4competios.EncodedAccessToken(tampered)); err == nil {
		t.Fatalf("expected a signature verification error, got nil")
	}
}

func TestVerifierRejectsEmptyToken(t *testing.T) {
	verifier, err := NewVerifier(chessDirection(t, testHMACKey(t, "kid-a"), NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if _, err := verifier.VerifyOperationGrant(context.Background(), ""); !errors.Is(err, contract4competios.ErrInvalidGrant) {
		t.Fatalf("err = %v, want ErrInvalidGrant", err)
	}
}

func TestNilVerifierIsSafe(t *testing.T) {
	var verifier *Verifier
	if _, err := verifier.VerifyOperationGrant(context.Background(), "anything"); !errors.Is(err, contract4competios.ErrInvalidGrant) {
		t.Fatalf("err = %v, want ErrInvalidGrant", err)
	}
}

func TestNilIssuerIsSafe(t *testing.T) {
	var issuer *Issuer
	if _, err := issuer.IssueOperationGrant(context.Background(), contract4competios.OperationGrantRequest{}); !errors.Is(err, contract4competios.ErrInvalidGrant) {
		t.Fatalf("err = %v, want ErrInvalidGrant", err)
	}
}

// TestVerifierEnforcesDirectionPurposesEvenForValidSignature proves that a
// structurally valid, correctly signed, unexpired token is STILL refused
// when its purpose falls outside the verifying Direction's permitted set --
// the direction-level purpose gate that keeps a chess-issued verifier from
// ever accepting an event-purpose token even if (hypothetically) it shared a
// key with the event direction.
func TestVerifierEnforcesDirectionPurposesEvenForValidSignature(t *testing.T) {
	key := testHMACKey(t, "shared-kid")
	overPermissiveIssuer, err := NewIssuer(Direction{
		Issuer: testChessIssuer, Subject: testChessSubject, Audience: testChessAudience,
		Purposes: []contract4competios.GrantPurpose{contract4competios.GrantPurposeContestStarted},
		Key:      key, Now: testClock,
	})
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	issued, err := overPermissiveIssuer.IssueOperationGrant(context.Background(), requestForPurpose(contract4competios.GrantPurposeContestStarted))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	chessVerifier, err := NewVerifier(chessDirection(t, key, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	_, err = chessVerifier.VerifyOperationGrant(context.Background(), issued.AccessToken)
	if !errors.Is(err, ErrPurposeNotPermitted) {
		t.Fatalf("err = %v, want ErrPurposeNotPermitted", err)
	}
}

// grantForSigning builds a claims struct exactly like an Issuer would, for
// tests that need to hand-craft a token's JOSE header (e.g. a wrong typ)
// that the public Issuer API deliberately never allows a caller to set.
func grantForSigning(t *testing.T, direction Direction, key KeyMaterial, request contract4competios.OperationGrantRequest, now time.Time) accessTokenClaims {
	t.Helper()
	grant := contract4competios.OperationGrant{
		Issuer: direction.Issuer, Subject: direction.Subject, Audience: direction.Audience,
		TokenType: contract4competios.GrantTokenTypeAccessJWT,
		Scope:     contract4competios.GrantScopeForPurpose(request.Purpose), Purpose: request.Purpose,
		KeyID: key.KeyID(), TokenID: "hand-crafted-" + strings.ToLower(string(request.Purpose)),
		IssuedAt: now, NotBefore: now, ExpiresAt: now.Add(TokenLifetime),
		ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		CompetitionID: request.CompetitionID, ContestID: request.ContestID, RequestID: request.RequestID,
		ProviderInstanceID: request.ProviderInstanceID, CommandID: request.CommandID,
		TypedPayloadDigest: request.TypedPayloadDigest, TransportContentType: request.TransportContentType,
		RawTransportDigest: request.RawTransportDigest, Method: request.Method, Resource: request.Resource,
		ParticipantID: request.ParticipantID, ParticipantVersionID: request.ParticipantVersionID,
		RepositoryNodeID: request.RepositoryNodeID, CommitOID: request.CommitOID, ManifestPath: request.ManifestPath,
		ManifestEntryKind:      request.ManifestEntryKind,
		RawManifestBytesDigest: request.RawManifestBytesDigest, ManifestByteLimit: request.ManifestByteLimit,
		ClosurePlanID: request.ClosurePlanID, ClosurePlanDigest: request.ClosurePlanDigest,
		CandidateTransferredBytesDigest:       request.CandidateTransferredBytesDigest,
		PublicCandidateTransferredBytesDigest: request.PublicCandidateTransferredBytesDigest,
		AggregateByteLimit:                    request.AggregateByteLimit, RetentionReceiptID: request.RetentionReceiptID,
		ArtifactDigest: request.ArtifactDigest, DisclosureReceiptID: request.DisclosureReceiptID,
		DisclosureRequestDigest: request.DisclosureRequestDigest,
	}
	return claimsFromGrant(grant)
}
