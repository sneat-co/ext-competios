package contract4competios

import (
	"context"
	"time"
)

type ArtifactDisclosureVerificationReceiptID string

const artifactDisclosureVerificationDigestDomain = "competios.artifact-disclosure-verification-request-payload.v1"

type ArtifactDisclosureVerificationRequestPayload struct {
	ProviderID                            ProviderID                 `json:"providerID"`
	AdapterID                             AdapterID                  `json:"adapterID"`
	CommandID                             CommandID                  `json:"commandID"`
	ParticipantID                         ParticipantID              `json:"participantID"`
	ParticipantVersionID                  ParticipantVersionID       `json:"participantVersionID"`
	RepositoryNodeID                      string                     `json:"repositoryNodeID"`
	Commit                                string                     `json:"commit"`
	ClosurePlanID                         ClosurePlanID              `json:"closurePlanID"`
	ClosurePlanDigest                     PayloadDigest              `json:"closurePlanDigest"`
	AggregateByteLimit                    uint64                     `json:"aggregateByteLimit"`
	RetentionReceiptID                    ArtifactRetentionReceiptID `json:"retentionReceiptID"`
	ArtifactDigest                        ArtifactDigest             `json:"artifactDigest"`
	PublicCandidateTransferredBytesDigest ArtifactDigest             `json:"publicCandidateTransferredBytesDigest"`
}

// ArtifactDisclosureVerificationRequest binds an unauthenticated fetch of the
// frozen public commit to the retained provider artifact. Competios supplies
// bytes; only the provider decides whether the canonical closures match.
type ArtifactDisclosureVerificationRequest struct {
	ProviderID                            ProviderID                 `json:"providerID"`
	AdapterID                             AdapterID                  `json:"adapterID"`
	CommandID                             CommandID                  `json:"commandID"`
	ParticipantID                         ParticipantID              `json:"participantID"`
	ParticipantVersionID                  ParticipantVersionID       `json:"participantVersionID"`
	RepositoryNodeID                      string                     `json:"repositoryNodeID"`
	Commit                                string                     `json:"commit"`
	ClosurePlanID                         ClosurePlanID              `json:"closurePlanID"`
	ClosurePlanDigest                     PayloadDigest              `json:"closurePlanDigest"`
	AggregateByteLimit                    uint64                     `json:"aggregateByteLimit"`
	RetentionReceiptID                    ArtifactRetentionReceiptID `json:"retentionReceiptID"`
	ArtifactDigest                        ArtifactDigest             `json:"artifactDigest"`
	PublicCandidateTransferredBytesDigest ArtifactDigest             `json:"publicCandidateTransferredBytesDigest"`
	TypedPayloadDigest                    PayloadDigest              `json:"typedPayloadDigest"`
}

func NewArtifactDisclosureVerificationRequest(payload ArtifactDisclosureVerificationRequestPayload) (ArtifactDisclosureVerificationRequest, error) {
	digest, err := DigestArtifactDisclosureVerificationRequestPayload(payload)
	if err != nil {
		return ArtifactDisclosureVerificationRequest{}, err
	}
	request := ArtifactDisclosureVerificationRequest{
		ProviderID: payload.ProviderID, AdapterID: payload.AdapterID, CommandID: payload.CommandID,
		ParticipantID: payload.ParticipantID, ParticipantVersionID: payload.ParticipantVersionID,
		RepositoryNodeID: payload.RepositoryNodeID, Commit: payload.Commit,
		ClosurePlanID: payload.ClosurePlanID, ClosurePlanDigest: payload.ClosurePlanDigest,
		AggregateByteLimit: payload.AggregateByteLimit, RetentionReceiptID: payload.RetentionReceiptID,
		ArtifactDigest:                        payload.ArtifactDigest,
		PublicCandidateTransferredBytesDigest: payload.PublicCandidateTransferredBytesDigest,
		TypedPayloadDigest:                    digest,
	}
	if err := ValidateArtifactDisclosureVerificationRequest(request); err != nil {
		return ArtifactDisclosureVerificationRequest{}, err
	}
	return request, nil
}

func (r ArtifactDisclosureVerificationRequest) Payload() ArtifactDisclosureVerificationRequestPayload {
	return ArtifactDisclosureVerificationRequestPayload{
		ProviderID: r.ProviderID, AdapterID: r.AdapterID, CommandID: r.CommandID,
		ParticipantID: r.ParticipantID, ParticipantVersionID: r.ParticipantVersionID,
		RepositoryNodeID: r.RepositoryNodeID, Commit: r.Commit,
		ClosurePlanID: r.ClosurePlanID, ClosurePlanDigest: r.ClosurePlanDigest,
		AggregateByteLimit: r.AggregateByteLimit, RetentionReceiptID: r.RetentionReceiptID,
		ArtifactDigest:                        r.ArtifactDigest,
		PublicCandidateTransferredBytesDigest: r.PublicCandidateTransferredBytesDigest,
	}
}

func DigestArtifactDisclosureVerificationRequestPayload(payload ArtifactDisclosureVerificationRequestPayload) (PayloadDigest, error) {
	return digestJSON(artifactDisclosureVerificationDigestDomain, payload)
}

func ValidateArtifactDisclosureVerificationRequest(value ArtifactDisclosureVerificationRequest) error {
	if value.ProviderID == "" || value.AdapterID == "" || value.CommandID == "" || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || value.ClosurePlanID == "" || !validSHA256Digest(string(value.ClosurePlanDigest)) || value.AggregateByteLimit == 0 || value.RetentionReceiptID == "" || !validSHA256Digest(string(value.ArtifactDigest)) || !validSHA256Digest(string(value.PublicCandidateTransferredBytesDigest)) || !validSHA256Digest(string(value.TypedPayloadDigest)) {
		return ErrInvalidGrant
	}
	digest, err := DigestArtifactDisclosureVerificationRequestPayload(value.Payload())
	if err != nil || digest != value.TypedPayloadDigest {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateArtifactDisclosureInput(request ArtifactDisclosureVerificationRequest, plan ClosurePlan, transfer CandidateClosureTransfer) error {
	if ValidateArtifactDisclosureVerificationRequest(request) != nil || ValidateClosurePlan(plan) != nil || request.ProviderID != plan.ProviderID || request.AdapterID != plan.AdapterID || request.ParticipantID != plan.ParticipantID || request.ParticipantVersionID != plan.ParticipantVersionID || request.RepositoryNodeID != plan.RepositoryNodeID || request.Commit != plan.Commit || request.ClosurePlanID != plan.ClosurePlanID || request.ClosurePlanDigest != plan.ClosurePlanDigest || request.AggregateByteLimit != plan.AggregateByteLimit {
		return ErrInvalidGrant
	}
	return validateTransferAgainstPlan(plan, transfer, request.PublicCandidateTransferredBytesDigest, request.AggregateByteLimit)
}

type ArtifactDisclosureVerdict string

const (
	ArtifactDisclosureMatched    ArtifactDisclosureVerdict = "matched"
	ArtifactDisclosureMismatched ArtifactDisclosureVerdict = "mismatched"
)

type ArtifactDisclosureVerificationReceipt struct {
	ReceiptID                 ArtifactDisclosureVerificationReceiptID `json:"receiptID"`
	ProviderID                ProviderID                              `json:"providerID"`
	AdapterID                 AdapterID                               `json:"adapterID"`
	CommandID                 CommandID                               `json:"commandID"`
	ParticipantID             ParticipantID                           `json:"participantID"`
	ParticipantVersionID      ParticipantVersionID                    `json:"participantVersionID"`
	RetentionReceiptID        ArtifactRetentionReceiptID              `json:"retentionReceiptID"`
	ArtifactDigest            ArtifactDigest                          `json:"artifactDigest"`
	VerificationRequestDigest PayloadDigest                           `json:"verificationRequestDigest"`
	Verdict                   ArtifactDisclosureVerdict               `json:"verdict"`
	VerifiedAt                time.Time                               `json:"verifiedAt"`
}

func ValidateArtifactDisclosureVerificationReceiptForRequest(receipt ArtifactDisclosureVerificationReceipt, request ArtifactDisclosureVerificationRequest) error {
	if ValidateArtifactDisclosureVerificationRequest(request) != nil || receipt.ReceiptID == "" || receipt.ProviderID != request.ProviderID || receipt.AdapterID != request.AdapterID || receipt.CommandID != request.CommandID || receipt.ParticipantID != request.ParticipantID || receipt.ParticipantVersionID != request.ParticipantVersionID || receipt.RetentionReceiptID != request.RetentionReceiptID || receipt.ArtifactDigest != request.ArtifactDigest || receipt.VerificationRequestDigest != request.TypedPayloadDigest || receipt.VerifiedAt.IsZero() {
		return ErrInvalidExecution
	}
	switch receipt.Verdict {
	case ArtifactDisclosureMatched, ArtifactDisclosureMismatched:
		return nil
	default:
		return ErrInvalidExecution
	}
}

type ArtifactDisclosureVerifier interface {
	VerifyArtifactDisclosure(context.Context, VerifiedOperationGrant, ArtifactDisclosureVerificationRequest, CandidateClosureTransfer) (ArtifactDisclosureVerificationReceipt, error)
}
