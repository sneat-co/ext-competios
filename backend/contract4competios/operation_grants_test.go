package contract4competios

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func launchGrantFixture(t *testing.T) (ExecutionRequest, OperationGrant, OperationRouteBinding) {
	t.Helper()
	request := mustExecutionRequest(t, providerRequestPayloadFixture())
	rawDigest := DigestRawTransportBody("application/json", []byte(`{"launch":true}`))
	grant := OperationGrant{
		Issuer:               "https://game.example",
		Subject:              "competios-service",
		Audience:             "game-provider",
		TokenType:            GrantTokenTypeAccessJWT,
		Scope:                GrantScopeContestLaunch,
		Purpose:              GrantPurposeContestLaunch,
		KeyID:                "key-1",
		TokenID:              "token-1",
		IssuedAt:             contractTestTime,
		NotBefore:            contractTestTime,
		ExpiresAt:            contractTestTime.Add(time.Minute),
		ProviderID:           request.ProviderID,
		AdapterID:            request.AdapterID,
		CompetitionID:        request.CompetitionID,
		ContestID:            request.ContestID,
		RequestID:            request.ID,
		CommandID:            request.CommandID,
		TypedPayloadDigest:   request.TypedPayloadDigest,
		TransportContentType: "application/json",
		RawTransportDigest:   rawDigest,
		Method:               "POST",
		Resource:             "/providers/provider-1/executions",
	}
	return request, grant, routeBindingForGrant(grant)
}

func routeBindingForGrant(grant OperationGrant) OperationRouteBinding {
	return OperationRouteBinding{
		Issuer: grant.Issuer, Subject: grant.Subject, Audience: grant.Audience,
		TokenType: grant.TokenType, Scope: grant.Scope, Purpose: grant.Purpose,
		ProviderID: grant.ProviderID, AdapterID: grant.AdapterID,
		TransportContentType: grant.TransportContentType, RawTransportDigest: grant.RawTransportDigest,
		Method: grant.Method, Resource: grant.Resource,
	}
}

func TestLaunchGrantBindsTypedAndRawOperation(t *testing.T) {
	request, grant, route := launchGrantFixture(t)
	if err := ValidateLaunchGrantForRequest(VerifiedOperationGrant{Claims: grant}, route, request); err != nil {
		t.Fatalf("valid launch grant rejected: %v", err)
	}

	wrongRaw := route
	wrongRaw.RawTransportDigest = DigestRawTransportBody(route.TransportContentType, []byte("different"))
	if err := ValidateLaunchGrantForRequest(VerifiedOperationGrant{Claims: grant}, wrongRaw, request); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("wrong observed raw body error = %v, want ErrInvalidGrant", err)
	}

	otherPayload := providerRequestPayloadFixture()
	otherPayload.ProviderID = "other-provider"
	otherRequest := mustExecutionRequest(t, otherPayload)
	grant.TypedPayloadDigest = otherRequest.TypedPayloadDigest
	if err := ValidateLaunchGrantForRequest(VerifiedOperationGrant{Claims: grant}, route, otherRequest); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("request/provider mismatch error = %v, want ErrInvalidGrant", err)
	}
}

func TestStableRouteAllowsFreshTokenAndRotatedVerifiedKey(t *testing.T) {
	request, grant, route := launchGrantFixture(t)
	fresh := grant
	fresh.TokenID = "token-2"
	fresh.KeyID = "rotated-key"
	fresh.IssuedAt = fresh.IssuedAt.Add(time.Second)
	fresh.NotBefore = fresh.NotBefore.Add(time.Second)
	fresh.ExpiresAt = fresh.ExpiresAt.Add(time.Second)
	if err := ValidateLaunchGrantForRequest(VerifiedOperationGrant{Claims: fresh}, route, request); err != nil {
		t.Fatalf("fresh token signed by an already-verified rotated key rejected: %v", err)
	}
	if err := ValidateExactOperationGrant(fresh, grant); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("exact token replay identity change error = %v, want ErrInvalidGrant", err)
	}
}

func TestExecutionGrantsRejectEverySourceOnlyField(t *testing.T) {
	_, grant, _ := launchGrantFixture(t)
	tests := map[string]func(*OperationGrant){
		"participant":                 func(v *OperationGrant) { v.ParticipantID = "participant" },
		"participant version":         func(v *OperationGrant) { v.ParticipantVersionID = "version" },
		"repository":                  func(v *OperationGrant) { v.RepositoryNodeID = "repository" },
		"commit":                      func(v *OperationGrant) { v.Commit = "full-commit" },
		"manifest path":               func(v *OperationGrant) { v.ManifestPath = "bot.json" },
		"manifest entry kind":         func(v *OperationGrant) { v.ManifestEntryKind = SourceEntryRegular },
		"raw manifest bytes digest":   func(v *OperationGrant) { v.RawManifestBytesDigest = testArtifactDigest("1") },
		"manifest byte limit":         func(v *OperationGrant) { v.ManifestByteLimit = 1024 },
		"closure plan ID":             func(v *OperationGrant) { v.ClosurePlanID = "plan" },
		"closure plan digest":         func(v *OperationGrant) { v.ClosurePlanDigest = testPayloadDigest("2") },
		"candidate transferred bytes": func(v *OperationGrant) { v.CandidateTransferredBytesDigest = testArtifactDigest("3") },
		"public candidate bytes":      func(v *OperationGrant) { v.PublicCandidateTransferredBytesDigest = testArtifactDigest("8") },
		"aggregate byte limit":        func(v *OperationGrant) { v.AggregateByteLimit = 2048 },
		"retention receipt":           func(v *OperationGrant) { v.RetentionReceiptID = "retention" },
		"artifact digest":             func(v *OperationGrant) { v.ArtifactDigest = testArtifactDigest("4") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changed := grant
			mutate(&changed)
			if err := ValidateOperationGrant(changed); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("ValidateOperationGrant() error = %v, want ErrInvalidGrant", err)
			}
		})
	}
}

func manifestRequestFixture(t *testing.T) ManifestClosurePlanRequest {
	t.Helper()
	request, err := NewManifestClosurePlanRequest(ManifestClosurePlanRequestPayload{
		ProviderID: "provider-1", AdapterID: "adapter-1", CommandID: "manifest-command",
		ParticipantID: "participant-a", ParticipantVersionID: "version-a",
		RepositoryNodeID: "repository-node-1", Commit: "0123456789abcdef0123456789abcdef01234567",
		ManifestPath: "bots/manifest.json", ManifestEntryKind: SourceEntryRegular,
		RawManifestBytesDigest: DigestRawManifestBytes([]byte(`{"entry":"bot.star"}`)),
		ManifestByteLimit:      32768,
	})
	if err != nil {
		t.Fatalf("NewManifestClosurePlanRequest() error = %v", err)
	}
	return request
}

func manifestGrantFixture(t *testing.T) (ManifestClosurePlanRequest, OperationGrant, OperationRouteBinding) {
	t.Helper()
	request := manifestRequestFixture(t)
	_, grant, _ := launchGrantFixture(t)
	grant.Scope, grant.Purpose = GrantScopeManifestClosurePlan, GrantPurposeManifestClosurePlan
	grant.CompetitionID, grant.ContestID, grant.RequestID, grant.ProviderInstanceID = "", "", "", ""
	grant.CommandID, grant.TypedPayloadDigest = request.CommandID, request.TypedPayloadDigest
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RepositoryNodeID, grant.Commit = request.RepositoryNodeID, request.Commit
	grant.ManifestPath, grant.ManifestEntryKind = request.ManifestPath, request.ManifestEntryKind
	grant.RawManifestBytesDigest, grant.ManifestByteLimit = request.RawManifestBytesDigest, request.ManifestByteLimit
	grant.Resource = "/providers/provider-1/closure-plans"
	return request, grant, routeBindingForGrant(grant)
}

func candidateRequestFixture(t *testing.T) CandidateClosureRetentionRequest {
	t.Helper()
	plan := closurePlanFixture(t)
	transfer := candidateTransferFixture()
	transferDigest, err := DigestCandidateTransferredFiles(transfer.Files)
	if err != nil {
		t.Fatalf("DigestCandidateTransferredFiles() error = %v", err)
	}
	request, err := NewCandidateClosureRetentionRequest(CandidateClosureRetentionRequestPayload{
		ProviderID: "provider-1", AdapterID: "adapter-1", CommandID: "candidate-command",
		ParticipantID: "participant-a", ParticipantVersionID: "version-a",
		RepositoryNodeID: "repository-node-1", Commit: "0123456789abcdef0123456789abcdef01234567",
		ClosurePlanID: plan.ClosurePlanID, ClosurePlanDigest: plan.ClosurePlanDigest,
		CandidateTransferredBytesDigest: transferDigest,
		AggregateByteLimit:              plan.AggregateByteLimit,
	})
	if err != nil {
		t.Fatalf("NewCandidateClosureRetentionRequest() error = %v", err)
	}
	return request
}

func closurePlanFixture(t *testing.T) ClosurePlan {
	t.Helper()
	manifest := manifestRequestFixture(t)
	plan, err := NewClosurePlan(ClosurePlanPayload{
		ClosurePlanID: "plan-1", ProviderID: manifest.ProviderID, AdapterID: manifest.AdapterID,
		ParticipantID: manifest.ParticipantID, ParticipantVersionID: manifest.ParticipantVersionID,
		RepositoryNodeID: manifest.RepositoryNodeID, Commit: manifest.Commit,
		ManifestPath: manifest.ManifestPath, ManifestEntryKind: manifest.ManifestEntryKind,
		ManifestRequestDigest:  manifest.TypedPayloadDigest,
		RawManifestBytesDigest: manifest.RawManifestBytesDigest,
		Files: []PlannedSourceFile{
			{CanonicalPath: "bots/bot.star", EntryKind: SourceEntryRegular, ByteLimit: 65536},
			{CanonicalPath: "bots/opening.json", EntryKind: SourceEntryRegular, ByteLimit: 32768},
		},
		AggregateByteLimit: 98304,
	})
	if err != nil {
		t.Fatalf("NewClosurePlan() error = %v", err)
	}
	return plan
}

func candidateTransferFixture() CandidateClosureTransfer {
	return CandidateClosureTransfer{Files: []CandidateSourceFile{
		{CanonicalPath: "bots/bot.star", EntryKind: SourceEntryRegular, Bytes: []byte("function move() { return 1 }")},
		{CanonicalPath: "bots/opening.json", EntryKind: SourceEntryRegular, Bytes: []byte(`{"opening":"center"}`)},
	}}
}

func candidateGrantFixture(t *testing.T) (CandidateClosureRetentionRequest, OperationGrant, OperationRouteBinding) {
	t.Helper()
	request := candidateRequestFixture(t)
	_, grant, _ := launchGrantFixture(t)
	grant.Scope, grant.Purpose = GrantScopeCandidateValidateRetain, GrantPurposeCandidateValidateRetain
	grant.CompetitionID, grant.ContestID, grant.RequestID, grant.ProviderInstanceID = "", "", "", ""
	grant.CommandID, grant.TypedPayloadDigest = request.CommandID, request.TypedPayloadDigest
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RepositoryNodeID, grant.Commit = request.RepositoryNodeID, request.Commit
	grant.ClosurePlanID, grant.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	grant.CandidateTransferredBytesDigest, grant.AggregateByteLimit = request.CandidateTransferredBytesDigest, request.AggregateByteLimit
	grant.Resource = "/providers/provider-1/candidate-closures"
	return request, grant, routeBindingForGrant(grant)
}

func publicationRequestFixture(t *testing.T) ArtifactPublicationRequest {
	t.Helper()
	request, err := NewArtifactPublicationRequest(ArtifactPublicationRequestPayload{
		ProviderID: "provider-1", AdapterID: "adapter-1", CommandID: "publish-command",
		ParticipantID: "participant-a", ParticipantVersionID: "version-a",
		RetentionReceiptID: "retention-1", ArtifactDigest: testArtifactDigest("9"),
	})
	if err != nil {
		t.Fatalf("NewArtifactPublicationRequest() error = %v", err)
	}
	return request
}

func publicationGrantFixture(t *testing.T) (ArtifactPublicationRequest, OperationGrant, OperationRouteBinding) {
	t.Helper()
	request := publicationRequestFixture(t)
	_, grant, _ := launchGrantFixture(t)
	grant.Scope, grant.Purpose = GrantScopeArtifactPublish, GrantPurposeArtifactPublish
	grant.CompetitionID, grant.ContestID, grant.RequestID, grant.ProviderInstanceID = "", "", "", ""
	grant.CommandID, grant.TypedPayloadDigest = request.CommandID, request.TypedPayloadDigest
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RetentionReceiptID, grant.ArtifactDigest = request.RetentionReceiptID, request.ArtifactDigest
	grant.Resource = "/providers/provider-1/artifacts/publish"
	return request, grant, routeBindingForGrant(grant)
}

func TestStagedSourceGrantBindingsNeverCross(t *testing.T) {
	manifestRequest, manifestGrant, manifestRoute := manifestGrantFixture(t)
	candidateRequest, candidateGrant, candidateRoute := candidateGrantFixture(t)
	publicationRequest, publicationGrant, publicationRoute := publicationGrantFixture(t)

	if err := ValidateManifestClosurePlanGrantForRequest(VerifiedOperationGrant{Claims: manifestGrant}, manifestRoute, manifestRequest); err != nil {
		t.Fatalf("manifest plan grant rejected: %v", err)
	}
	if err := ValidateCandidateRetentionGrantForRequest(VerifiedOperationGrant{Claims: candidateGrant}, candidateRoute, candidateRequest); err != nil {
		t.Fatalf("candidate retention grant rejected: %v", err)
	}
	if err := ValidateArtifactPublicationGrantForRequest(VerifiedOperationGrant{Claims: publicationGrant}, publicationRoute, publicationRequest); err != nil {
		t.Fatalf("publication grant rejected: %v", err)
	}

	if err := ValidateCandidateRetentionGrantForRequest(VerifiedOperationGrant{Claims: manifestGrant}, manifestRoute, candidateRequest); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("manifest grant crossed to candidate operation: %v", err)
	}
	if err := ValidateArtifactPublicationGrantForRequest(VerifiedOperationGrant{Claims: candidateGrant}, candidateRoute, publicationRequest); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("candidate grant crossed to publication: %v", err)
	}
	if err := ValidateManifestClosurePlanGrantForRequest(VerifiedOperationGrant{Claims: publicationGrant}, publicationRoute, manifestRequest); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("publication grant crossed to manifest operation: %v", err)
	}
}

func TestEveryStagedSourceGrantFactIsExactlyBound(t *testing.T) {
	manifestRequest, manifestGrant, manifestRoute := manifestGrantFixture(t)
	for name, mutate := range map[string]func(*OperationGrant){
		"provider":            func(value *OperationGrant) { value.ProviderID = "other-provider" },
		"adapter":             func(value *OperationGrant) { value.AdapterID = "other-adapter" },
		"command":             func(value *OperationGrant) { value.CommandID = "other-command" },
		"typed digest":        func(value *OperationGrant) { value.TypedPayloadDigest = testPayloadDigest("8") },
		"participant":         func(value *OperationGrant) { value.ParticipantID = "other-participant" },
		"participant version": func(value *OperationGrant) { value.ParticipantVersionID = "other-version" },
		"repository":          func(value *OperationGrant) { value.RepositoryNodeID = "other-repository" },
		"commit":              func(value *OperationGrant) { value.Commit = "other-commit" },
		"manifest path":       func(value *OperationGrant) { value.ManifestPath = "other/manifest.json" },
		"manifest entry kind": func(value *OperationGrant) { value.ManifestEntryKind = SourceEntrySymlink },
		"manifest digest":     func(value *OperationGrant) { value.RawManifestBytesDigest = testArtifactDigest("8") },
		"manifest limit":      func(value *OperationGrant) { value.ManifestByteLimit++ },
	} {
		t.Run("manifest/"+name, func(t *testing.T) {
			changed := manifestGrant
			mutate(&changed)
			if err := ValidateManifestClosurePlanGrantForRequest(VerifiedOperationGrant{Claims: changed}, manifestRoute, manifestRequest); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("error = %v, want ErrInvalidGrant", err)
			}
		})
	}

	candidateRequest, candidateGrant, candidateRoute := candidateGrantFixture(t)
	for name, mutate := range map[string]func(*OperationGrant){
		"provider":            func(value *OperationGrant) { value.ProviderID = "other-provider" },
		"adapter":             func(value *OperationGrant) { value.AdapterID = "other-adapter" },
		"command":             func(value *OperationGrant) { value.CommandID = "other-command" },
		"typed digest":        func(value *OperationGrant) { value.TypedPayloadDigest = testPayloadDigest("8") },
		"participant":         func(value *OperationGrant) { value.ParticipantID = "other-participant" },
		"participant version": func(value *OperationGrant) { value.ParticipantVersionID = "other-version" },
		"repository":          func(value *OperationGrant) { value.RepositoryNodeID = "other-repository" },
		"commit":              func(value *OperationGrant) { value.Commit = "other-commit" },
		"plan ID":             func(value *OperationGrant) { value.ClosurePlanID = "other-plan" },
		"plan digest":         func(value *OperationGrant) { value.ClosurePlanDigest = testPayloadDigest("8") },
		"candidate digest":    func(value *OperationGrant) { value.CandidateTransferredBytesDigest = testArtifactDigest("8") },
		"aggregate limit":     func(value *OperationGrant) { value.AggregateByteLimit++ },
	} {
		t.Run("candidate/"+name, func(t *testing.T) {
			changed := candidateGrant
			mutate(&changed)
			if err := ValidateCandidateRetentionGrantForRequest(VerifiedOperationGrant{Claims: changed}, candidateRoute, candidateRequest); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("error = %v, want ErrInvalidGrant", err)
			}
		})
	}

	publicationRequest, publicationGrant, publicationRoute := publicationGrantFixture(t)
	for name, mutate := range map[string]func(*OperationGrant){
		"provider":            func(value *OperationGrant) { value.ProviderID = "other-provider" },
		"adapter":             func(value *OperationGrant) { value.AdapterID = "other-adapter" },
		"command":             func(value *OperationGrant) { value.CommandID = "other-command" },
		"typed digest":        func(value *OperationGrant) { value.TypedPayloadDigest = testPayloadDigest("8") },
		"participant":         func(value *OperationGrant) { value.ParticipantID = "other-participant" },
		"participant version": func(value *OperationGrant) { value.ParticipantVersionID = "other-version" },
		"retention receipt":   func(value *OperationGrant) { value.RetentionReceiptID = "other-retention" },
		"artifact digest":     func(value *OperationGrant) { value.ArtifactDigest = testArtifactDigest("8") },
	} {
		t.Run("publication/"+name, func(t *testing.T) {
			changed := publicationGrant
			mutate(&changed)
			if err := ValidateArtifactPublicationGrantForRequest(VerifiedOperationGrant{Claims: changed}, publicationRoute, publicationRequest); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("error = %v, want ErrInvalidGrant", err)
			}
		})
	}

	disclosureRequest, disclosureGrant, disclosureRoute := disclosureGrantFixture(t)
	for name, mutate := range map[string]func(*OperationGrant){
		"provider":                func(value *OperationGrant) { value.ProviderID = "other-provider" },
		"adapter":                 func(value *OperationGrant) { value.AdapterID = "other-adapter" },
		"command":                 func(value *OperationGrant) { value.CommandID = "other-command" },
		"typed digest":            func(value *OperationGrant) { value.TypedPayloadDigest = testPayloadDigest("8") },
		"participant":             func(value *OperationGrant) { value.ParticipantID = "other-participant" },
		"participant version":     func(value *OperationGrant) { value.ParticipantVersionID = "other-version" },
		"repository":              func(value *OperationGrant) { value.RepositoryNodeID = "other-repository" },
		"commit":                  func(value *OperationGrant) { value.Commit = "other-commit" },
		"plan ID":                 func(value *OperationGrant) { value.ClosurePlanID = "other-plan" },
		"plan digest":             func(value *OperationGrant) { value.ClosurePlanDigest = testPayloadDigest("8") },
		"public candidate digest": func(value *OperationGrant) { value.PublicCandidateTransferredBytesDigest = testArtifactDigest("8") },
		"aggregate limit":         func(value *OperationGrant) { value.AggregateByteLimit++ },
		"retention receipt":       func(value *OperationGrant) { value.RetentionReceiptID = "other-retention" },
		"artifact digest":         func(value *OperationGrant) { value.ArtifactDigest = testArtifactDigest("8") },
	} {
		t.Run("disclosure/"+name, func(t *testing.T) {
			changed := disclosureGrant
			mutate(&changed)
			if err := ValidateArtifactDisclosureGrantForRequest(VerifiedOperationGrant{Claims: changed}, disclosureRoute, disclosureRequest); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("error = %v, want ErrInvalidGrant", err)
			}
		})
	}
}

func TestCanonicalArtifactAuthorityBeginsAtProviderReceipt(t *testing.T) {
	for _, value := range []any{ManifestClosurePlanRequest{}, CandidateClosureRetentionRequest{}} {
		if _, exists := reflect.TypeOf(value).FieldByName("ArtifactDigest"); exists {
			t.Fatalf("pre-acceptance %T claims canonical ArtifactDigest authority", value)
		}
	}
	candidate := candidateRequestFixture(t)
	encoded, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"artifactDigest"`) {
		t.Fatalf("candidate request claims canonical artifact digest: %s", encoded)
	}

	receipt := ArtifactRetentionReceipt{
		ReceiptID: "retention-1", ProviderID: candidate.ProviderID, AdapterID: candidate.AdapterID,
		CommandID: candidate.CommandID, ParticipantID: candidate.ParticipantID,
		ParticipantVersionID: candidate.ParticipantVersionID,
		ClosurePlanID:        candidate.ClosurePlanID, ClosurePlanDigest: candidate.ClosurePlanDigest,
		CandidateRequestDigest: candidate.TypedPayloadDigest,
		ArtifactDigest:         testArtifactDigest("9"), Status: ArtifactRetentionAccepted,
	}
	if err := ValidateArtifactRetentionReceiptForRequest(receipt, candidate); err != nil {
		t.Fatalf("provider acceptance receipt rejected: %v", err)
	}

	publication := publicationRequestFixture(t)
	publicationReceipt := ArtifactPublicationReceipt{
		ReceiptID: "publication-1", ProviderID: publication.ProviderID, AdapterID: publication.AdapterID,
		CommandID: publication.CommandID, ParticipantID: publication.ParticipantID,
		ParticipantVersionID:     publication.ParticipantVersionID,
		RetentionReceiptID:       publication.RetentionReceiptID,
		PublicationRequestDigest: publication.TypedPayloadDigest,
		ArtifactDigest:           publication.ArtifactDigest, PublishedAt: contractTestTime,
		PublicReference: "public-artifact-1",
		Status:          ArtifactPublicationAccepted,
	}
	if err := ValidateArtifactPublicationReceiptForRequest(publicationReceipt, publication); err != nil {
		t.Fatalf("publication receipt rejected: %v", err)
	}
}

func TestSourceBodiesPlansKindsAndLimitsFailClosed(t *testing.T) {
	manifest := manifestRequestFixture(t)
	manifestBytes := []byte(`{"entry":"bot.star"}`)
	if err := ValidateManifestClosurePlanInput(manifest, manifestBytes); err != nil {
		t.Fatalf("valid manifest bytes rejected: %v", err)
	}
	if err := ValidateManifestClosurePlanInput(manifest, append(manifestBytes, ' ')); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("changed manifest body error = %v, want ErrInvalidGrant", err)
	}
	for _, kind := range []SourceEntryKind{SourceEntrySymlink, SourceEntrySubmodule} {
		payload := manifest.Payload()
		payload.ManifestEntryKind = kind
		if _, err := NewManifestClosurePlanRequest(payload); !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("%s manifest entry error = %v, want ErrInvalidGrant", kind, err)
		}
	}

	plan := closurePlanFixture(t)
	planReceipt := ClosurePlanReceipt{
		ProviderID: plan.ProviderID, AdapterID: plan.AdapterID, CommandID: manifest.CommandID,
		ParticipantID: manifest.ParticipantID, ParticipantVersionID: manifest.ParticipantVersionID,
		RequestPayloadDigest: manifest.TypedPayloadDigest, Plan: plan, Status: ClosurePlanReceiptAccepted,
	}
	if err := ValidateClosurePlanReceiptForRequest(planReceipt, manifest); err != nil {
		t.Fatalf("valid closure plan receipt rejected: %v", err)
	}

	candidate := candidateRequestFixture(t)
	transfer := candidateTransferFixture()
	if err := ValidateCandidateClosureInput(candidate, plan, transfer); err != nil {
		t.Fatalf("valid candidate transfer rejected: %v", err)
	}

	changedBytes := candidateTransferFixture()
	changedBytes.Files[0].Bytes = append(changedBytes.Files[0].Bytes, '!')
	if err := ValidateCandidateClosureInput(candidate, plan, changedBytes); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("changed candidate bytes error = %v, want ErrInvalidGrant", err)
	}

	symlink := candidateTransferFixture()
	symlink.Files[0].EntryKind = SourceEntrySymlink
	symlinkDigest, err := DigestCandidateTransferredFiles(symlink.Files)
	if err != nil {
		t.Fatal(err)
	}
	symlinkPayload := candidate.Payload()
	symlinkPayload.CandidateTransferredBytesDigest = symlinkDigest
	symlinkRequest, err := NewCandidateClosureRetentionRequest(symlinkPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCandidateClosureInput(symlinkRequest, plan, symlink); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("flattened symlink error = %v, want ErrInvalidGrant", err)
	}

	otherRepositoryPayload := candidate.Payload()
	otherRepositoryPayload.RepositoryNodeID = "other-repository"
	otherRepository, err := NewCandidateClosureRetentionRequest(otherRepositoryPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCandidateClosureInput(otherRepository, plan, transfer); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("cross-repository plan reuse error = %v, want ErrInvalidGrant", err)
	}

	for _, unsafePath := range []string{"bots/z.star\nbad", "bots/z.star\rbad", "bots/\x7fbad"} {
		payload := plan.Payload()
		payload.Files = []PlannedSourceFile{{CanonicalPath: unsafePath, EntryKind: SourceEntryRegular, ByteLimit: 10}}
		if _, err := NewClosurePlan(payload); !errors.Is(err, ErrInvalidExecution) {
			t.Fatalf("unsafe path %q error = %v, want ErrInvalidExecution", unsafePath, err)
		}
	}

	unordered := plan.Payload()
	unordered.Files[0], unordered.Files[1] = unordered.Files[1], unordered.Files[0]
	if _, err := NewClosurePlan(unordered); !errors.Is(err, ErrInvalidExecution) {
		t.Fatalf("unordered plan error = %v, want ErrInvalidExecution", err)
	}

	overflowPlanPayload := plan.Payload()
	overflowPlanPayload.ClosurePlanID = "overflow-plan"
	overflowPlanPayload.Files = []PlannedSourceFile{{CanonicalPath: "bots/bot.star", EntryKind: SourceEntryRegular, ByteLimit: 100}}
	overflowPlanPayload.AggregateByteLimit = 3
	overflowPlan, err := NewClosurePlan(overflowPlanPayload)
	if err != nil {
		t.Fatal(err)
	}
	overflowTransfer := CandidateClosureTransfer{Files: []CandidateSourceFile{{CanonicalPath: "bots/bot.star", EntryKind: SourceEntryRegular, Bytes: []byte("four")}}}
	overflowDigest, err := DigestCandidateTransferredFiles(overflowTransfer.Files)
	if err != nil {
		t.Fatal(err)
	}
	overflowRequestPayload := candidate.Payload()
	overflowRequestPayload.ClosurePlanID = overflowPlan.ClosurePlanID
	overflowRequestPayload.ClosurePlanDigest = overflowPlan.ClosurePlanDigest
	overflowRequestPayload.CandidateTransferredBytesDigest = overflowDigest
	overflowRequestPayload.AggregateByteLimit = overflowPlan.AggregateByteLimit
	overflowRequest, err := NewCandidateClosureRetentionRequest(overflowRequestPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCandidateClosureInput(overflowRequest, overflowPlan, overflowTransfer); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("single-file aggregate overflow error = %v, want ErrInvalidGrant", err)
	}
}

func disclosureRequestFixture(t *testing.T) ArtifactDisclosureVerificationRequest {
	t.Helper()
	plan := closurePlanFixture(t)
	transfer := candidateTransferFixture()
	digest, err := DigestCandidateTransferredFiles(transfer.Files)
	if err != nil {
		t.Fatal(err)
	}
	request, err := NewArtifactDisclosureVerificationRequest(ArtifactDisclosureVerificationRequestPayload{
		ProviderID: plan.ProviderID, AdapterID: plan.AdapterID, CommandID: "disclosure-command",
		ParticipantID: plan.ParticipantID, ParticipantVersionID: plan.ParticipantVersionID,
		RepositoryNodeID: plan.RepositoryNodeID, Commit: plan.Commit,
		ClosurePlanID: plan.ClosurePlanID, ClosurePlanDigest: plan.ClosurePlanDigest,
		AggregateByteLimit: plan.AggregateByteLimit, RetentionReceiptID: "retention-1",
		ArtifactDigest: testArtifactDigest("9"), PublicCandidateTransferredBytesDigest: digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func disclosureGrantFixture(t *testing.T) (ArtifactDisclosureVerificationRequest, OperationGrant, OperationRouteBinding) {
	t.Helper()
	request := disclosureRequestFixture(t)
	_, grant, _ := launchGrantFixture(t)
	grant.Scope, grant.Purpose = GrantScopeArtifactDisclosureVerify, GrantPurposeArtifactDisclosureVerify
	grant.CompetitionID, grant.ContestID, grant.RequestID, grant.ProviderInstanceID = "", "", "", ""
	grant.CommandID, grant.TypedPayloadDigest = request.CommandID, request.TypedPayloadDigest
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RepositoryNodeID, grant.Commit = request.RepositoryNodeID, request.Commit
	grant.ClosurePlanID, grant.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	grant.PublicCandidateTransferredBytesDigest = request.PublicCandidateTransferredBytesDigest
	grant.AggregateByteLimit = request.AggregateByteLimit
	grant.RetentionReceiptID, grant.ArtifactDigest = request.RetentionReceiptID, request.ArtifactDigest
	grant.Resource = "/providers/provider-1/artifacts/disclosure-verify"
	return request, grant, routeBindingForGrant(grant)
}

func TestDisclosureVerificationIsProviderAuthoritativeAndExactlyBound(t *testing.T) {
	request, grant, route := disclosureGrantFixture(t)
	if err := ValidateArtifactDisclosureGrantForRequest(VerifiedOperationGrant{Claims: grant}, route, request); err != nil {
		t.Fatalf("valid disclosure grant rejected: %v", err)
	}
	if err := ValidateArtifactDisclosureInput(request, closurePlanFixture(t), candidateTransferFixture()); err != nil {
		t.Fatalf("valid public candidate rejected: %v", err)
	}

	receipt := ArtifactDisclosureVerificationReceipt{
		ReceiptID: "disclosure-1", ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		CommandID: request.CommandID, ParticipantID: request.ParticipantID,
		ParticipantVersionID: request.ParticipantVersionID, RetentionReceiptID: request.RetentionReceiptID,
		ArtifactDigest: request.ArtifactDigest, VerificationRequestDigest: request.TypedPayloadDigest,
		Verdict: ArtifactDisclosureMatched, VerifiedAt: contractTestTime,
	}
	if err := ValidateArtifactDisclosureVerificationReceiptForRequest(receipt, request); err != nil {
		t.Fatalf("provider disclosure verdict rejected: %v", err)
	}

	changed := request
	changed.ArtifactDigest = testArtifactDigest("8")
	if err := ValidateArtifactDisclosureGrantForRequest(VerifiedOperationGrant{Claims: grant}, route, changed); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("changed retained artifact error = %v, want ErrInvalidGrant", err)
	}
}

func TestTokenPortsUseOpaqueTokensNotSelfAssertedClaims(t *testing.T) {
	verifier := reflect.TypeOf((*OperationGrantVerifier)(nil)).Elem().Method(0)
	if verifier.Type.In(1) != reflect.TypeOf(EncodedAccessToken("")) || verifier.Type.Out(0) != reflect.TypeOf(VerifiedOperationGrant{}) {
		t.Fatalf("verifier method = %v", verifier.Type)
	}
	issuer := reflect.TypeOf((*OperationGrantIssuer)(nil)).Elem().Method(0)
	if issuer.Type.Out(0) != reflect.TypeOf(IssuedOperationAccessToken{}) {
		t.Fatalf("issuer method = %v", issuer.Type)
	}
	encoded, err := json.Marshal(IssuedOperationAccessToken{AccessToken: "secret-bearer", TokenType: GrantTokenTypeAccessJWT, ExpiresAt: contractTestTime})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-bearer") {
		t.Fatalf("issued-token metadata serialized bearer token: %s", encoded)
	}

	request, grant, _ := launchGrantFixture(t)
	_ = request
	issuance := grant.RequestedOperation()
	if err := ValidateOperationGrantRequest(issuance); err != nil {
		t.Fatalf("valid issuance request rejected: %v", err)
	}
	if err := ValidateIssuedOperationGrantForRequest(grant, issuance); err != nil {
		t.Fatalf("issued grant does not bind request: %v", err)
	}
	requestType := reflect.TypeOf(OperationGrantRequest{})
	for _, issuerOwned := range []string{"Issuer", "Subject", "Audience", "TokenType", "Scope", "KeyID", "TokenID", "IssuedAt", "NotBefore", "ExpiresAt"} {
		if _, exists := requestType.FieldByName(issuerOwned); exists {
			t.Fatalf("caller can choose issuer-owned %s", issuerOwned)
		}
	}
}

func TestStagedSourceCanonicalDigestVectors(t *testing.T) {
	for name, gotWant := range map[string][2]PayloadDigest{
		"manifest":    {manifestRequestFixture(t).TypedPayloadDigest, "sha256:686978b317452498653cef02e880079e2d14f0fdc123eba4a235685b3dbba4b1"},
		"plan":        {closurePlanFixture(t).ClosurePlanDigest, "sha256:a458f88eba9d11f0bf60595657a748962cb2272eb463dee84054a259d1ffb306"},
		"candidate":   {candidateRequestFixture(t).TypedPayloadDigest, "sha256:b9cad84a391062c0a0008f6cd3355c61ee671fa447f558312f8a34b673b8986c"},
		"publication": {publicationRequestFixture(t).TypedPayloadDigest, "sha256:addbfe1efccd90efbd3f205a38862733f1e453870932530f493f869244ee7b55"},
	} {
		t.Run(name, func(t *testing.T) {
			if gotWant[0] != gotWant[1] {
				t.Fatalf("digest = %q, want %q", gotWant[0], gotWant[1])
			}
		})
	}
}

func TestGrantPurposeScopeTokenTypeAndDigestFormats(t *testing.T) {
	for name, gotWant := range map[string][2]GrantScope{
		"manifest plan":       {GrantScopeManifestClosurePlan, "participant.version.manifest.plan"},
		"validate and retain": {GrantScopeCandidateValidateRetain, "participant.version.validate-and-retain"},
		"disclosure match":    {GrantScopeArtifactDisclosureVerify, "participant.version.disclosure.match"},
		"publish":             {GrantScopeArtifactPublish, "participant.version.publish"},
		"contest launch":      {GrantScopeContestLaunch, "competition.contest.launch"},
	} {
		if gotWant[0] != gotWant[1] {
			t.Fatalf("%s scope = %q, want %q", name, gotWant[0], gotWant[1])
		}
	}

	_, grant, _ := launchGrantFixture(t)
	for name, mutate := range map[string]func(*OperationGrant){
		"token type":    func(v *OperationGrant) { v.TokenType = "JWT" },
		"purpose":       func(v *OperationGrant) { v.Purpose = GrantPurposeContestStarted },
		"scope":         func(v *OperationGrant) { v.Scope = GrantScopeContestStarted },
		"typed payload": func(v *OperationGrant) { v.TypedPayloadDigest = "sha256:a" },
		"raw transport": func(v *OperationGrant) { v.RawTransportDigest = PayloadDigest("sha256:" + strings.Repeat("A", 64)) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := grant
			mutate(&changed)
			if err := ValidateOperationGrant(changed); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("error = %v, want ErrInvalidGrant", err)
			}
		})
	}
}
