package contract4competiostest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type sourceCommandRecord struct {
	purpose contract4competios.GrantPurpose
	digest  contract4competios.PayloadDigest
}

type disclosureRecord struct {
	request contract4competios.ArtifactDisclosureVerificationRequest
	receipt contract4competios.ArtifactDisclosureVerificationReceipt
}

type retainedRecord struct {
	receipt        contract4competios.ArtifactRetentionReceipt
	transferDigest contract4competios.ArtifactDigest
	plan           contract4competios.ClosurePlan
}

type referenceSourceProvider struct {
	plansByID     map[contract4competios.ClosurePlanID]contract4competios.ClosurePlan
	planCommands  map[contract4competios.CommandID]contract4competios.ClosurePlanReceipt
	commands      map[contract4competios.CommandID]sourceCommandRecord
	candidates    map[contract4competios.CommandID]contract4competios.ArtifactRetentionReceipt
	retained      map[contract4competios.ArtifactRetentionReceiptID]retainedRecord
	publications  map[contract4competios.CommandID]contract4competios.ArtifactPublicationReceipt
	disclosures   map[contract4competios.CommandID]disclosureRecord
	disclosureIDs map[contract4competios.ArtifactDisclosureVerificationReceiptID]disclosureRecord
	// allowManifestToDisclosureCollision deliberately models a partial-ledger
	// defect so the all-pairs conformance proof has a focused negative fake.
	allowManifestToDisclosureCollision bool
}

func newReferenceSourceProvider() *referenceSourceProvider {
	return &referenceSourceProvider{
		plansByID:     map[contract4competios.ClosurePlanID]contract4competios.ClosurePlan{},
		planCommands:  map[contract4competios.CommandID]contract4competios.ClosurePlanReceipt{},
		commands:      map[contract4competios.CommandID]sourceCommandRecord{},
		candidates:    map[contract4competios.CommandID]contract4competios.ArtifactRetentionReceipt{},
		retained:      map[contract4competios.ArtifactRetentionReceiptID]retainedRecord{},
		publications:  map[contract4competios.CommandID]contract4competios.ArtifactPublicationReceipt{},
		disclosures:   map[contract4competios.CommandID]disclosureRecord{},
		disclosureIDs: map[contract4competios.ArtifactDisclosureVerificationReceiptID]disclosureRecord{},
	}
}

func (p *referenceSourceProvider) claimCommand(commandID contract4competios.CommandID, purpose contract4competios.GrantPurpose, digest contract4competios.PayloadDigest) (bool, error) {
	if prior, exists := p.commands[commandID]; exists {
		if p.allowManifestToDisclosureCollision && prior.purpose == contract4competios.GrantPurposeManifestClosurePlan && purpose == contract4competios.GrantPurposeArtifactDisclosureVerify {
			p.commands[commandID] = sourceCommandRecord{purpose: purpose, digest: digest}
			return false, nil
		}
		if prior.purpose != purpose || prior.digest != digest {
			return false, contract4competios.ErrCommandConflict
		}
		return true, nil
	}
	p.commands[commandID] = sourceCommandRecord{purpose: purpose, digest: digest}
	return false, nil
}

func (p *referenceSourceProvider) PlanManifestClosure(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ManifestClosurePlanRequest, manifestBytes []byte) (contract4competios.ClosurePlanReceipt, error) {
	_, route := sourceManifestGrantFixture(request, manifestBytes, "expected", "expected")
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateManifestClosurePlanGrantForRequest(grant, route, request) != nil || contract4competios.ValidateManifestClosurePlanInput(request, manifestBytes) != nil {
		return contract4competios.ClosurePlanReceipt{}, contract4competios.ErrInvalidGrant
	}
	replayed, err := p.claimCommand(request.CommandID, contract4competios.GrantPurposeManifestClosurePlan, request.TypedPayloadDigest)
	if err != nil {
		return contract4competios.ClosurePlanReceipt{}, err
	}
	if replayed {
		prior := p.planCommands[request.CommandID]
		prior.Status = contract4competios.ClosurePlanReceiptReplayed
		return prior, nil
	}
	plan := sourcePlanFixture(request)
	receipt := contract4competios.ClosurePlanReceipt{
		ProviderID: request.ProviderID, AdapterID: request.AdapterID, CommandID: request.CommandID,
		ParticipantID: request.ParticipantID, ParticipantVersionID: request.ParticipantVersionID,
		RequestPayloadDigest: request.TypedPayloadDigest, Plan: plan,
		Status: contract4competios.ClosurePlanReceiptAccepted,
	}
	p.plansByID[plan.ClosurePlanID], p.planCommands[request.CommandID] = plan, receipt
	return receipt, nil
}

func (p *referenceSourceProvider) ValidateAndRetainCandidate(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.CandidateClosureRetentionRequest, transfer contract4competios.CandidateClosureTransfer) (contract4competios.ArtifactRetentionReceipt, error) {
	_, route := sourceCandidateGrantFixture(request, transfer, "expected", "expected")
	plan, exists := p.plansByID[request.ClosurePlanID]
	if !exists || contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateCandidateRetentionGrantForRequest(grant, route, request) != nil || contract4competios.ValidateCandidateClosureInput(request, plan, transfer) != nil {
		return contract4competios.ArtifactRetentionReceipt{}, contract4competios.ErrInvalidGrant
	}
	replayed, err := p.claimCommand(request.CommandID, contract4competios.GrantPurposeCandidateValidateRetain, request.TypedPayloadDigest)
	if err != nil {
		return contract4competios.ArtifactRetentionReceipt{}, err
	}
	if replayed {
		receipt := p.candidates[request.CommandID]
		receipt.Status = contract4competios.ArtifactRetentionReplayed
		return receipt, nil
	}
	transferDigest, _ := contract4competios.DigestCandidateTransferredFiles(transfer.Files)
	receipt := contract4competios.ArtifactRetentionReceipt{
		ReceiptID:  contract4competios.ArtifactRetentionReceiptID("retention-" + request.ParticipantVersionID),
		ProviderID: request.ProviderID, AdapterID: request.AdapterID, CommandID: request.CommandID,
		ParticipantID: request.ParticipantID, ParticipantVersionID: request.ParticipantVersionID,
		ClosurePlanID: request.ClosurePlanID, ClosurePlanDigest: request.ClosurePlanDigest,
		CandidateRequestDigest: request.TypedPayloadDigest, ArtifactDigest: artifactDigest("9"),
		Status: contract4competios.ArtifactRetentionAccepted,
	}
	p.candidates[request.CommandID] = receipt
	p.retained[receipt.ReceiptID] = retainedRecord{receipt: receipt, transferDigest: transferDigest, plan: plan}
	return receipt, nil
}

func (p *referenceSourceProvider) PublishArtifact(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactPublicationRequest) (contract4competios.ArtifactPublicationReceipt, error) {
	_, route := sourcePublicationGrantFixture(request, "expected", "expected")
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateArtifactPublicationGrantForRequest(grant, route, request) != nil {
		return contract4competios.ArtifactPublicationReceipt{}, contract4competios.ErrInvalidGrant
	}
	if _, exists := p.commands[request.CommandID]; exists {
		replayed, err := p.claimCommand(request.CommandID, contract4competios.GrantPurposeArtifactPublish, request.TypedPayloadDigest)
		if err != nil {
			return contract4competios.ArtifactPublicationReceipt{}, err
		}
		if replayed {
			prior := p.publications[request.CommandID]
			prior.Status = contract4competios.ArtifactPublicationReplayed
			return prior, nil
		}
	}
	retained, exists := p.retained[request.RetentionReceiptID]
	disclosed, disclosedExists := p.disclosureIDs[request.DisclosureReceiptID]
	if !exists || !disclosedExists || contract4competios.ValidateArtifactPublicationPrerequisites(request, retained.receipt, disclosed.request, disclosed.receipt) != nil {
		return contract4competios.ArtifactPublicationReceipt{}, contract4competios.ErrInvalidGrant
	}
	replayed, err := p.claimCommand(request.CommandID, contract4competios.GrantPurposeArtifactPublish, request.TypedPayloadDigest)
	if err != nil {
		return contract4competios.ArtifactPublicationReceipt{}, err
	}
	if replayed {
		panic("unreachable publication replay without a stored command")
	}
	receipt := contract4competios.ArtifactPublicationReceipt{
		ReceiptID: "publication-1", ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		CommandID: request.CommandID, ParticipantID: request.ParticipantID,
		ParticipantVersionID: request.ParticipantVersionID, RetentionReceiptID: request.RetentionReceiptID,
		DisclosureReceiptID: request.DisclosureReceiptID, DisclosureRequestDigest: request.DisclosureRequestDigest,
		PublicationRequestDigest: request.TypedPayloadDigest, ArtifactDigest: request.ArtifactDigest,
		PublishedAt: fixtureTime.Add(10 * time.Minute), PublicReference: "https://game.example/public/artifact-1",
		Status: contract4competios.ArtifactPublicationAccepted,
	}
	p.publications[request.CommandID] = receipt
	return receipt, nil
}

func (p *referenceSourceProvider) VerifyArtifactDisclosure(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactDisclosureVerificationRequest, transfer contract4competios.CandidateClosureTransfer) (contract4competios.ArtifactDisclosureVerificationReceipt, error) {
	_, route := sourceDisclosureGrantFixture(request, transfer, "expected", "expected")
	retained, exists := p.retained[request.RetentionReceiptID]
	if !exists || contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateArtifactDisclosureGrantForRequest(grant, route, request) != nil || contract4competios.ValidateArtifactDisclosureInput(request, retained.plan, transfer) != nil || retained.receipt.ArtifactDigest != request.ArtifactDigest {
		return contract4competios.ArtifactDisclosureVerificationReceipt{}, contract4competios.ErrInvalidGrant
	}
	replayed, err := p.claimCommand(request.CommandID, contract4competios.GrantPurposeArtifactDisclosureVerify, request.TypedPayloadDigest)
	if err != nil {
		return contract4competios.ArtifactDisclosureVerificationReceipt{}, err
	}
	if replayed {
		return p.disclosures[request.CommandID].receipt, nil
	}
	publicDigest, _ := contract4competios.DigestCandidateTransferredFiles(transfer.Files)
	verdict := contract4competios.ArtifactDisclosureMismatched
	if publicDigest == retained.transferDigest {
		verdict = contract4competios.ArtifactDisclosureMatched
	}
	receipt := contract4competios.ArtifactDisclosureVerificationReceipt{
		ReceiptID:  "disclosure-" + contract4competios.ArtifactDisclosureVerificationReceiptID(request.CommandID),
		ProviderID: request.ProviderID, AdapterID: request.AdapterID, CommandID: request.CommandID,
		ParticipantID: request.ParticipantID, ParticipantVersionID: request.ParticipantVersionID,
		RetentionReceiptID: request.RetentionReceiptID, ArtifactDigest: request.ArtifactDigest,
		VerificationRequestDigest: request.TypedPayloadDigest, Verdict: verdict,
		VerifiedAt: fixtureTime.Add(11 * time.Minute),
	}
	record := disclosureRecord{request: request, receipt: receipt}
	p.disclosures[request.CommandID] = record
	p.disclosureIDs[receipt.ReceiptID] = record
	return receipt, nil
}

type unsafeSourceProvider struct{}

func (unsafeSourceProvider) PlanManifestClosure(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.ManifestClosurePlanRequest, _ []byte) (contract4competios.ClosurePlanReceipt, error) {
	return contract4competios.ClosurePlanReceipt{CommandID: request.CommandID, Status: contract4competios.ClosurePlanReceiptAccepted}, nil
}

func (unsafeSourceProvider) ValidateAndRetainCandidate(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.CandidateClosureRetentionRequest, _ contract4competios.CandidateClosureTransfer) (contract4competios.ArtifactRetentionReceipt, error) {
	return contract4competios.ArtifactRetentionReceipt{CommandID: request.CommandID, Status: contract4competios.ArtifactRetentionAccepted}, nil
}

func (unsafeSourceProvider) PublishArtifact(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactPublicationRequest) (contract4competios.ArtifactPublicationReceipt, error) {
	return contract4competios.ArtifactPublicationReceipt{CommandID: request.CommandID, Status: contract4competios.ArtifactPublicationAccepted, PublishedAt: time.Now()}, nil
}

func (unsafeSourceProvider) VerifyArtifactDisclosure(_ context.Context, _ contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactDisclosureVerificationRequest, _ contract4competios.CandidateClosureTransfer) (contract4competios.ArtifactDisclosureVerificationReceipt, error) {
	return contract4competios.ArtifactDisclosureVerificationReceipt{CommandID: request.CommandID, Verdict: contract4competios.ArtifactDisclosureMatched, VerifiedAt: time.Now()}, nil
}

// replayBeforeAuthoritySourceProvider deliberately returns an existing exact
// command receipt before validating its replay grant. New commands delegate to
// the safe reference provider, isolating the ledger/authority ordering defect.
type replayBeforeAuthoritySourceProvider struct{ delegate *referenceSourceProvider }

func (p *replayBeforeAuthoritySourceProvider) PlanManifestClosure(ctx context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ManifestClosurePlanRequest, manifestBytes []byte) (contract4competios.ClosurePlanReceipt, error) {
	if command, exists := p.delegate.commands[request.CommandID]; exists && command.purpose == contract4competios.GrantPurposeManifestClosurePlan && command.digest == request.TypedPayloadDigest {
		replay := p.delegate.planCommands[request.CommandID]
		replay.Status = contract4competios.ClosurePlanReceiptReplayed
		return replay, nil
	}
	return p.delegate.PlanManifestClosure(ctx, grant, request, manifestBytes)
}

func (p *replayBeforeAuthoritySourceProvider) ValidateAndRetainCandidate(ctx context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.CandidateClosureRetentionRequest, transfer contract4competios.CandidateClosureTransfer) (contract4competios.ArtifactRetentionReceipt, error) {
	if command, exists := p.delegate.commands[request.CommandID]; exists && command.purpose == contract4competios.GrantPurposeCandidateValidateRetain && command.digest == request.TypedPayloadDigest {
		replay := p.delegate.candidates[request.CommandID]
		replay.Status = contract4competios.ArtifactRetentionReplayed
		return replay, nil
	}
	return p.delegate.ValidateAndRetainCandidate(ctx, grant, request, transfer)
}

func (p *replayBeforeAuthoritySourceProvider) VerifyArtifactDisclosure(ctx context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactDisclosureVerificationRequest, transfer contract4competios.CandidateClosureTransfer) (contract4competios.ArtifactDisclosureVerificationReceipt, error) {
	if command, exists := p.delegate.commands[request.CommandID]; exists && command.purpose == contract4competios.GrantPurposeArtifactDisclosureVerify && command.digest == request.TypedPayloadDigest {
		return p.delegate.disclosures[request.CommandID].receipt, nil
	}
	return p.delegate.VerifyArtifactDisclosure(ctx, grant, request, transfer)
}

func (p *replayBeforeAuthoritySourceProvider) PublishArtifact(ctx context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactPublicationRequest) (contract4competios.ArtifactPublicationReceipt, error) {
	if command, exists := p.delegate.commands[request.CommandID]; exists && command.purpose == contract4competios.GrantPurposeArtifactPublish && command.digest == request.TypedPayloadDigest {
		replay := p.delegate.publications[request.CommandID]
		replay.Status = contract4competios.ArtifactPublicationReplayed
		return replay, nil
	}
	return p.delegate.PublishArtifact(ctx, grant, request)
}

func TestSourceArtifactProviderConformance(t *testing.T) {
	if violations := CheckSourceArtifactProvider(func() contract4competios.SourceArtifactProvider { return newReferenceSourceProvider() }); len(violations) != 0 {
		t.Fatalf("reference source provider violations: %v", violations)
	}
	if violations := CheckSourceArtifactProvider(func() contract4competios.SourceArtifactProvider { return unsafeSourceProvider{} }); len(violations) < 5 {
		t.Fatalf("unsafe source provider was not decisively rejected: %v", violations)
	}
	if violations := CheckSourceArtifactProvider(func() contract4competios.SourceArtifactProvider {
		provider := newReferenceSourceProvider()
		provider.allowManifestToDisclosureCollision = true
		return provider
	}); len(violations) == 0 {
		t.Fatal("provider with a non-adjacent source command ledger gap unexpectedly passed conformance")
	}
	if violations := CheckSourceArtifactProvider(func() contract4competios.SourceArtifactProvider {
		return &replayBeforeAuthoritySourceProvider{delegate: newReferenceSourceProvider()}
	}); len(violations) == 0 {
		t.Fatal("source provider that replays before authority validation unexpectedly passed conformance")
	}
}

func TestReferenceSourceProviderRejectsGrantPurposeCrossing(t *testing.T) {
	provider := newReferenceSourceProvider()
	manifestBytes := sourceManifestBytesFixture()
	manifest := sourceManifestRequestFixture(manifestBytes)
	candidatePlan := sourcePlanFixture(manifest)
	transfer := sourceCandidateTransferFixture()
	candidate := sourceCandidateRequestFixture(candidatePlan, transfer, "candidate-command")
	manifestGrant, _ := sourceManifestGrantFixture(manifest, manifestBytes, "manifest-token", "key-a")
	if _, err := provider.ValidateAndRetainCandidate(context.Background(), manifestGrant, candidate, transfer); !errors.Is(err, contract4competios.ErrInvalidGrant) {
		t.Fatalf("manifest grant crossed into candidate retention: %v", err)
	}
}
