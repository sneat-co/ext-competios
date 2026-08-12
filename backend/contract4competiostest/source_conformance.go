package contract4competiostest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type SourceArtifactProviderFactory func() contract4competios.SourceArtifactProvider

func CheckSourceArtifactProvider(factory SourceArtifactProviderFactory) []error {
	ctx := context.Background()
	provider := factory()
	manifestBytes := sourceManifestBytesFixture()
	manifest := sourceManifestRequestFixture(manifestBytes)
	manifestGrant, manifestRoute := sourceManifestGrantFixture(manifest, manifestBytes, "manifest-token", "key-a")
	var violations []error
	for _, kind := range []contract4competios.SourceEntryKind{contract4competios.SourceEntrySymlink, contract4competios.SourceEntrySubmodule} {
		wrongKind := manifest
		wrongKind.ManifestEntryKind = kind
		wrongKind.TypedPayloadDigest, _ = contract4competios.DigestManifestClosurePlanRequestPayload(wrongKind.Payload())
		wrongGrant, _ := sourceManifestGrantFixture(wrongKind, manifestBytes, "manifest-kind-"+string(kind), "key-a")
		if _, planErr := provider.PlanManifestClosure(ctx, wrongGrant, wrongKind, manifestBytes); planErr == nil {
			violations = append(violations, fmt.Errorf("%s manifest entry was accepted", kind))
		}
	}

	planReceipt, err := provider.PlanManifestClosure(ctx, manifestGrant, manifest, manifestBytes)
	if err != nil {
		return append(violations, fmt.Errorf("manifest closure plan: %w", err))
	}
	if validationErr := contract4competios.ValidateClosurePlanReceiptForRequest(planReceipt, manifest); validationErr != nil || planReceipt.Status != contract4competios.ClosurePlanReceiptAccepted {
		violations = append(violations, fmt.Errorf("closure plan receipt = %+v: %v", planReceipt, validationErr))
		planReceipt = contract4competios.ClosurePlanReceipt{
			ProviderID: manifest.ProviderID, AdapterID: manifest.AdapterID, CommandID: manifest.CommandID,
			ParticipantID: manifest.ParticipantID, ParticipantVersionID: manifest.ParticipantVersionID,
			RequestPayloadDigest: manifest.TypedPayloadDigest, Plan: sourcePlanFixture(manifest),
			Status: contract4competios.ClosurePlanReceiptAccepted,
		}
	}
	_ = manifestRoute

	freshManifestGrant, _ := sourceManifestGrantFixture(manifest, manifestBytes, "manifest-token-fresh", "key-rotated")
	replayedPlan, err := provider.PlanManifestClosure(ctx, freshManifestGrant, manifest, manifestBytes)
	if err != nil || replayedPlan.Status != contract4competios.ClosurePlanReceiptReplayed || replayedPlan.Plan.ClosurePlanDigest != planReceipt.Plan.ClosurePlanDigest || replayedPlan.Plan.ClosurePlanID != planReceipt.Plan.ClosurePlanID {
		violations = append(violations, fmt.Errorf("closure plan replay = %+v: %v", replayedPlan, err))
	}

	changedManifest := append(append([]byte(nil), manifestBytes...), ' ')
	changedManifestGrant, _ := sourceManifestGrantFixture(manifest, changedManifest, "manifest-body-mismatch", "key-a")
	if _, err := provider.PlanManifestClosure(ctx, changedManifestGrant, manifest, changedManifest); err == nil {
		violations = append(violations, errors.New("manifest digest/body mismatch was accepted"))
	}
	changedManifestRequestPayload := manifest.Payload()
	changedManifestRequestPayload.ParticipantVersionID = "changed-version"
	changedManifestRequest, buildErr := contract4competios.NewManifestClosurePlanRequest(changedManifestRequestPayload)
	if buildErr != nil {
		violations = append(violations, buildErr)
	} else {
		changedRequestGrant, _ := sourceManifestGrantFixture(changedManifestRequest, manifestBytes, "manifest-command-conflict", "key-a")
		if _, planErr := provider.PlanManifestClosure(ctx, changedRequestGrant, changedManifestRequest, manifestBytes); !errors.Is(planErr, contract4competios.ErrCommandConflict) {
			violations = append(violations, fmt.Errorf("changed manifest command error = %v", planErr))
		}
	}

	transfer := sourceCandidateTransferFixture()
	candidate := sourceCandidateRequestFixture(planReceipt.Plan, transfer, "candidate-command")
	candidateGrant, _ := sourceCandidateGrantFixture(candidate, transfer, "candidate-token", "key-a")
	retention, err := provider.ValidateAndRetainCandidate(ctx, candidateGrant, candidate, transfer)
	if err != nil {
		return append(violations, fmt.Errorf("candidate retention: %w", err))
	}
	if validationErr := contract4competios.ValidateArtifactRetentionReceiptForRequest(retention, candidate); validationErr != nil || retention.Status != contract4competios.ArtifactRetentionAccepted {
		violations = append(violations, fmt.Errorf("retention receipt = %+v: %v", retention, validationErr))
		retention = contract4competios.ArtifactRetentionReceipt{
			ReceiptID: "fallback-retention", ProviderID: candidate.ProviderID, AdapterID: candidate.AdapterID,
			CommandID: candidate.CommandID, ParticipantID: candidate.ParticipantID,
			ParticipantVersionID: candidate.ParticipantVersionID, ClosurePlanID: candidate.ClosurePlanID,
			ClosurePlanDigest: candidate.ClosurePlanDigest, CandidateRequestDigest: candidate.TypedPayloadDigest,
			ArtifactDigest: artifactDigest("9"), Status: contract4competios.ArtifactRetentionAccepted,
		}
	}

	freshCandidateGrant, _ := sourceCandidateGrantFixture(candidate, transfer, "candidate-token-fresh", "key-rotated")
	replayedRetention, err := provider.ValidateAndRetainCandidate(ctx, freshCandidateGrant, candidate, transfer)
	if err != nil || replayedRetention.Status != contract4competios.ArtifactRetentionReplayed || replayedRetention.ReceiptID != retention.ReceiptID || replayedRetention.ArtifactDigest != retention.ArtifactDigest {
		violations = append(violations, fmt.Errorf("candidate replay = %+v: %v", replayedRetention, err))
	}

	changedTransfer := copyCandidateTransfer(transfer)
	changedTransfer.Files[0].Bytes = append(changedTransfer.Files[0].Bytes, '!')
	changedGrant, _ := sourceCandidateGrantFixture(candidate, changedTransfer, "candidate-body-mismatch", "key-a")
	if _, err := provider.ValidateAndRetainCandidate(ctx, changedGrant, candidate, changedTransfer); err == nil {
		violations = append(violations, errors.New("candidate digest/body mismatch was accepted"))
	}
	changedCandidate := sourceCandidateRequestFixture(planReceipt.Plan, changedTransfer, candidate.CommandID)
	changedCommandGrant, _ := sourceCandidateGrantFixture(changedCandidate, changedTransfer, "candidate-command-conflict", "key-a")
	if _, err := provider.ValidateAndRetainCandidate(ctx, changedCommandGrant, changedCandidate, changedTransfer); !errors.Is(err, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("changed candidate command error = %v", err))
	}

	wrongPath := copyCandidateTransfer(transfer)
	wrongPath.Files[0].CanonicalPath = "bots/other.star"
	wrongPathCandidate := sourceCandidateRequestFixture(planReceipt.Plan, wrongPath, "wrong-path-command")
	wrongPathGrant, _ := sourceCandidateGrantFixture(wrongPathCandidate, wrongPath, "wrong-path-token", "key-a")
	if _, err := provider.ValidateAndRetainCandidate(ctx, wrongPathGrant, wrongPathCandidate, wrongPath); err == nil {
		violations = append(violations, errors.New("candidate path/plan mismatch was accepted"))
	}

	symlink := copyCandidateTransfer(transfer)
	symlink.Files[0].EntryKind = contract4competios.SourceEntrySymlink
	symlinkCandidate := sourceCandidateRequestFixture(planReceipt.Plan, symlink, "symlink-command")
	symlinkGrant, _ := sourceCandidateGrantFixture(symlinkCandidate, symlink, "symlink-token", "key-a")
	if _, err := provider.ValidateAndRetainCandidate(ctx, symlinkGrant, symlinkCandidate, symlink); err == nil {
		violations = append(violations, errors.New("flattened symlink candidate was accepted"))
	}

	wrongPlanPayload := candidate.Payload()
	wrongPlanPayload.CommandID = "wrong-plan-command"
	wrongPlanPayload.ClosurePlanID = "other-plan"
	wrongPlan, buildErr := contract4competios.NewCandidateClosureRetentionRequest(wrongPlanPayload)
	if buildErr != nil {
		violations = append(violations, buildErr)
	} else {
		wrongPlanGrant, _ := sourceCandidateGrantFixture(wrongPlan, transfer, "wrong-plan-token", "key-a")
		if _, err := provider.ValidateAndRetainCandidate(ctx, wrongPlanGrant, wrongPlan, transfer); err == nil {
			violations = append(violations, errors.New("unknown closure plan was accepted"))
		}
	}

	publication := sourcePublicationRequestFixture(retention)
	publicationGrant, _ := sourcePublicationGrantFixture(publication, "publication-token", "key-a")
	published, err := provider.PublishArtifact(ctx, publicationGrant, publication)
	if err != nil || contract4competios.ValidateArtifactPublicationReceiptForRequest(published, publication) != nil || published.Status != contract4competios.ArtifactPublicationAccepted {
		violations = append(violations, fmt.Errorf("publication receipt = %+v: %v", published, err))
	}
	freshPublicationGrant, _ := sourcePublicationGrantFixture(publication, "publication-token-fresh", "key-rotated")
	replayedPublication, err := provider.PublishArtifact(ctx, freshPublicationGrant, publication)
	if err != nil || replayedPublication.Status != contract4competios.ArtifactPublicationReplayed || replayedPublication.ReceiptID != published.ReceiptID || replayedPublication.PublicReference != published.PublicReference || !replayedPublication.PublishedAt.Equal(published.PublishedAt) {
		violations = append(violations, fmt.Errorf("publication replay = %+v: %v", replayedPublication, err))
	}
	changedPublicationPayload := publication.Payload()
	changedPublicationPayload.ParticipantVersionID = "changed-version"
	changedPublication, buildErr := contract4competios.NewArtifactPublicationRequest(changedPublicationPayload)
	if buildErr != nil {
		violations = append(violations, buildErr)
	} else {
		changedPublicationGrant, _ := sourcePublicationGrantFixture(changedPublication, "publication-command-conflict", "key-a")
		if _, publishErr := provider.PublishArtifact(ctx, changedPublicationGrant, changedPublication); !errors.Is(publishErr, contract4competios.ErrCommandConflict) {
			violations = append(violations, fmt.Errorf("changed publication command error = %v", publishErr))
		}
	}

	disclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, transfer, "disclosure-command")
	disclosureGrant, _ := sourceDisclosureGrantFixture(disclosure, transfer, "disclosure-token", "key-a")
	verifiedDisclosure, err := provider.VerifyArtifactDisclosure(ctx, disclosureGrant, disclosure, transfer)
	if err != nil || contract4competios.ValidateArtifactDisclosureVerificationReceiptForRequest(verifiedDisclosure, disclosure) != nil || verifiedDisclosure.Verdict != contract4competios.ArtifactDisclosureMatched {
		violations = append(violations, fmt.Errorf("matching disclosure = %+v: %v", verifiedDisclosure, err))
	}
	freshDisclosureGrant, _ := sourceDisclosureGrantFixture(disclosure, transfer, "disclosure-token-fresh", "key-rotated")
	replayedDisclosure, err := provider.VerifyArtifactDisclosure(ctx, freshDisclosureGrant, disclosure, transfer)
	if err != nil || replayedDisclosure != verifiedDisclosure {
		violations = append(violations, fmt.Errorf("disclosure replay = %+v: %v", replayedDisclosure, err))
	}

	publiclyChanged := copyCandidateTransfer(transfer)
	publiclyChanged.Files[0].Bytes = append(publiclyChanged.Files[0].Bytes, '!')
	conflictingDisclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, publiclyChanged, disclosure.CommandID)
	conflictGrant, _ := sourceDisclosureGrantFixture(conflictingDisclosure, publiclyChanged, "disclosure-command-conflict", "key-a")
	if _, disclosureErr := provider.VerifyArtifactDisclosure(ctx, conflictGrant, conflictingDisclosure, publiclyChanged); !errors.Is(disclosureErr, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("changed disclosure command error = %v", disclosureErr))
	}
	mismatchDisclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, publiclyChanged, "mismatch-disclosure-command")
	mismatchGrant, _ := sourceDisclosureGrantFixture(mismatchDisclosure, publiclyChanged, "mismatch-disclosure-token", "key-a")
	mismatchReceipt, err := provider.VerifyArtifactDisclosure(ctx, mismatchGrant, mismatchDisclosure, publiclyChanged)
	if err != nil || mismatchReceipt.Verdict != contract4competios.ArtifactDisclosureMismatched || contract4competios.ValidateArtifactDisclosureVerificationReceiptForRequest(mismatchReceipt, mismatchDisclosure) != nil {
		violations = append(violations, fmt.Errorf("mismatching disclosure = %+v: %v", mismatchReceipt, err))
	}
	return violations
}

func sourceManifestBytesFixture() []byte {
	return []byte(`{"entry":"bots/bot.star","support":["bots/opening.json"]}`)
}

func sourceManifestRequestFixture(manifestBytes []byte) contract4competios.ManifestClosurePlanRequest {
	request, err := contract4competios.NewManifestClosurePlanRequest(contract4competios.ManifestClosurePlanRequestPayload{
		ProviderID: "provider", AdapterID: "adapter", CommandID: "manifest-command",
		ParticipantID: "participant-a", ParticipantVersionID: "version-a",
		RepositoryNodeID: "repository-node", Commit: "0123456789abcdef0123456789abcdef01234567",
		ManifestPath: "bots/manifest.json", ManifestEntryKind: contract4competios.SourceEntryRegular,
		RawManifestBytesDigest: contract4competios.DigestRawManifestBytes(manifestBytes),
		ManifestByteLimit:      32768,
	})
	if err != nil {
		panic(err)
	}
	return request
}

func sourcePlanFixture(request contract4competios.ManifestClosurePlanRequest) contract4competios.ClosurePlan {
	plan, err := contract4competios.NewClosurePlan(contract4competios.ClosurePlanPayload{
		ClosurePlanID: "closure-plan", ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		ParticipantID: request.ParticipantID, ParticipantVersionID: request.ParticipantVersionID,
		RepositoryNodeID: request.RepositoryNodeID, Commit: request.Commit, ManifestPath: request.ManifestPath,
		ManifestEntryKind:     request.ManifestEntryKind,
		ManifestRequestDigest: request.TypedPayloadDigest, RawManifestBytesDigest: request.RawManifestBytesDigest,
		Files: []contract4competios.PlannedSourceFile{
			{CanonicalPath: "bots/bot.star", EntryKind: contract4competios.SourceEntryRegular, ByteLimit: 65536},
			{CanonicalPath: "bots/opening.json", EntryKind: contract4competios.SourceEntryRegular, ByteLimit: 32768},
		},
		AggregateByteLimit: 98304,
	})
	if err != nil {
		panic(err)
	}
	return plan
}

func sourceCandidateTransferFixture() contract4competios.CandidateClosureTransfer {
	return contract4competios.CandidateClosureTransfer{Files: []contract4competios.CandidateSourceFile{
		{CanonicalPath: "bots/bot.star", EntryKind: contract4competios.SourceEntryRegular, Bytes: []byte("function move() { return 1 }")},
		{CanonicalPath: "bots/opening.json", EntryKind: contract4competios.SourceEntryRegular, Bytes: []byte(`{"opening":"center"}`)},
	}}
}

func copyCandidateTransfer(value contract4competios.CandidateClosureTransfer) contract4competios.CandidateClosureTransfer {
	encoded, _ := json.Marshal(value)
	var copied contract4competios.CandidateClosureTransfer
	_ = json.Unmarshal(encoded, &copied)
	return copied
}

func sourceCandidateRequestFixture(plan contract4competios.ClosurePlan, transfer contract4competios.CandidateClosureTransfer, command contract4competios.CommandID) contract4competios.CandidateClosureRetentionRequest {
	digest, err := contract4competios.DigestCandidateTransferredFiles(transfer.Files)
	if err != nil {
		panic(err)
	}
	request, err := contract4competios.NewCandidateClosureRetentionRequest(contract4competios.CandidateClosureRetentionRequestPayload{
		ProviderID: plan.ProviderID, AdapterID: plan.AdapterID, CommandID: command,
		ParticipantID: plan.ParticipantID, ParticipantVersionID: plan.ParticipantVersionID,
		RepositoryNodeID: plan.RepositoryNodeID, Commit: plan.Commit,
		ClosurePlanID: plan.ClosurePlanID, ClosurePlanDigest: plan.ClosurePlanDigest,
		CandidateTransferredBytesDigest: digest, AggregateByteLimit: plan.AggregateByteLimit,
	})
	if err != nil {
		panic(err)
	}
	return request
}

func sourcePublicationRequestFixture(retention contract4competios.ArtifactRetentionReceipt) contract4competios.ArtifactPublicationRequest {
	request, err := contract4competios.NewArtifactPublicationRequest(contract4competios.ArtifactPublicationRequestPayload{
		ProviderID: retention.ProviderID, AdapterID: retention.AdapterID, CommandID: "publication-command",
		ParticipantID: retention.ParticipantID, ParticipantVersionID: retention.ParticipantVersionID,
		RetentionReceiptID: retention.ReceiptID, ArtifactDigest: retention.ArtifactDigest,
	})
	if err != nil {
		panic(err)
	}
	return request
}

func sourceDisclosureRequestFixture(plan contract4competios.ClosurePlan, retention contract4competios.ArtifactRetentionReceipt, transfer contract4competios.CandidateClosureTransfer, command contract4competios.CommandID) contract4competios.ArtifactDisclosureVerificationRequest {
	digest, err := contract4competios.DigestCandidateTransferredFiles(transfer.Files)
	if err != nil {
		panic(err)
	}
	request, err := contract4competios.NewArtifactDisclosureVerificationRequest(contract4competios.ArtifactDisclosureVerificationRequestPayload{
		ProviderID: plan.ProviderID, AdapterID: plan.AdapterID, CommandID: command,
		ParticipantID: plan.ParticipantID, ParticipantVersionID: plan.ParticipantVersionID,
		RepositoryNodeID: plan.RepositoryNodeID, Commit: plan.Commit,
		ClosurePlanID: plan.ClosurePlanID, ClosurePlanDigest: plan.ClosurePlanDigest,
		AggregateByteLimit: plan.AggregateByteLimit, RetentionReceiptID: retention.ReceiptID,
		ArtifactDigest: retention.ArtifactDigest, PublicCandidateTransferredBytesDigest: digest,
	})
	if err != nil {
		panic(err)
	}
	return request
}

func sourceGrantBase(tokenID, keyID string, purpose contract4competios.GrantPurpose, scope contract4competios.GrantScope, typedDigest contract4competios.PayloadDigest, rawBody []byte, resource string) contract4competios.OperationGrant {
	return contract4competios.OperationGrant{
		Issuer: fixtureIssuer, Subject: fixtureSubject, Audience: fixtureAudience,
		TokenType: contract4competios.GrantTokenTypeAccessJWT, Scope: scope, Purpose: purpose,
		KeyID: keyID, TokenID: tokenID, IssuedAt: fixtureTime, NotBefore: fixtureTime,
		ExpiresAt: fixtureTime.Add(5 * time.Minute), ProviderID: "provider", AdapterID: "adapter",
		CommandID: "placeholder", TypedPayloadDigest: typedDigest,
		TransportContentType: fixtureContentType,
		RawTransportDigest:   contract4competios.DigestRawTransportBody(fixtureContentType, rawBody),
		Method:               fixtureMethod, Resource: resource,
	}
}

func routeForSourceGrant(grant contract4competios.OperationGrant) contract4competios.OperationRouteBinding {
	return contract4competios.OperationRouteBinding{
		Issuer: grant.Issuer, Subject: grant.Subject, Audience: grant.Audience,
		TokenType: grant.TokenType, Scope: grant.Scope, Purpose: grant.Purpose,
		ProviderID: grant.ProviderID, AdapterID: grant.AdapterID,
		TransportContentType: grant.TransportContentType, RawTransportDigest: grant.RawTransportDigest,
		Method: grant.Method, Resource: grant.Resource,
	}
}

func sourceManifestGrantFixture(request contract4competios.ManifestClosurePlanRequest, manifestBytes []byte, tokenID, keyID string) (contract4competios.VerifiedOperationGrant, contract4competios.OperationRouteBinding) {
	rawBody, _ := json.Marshal(struct {
		Request contract4competios.ManifestClosurePlanRequest `json:"request"`
		Bytes   []byte                                        `json:"bytes"`
	}{request, manifestBytes})
	grant := sourceGrantBase(tokenID, keyID, contract4competios.GrantPurposeManifestClosurePlan, contract4competios.GrantScopeManifestClosurePlan, request.TypedPayloadDigest, rawBody, "/game/source/closure-plans")
	grant.CommandID = request.CommandID
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RepositoryNodeID, grant.Commit, grant.ManifestPath = request.RepositoryNodeID, request.Commit, request.ManifestPath
	grant.ManifestEntryKind = request.ManifestEntryKind
	grant.RawManifestBytesDigest, grant.ManifestByteLimit = request.RawManifestBytesDigest, request.ManifestByteLimit
	return contract4competios.VerifiedOperationGrant{Claims: grant}, routeForSourceGrant(grant)
}

func sourceCandidateGrantFixture(request contract4competios.CandidateClosureRetentionRequest, transfer contract4competios.CandidateClosureTransfer, tokenID, keyID string) (contract4competios.VerifiedOperationGrant, contract4competios.OperationRouteBinding) {
	rawBody, _ := json.Marshal(struct {
		Request  contract4competios.CandidateClosureRetentionRequest `json:"request"`
		Transfer contract4competios.CandidateClosureTransfer         `json:"transfer"`
	}{request, transfer})
	grant := sourceGrantBase(tokenID, keyID, contract4competios.GrantPurposeCandidateValidateRetain, contract4competios.GrantScopeCandidateValidateRetain, request.TypedPayloadDigest, rawBody, "/game/source/candidate-closures")
	grant.CommandID = request.CommandID
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RepositoryNodeID, grant.Commit = request.RepositoryNodeID, request.Commit
	grant.ClosurePlanID, grant.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	grant.CandidateTransferredBytesDigest, grant.AggregateByteLimit = request.CandidateTransferredBytesDigest, request.AggregateByteLimit
	return contract4competios.VerifiedOperationGrant{Claims: grant}, routeForSourceGrant(grant)
}

func sourcePublicationGrantFixture(request contract4competios.ArtifactPublicationRequest, tokenID, keyID string) (contract4competios.VerifiedOperationGrant, contract4competios.OperationRouteBinding) {
	rawBody, _ := json.Marshal(request)
	grant := sourceGrantBase(tokenID, keyID, contract4competios.GrantPurposeArtifactPublish, contract4competios.GrantScopeArtifactPublish, request.TypedPayloadDigest, rawBody, "/game/artifacts/publish")
	grant.CommandID = request.CommandID
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RetentionReceiptID, grant.ArtifactDigest = request.RetentionReceiptID, request.ArtifactDigest
	return contract4competios.VerifiedOperationGrant{Claims: grant}, routeForSourceGrant(grant)
}

func sourceDisclosureGrantFixture(request contract4competios.ArtifactDisclosureVerificationRequest, transfer contract4competios.CandidateClosureTransfer, tokenID, keyID string) (contract4competios.VerifiedOperationGrant, contract4competios.OperationRouteBinding) {
	rawBody, _ := json.Marshal(struct {
		Request  contract4competios.ArtifactDisclosureVerificationRequest `json:"request"`
		Transfer contract4competios.CandidateClosureTransfer              `json:"transfer"`
	}{request, transfer})
	grant := sourceGrantBase(tokenID, keyID, contract4competios.GrantPurposeArtifactDisclosureVerify, contract4competios.GrantScopeArtifactDisclosureVerify, request.TypedPayloadDigest, rawBody, "/game/artifacts/disclosure-verify")
	grant.CommandID = request.CommandID
	grant.ParticipantID, grant.ParticipantVersionID = request.ParticipantID, request.ParticipantVersionID
	grant.RepositoryNodeID, grant.Commit = request.RepositoryNodeID, request.Commit
	grant.ClosurePlanID, grant.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	grant.PublicCandidateTransferredBytesDigest, grant.AggregateByteLimit = request.PublicCandidateTransferredBytesDigest, request.AggregateByteLimit
	grant.RetentionReceiptID, grant.ArtifactDigest = request.RetentionReceiptID, request.ArtifactDigest
	return contract4competios.VerifiedOperationGrant{Claims: grant}, routeForSourceGrant(grant)
}
