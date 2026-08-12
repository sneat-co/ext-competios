package contract4competiostest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type OperationGrantVerifierFactory func(map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant) contract4competios.OperationGrantVerifier

func CheckOperationGrantVerifier(factory OperationGrantVerifierFactory) []error {
	ctx := context.Background()
	request := executionFixture()
	good := launchGrantFixture(request, "token-good", "key-a").Claims
	fresh := good
	fresh.TokenID = "token-fresh"
	fresh.KeyID = "key-rotated"
	fresh.IssuedAt, fresh.NotBefore, fresh.ExpiresAt = fixtureTime.Add(time.Minute), fixtureTime.Add(time.Minute), fixtureTime.Add(5*time.Minute)
	registry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{
		"opaque-good-a": good,
		"opaque-good-b": good,
		"opaque-fresh":  fresh,
	}
	verifier := factory(registry)
	verified, err := verifier.VerifyOperationGrant(ctx, "opaque-good-a")
	if err != nil || verified.Claims != good {
		return []error{fmt.Errorf("good opaque token: %v", err)}
	}
	var violations []error
	secondEncoding, err := verifier.VerifyOperationGrant(ctx, "opaque-good-b")
	if err != nil || secondEncoding.Claims != good {
		violations = append(violations, fmt.Errorf("second encoding for same exact jti: %v", err))
	}
	if _, err := verifier.VerifyOperationGrant(ctx, contract4competios.EncodedAccessToken(`{"issuer":"forged"}`)); err == nil {
		violations = append(violations, errors.New("self-asserted raw claims bypassed opaque-token verification"))
	}

	for _, scenario := range operationGrantAuthorityScenarios() {
		baseline := scenario.fixtureGrant.Claims
		for name, mutate := range allGrantClaimMutations() {
			claims := baseline
			localRegistry := map[contract4competios.EncodedAccessToken]contract4competios.OperationGrant{
				"replayed-a": claims,
				"replayed-b": claims,
			}
			localVerifier := factory(localRegistry)
			if _, verifyErr := localVerifier.VerifyOperationGrant(ctx, "replayed-a"); verifyErr != nil {
				violations = append(violations, fmt.Errorf("%s/%s replay setup: %v", scenario.name, name, verifyErr))
				continue
			}
			mutate(&claims)
			localRegistry["replayed-b"] = claims
			if _, verifyErr := localVerifier.VerifyOperationGrant(ctx, "replayed-b"); !errors.Is(verifyErr, contract4competios.ErrTokenReplayConflict) {
				violations = append(violations, fmt.Errorf("%s same token ID changed %s error = %v", scenario.name, name, verifyErr))
			}
		}
	}

	freshVerified, err := verifier.VerifyOperationGrant(ctx, "opaque-fresh")
	if err != nil || freshVerified.Claims.TokenID != fresh.TokenID || freshVerified.Claims.KeyID != fresh.KeyID || contract4competios.ValidateLaunchGrantForRequest(freshVerified, launchRouteFixture(request), request) != nil {
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

type OperationGrantAuthorityFactory func(contract4competios.OperationGrantRequest) (contract4competios.OperationGrantIssuer, contract4competios.OperationGrantVerifier)

type operationGrantAuthorityScenario struct {
	name         string
	fixtureGrant contract4competios.VerifiedOperationGrant
	operation    contract4competios.OperationGrantRequest
	validate     func(contract4competios.VerifiedOperationGrant) error
}

// CheckOperationGrantAuthority exercises the complete bilateral chain for
// every operation: exact issuer policy -> opaque token -> verifier -> narrow
// operation-specific binder. Caller-requested facts never define policy.
func CheckOperationGrantAuthority(factory OperationGrantAuthorityFactory) []error {
	ctx := context.Background()
	var violations []error
	mutations := operationGrantRequestMutations()
	requestType := reflect.TypeOf(contract4competios.OperationGrantRequest{})
	if len(mutations) != requestType.NumField() {
		violations = append(violations, fmt.Errorf("operation request mutation coverage = %d fields, want %d", len(mutations), requestType.NumField()))
	}
	for index := 0; index < requestType.NumField(); index++ {
		if _, covered := mutations[requestType.Field(index).Name]; !covered {
			violations = append(violations, fmt.Errorf("operation request field %s lacks an authority mutation", requestType.Field(index).Name))
		}
	}
	for _, scenario := range operationGrantAuthorityScenarios() {
		issuer, verifier := factory(scenario.operation)
		issued, err := issuer.IssueOperationGrant(ctx, scenario.operation)
		if err != nil {
			violations = append(violations, fmt.Errorf("%s issue valid operation token: %w", scenario.name, err))
			continue
		}
		if issued.AccessToken == "" || issued.TokenType != contract4competios.GrantTokenTypeAccessJWT || !issued.ExpiresAt.After(fixtureTime) || issued.ExpiresAt.After(fixtureTime.Add(5*time.Minute)) {
			violations = append(violations, fmt.Errorf("%s issued token metadata = %+v", scenario.name, issued))
		}
		verified, verifyErr := verifier.VerifyOperationGrant(ctx, issued.AccessToken)
		if verifyErr != nil || contract4competios.ValidateIssuedOperationGrantForRequest(verified.Claims, scenario.operation) != nil || scenario.validate(verified) != nil {
			violations = append(violations, fmt.Errorf("%s issued token verification/binding: %v", scenario.name, verifyErr))
			continue
		}

		fresh, issueErr := issuer.IssueOperationGrant(ctx, scenario.operation)
		freshVerified, freshVerifyErr := verifier.VerifyOperationGrant(ctx, fresh.AccessToken)
		if issueErr != nil || freshVerifyErr != nil || fresh.AccessToken == issued.AccessToken || freshVerified.Claims.TokenID == verified.Claims.TokenID || freshVerified.Claims.KeyID == verified.Claims.KeyID || contract4competios.ValidateIssuedOperationGrantForRequest(freshVerified.Claims, scenario.operation) != nil || scenario.validate(freshVerified) != nil {
			violations = append(violations, fmt.Errorf("%s fresh rotated-key operation token = %+v: issue=%v verify=%v", scenario.name, freshVerified.Claims, issueErr, freshVerifyErr))
		}

		for name, mutate := range mutations {
			broadened := scenario.operation
			mutate(&broadened)
			if token, mutationErr := issuer.IssueOperationGrant(ctx, broadened); mutationErr == nil || token.AccessToken != "" {
				violations = append(violations, fmt.Errorf("%s caller changed %s", scenario.name, name))
			}
		}
	}
	return violations
}

func operationGrantAuthorityScenarios() []operationGrantAuthorityScenario {
	execution := executionFixture()
	launch := launchGrantFixture(execution, "scenario-launch", "key-a")
	startedEvent := startFixture("instance")
	started := eventGrantFixture(startedEvent, "scenario-started", "key-a")
	terminalEvent := resultFixture("instance")
	terminal := eventGrantFixture(terminalEvent, "scenario-terminal", "key-a")
	manifestBytes := sourceManifestBytesFixture()
	manifestRequest := sourceManifestRequestFixture(manifestBytes)
	manifest, manifestRoute := sourceManifestGrantFixture(manifestRequest, manifestBytes, "scenario-manifest", "key-a")
	plan := sourcePlanFixture(manifestRequest)
	transfer := sourceCandidateTransferFixture()
	candidateRequest := sourceCandidateRequestFixture(plan, transfer, "candidate-command")
	candidate, candidateRoute := sourceCandidateGrantFixture(candidateRequest, transfer, "scenario-candidate", "key-a")
	retention := contract4competios.ArtifactRetentionReceipt{
		ReceiptID: "retention-version-a", ProviderID: plan.ProviderID, AdapterID: plan.AdapterID,
		CommandID: candidateRequest.CommandID, ParticipantID: plan.ParticipantID, ParticipantVersionID: plan.ParticipantVersionID,
		ClosurePlanID: plan.ClosurePlanID, ClosurePlanDigest: plan.ClosurePlanDigest,
		CandidateRequestDigest: candidateRequest.TypedPayloadDigest, ArtifactDigest: artifactDigest("9"),
		Status: contract4competios.ArtifactRetentionAccepted,
	}
	disclosureRequest := sourceDisclosureRequestFixture(plan, retention, transfer, "disclosure-command")
	disclosure, disclosureRoute := sourceDisclosureGrantFixture(disclosureRequest, transfer, "scenario-disclosure", "key-a")
	disclosureReceipt := contract4competios.ArtifactDisclosureVerificationReceipt{
		ReceiptID: "disclosure-disclosure-command", ProviderID: plan.ProviderID, AdapterID: plan.AdapterID,
		CommandID: disclosureRequest.CommandID, ParticipantID: plan.ParticipantID, ParticipantVersionID: plan.ParticipantVersionID,
		RetentionReceiptID: retention.ReceiptID, ArtifactDigest: retention.ArtifactDigest,
		VerificationRequestDigest: disclosureRequest.TypedPayloadDigest,
		Verdict:                   contract4competios.ArtifactDisclosureMatched, VerifiedAt: fixtureTime.Add(time.Minute),
	}
	publicationRequest := sourcePublicationRequestFixture(retention, disclosureRequest, disclosureReceipt, "publication-command")
	publication, publicationRoute := sourcePublicationGrantFixture(publicationRequest, "scenario-publication", "key-a")

	return []operationGrantAuthorityScenario{
		{name: "contest launch", fixtureGrant: launch, operation: launch.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateLaunchGrantForRequest(grant, launchRouteFixture(execution), execution)
		}},
		{name: "contest started", fixtureGrant: started, operation: started.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateEventGrantForEvent(grant, eventRouteFixture(startedEvent), startedEvent)
		}},
		{name: "contest terminal", fixtureGrant: terminal, operation: terminal.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateEventGrantForEvent(grant, eventRouteFixture(terminalEvent), terminalEvent)
		}},
		{name: "manifest plan", fixtureGrant: manifest, operation: manifest.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateManifestClosurePlanGrantForRequest(grant, manifestRoute, manifestRequest)
		}},
		{name: "candidate retain", fixtureGrant: candidate, operation: candidate.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateCandidateRetentionGrantForRequest(grant, candidateRoute, candidateRequest)
		}},
		{name: "disclosure match", fixtureGrant: disclosure, operation: disclosure.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateArtifactDisclosureGrantForRequest(grant, disclosureRoute, disclosureRequest)
		}},
		{name: "artifact publish", fixtureGrant: publication, operation: publication.Claims.RequestedOperation(), validate: func(grant contract4competios.VerifiedOperationGrant) error {
			return contract4competios.ValidateArtifactPublicationGrantForRequest(grant, publicationRoute, publicationRequest)
		}},
	}
}

func operationGrantRequestMutations() map[string]func(*contract4competios.OperationGrantRequest) {
	return map[string]func(*contract4competios.OperationGrantRequest){
		"Purpose":              func(v *contract4competios.OperationGrantRequest) { v.Purpose = "other" },
		"ProviderID":           func(v *contract4competios.OperationGrantRequest) { v.ProviderID = "other" },
		"AdapterID":            func(v *contract4competios.OperationGrantRequest) { v.AdapterID = "other" },
		"CompetitionID":        func(v *contract4competios.OperationGrantRequest) { v.CompetitionID = "other" },
		"ContestID":            func(v *contract4competios.OperationGrantRequest) { v.ContestID = "other" },
		"RequestID":            func(v *contract4competios.OperationGrantRequest) { v.RequestID = "other" },
		"ProviderInstanceID":   func(v *contract4competios.OperationGrantRequest) { v.ProviderInstanceID = "other" },
		"CommandID":            func(v *contract4competios.OperationGrantRequest) { v.CommandID = "other" },
		"TypedPayloadDigest":   func(v *contract4competios.OperationGrantRequest) { v.TypedPayloadDigest = payloadDigest("8") },
		"TransportContentType": func(v *contract4competios.OperationGrantRequest) { v.TransportContentType = "application/other" },
		"RawTransportDigest":   func(v *contract4competios.OperationGrantRequest) { v.RawTransportDigest = payloadDigest("7") },
		"Method":               func(v *contract4competios.OperationGrantRequest) { v.Method = "PUT" },
		"Resource":             func(v *contract4competios.OperationGrantRequest) { v.Resource = "/other" },
		"ParticipantID":        func(v *contract4competios.OperationGrantRequest) { v.ParticipantID = "other" },
		"ParticipantVersionID": func(v *contract4competios.OperationGrantRequest) { v.ParticipantVersionID = "other" },
		"RepositoryNodeID":     func(v *contract4competios.OperationGrantRequest) { v.RepositoryNodeID = "other" },
		"CommitOID": func(v *contract4competios.OperationGrantRequest) {
			v.CommitOID = "sha1:1123456789abcdef0123456789abcdef01234567"
		},
		"ManifestPath": func(v *contract4competios.OperationGrantRequest) { v.ManifestPath = "other/manifest.json" },
		"ManifestEntryKind": func(v *contract4competios.OperationGrantRequest) {
			v.ManifestEntryKind = contract4competios.SourceEntrySymlink
		},
		"RawManifestBytesDigest": func(v *contract4competios.OperationGrantRequest) { v.RawManifestBytesDigest = artifactDigest("6") },
		"ManifestByteLimit":      func(v *contract4competios.OperationGrantRequest) { v.ManifestByteLimit++ },
		"ClosurePlanID":          func(v *contract4competios.OperationGrantRequest) { v.ClosurePlanID = "other" },
		"ClosurePlanDigest":      func(v *contract4competios.OperationGrantRequest) { v.ClosurePlanDigest = payloadDigest("5") },
		"CandidateTransferredBytesDigest": func(v *contract4competios.OperationGrantRequest) {
			v.CandidateTransferredBytesDigest = artifactDigest("4")
		},
		"PublicCandidateTransferredBytesDigest": func(v *contract4competios.OperationGrantRequest) {
			v.PublicCandidateTransferredBytesDigest = artifactDigest("3")
		},
		"AggregateByteLimit":      func(v *contract4competios.OperationGrantRequest) { v.AggregateByteLimit++ },
		"RetentionReceiptID":      func(v *contract4competios.OperationGrantRequest) { v.RetentionReceiptID = "other" },
		"ArtifactDigest":          func(v *contract4competios.OperationGrantRequest) { v.ArtifactDigest = artifactDigest("2") },
		"DisclosureReceiptID":     func(v *contract4competios.OperationGrantRequest) { v.DisclosureReceiptID = "other" },
		"DisclosureRequestDigest": func(v *contract4competios.OperationGrantRequest) { v.DisclosureRequestDigest = payloadDigest("1") },
	}
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
		"instance":     func(v *contract4competios.OperationGrant) { v.ProviderInstanceID = "other" },
		"command":      func(v *contract4competios.OperationGrant) { v.CommandID = "other" },
		"typed digest": func(v *contract4competios.OperationGrant) { v.TypedPayloadDigest = payloadDigest("9") },
		"content type": func(v *contract4competios.OperationGrant) { v.TransportContentType = "application/json" },
		"raw digest":   func(v *contract4competios.OperationGrant) { v.RawTransportDigest = payloadDigest("8") },
		"method":       func(v *contract4competios.OperationGrant) { v.Method = "PUT" },
		"resource":     func(v *contract4competios.OperationGrant) { v.Resource = "/other" },
		"participant": func(v *contract4competios.OperationGrant) {
			v.ParticipantID = "other"
		},
		"participant version": func(v *contract4competios.OperationGrant) {
			v.ParticipantVersionID = "other"
		},
		"repository": func(v *contract4competios.OperationGrant) { v.RepositoryNodeID = "other" },
		"commit OID": func(v *contract4competios.OperationGrant) {
			v.CommitOID = "sha1:1123456789abcdef0123456789abcdef01234567"
		},
		"manifest path": func(v *contract4competios.OperationGrant) { v.ManifestPath = "other/manifest.json" },
		"manifest entry kind": func(v *contract4competios.OperationGrant) {
			v.ManifestEntryKind = contract4competios.SourceEntrySymlink
		},
		"raw manifest digest": func(v *contract4competios.OperationGrant) { v.RawManifestBytesDigest = artifactDigest("6") },
		"manifest byte limit": func(v *contract4competios.OperationGrant) { v.ManifestByteLimit++ },
		"closure plan ID":     func(v *contract4competios.OperationGrant) { v.ClosurePlanID = "other" },
		"closure plan digest": func(v *contract4competios.OperationGrant) { v.ClosurePlanDigest = payloadDigest("5") },
		"candidate digest": func(v *contract4competios.OperationGrant) {
			v.CandidateTransferredBytesDigest = artifactDigest("4")
		},
		"public candidate digest": func(v *contract4competios.OperationGrant) {
			v.PublicCandidateTransferredBytesDigest = artifactDigest("3")
		},
		"aggregate byte limit": func(v *contract4competios.OperationGrant) { v.AggregateByteLimit++ },
		"retention receipt":    func(v *contract4competios.OperationGrant) { v.RetentionReceiptID = "other" },
		"artifact digest":      func(v *contract4competios.OperationGrant) { v.ArtifactDigest = artifactDigest("2") },
		"disclosure receipt":   func(v *contract4competios.OperationGrant) { v.DisclosureReceiptID = "other" },
		"disclosure request": func(v *contract4competios.OperationGrant) {
			v.DisclosureRequestDigest = payloadDigest("1")
		},
	}
}
