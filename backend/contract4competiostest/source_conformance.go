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
	manifestCrossPurpose, _ := sourceCandidateGrantFixture(sourceCandidateRequestFixture(planReceipt.Plan, sourceCandidateTransferFixture(), "manifest-cross-purpose-command"), sourceCandidateTransferFixture(), "manifest-cross-purpose-token", "key-a")
	violations = append(violations, checkPopulatedManifestReplayAuthority(provider, manifest, manifestBytes, planReceipt, manifestCrossPurpose.Claims)...)

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
	crossStageCandidate := sourceCandidateRequestFixture(planReceipt.Plan, transfer, manifest.CommandID)
	crossStageCandidateGrant, _ := sourceCandidateGrantFixture(crossStageCandidate, transfer, "cross-stage-candidate-token", "key-a")
	if _, candidateErr := provider.ValidateAndRetainCandidate(ctx, crossStageCandidateGrant, crossStageCandidate, transfer); !errors.Is(candidateErr, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("manifest command reused for candidate error = %v", candidateErr))
	}
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
	violations = append(violations, checkPopulatedCandidateReplayAuthority(provider, candidate, transfer, retention, manifestGrant.Claims)...)

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
	crossStageDisclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, transfer, candidate.CommandID)
	crossStageDisclosureGrant, _ := sourceDisclosureGrantFixture(crossStageDisclosure, transfer, "cross-stage-disclosure-token", "key-a")
	if _, disclosureErr := provider.VerifyArtifactDisclosure(ctx, crossStageDisclosureGrant, crossStageDisclosure, transfer); !errors.Is(disclosureErr, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("candidate command reused for disclosure error = %v", disclosureErr))
	}
	for name, mutate := range map[string]func(*contract4competios.ArtifactDisclosureVerificationRequestPayload){
		"id": func(value *contract4competios.ArtifactDisclosureVerificationRequestPayload) {
			value.ClosurePlanID = "other-plan"
		},
		"digest": func(value *contract4competios.ArtifactDisclosureVerificationRequestPayload) {
			value.ClosurePlanDigest = payloadDigest("4")
		},
	} {
		command := contract4competios.CommandID("wrong-disclosure-plan-" + name)
		valid := sourceDisclosureRequestFixture(planReceipt.Plan, retention, transfer, command)
		payload := valid.Payload()
		mutate(&payload)
		wrong, buildErr := contract4competios.NewArtifactDisclosureVerificationRequest(payload)
		if buildErr != nil {
			violations = append(violations, buildErr)
			continue
		}
		wrongGrant, _ := sourceDisclosureGrantFixture(wrong, transfer, "wrong-disclosure-plan-"+name+"-token", "key-a")
		if _, disclosureErr := provider.VerifyArtifactDisclosure(ctx, wrongGrant, wrong, transfer); disclosureErr == nil {
			violations = append(violations, fmt.Errorf("disclosure with wrong closure plan %s was accepted", name))
		}
		validGrant, _ := sourceDisclosureGrantFixture(valid, transfer, "valid-disclosure-after-wrong-plan-"+name, "key-a")
		receipt, disclosureErr := provider.VerifyArtifactDisclosure(ctx, validGrant, valid, transfer)
		if disclosureErr != nil || receipt.Verdict != contract4competios.ArtifactDisclosureMatched {
			violations = append(violations, fmt.Errorf("valid disclosure after wrong closure plan %s = %+v: %v", name, receipt, disclosureErr))
		}
	}

	// Publication is impossible until the provider has durably matched the
	// public closure. The rejected command must remain available for the later
	// valid publication, proving rejection did not mutate idempotency state.
	preDisclosure := sourcePublicationRequestWithBinding(retention, "missing-disclosure", payloadDigest("7"), "publication-command")
	preDisclosureGrant, _ := sourcePublicationGrantFixture(preDisclosure, "pre-disclosure-publication-token", "key-a")
	if _, publishErr := provider.PublishArtifact(ctx, preDisclosureGrant, preDisclosure); publishErr == nil {
		violations = append(violations, errors.New("publication before matched disclosure was accepted"))
	}

	publiclyChanged := copyCandidateTransfer(transfer)
	publiclyChanged.Files[0].Bytes = append(publiclyChanged.Files[0].Bytes, '!')
	mismatchDisclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, publiclyChanged, "mismatch-disclosure-command")
	mismatchGrant, _ := sourceDisclosureGrantFixture(mismatchDisclosure, publiclyChanged, "mismatch-disclosure-token", "key-a")
	mismatchReceipt, err := provider.VerifyArtifactDisclosure(ctx, mismatchGrant, mismatchDisclosure, publiclyChanged)
	if err != nil || mismatchReceipt.Verdict != contract4competios.ArtifactDisclosureMismatched || contract4competios.ValidateArtifactDisclosureVerificationReceiptForRequest(mismatchReceipt, mismatchDisclosure) != nil {
		return append(violations, fmt.Errorf("mismatching disclosure = %+v: %v", mismatchReceipt, err))
	}
	mismatchPublication := sourcePublicationRequestFixture(retention, mismatchDisclosure, mismatchReceipt, "mismatch-publication-command")
	mismatchPublicationGrant, _ := sourcePublicationGrantFixture(mismatchPublication, "mismatch-publication-token", "key-a")
	if _, publishErr := provider.PublishArtifact(ctx, mismatchPublicationGrant, mismatchPublication); publishErr == nil {
		violations = append(violations, errors.New("publication after mismatched disclosure was accepted"))
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
	violations = append(violations, checkPopulatedDisclosureReplayAuthority(provider, disclosure, transfer, verifiedDisclosure, candidateGrant.Claims)...)
	crossStagePublication := sourcePublicationRequestFixture(retention, disclosure, verifiedDisclosure, disclosure.CommandID)
	crossStagePublicationGrant, _ := sourcePublicationGrantFixture(crossStagePublication, "cross-stage-publication-token", "key-a")
	if _, publishErr := provider.PublishArtifact(ctx, crossStagePublicationGrant, crossStagePublication); !errors.Is(publishErr, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("disclosure command reused for publication error = %v", publishErr))
	}

	conflictingDisclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, publiclyChanged, disclosure.CommandID)
	conflictGrant, _ := sourceDisclosureGrantFixture(conflictingDisclosure, publiclyChanged, "disclosure-command-conflict", "key-a")
	if _, disclosureErr := provider.VerifyArtifactDisclosure(ctx, conflictGrant, conflictingDisclosure, publiclyChanged); !errors.Is(disclosureErr, contract4competios.ErrCommandConflict) {
		violations = append(violations, fmt.Errorf("changed disclosure command error = %v", disclosureErr))
	}

	publication := sourcePublicationRequestFixture(retention, disclosure, verifiedDisclosure, "publication-command")
	publicationGrant, _ := sourcePublicationGrantFixture(publication, "publication-token", "key-a")
	published, err := provider.PublishArtifact(ctx, publicationGrant, publication)
	if err != nil || contract4competios.ValidateArtifactPublicationReceiptForRequest(published, publication) != nil || published.Status != contract4competios.ArtifactPublicationAccepted {
		violations = append(violations, fmt.Errorf("publication receipt = %+v: %v", published, err))
	}
	freshPublicationGrant, _ := sourcePublicationGrantFixture(publication, "publication-token-fresh", "key-rotated")
	replayedPublication, err := provider.PublishArtifact(ctx, freshPublicationGrant, publication)
	if err != nil || replayedPublication.Status != contract4competios.ArtifactPublicationReplayed || replayedPublication.ReceiptID != published.ReceiptID || replayedPublication.PublicReference != published.PublicReference || !replayedPublication.PublishedAt.Equal(published.PublishedAt) || replayedPublication.DisclosureReceiptID != published.DisclosureReceiptID || replayedPublication.DisclosureRequestDigest != published.DisclosureRequestDigest {
		violations = append(violations, fmt.Errorf("publication replay = %+v: %v", replayedPublication, err))
	}
	violations = append(violations, checkPopulatedPublicationReplayAuthority(provider, publication, published, disclosureGrant.Claims)...)
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
	crossStageManifestPayload := manifest.Payload()
	crossStageManifestPayload.CommandID = publication.CommandID
	crossStageManifest, buildErr := contract4competios.NewManifestClosurePlanRequest(crossStageManifestPayload)
	if buildErr != nil {
		violations = append(violations, buildErr)
	} else {
		crossStageManifestGrant, _ := sourceManifestGrantFixture(crossStageManifest, manifestBytes, "cross-stage-manifest-token", "key-a")
		if _, planErr := provider.PlanManifestClosure(ctx, crossStageManifestGrant, crossStageManifest, manifestBytes); !errors.Is(planErr, contract4competios.ErrCommandConflict) {
			violations = append(violations, fmt.Errorf("publication command reused for manifest error = %v", planErr))
		}
	}
	return append(violations, checkSourceCommandLedgerPairs(factory)...)
}

func checkPopulatedManifestReplayAuthority(provider contract4competios.SourceArtifactProvider, request contract4competios.ManifestClosurePlanRequest, manifestBytes []byte, original contract4competios.ClosurePlanReceipt, crossPurpose contract4competios.OperationGrant) []error {
	var violations []error
	for name, mutate := range sourceReplayGrantMutationsForPurpose(contract4competios.GrantPurposeManifestClosurePlan) {
		bad, _ := sourceManifestGrantFixture(request, manifestBytes, "populated-manifest-bad-"+name, "key-a")
		mutate(&bad.Claims)
		receipt, err := provider.PlanManifestClosure(context.Background(), bad, request, manifestBytes)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || !emptyClosurePlanReceipt(receipt) {
			violations = append(violations, fmt.Errorf("populated manifest %s grant = %+v: %v, want empty receipt and ErrInvalidGrant", name, receipt, err))
		}
		valid, _ := sourceManifestGrantFixture(request, manifestBytes, "populated-manifest-retry-"+name, "key-rotated")
		replayed, replayErr := provider.PlanManifestClosure(context.Background(), valid, request, manifestBytes)
		if replayErr != nil || replayed.Status != contract4competios.ClosurePlanReceiptReplayed || !sameClosurePlanReceiptEvidence(original, replayed) {
			violations = append(violations, fmt.Errorf("populated manifest %s valid retry = %+v: %v", name, replayed, replayErr))
		}
	}
	violations = append(violations, checkManifestCrossPurposeReplay(provider, request, manifestBytes, original, crossPurpose)...)
	return violations
}

func emptyClosurePlanReceipt(value contract4competios.ClosurePlanReceipt) bool {
	encoded, _ := json.Marshal(value)
	empty, _ := json.Marshal(contract4competios.ClosurePlanReceipt{})
	return string(encoded) == string(empty)
}

func checkPopulatedCandidateReplayAuthority(provider contract4competios.SourceArtifactProvider, request contract4competios.CandidateClosureRetentionRequest, transfer contract4competios.CandidateClosureTransfer, original contract4competios.ArtifactRetentionReceipt, crossPurpose contract4competios.OperationGrant) []error {
	var violations []error
	for name, mutate := range sourceReplayGrantMutationsForPurpose(contract4competios.GrantPurposeCandidateValidateRetain) {
		bad, _ := sourceCandidateGrantFixture(request, transfer, "populated-candidate-bad-"+name, "key-a")
		mutate(&bad.Claims)
		receipt, err := provider.ValidateAndRetainCandidate(context.Background(), bad, request, transfer)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || receipt != (contract4competios.ArtifactRetentionReceipt{}) {
			violations = append(violations, fmt.Errorf("populated candidate %s grant = %+v: %v, want empty receipt and ErrInvalidGrant", name, receipt, err))
		}
		valid, _ := sourceCandidateGrantFixture(request, transfer, "populated-candidate-retry-"+name, "key-rotated")
		replayed, replayErr := provider.ValidateAndRetainCandidate(context.Background(), valid, request, transfer)
		if replayErr != nil || replayed.Status != contract4competios.ArtifactRetentionReplayed || !sameRetentionReceiptEvidence(original, replayed) {
			violations = append(violations, fmt.Errorf("populated candidate %s valid retry = %+v: %v", name, replayed, replayErr))
		}
	}
	violations = append(violations, checkCandidateCrossPurposeReplay(provider, request, transfer, original, crossPurpose)...)
	return violations
}

func checkPopulatedDisclosureReplayAuthority(provider contract4competios.SourceArtifactProvider, request contract4competios.ArtifactDisclosureVerificationRequest, transfer contract4competios.CandidateClosureTransfer, original contract4competios.ArtifactDisclosureVerificationReceipt, crossPurpose contract4competios.OperationGrant) []error {
	var violations []error
	for name, mutate := range sourceReplayGrantMutationsForPurpose(contract4competios.GrantPurposeArtifactDisclosureVerify) {
		bad, _ := sourceDisclosureGrantFixture(request, transfer, "populated-disclosure-bad-"+name, "key-a")
		mutate(&bad.Claims)
		receipt, err := provider.VerifyArtifactDisclosure(context.Background(), bad, request, transfer)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || receipt != (contract4competios.ArtifactDisclosureVerificationReceipt{}) {
			violations = append(violations, fmt.Errorf("populated disclosure %s grant = %+v: %v, want empty receipt and ErrInvalidGrant", name, receipt, err))
		}
		valid, _ := sourceDisclosureGrantFixture(request, transfer, "populated-disclosure-retry-"+name, "key-rotated")
		replayed, replayErr := provider.VerifyArtifactDisclosure(context.Background(), valid, request, transfer)
		if replayErr != nil || replayed != original {
			violations = append(violations, fmt.Errorf("populated disclosure %s valid retry = %+v: %v", name, replayed, replayErr))
		}
	}
	violations = append(violations, checkDisclosureCrossPurposeReplay(provider, request, transfer, original, crossPurpose)...)
	return violations
}

func checkPopulatedPublicationReplayAuthority(provider contract4competios.SourceArtifactProvider, request contract4competios.ArtifactPublicationRequest, original contract4competios.ArtifactPublicationReceipt, crossPurpose contract4competios.OperationGrant) []error {
	var violations []error
	for name, mutate := range sourceReplayGrantMutationsForPurpose(contract4competios.GrantPurposeArtifactPublish) {
		bad, _ := sourcePublicationGrantFixture(request, "populated-publication-bad-"+name, "key-a")
		mutate(&bad.Claims)
		receipt, err := provider.PublishArtifact(context.Background(), bad, request)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || receipt != (contract4competios.ArtifactPublicationReceipt{}) {
			violations = append(violations, fmt.Errorf("populated publication %s grant = %+v: %v, want empty receipt and ErrInvalidGrant", name, receipt, err))
		}
		valid, _ := sourcePublicationGrantFixture(request, "populated-publication-retry-"+name, "key-rotated")
		replayed, replayErr := provider.PublishArtifact(context.Background(), valid, request)
		if replayErr != nil || replayed.Status != contract4competios.ArtifactPublicationReplayed || !samePublicationReceiptEvidence(original, replayed) {
			violations = append(violations, fmt.Errorf("populated publication %s valid retry = %+v: %v", name, replayed, replayErr))
		}
	}
	violations = append(violations, checkPublicationCrossPurposeReplay(provider, request, original, crossPurpose)...)
	return violations
}

func sourceReplayGrantMutationsForPurpose(purpose contract4competios.GrantPurpose) map[string]func(*contract4competios.OperationGrant) {
	mutations := map[string]func(*contract4competios.OperationGrant){
		"issuer":       func(value *contract4competios.OperationGrant) { value.Issuer = "other" },
		"subject":      func(value *contract4competios.OperationGrant) { value.Subject = "other" },
		"audience":     func(value *contract4competios.OperationGrant) { value.Audience = "other" },
		"token type":   func(value *contract4competios.OperationGrant) { value.TokenType = "other" },
		"scope":        func(value *contract4competios.OperationGrant) { value.Scope = "other" },
		"purpose":      func(value *contract4competios.OperationGrant) { value.Purpose = "other" },
		"key":          func(value *contract4competios.OperationGrant) { value.KeyID = "" },
		"token ID":     func(value *contract4competios.OperationGrant) { value.TokenID = "" },
		"issued time":  func(value *contract4competios.OperationGrant) { value.IssuedAt = value.NotBefore.Add(time.Second) },
		"not before":   func(value *contract4competios.OperationGrant) { value.NotBefore = value.IssuedAt.Add(time.Hour) },
		"expiry":       func(value *contract4competios.OperationGrant) { value.ExpiresAt = value.NotBefore },
		"provider":     func(value *contract4competios.OperationGrant) { value.ProviderID = "other" },
		"adapter":      func(value *contract4competios.OperationGrant) { value.AdapterID = "other" },
		"command":      func(value *contract4competios.OperationGrant) { value.CommandID = "other" },
		"typed digest": func(value *contract4competios.OperationGrant) { value.TypedPayloadDigest = payloadDigest("9") },
		"content type": func(value *contract4competios.OperationGrant) { value.TransportContentType = "application/json" },
		"raw digest":   func(value *contract4competios.OperationGrant) { value.RawTransportDigest = payloadDigest("8") },
		"method":       func(value *contract4competios.OperationGrant) { value.Method = "PUT" },
		"resource":     func(value *contract4competios.OperationGrant) { value.Resource = "/other" },
	}
	switch purpose {
	case contract4competios.GrantPurposeManifestClosurePlan:
		mutations["participant"] = func(value *contract4competios.OperationGrant) { value.ParticipantID = "other" }
		mutations["participant version"] = func(value *contract4competios.OperationGrant) { value.ParticipantVersionID = "other" }
		mutations["repository"] = func(value *contract4competios.OperationGrant) { value.RepositoryNodeID = "other" }
		mutations["commit"] = func(value *contract4competios.OperationGrant) {
			value.CommitOID = "sha1:1123456789abcdef0123456789abcdef01234567"
		}
		mutations["manifest path"] = func(value *contract4competios.OperationGrant) { value.ManifestPath = "other/manifest.json" }
		mutations["manifest kind"] = func(value *contract4competios.OperationGrant) {
			value.ManifestEntryKind = contract4competios.SourceEntrySymlink
		}
		mutations["manifest digest"] = func(value *contract4competios.OperationGrant) { value.RawManifestBytesDigest = artifactDigest("7") }
		mutations["manifest limit"] = func(value *contract4competios.OperationGrant) { value.ManifestByteLimit++ }
	case contract4competios.GrantPurposeCandidateValidateRetain:
		mutations["participant"] = func(value *contract4competios.OperationGrant) { value.ParticipantID = "other" }
		mutations["participant version"] = func(value *contract4competios.OperationGrant) { value.ParticipantVersionID = "other" }
		mutations["repository"] = func(value *contract4competios.OperationGrant) { value.RepositoryNodeID = "other" }
		mutations["commit"] = func(value *contract4competios.OperationGrant) {
			value.CommitOID = "sha1:1123456789abcdef0123456789abcdef01234567"
		}
		mutations["plan ID"] = func(value *contract4competios.OperationGrant) { value.ClosurePlanID = "other" }
		mutations["plan digest"] = func(value *contract4competios.OperationGrant) { value.ClosurePlanDigest = payloadDigest("6") }
		mutations["candidate digest"] = func(value *contract4competios.OperationGrant) {
			value.CandidateTransferredBytesDigest = artifactDigest("5")
		}
		mutations["aggregate limit"] = func(value *contract4competios.OperationGrant) { value.AggregateByteLimit++ }
	case contract4competios.GrantPurposeArtifactDisclosureVerify:
		mutations["participant"] = func(value *contract4competios.OperationGrant) { value.ParticipantID = "other" }
		mutations["participant version"] = func(value *contract4competios.OperationGrant) { value.ParticipantVersionID = "other" }
		mutations["repository"] = func(value *contract4competios.OperationGrant) { value.RepositoryNodeID = "other" }
		mutations["commit"] = func(value *contract4competios.OperationGrant) {
			value.CommitOID = "sha1:1123456789abcdef0123456789abcdef01234567"
		}
		mutations["plan ID"] = func(value *contract4competios.OperationGrant) { value.ClosurePlanID = "other" }
		mutations["plan digest"] = func(value *contract4competios.OperationGrant) { value.ClosurePlanDigest = payloadDigest("6") }
		mutations["public candidate digest"] = func(value *contract4competios.OperationGrant) {
			value.PublicCandidateTransferredBytesDigest = artifactDigest("4")
		}
		mutations["aggregate limit"] = func(value *contract4competios.OperationGrant) { value.AggregateByteLimit++ }
		mutations["retention receipt"] = func(value *contract4competios.OperationGrant) { value.RetentionReceiptID = "other" }
		mutations["artifact digest"] = func(value *contract4competios.OperationGrant) { value.ArtifactDigest = artifactDigest("3") }
	case contract4competios.GrantPurposeArtifactPublish:
		mutations["participant"] = func(value *contract4competios.OperationGrant) { value.ParticipantID = "other" }
		mutations["participant version"] = func(value *contract4competios.OperationGrant) { value.ParticipantVersionID = "other" }
		mutations["retention receipt"] = func(value *contract4competios.OperationGrant) { value.RetentionReceiptID = "other" }
		mutations["artifact digest"] = func(value *contract4competios.OperationGrant) { value.ArtifactDigest = artifactDigest("3") }
		mutations["disclosure receipt"] = func(value *contract4competios.OperationGrant) { value.DisclosureReceiptID = "other" }
		mutations["disclosure request"] = func(value *contract4competios.OperationGrant) { value.DisclosureRequestDigest = payloadDigest("2") }
	}
	return mutations
}

func checkManifestCrossPurposeReplay(provider contract4competios.SourceArtifactProvider, request contract4competios.ManifestClosurePlanRequest, manifestBytes []byte, original contract4competios.ClosurePlanReceipt, claims contract4competios.OperationGrant) []error {
	target, _ := sourceManifestGrantFixture(request, manifestBytes, "populated-manifest-target", "key-a")
	var violations []error
	for _, probe := range crossPurposeReplayProbes(target.Claims, claims) {
		receipt, err := provider.PlanManifestClosure(context.Background(), probe.grant, request, manifestBytes)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || !emptyClosurePlanReceipt(receipt) {
			violations = append(violations, fmt.Errorf("populated manifest %s cross-purpose grant = %+v: %v", probe.name, receipt, err))
		}
		valid, _ := sourceManifestGrantFixture(request, manifestBytes, "populated-manifest-"+probe.name+"-retry", "key-rotated")
		replayed, replayErr := provider.PlanManifestClosure(context.Background(), valid, request, manifestBytes)
		if replayErr != nil || replayed.Status != contract4competios.ClosurePlanReceiptReplayed || !sameClosurePlanReceiptEvidence(original, replayed) {
			violations = append(violations, fmt.Errorf("populated manifest valid retry after %s cross-purpose grant = %+v: %v", probe.name, replayed, replayErr))
		}
	}
	return violations
}

func checkCandidateCrossPurposeReplay(provider contract4competios.SourceArtifactProvider, request contract4competios.CandidateClosureRetentionRequest, transfer contract4competios.CandidateClosureTransfer, original contract4competios.ArtifactRetentionReceipt, claims contract4competios.OperationGrant) []error {
	target, _ := sourceCandidateGrantFixture(request, transfer, "populated-candidate-target", "key-a")
	var violations []error
	for _, probe := range crossPurposeReplayProbes(target.Claims, claims) {
		receipt, err := provider.ValidateAndRetainCandidate(context.Background(), probe.grant, request, transfer)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || receipt != (contract4competios.ArtifactRetentionReceipt{}) {
			violations = append(violations, fmt.Errorf("populated candidate %s cross-purpose grant = %+v: %v", probe.name, receipt, err))
		}
		valid, _ := sourceCandidateGrantFixture(request, transfer, "populated-candidate-"+probe.name+"-retry", "key-rotated")
		replayed, replayErr := provider.ValidateAndRetainCandidate(context.Background(), valid, request, transfer)
		if replayErr != nil || replayed.Status != contract4competios.ArtifactRetentionReplayed || !sameRetentionReceiptEvidence(original, replayed) {
			violations = append(violations, fmt.Errorf("populated candidate valid retry after %s cross-purpose grant = %+v: %v", probe.name, replayed, replayErr))
		}
	}
	return violations
}

func checkDisclosureCrossPurposeReplay(provider contract4competios.SourceArtifactProvider, request contract4competios.ArtifactDisclosureVerificationRequest, transfer contract4competios.CandidateClosureTransfer, original contract4competios.ArtifactDisclosureVerificationReceipt, claims contract4competios.OperationGrant) []error {
	target, _ := sourceDisclosureGrantFixture(request, transfer, "populated-disclosure-target", "key-a")
	var violations []error
	for _, probe := range crossPurposeReplayProbes(target.Claims, claims) {
		receipt, err := provider.VerifyArtifactDisclosure(context.Background(), probe.grant, request, transfer)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || receipt != (contract4competios.ArtifactDisclosureVerificationReceipt{}) {
			violations = append(violations, fmt.Errorf("populated disclosure %s cross-purpose grant = %+v: %v", probe.name, receipt, err))
		}
		valid, _ := sourceDisclosureGrantFixture(request, transfer, "populated-disclosure-"+probe.name+"-retry", "key-rotated")
		replayed, replayErr := provider.VerifyArtifactDisclosure(context.Background(), valid, request, transfer)
		if replayErr != nil || replayed != original {
			violations = append(violations, fmt.Errorf("populated disclosure valid retry after %s cross-purpose grant = %+v: %v", probe.name, replayed, replayErr))
		}
	}
	return violations
}

func checkPublicationCrossPurposeReplay(provider contract4competios.SourceArtifactProvider, request contract4competios.ArtifactPublicationRequest, original contract4competios.ArtifactPublicationReceipt, claims contract4competios.OperationGrant) []error {
	target, _ := sourcePublicationGrantFixture(request, "populated-publication-target", "key-a")
	var violations []error
	for _, probe := range crossPurposeReplayProbes(target.Claims, claims) {
		receipt, err := provider.PublishArtifact(context.Background(), probe.grant, request)
		if !errors.Is(err, contract4competios.ErrInvalidGrant) || receipt != (contract4competios.ArtifactPublicationReceipt{}) {
			violations = append(violations, fmt.Errorf("populated publication %s cross-purpose grant = %+v: %v", probe.name, receipt, err))
		}
		valid, _ := sourcePublicationGrantFixture(request, "populated-publication-"+probe.name+"-retry", "key-rotated")
		replayed, replayErr := provider.PublishArtifact(context.Background(), valid, request)
		if replayErr != nil || replayed.Status != contract4competios.ArtifactPublicationReplayed || !samePublicationReceiptEvidence(original, replayed) {
			violations = append(violations, fmt.Errorf("populated publication valid retry after %s cross-purpose grant = %+v: %v", probe.name, replayed, replayErr))
		}
	}
	return violations
}

type crossPurposeReplayProbe struct {
	name  string
	grant contract4competios.VerifiedOperationGrant
}

// crossPurposeReplayProbes covers both useful representations of an operation
// crossing. purpose-only changes purpose/scope on the target grant (and uses a
// fresh token identity); closed discriminators can make that shape invalid.
// foreign-valid keeps a complete valid foreign-purpose claim set but rebinds
// every stable target request/body/route fact that is shared by both purposes.
// Purpose-specific discriminator fields intentionally remain foreign. Both
// probes must fail the target operation binder before command replay.
func crossPurposeReplayProbes(target, foreign contract4competios.OperationGrant) []crossPurposeReplayProbe {
	purposeOnly := target
	purposeOnly.Purpose, purposeOnly.Scope = foreign.Purpose, foreign.Scope
	purposeOnly.TokenID = "purpose-only-crossing"

	foreignValid := foreign
	foreignValid.Issuer, foreignValid.Subject, foreignValid.Audience = target.Issuer, target.Subject, target.Audience
	foreignValid.TokenType, foreignValid.KeyID = target.TokenType, target.KeyID
	foreignValid.TokenID = "foreign-valid-crossing"
	foreignValid.IssuedAt, foreignValid.NotBefore, foreignValid.ExpiresAt = target.IssuedAt, target.NotBefore, target.ExpiresAt
	foreignValid.ProviderID, foreignValid.AdapterID = target.ProviderID, target.AdapterID
	foreignValid.CompetitionID, foreignValid.ContestID, foreignValid.RequestID = target.CompetitionID, target.ContestID, target.RequestID
	foreignValid.CommandID, foreignValid.TypedPayloadDigest = target.CommandID, target.TypedPayloadDigest
	foreignValid.TransportContentType, foreignValid.RawTransportDigest = target.TransportContentType, target.RawTransportDigest
	foreignValid.Method, foreignValid.Resource = target.Method, target.Resource
	if contract4competios.ValidateOperationGrant(foreignValid) != nil || foreignValid.Purpose == target.Purpose || foreignValid.ProviderID != target.ProviderID || foreignValid.AdapterID != target.AdapterID || foreignValid.CompetitionID != target.CompetitionID || foreignValid.ContestID != target.ContestID || foreignValid.RequestID != target.RequestID || foreignValid.CommandID != target.CommandID || foreignValid.TypedPayloadDigest != target.TypedPayloadDigest || foreignValid.RawTransportDigest != target.RawTransportDigest || foreignValid.TransportContentType != target.TransportContentType || foreignValid.Method != target.Method || foreignValid.Resource != target.Resource {
		panic("invalid cross-purpose replay fixture")
	}
	return []crossPurposeReplayProbe{
		{name: "purpose-only", grant: contract4competios.VerifiedOperationGrant{Claims: purposeOnly}},
		{name: "foreign-valid", grant: contract4competios.VerifiedOperationGrant{Claims: foreignValid}},
	}
}

func sameClosurePlanReceiptEvidence(first, replay contract4competios.ClosurePlanReceipt) bool {
	first.Status, replay.Status = "", ""
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replay)
	return string(firstJSON) == string(replayJSON)
}

func sameRetentionReceiptEvidence(first, replay contract4competios.ArtifactRetentionReceipt) bool {
	first.Status, replay.Status = "", ""
	return first == replay
}

func samePublicationReceiptEvidence(first, replay contract4competios.ArtifactPublicationReceipt) bool {
	first.Status, replay.Status = "", ""
	return first == replay
}

type sourceLedgerPrerequisites struct {
	manifestBytes []byte
	manifest      contract4competios.ManifestClosurePlanRequest
	candidatePlan contract4competios.ClosurePlan
	retentionPlan contract4competios.ClosurePlan
	transfer      contract4competios.CandidateClosureTransfer
	retention     contract4competios.ArtifactRetentionReceipt
	disclosure    contract4competios.ArtifactDisclosureVerificationRequest
	matched       contract4competios.ArtifactDisclosureVerificationReceipt
}

func checkSourceCommandLedgerPairs(factory SourceArtifactProviderFactory) []error {
	purposes := []contract4competios.GrantPurpose{
		contract4competios.GrantPurposeManifestClosurePlan,
		contract4competios.GrantPurposeCandidateValidateRetain,
		contract4competios.GrantPurposeArtifactDisclosureVerify,
		contract4competios.GrantPurposeArtifactPublish,
	}
	var violations []error
	for _, first := range purposes {
		for _, second := range purposes {
			if first == second {
				continue
			}
			provider := factory()
			prerequisites, err := prepareSourceLedgerPrerequisites(provider)
			if err != nil {
				violations = append(violations, fmt.Errorf("source command ledger %s -> %s setup: %w", first, second, err))
				continue
			}
			const collisionCommand contract4competios.CommandID = "all-purpose-ledger-collision"
			if err := invokeSourceLedgerPurpose(provider, &prerequisites, first, collisionCommand); err != nil {
				violations = append(violations, fmt.Errorf("source command ledger first %s: %w", first, err))
				continue
			}
			if err := invokeSourceLedgerPurpose(provider, &prerequisites, second, collisionCommand); !errors.Is(err, contract4competios.ErrCommandConflict) {
				violations = append(violations, fmt.Errorf("source command ledger %s -> %s error = %v", first, second, err))
			}
		}
	}
	return violations
}

func prepareSourceLedgerPrerequisites(provider contract4competios.SourceArtifactProvider) (sourceLedgerPrerequisites, error) {
	ctx := context.Background()
	manifestBytes := sourceManifestBytesFixture()
	manifestPayload := sourceManifestRequestFixture(manifestBytes).Payload()
	manifestPayload.CommandID = "ledger-setup-manifest"
	manifest, err := contract4competios.NewManifestClosurePlanRequest(manifestPayload)
	if err != nil {
		return sourceLedgerPrerequisites{}, err
	}
	manifestGrant, _ := sourceManifestGrantFixture(manifest, manifestBytes, "ledger-setup-manifest-token", "key-a")
	planReceipt, err := provider.PlanManifestClosure(ctx, manifestGrant, manifest, manifestBytes)
	if err != nil || contract4competios.ValidateClosurePlanReceiptForRequest(planReceipt, manifest) != nil {
		return sourceLedgerPrerequisites{}, fmt.Errorf("manifest prerequisite: %w", err)
	}
	transfer := sourceCandidateTransferFixture()
	candidate := sourceCandidateRequestFixture(planReceipt.Plan, transfer, "ledger-setup-candidate")
	candidateGrant, _ := sourceCandidateGrantFixture(candidate, transfer, "ledger-setup-candidate-token", "key-a")
	retention, err := provider.ValidateAndRetainCandidate(ctx, candidateGrant, candidate, transfer)
	if err != nil || contract4competios.ValidateArtifactRetentionReceiptForRequest(retention, candidate) != nil {
		return sourceLedgerPrerequisites{}, fmt.Errorf("retention prerequisite: %w", err)
	}
	disclosure := sourceDisclosureRequestFixture(planReceipt.Plan, retention, transfer, "ledger-setup-disclosure")
	disclosureGrant, _ := sourceDisclosureGrantFixture(disclosure, transfer, "ledger-setup-disclosure-token", "key-a")
	matched, err := provider.VerifyArtifactDisclosure(ctx, disclosureGrant, disclosure, transfer)
	if err != nil || contract4competios.ValidateArtifactDisclosureVerificationReceiptForRequest(matched, disclosure) != nil || matched.Verdict != contract4competios.ArtifactDisclosureMatched {
		return sourceLedgerPrerequisites{}, fmt.Errorf("disclosure prerequisite: %w", err)
	}
	return sourceLedgerPrerequisites{
		manifestBytes: manifestBytes, manifest: manifest, candidatePlan: planReceipt.Plan,
		retentionPlan: planReceipt.Plan, transfer: transfer, retention: retention,
		disclosure: disclosure, matched: matched,
	}, nil
}

func invokeSourceLedgerPurpose(provider contract4competios.SourceArtifactProvider, prerequisites *sourceLedgerPrerequisites, purpose contract4competios.GrantPurpose, command contract4competios.CommandID) error {
	ctx := context.Background()
	switch purpose {
	case contract4competios.GrantPurposeManifestClosurePlan:
		payload := prerequisites.manifest.Payload()
		payload.CommandID = command
		request, err := contract4competios.NewManifestClosurePlanRequest(payload)
		if err != nil {
			return err
		}
		grant, _ := sourceManifestGrantFixture(request, prerequisites.manifestBytes, "ledger-manifest-token", "key-a")
		receipt, err := provider.PlanManifestClosure(ctx, grant, request, prerequisites.manifestBytes)
		if err == nil {
			prerequisites.candidatePlan = receipt.Plan
		}
		return err
	case contract4competios.GrantPurposeCandidateValidateRetain:
		request := sourceCandidateRequestFixture(prerequisites.candidatePlan, prerequisites.transfer, command)
		grant, _ := sourceCandidateGrantFixture(request, prerequisites.transfer, "ledger-candidate-token", "key-a")
		_, err := provider.ValidateAndRetainCandidate(ctx, grant, request, prerequisites.transfer)
		return err
	case contract4competios.GrantPurposeArtifactDisclosureVerify:
		request := sourceDisclosureRequestFixture(prerequisites.retentionPlan, prerequisites.retention, prerequisites.transfer, command)
		grant, _ := sourceDisclosureGrantFixture(request, prerequisites.transfer, "ledger-disclosure-token", "key-a")
		_, err := provider.VerifyArtifactDisclosure(ctx, grant, request, prerequisites.transfer)
		return err
	case contract4competios.GrantPurposeArtifactPublish:
		request := sourcePublicationRequestFixture(prerequisites.retention, prerequisites.disclosure, prerequisites.matched, command)
		grant, _ := sourcePublicationGrantFixture(request, "ledger-publication-token", "key-a")
		_, err := provider.PublishArtifact(ctx, grant, request)
		return err
	default:
		return fmt.Errorf("unsupported source purpose %q", purpose)
	}
}

func sourceManifestBytesFixture() []byte {
	return []byte(`{"entry":"bots/bot.star","support":["bots/opening.json"]}`)
}

func sourceManifestRequestFixture(manifestBytes []byte) contract4competios.ManifestClosurePlanRequest {
	request, err := contract4competios.NewManifestClosurePlanRequest(contract4competios.ManifestClosurePlanRequestPayload{
		ProviderID: "provider", AdapterID: "adapter", CommandID: "manifest-command",
		ParticipantID: "participant-a", ParticipantVersionID: "version-a",
		RepositoryNodeID: "repository-node", CommitOID: "sha1:0123456789abcdef0123456789abcdef01234567",
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
		RepositoryNodeID: request.RepositoryNodeID, CommitOID: request.CommitOID, ManifestPath: request.ManifestPath,
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
		RepositoryNodeID: plan.RepositoryNodeID, CommitOID: plan.CommitOID,
		ClosurePlanID: plan.ClosurePlanID, ClosurePlanDigest: plan.ClosurePlanDigest,
		CandidateTransferredBytesDigest: digest, AggregateByteLimit: plan.AggregateByteLimit,
	})
	if err != nil {
		panic(err)
	}
	return request
}

func sourcePublicationRequestFixture(retention contract4competios.ArtifactRetentionReceipt, disclosure contract4competios.ArtifactDisclosureVerificationRequest, receipt contract4competios.ArtifactDisclosureVerificationReceipt, command contract4competios.CommandID) contract4competios.ArtifactPublicationRequest {
	return sourcePublicationRequestWithBinding(retention, receipt.ReceiptID, disclosure.TypedPayloadDigest, command)
}

func sourcePublicationRequestWithBinding(retention contract4competios.ArtifactRetentionReceipt, disclosureReceiptID contract4competios.ArtifactDisclosureVerificationReceiptID, disclosureRequestDigest contract4competios.PayloadDigest, command contract4competios.CommandID) contract4competios.ArtifactPublicationRequest {
	request, err := contract4competios.NewArtifactPublicationRequest(contract4competios.ArtifactPublicationRequestPayload{
		ProviderID: retention.ProviderID, AdapterID: retention.AdapterID, CommandID: command,
		ParticipantID: retention.ParticipantID, ParticipantVersionID: retention.ParticipantVersionID,
		RetentionReceiptID: retention.ReceiptID, ArtifactDigest: retention.ArtifactDigest,
		DisclosureReceiptID: disclosureReceiptID, DisclosureRequestDigest: disclosureRequestDigest,
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
		RepositoryNodeID: plan.RepositoryNodeID, CommitOID: plan.CommitOID,
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
	grant.RepositoryNodeID, grant.CommitOID, grant.ManifestPath = request.RepositoryNodeID, request.CommitOID, request.ManifestPath
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
	grant.RepositoryNodeID, grant.CommitOID = request.RepositoryNodeID, request.CommitOID
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
	grant.DisclosureReceiptID, grant.DisclosureRequestDigest = request.DisclosureReceiptID, request.DisclosureRequestDigest
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
	grant.RepositoryNodeID, grant.CommitOID = request.RepositoryNodeID, request.CommitOID
	grant.ClosurePlanID, grant.ClosurePlanDigest = request.ClosurePlanID, request.ClosurePlanDigest
	grant.PublicCandidateTransferredBytesDigest, grant.AggregateByteLimit = request.PublicCandidateTransferredBytesDigest, request.AggregateByteLimit
	grant.RetentionReceiptID, grant.ArtifactDigest = request.RetentionReceiptID, request.ArtifactDigest
	return contract4competios.VerifiedOperationGrant{Claims: grant}, routeForSourceGrant(grant)
}
