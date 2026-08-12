package grants

import (
	"context"
	"errors"
	"testing"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

func TestIssuerRefusesPurposeOutsideDirection(t *testing.T) {
	key := testHMACKey(t, "kid")
	issuer, err := NewIssuer(chessDirection(t, key, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	request := requestForPurpose(contract4competios.GrantPurposeContestStarted)
	token, err := issuer.IssueOperationGrant(context.Background(), request)
	if !errors.Is(err, ErrPurposeNotPermitted) || token.AccessToken != "" {
		t.Fatalf("IssueOperationGrant(event purpose on chess direction) = (%+v, %v), want ErrPurposeNotPermitted and no token", token, err)
	}
}

func TestIssuerRefusesStructurallyInvalidRequest(t *testing.T) {
	key := testHMACKey(t, "kid")
	issuer, err := NewIssuer(chessDirection(t, key, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	request := requestForPurpose(contract4competios.GrantPurposeContestLaunch)
	request.CompetitionID = "" // now structurally invalid for this purpose
	token, err := issuer.IssueOperationGrant(context.Background(), request)
	if err == nil || token.AccessToken != "" {
		t.Fatalf("IssueOperationGrant(invalid request) = (%+v, %v), want an error and no token", token, err)
	}
}

func TestIssuerProducesFreshTokenIDPerCall(t *testing.T) {
	key := testHMACKey(t, "kid")
	direction := chessDirection(t, key, NewMemoryReplayStore(nil))
	issuer, err := NewIssuer(direction)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	verifier, err := NewVerifier(direction)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	request := requestForPurpose(contract4competios.GrantPurposeContestLaunch)
	ctx := context.Background()
	first, err := issuer.IssueOperationGrant(ctx, request)
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}
	second, err := issuer.IssueOperationGrant(ctx, request)
	if err != nil {
		t.Fatalf("second issue: %v", err)
	}
	if first.AccessToken == second.AccessToken {
		t.Fatalf("two issued tokens for the same request are byte-identical")
	}
	firstVerified, err := verifier.VerifyOperationGrant(ctx, first.AccessToken)
	if err != nil {
		t.Fatalf("verify first: %v", err)
	}
	secondVerified, err := verifier.VerifyOperationGrant(ctx, second.AccessToken)
	if err != nil {
		t.Fatalf("verify second: %v", err)
	}
	if firstVerified.Claims.TokenID == secondVerified.Claims.TokenID {
		t.Fatalf("two issued tokens share TokenID %q", firstVerified.Claims.TokenID)
	}
}

func TestIssuerStampsFiveMinuteLifetime(t *testing.T) {
	key := testHMACKey(t, "kid")
	issuer, err := NewIssuer(chessDirection(t, key, NewMemoryReplayStore(nil)))
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	issued, err := issuer.IssueOperationGrant(context.Background(), requestForPurpose(contract4competios.GrantPurposeContestLaunch))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if want := testFixtureTime.Add(TokenLifetime); !issued.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", issued.ExpiresAt, want)
	}
}
