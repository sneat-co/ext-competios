package contract4competiostest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

type sourceCommandRecord struct {
	digest contract4competios.PayloadDigest
}

type retainedRecord struct {
	receipt        contract4competios.ArtifactRetentionReceipt
	transferDigest contract4competios.ArtifactDigest
	plan           contract4competios.ClosurePlan
}

type referenceSourceProvider struct {
	plansByID    map[contract4competios.ClosurePlanID]contract4competios.ClosurePlan
	planCommands map[contract4competios.CommandID]contract4competios.ClosurePlanReceipt
	candidates   map[contract4competios.CommandID]sourceCommandRecord
	retained     map[contract4competios.ArtifactRetentionReceiptID]retainedRecord
	publications map[contract4competios.CommandID]contract4competios.ArtifactPublicationReceipt
	disclosures  map[contract4competios.CommandID]contract4competios.ArtifactDisclosureVerificationReceipt
}

func newReferenceSourceProvider() *referenceSourceProvider {
	return &referenceSourceProvider{
		plansByID:    map[contract4competios.ClosurePlanID]contract4competios.ClosurePlan{},
		planCommands: map[contract4competios.CommandID]contract4competios.ClosurePlanReceipt{},
		candidates:   map[contract4competios.CommandID]sourceCommandRecord{},
		retained:     map[contract4competios.ArtifactRetentionReceiptID]retainedRecord{},
		publications: map[contract4competios.CommandID]contract4competios.ArtifactPublicationReceipt{},
		disclosures:  map[contract4competios.CommandID]contract4competios.ArtifactDisclosureVerificationReceipt{},
	}
}

func (p *referenceSourceProvider) PlanManifestClosure(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ManifestClosurePlanRequest, manifestBytes []byte) (contract4competios.ClosurePlanReceipt, error) {
	_, route := sourceManifestGrantFixture(request, manifestBytes, "expected", "expected")
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateManifestClosurePlanGrantForRequest(grant, route, request) != nil || contract4competios.ValidateManifestClosurePlanInput(request, manifestBytes) != nil {
		return contract4competios.ClosurePlanReceipt{}, contract4competios.ErrInvalidGrant
	}
	if prior, exists := p.planCommands[request.CommandID]; exists {
		if prior.RequestPayloadDigest != request.TypedPayloadDigest {
			return contract4competios.ClosurePlanReceipt{}, contract4competios.ErrCommandConflict
		}
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
	if prior, replayed := p.candidates[request.CommandID]; replayed {
		if prior.digest != request.TypedPayloadDigest {
			return contract4competios.ArtifactRetentionReceipt{}, contract4competios.ErrCommandConflict
		}
		for _, retained := range p.retained {
			if retained.receipt.CommandID == request.CommandID {
				receipt := retained.receipt
				receipt.Status = contract4competios.ArtifactRetentionReplayed
				return receipt, nil
			}
		}
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
	p.candidates[request.CommandID] = sourceCommandRecord{digest: request.TypedPayloadDigest}
	p.retained[receipt.ReceiptID] = retainedRecord{receipt: receipt, transferDigest: transferDigest, plan: plan}
	return receipt, nil
}

func (p *referenceSourceProvider) PublishArtifact(_ context.Context, grant contract4competios.VerifiedOperationGrant, request contract4competios.ArtifactPublicationRequest) (contract4competios.ArtifactPublicationReceipt, error) {
	_, route := sourcePublicationGrantFixture(request, "expected", "expected")
	if contract4competios.ValidateOperationGrant(grant.Claims) != nil || contract4competios.ValidateArtifactPublicationGrantForRequest(grant, route, request) != nil {
		return contract4competios.ArtifactPublicationReceipt{}, contract4competios.ErrInvalidGrant
	}
	if prior, exists := p.publications[request.CommandID]; exists {
		if prior.PublicationRequestDigest != request.TypedPayloadDigest {
			return contract4competios.ArtifactPublicationReceipt{}, contract4competios.ErrCommandConflict
		}
		prior.Status = contract4competios.ArtifactPublicationReplayed
		return prior, nil
	}
	retained, exists := p.retained[request.RetentionReceiptID]
	if !exists || retained.receipt.ArtifactDigest != request.ArtifactDigest || retained.receipt.ParticipantVersionID != request.ParticipantVersionID {
		return contract4competios.ArtifactPublicationReceipt{}, contract4competios.ErrInvalidGrant
	}
	receipt := contract4competios.ArtifactPublicationReceipt{
		ReceiptID: "publication-1", ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		CommandID: request.CommandID, ParticipantID: request.ParticipantID,
		ParticipantVersionID: request.ParticipantVersionID, RetentionReceiptID: request.RetentionReceiptID,
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
	if prior, replayed := p.disclosures[request.CommandID]; replayed {
		if prior.VerificationRequestDigest != request.TypedPayloadDigest {
			return contract4competios.ArtifactDisclosureVerificationReceipt{}, contract4competios.ErrCommandConflict
		}
		return prior, nil
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
	p.disclosures[request.CommandID] = receipt
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

func TestSourceArtifactProviderConformance(t *testing.T) {
	if violations := CheckSourceArtifactProvider(func() contract4competios.SourceArtifactProvider { return newReferenceSourceProvider() }); len(violations) != 0 {
		t.Fatalf("reference source provider violations: %v", violations)
	}
	if violations := CheckSourceArtifactProvider(func() contract4competios.SourceArtifactProvider { return unsafeSourceProvider{} }); len(violations) < 5 {
		t.Fatalf("unsafe source provider was not decisively rejected: %v", violations)
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
