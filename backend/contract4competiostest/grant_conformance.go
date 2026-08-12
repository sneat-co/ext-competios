package contract4competiostest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type OperationGrantVerifierFactory func(map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant) contract4competios.OperationGrantVerifier

func CheckOperationGrantVerifier(factory OperationGrantVerifierFactory) []error {
	ctx := context.Background()
	request := executionFixture()
	good := launchGrantFixture(request, "token-good", "key-a").Claims
	registry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{"opaque-good": good}
	verifier := factory(registry)
	verified, err := verifier.VerifyOperationGrant(ctx, "opaque-good")
	if err != nil || verified.Claims != good {
		return []error{fmt.Errorf("good opaque token: %v", err)}
	}
	var violations []error
	if _, err := verifier.VerifyOperationGrant(ctx, contract4competios.EncodedAccessToken(`{"issuer":"forged"}`)); err == nil {
		violations = append(violations, errors.New("self-asserted raw claims bypassed opaque-token verification"))
	}

	for name, mutate := range allGrantClaimMutations() {
		claims := good
		localRegistry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{"replayed": claims}
		localVerifier := factory(localRegistry)
		if _, verifyErr := localVerifier.VerifyOperationGrant(ctx, "replayed"); verifyErr != nil {
			violations = append(violations, fmt.Errorf("%s replay setup: %v", name, verifyErr))
			continue
		}
		mutate(&claims)
		localRegistry["replayed"] = claims
		if _, verifyErr := localVerifier.VerifyOperationGrant(ctx, "replayed"); !errors.Is(verifyErr, contract4competios.ErrTokenReplayConflict) {
			violations = append(violations, fmt.Errorf("same token ID changed %s error = %v", name, verifyErr))
		}
	}

	fresh := good
	fresh.TokenID = "token-fresh"
	fresh.KeyID = "key-rotated"
	fresh.IssuedAt, fresh.NotBefore, fresh.ExpiresAt = fixtureTime.Add(time.Minute), fixtureTime.Add(time.Minute), fixtureTime.Add(4*time.Minute)
	freshRegistry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{"fresh": fresh}
	freshVerified, err := factory(freshRegistry).VerifyOperationGrant(ctx, "fresh")
	if err != nil || freshVerified.Claims.TokenID != fresh.TokenID || freshVerified.Claims.KeyID != fresh.KeyID {
		violations = append(violations, fmt.Errorf("fresh rotated-key token = %+v: %v", freshVerified.Claims, err))
	}

	for name, mutate := range map[string]func(*contract4competios.OperationGrant){
		"unknown key": func(v *contract4competios.OperationGrant) { v.KeyID = "unknown-key" },
		"expired":     func(v *contract4competios.OperationGrant) { v.ExpiresAt = fixtureTime },
		"not active": func(v *contract4competios.OperationGrant) {
			v.NotBefore = fixtureTime.Add(3 * time.Minute)
			v.ExpiresAt = fixtureTime.Add(4 * time.Minute)
		},
		"issuer":     func(v *contract4competios.OperationGrant) { v.Issuer = "other" },
		"audience":   func(v *contract4competios.OperationGrant) { v.Audience = "other" },
		"token type": func(v *contract4competios.OperationGrant) { v.TokenType = "other" },
	} {
		bad := good
		mutate(&bad)
		token := contract4competios.EncodedAccessToken("bad-" + name)
		if _, verifyErr := factory(map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{token: bad}).VerifyOperationGrant(ctx, token); verifyErr == nil {
			violations = append(violations, fmt.Errorf("%s token was trusted", name))
		}
	}
	return violations
}

type OperationGrantAuthorityFactory func() (contract4competios.OperationGrantIssuer, contract4competios.OperationGrantVerifier)

// CheckOperationGrantAuthority exercises an issuer configured to authorize
// only contest-launch operations for its authenticated caller. Structurally
// valid event/source issuance requests must still be refused as scope
// broadening; caller-supplied request facts never define issuer policy.
func CheckOperationGrantAuthority(factory OperationGrantAuthorityFactory) []error {
	ctx := context.Background()
	request := executionFixture()
	operation := launchGrantFixture(request, "unused", "unused").Claims.RequestedOperation()
	issuer, verifier := factory()
	issued, err := issuer.IssueOperationGrant(ctx, contract4competios.OperationGrantRequest{Purpose: operation.Purpose, ProviderID: operation.ProviderID, AdapterID: operation.AdapterID, CompetitionID: operation.CompetitionID, ContestID: operation.ContestID, RequestID: operation.RequestID, ProviderInstanceID: operation.ProviderInstanceID, CommandID: operation.CommandID, TypedPayloadDigest: operation.TypedPayloadDigest, TransportContentType: operation.TransportContentType, RawTransportDigest: operation.RawTransportDigest, Method: operation.Method, Resource: operation.Resource})
	if err != nil {
		return []error{fmt.Errorf("issue valid operation token: %w", err)}
	}
	var violations []error
	if issued.AccessToken == "" || issued.TokenType != contract4competios.GrantTokenTypeAccessJWT || !issued.ExpiresAt.After(fixtureTime) || issued.ExpiresAt.After(fixtureTime.Add(5*time.Minute)) {
		violations = append(violations, fmt.Errorf("issued token metadata = %+v", issued))
	}
	verified, err := verifier.VerifyOperationGrant(ctx, issued.AccessToken)
	if err != nil || contract4competios.ValidateIssuedOperationGrantForRequest(verified.Claims, operation) != nil {
		violations = append(violations, fmt.Errorf("issued token verification: %v", err))
	}

	broadened := eventGrantFixture(startFixture("instance"), "unused-event", "unused-key").Claims.RequestedOperation()
	if token, issueErr := issuer.IssueOperationGrant(ctx, broadened); issueErr == nil || token.AccessToken != "" {
		violations = append(violations, errors.New("caller broadened launch issuance to event scope"))
	}
	manifestBytes := sourceManifestBytesFixture()
	manifest := sourceManifestRequestFixture(manifestBytes)
	manifestGrant, _ := sourceManifestGrantFixture(manifest, manifestBytes, "unused-source", "unused-key")
	if token, issueErr := issuer.IssueOperationGrant(ctx, manifestGrant.Claims.RequestedOperation()); issueErr == nil || token.AccessToken != "" {
		violations = append(violations, errors.New("caller mixed source authority into launch issuance"))
	}
	return violations
}

func allGrantClaimMutations() map[string]func(*contract4competios.OperationGrant) {
	return map[string]func(*contract4competios.OperationGrant){
		"issuer":       func(v *contract4competios.OperationGrant) { v.Issuer = "other" },
		"subject":      func(v *contract4competios.OperationGrant) { v.Subject = "other" },
		"audience":     func(v *contract4competios.OperationGrant) { v.Audience = "other" },
		"token type":   func(v *contract4competios.OperationGrant) { v.TokenType = "other" },
		"scope":        func(v *contract4competios.OperationGrant) { v.Scope = "other" },
		"purpose":      func(v *contract4competios.OperationGrant) { v.Purpose = "other" },
		"key":          func(v *contract4competios.OperationGrant) { v.KeyID = "key-rotated" },
		"issued":       func(v *contract4competios.OperationGrant) { v.IssuedAt = v.IssuedAt.Add(time.Second) },
		"not before":   func(v *contract4competios.OperationGrant) { v.NotBefore = v.NotBefore.Add(time.Second) },
		"expiry":       func(v *contract4competios.OperationGrant) { v.ExpiresAt = v.ExpiresAt.Add(time.Second) },
		"provider":     func(v *contract4competios.OperationGrant) { v.ProviderID = "other" },
		"adapter":      func(v *contract4competios.OperationGrant) { v.AdapterID = "other" },
		"competition":  func(v *contract4competios.OperationGrant) { v.CompetitionID = "other" },
		"contest":      func(v *contract4competios.OperationGrant) { v.ContestID = "other" },
		"request":      func(v *contract4competios.OperationGrant) { v.RequestID = "other" },
		"command":      func(v *contract4competios.OperationGrant) { v.CommandID = "other" },
		"typed digest": func(v *contract4competios.OperationGrant) { v.TypedPayloadDigest = payloadDigest("9") },
		"content type": func(v *contract4competios.OperationGrant) { v.TransportContentType = "application/json" },
		"raw digest":   func(v *contract4competios.OperationGrant) { v.RawTransportDigest = payloadDigest("8") },
		"method":       func(v *contract4competios.OperationGrant) { v.Method = "PUT" },
		"resource":     func(v *contract4competios.OperationGrant) { v.Resource = "/other" },
	}
}
