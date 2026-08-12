package contract4competios

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"path"
	"strings"
	"time"
)

type ClosurePlanID string
type ArtifactRetentionReceiptID string
type ArtifactPublicationReceiptID string
type PublicArtifactReference string

const (
	manifestClosurePlanDigestDomain       = "competios.manifest-closure-plan-request-payload.v1"
	closurePlanDigestDomain               = "competios.provider-closure-plan-payload.v1"
	candidateClosureRetentionDigestDomain = "competios.candidate-closure-retention-request-payload.v1"
	artifactPublicationDigestDomain       = "competios.artifact-publication-request-payload.v1"
	rawManifestBytesDigestDomain          = "competios.raw-manifest-bytes.v1"
	candidateTransferredBytesDigestDomain = "competios.candidate-transferred-files.v1"
)

// DigestRawManifestBytes hashes exactly the manifest bytes transferred for a
// closure-plan request. It is distinct from the HTTP transport-body digest.
func DigestRawManifestBytes(value []byte) ArtifactDigest {
	return ArtifactDigest(digestParts(rawManifestBytesDigestDomain, value))
}

type ManifestClosurePlanRequestPayload struct {
	ProviderID             ProviderID           `json:"providerID"`
	AdapterID              AdapterID            `json:"adapterID"`
	CommandID              CommandID            `json:"commandID"`
	ParticipantID          ParticipantID        `json:"participantID"`
	ParticipantVersionID   ParticipantVersionID `json:"participantVersionID"`
	RepositoryNodeID       string               `json:"repositoryNodeID"`
	Commit                 string               `json:"commit"`
	ManifestPath           string               `json:"manifestPath"`
	ManifestEntryKind      SourceEntryKind      `json:"manifestEntryKind"`
	RawManifestBytesDigest ArtifactDigest       `json:"rawManifestBytesDigest"`
	ManifestByteLimit      uint64               `json:"manifestByteLimit"`
}

// ManifestClosurePlanRequest authorizes only manifest inspection and planning;
// it cannot authorize candidate validation, retention, or publication.
type ManifestClosurePlanRequest struct {
	ProviderID             ProviderID           `json:"providerID"`
	AdapterID              AdapterID            `json:"adapterID"`
	CommandID              CommandID            `json:"commandID"`
	ParticipantID          ParticipantID        `json:"participantID"`
	ParticipantVersionID   ParticipantVersionID `json:"participantVersionID"`
	RepositoryNodeID       string               `json:"repositoryNodeID"`
	Commit                 string               `json:"commit"`
	ManifestPath           string               `json:"manifestPath"`
	ManifestEntryKind      SourceEntryKind      `json:"manifestEntryKind"`
	RawManifestBytesDigest ArtifactDigest       `json:"rawManifestBytesDigest"`
	ManifestByteLimit      uint64               `json:"manifestByteLimit"`
	TypedPayloadDigest     PayloadDigest        `json:"typedPayloadDigest"`
}

func NewManifestClosurePlanRequest(payload ManifestClosurePlanRequestPayload) (ManifestClosurePlanRequest, error) {
	digest, err := DigestManifestClosurePlanRequestPayload(payload)
	if err != nil {
		return ManifestClosurePlanRequest{}, err
	}
	request := ManifestClosurePlanRequest{
		ProviderID: payload.ProviderID, AdapterID: payload.AdapterID, CommandID: payload.CommandID,
		ParticipantID: payload.ParticipantID, ParticipantVersionID: payload.ParticipantVersionID,
		RepositoryNodeID: payload.RepositoryNodeID, Commit: payload.Commit, ManifestPath: payload.ManifestPath,
		ManifestEntryKind:      payload.ManifestEntryKind,
		RawManifestBytesDigest: payload.RawManifestBytesDigest, ManifestByteLimit: payload.ManifestByteLimit,
		TypedPayloadDigest: digest,
	}
	if err := ValidateManifestClosurePlanRequest(request); err != nil {
		return ManifestClosurePlanRequest{}, err
	}
	return request, nil
}

func (r ManifestClosurePlanRequest) Payload() ManifestClosurePlanRequestPayload {
	return ManifestClosurePlanRequestPayload{
		ProviderID: r.ProviderID, AdapterID: r.AdapterID, CommandID: r.CommandID,
		ParticipantID: r.ParticipantID, ParticipantVersionID: r.ParticipantVersionID,
		RepositoryNodeID: r.RepositoryNodeID, Commit: r.Commit, ManifestPath: r.ManifestPath,
		ManifestEntryKind:      r.ManifestEntryKind,
		RawManifestBytesDigest: r.RawManifestBytesDigest, ManifestByteLimit: r.ManifestByteLimit,
	}
}

func DigestManifestClosurePlanRequestPayload(payload ManifestClosurePlanRequestPayload) (PayloadDigest, error) {
	return digestJSON(manifestClosurePlanDigestDomain, payload)
}

func ValidateManifestClosurePlanRequest(value ManifestClosurePlanRequest) error {
	if value.ProviderID == "" || value.AdapterID == "" || value.CommandID == "" || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || !validCanonicalSourcePath(value.ManifestPath) || value.ManifestEntryKind != SourceEntryRegular || !validSHA256Digest(string(value.RawManifestBytesDigest)) || value.ManifestByteLimit == 0 || !validSHA256Digest(string(value.TypedPayloadDigest)) {
		return ErrInvalidGrant
	}
	digest, err := DigestManifestClosurePlanRequestPayload(value.Payload())
	if err != nil || digest != value.TypedPayloadDigest {
		return ErrInvalidGrant
	}
	return nil
}

// ValidateManifestClosurePlanInput binds the typed request to the exact
// transient manifest bytes supplied to a provider port.
func ValidateManifestClosurePlanInput(request ManifestClosurePlanRequest, manifestBytes []byte) error {
	if ValidateManifestClosurePlanRequest(request) != nil || uint64(len(manifestBytes)) > request.ManifestByteLimit || DigestRawManifestBytes(manifestBytes) != request.RawManifestBytesDigest {
		return ErrInvalidGrant
	}
	return nil
}

type SourceEntryKind string

const (
	SourceEntryRegular   SourceEntryKind = "regular"
	SourceEntrySymlink   SourceEntryKind = "symlink"
	SourceEntrySubmodule SourceEntryKind = "submodule"
)

type PlannedSourceFile struct {
	CanonicalPath string          `json:"canonicalPath"`
	EntryKind     SourceEntryKind `json:"entryKind"`
	ByteLimit     uint64          `json:"byteLimit"`
}

// ClosurePlanPayload is the complete generic provider plan. Competios can
// fetch precisely these paths without decoding a game-specific opaque blob.
type ClosurePlanPayload struct {
	ClosurePlanID          ClosurePlanID        `json:"closurePlanID"`
	ProviderID             ProviderID           `json:"providerID"`
	AdapterID              AdapterID            `json:"adapterID"`
	ParticipantID          ParticipantID        `json:"participantID"`
	ParticipantVersionID   ParticipantVersionID `json:"participantVersionID"`
	RepositoryNodeID       string               `json:"repositoryNodeID"`
	Commit                 string               `json:"commit"`
	ManifestPath           string               `json:"manifestPath"`
	ManifestEntryKind      SourceEntryKind      `json:"manifestEntryKind"`
	ManifestRequestDigest  PayloadDigest        `json:"manifestRequestDigest"`
	RawManifestBytesDigest ArtifactDigest       `json:"rawManifestBytesDigest"`
	Files                  []PlannedSourceFile  `json:"files"`
	AggregateByteLimit     uint64               `json:"aggregateByteLimit"`
}

type ClosurePlan struct {
	ClosurePlanID          ClosurePlanID        `json:"closurePlanID"`
	ProviderID             ProviderID           `json:"providerID"`
	AdapterID              AdapterID            `json:"adapterID"`
	ParticipantID          ParticipantID        `json:"participantID"`
	ParticipantVersionID   ParticipantVersionID `json:"participantVersionID"`
	RepositoryNodeID       string               `json:"repositoryNodeID"`
	Commit                 string               `json:"commit"`
	ManifestPath           string               `json:"manifestPath"`
	ManifestEntryKind      SourceEntryKind      `json:"manifestEntryKind"`
	ManifestRequestDigest  PayloadDigest        `json:"manifestRequestDigest"`
	RawManifestBytesDigest ArtifactDigest       `json:"rawManifestBytesDigest"`
	Files                  []PlannedSourceFile  `json:"files"`
	AggregateByteLimit     uint64               `json:"aggregateByteLimit"`
	ClosurePlanDigest      PayloadDigest        `json:"closurePlanDigest"`
}

func NewClosurePlan(payload ClosurePlanPayload) (ClosurePlan, error) {
	digest, err := DigestClosurePlanPayload(payload)
	if err != nil {
		return ClosurePlan{}, err
	}
	plan := ClosurePlan{
		ClosurePlanID: payload.ClosurePlanID, ProviderID: payload.ProviderID, AdapterID: payload.AdapterID,
		ParticipantID: payload.ParticipantID, ParticipantVersionID: payload.ParticipantVersionID,
		RepositoryNodeID: payload.RepositoryNodeID, Commit: payload.Commit,
		ManifestPath: payload.ManifestPath, ManifestEntryKind: payload.ManifestEntryKind,
		ManifestRequestDigest:  payload.ManifestRequestDigest,
		RawManifestBytesDigest: payload.RawManifestBytesDigest,
		Files:                  append([]PlannedSourceFile(nil), payload.Files...), AggregateByteLimit: payload.AggregateByteLimit,
		ClosurePlanDigest: digest,
	}
	if err := ValidateClosurePlan(plan); err != nil {
		return ClosurePlan{}, err
	}
	return plan, nil
}

func (p ClosurePlan) Payload() ClosurePlanPayload {
	return ClosurePlanPayload{
		ClosurePlanID: p.ClosurePlanID, ProviderID: p.ProviderID, AdapterID: p.AdapterID,
		ParticipantID: p.ParticipantID, ParticipantVersionID: p.ParticipantVersionID,
		RepositoryNodeID: p.RepositoryNodeID, Commit: p.Commit,
		ManifestPath: p.ManifestPath, ManifestEntryKind: p.ManifestEntryKind,
		ManifestRequestDigest:  p.ManifestRequestDigest,
		RawManifestBytesDigest: p.RawManifestBytesDigest,
		Files:                  append([]PlannedSourceFile(nil), p.Files...), AggregateByteLimit: p.AggregateByteLimit,
	}
}

func DigestClosurePlanPayload(payload ClosurePlanPayload) (PayloadDigest, error) {
	return digestJSON(closurePlanDigestDomain, payload)
}

func ValidateClosurePlan(value ClosurePlan) error {
	if value.ClosurePlanID == "" || value.ProviderID == "" || value.AdapterID == "" || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || !validCanonicalSourcePath(value.ManifestPath) || value.ManifestEntryKind != SourceEntryRegular || !validSHA256Digest(string(value.ManifestRequestDigest)) || !validSHA256Digest(string(value.RawManifestBytesDigest)) || len(value.Files) == 0 || value.AggregateByteLimit == 0 || !validSHA256Digest(string(value.ClosurePlanDigest)) {
		return ErrInvalidExecution
	}
	seen := map[string]bool{}
	previousPath := ""
	for _, file := range value.Files {
		if !validCanonicalSourcePath(file.CanonicalPath) || file.EntryKind != SourceEntryRegular || file.ByteLimit == 0 || seen[file.CanonicalPath] || previousPath != "" && file.CanonicalPath <= previousPath {
			return ErrInvalidExecution
		}
		seen[file.CanonicalPath] = true
		previousPath = file.CanonicalPath
	}
	digest, err := DigestClosurePlanPayload(value.Payload())
	if err != nil || digest != value.ClosurePlanDigest {
		return ErrInvalidExecution
	}
	return nil
}

type ClosurePlanReceiptStatus string

const (
	ClosurePlanReceiptAccepted ClosurePlanReceiptStatus = "accepted"
	ClosurePlanReceiptReplayed ClosurePlanReceiptStatus = "replayed"
)

type ClosurePlanReceipt struct {
	ProviderID           ProviderID               `json:"providerID"`
	AdapterID            AdapterID                `json:"adapterID"`
	CommandID            CommandID                `json:"commandID"`
	ParticipantID        ParticipantID            `json:"participantID"`
	ParticipantVersionID ParticipantVersionID     `json:"participantVersionID"`
	RequestPayloadDigest PayloadDigest            `json:"requestPayloadDigest"`
	Plan                 ClosurePlan              `json:"plan"`
	Status               ClosurePlanReceiptStatus `json:"status"`
}

func ValidateClosurePlanReceiptForRequest(receipt ClosurePlanReceipt, request ManifestClosurePlanRequest) error {
	if ValidateManifestClosurePlanRequest(request) != nil || ValidateClosurePlan(receipt.Plan) != nil || receipt.ProviderID != request.ProviderID || receipt.AdapterID != request.AdapterID || receipt.CommandID != request.CommandID || receipt.ParticipantID != request.ParticipantID || receipt.ParticipantVersionID != request.ParticipantVersionID || receipt.RequestPayloadDigest != request.TypedPayloadDigest || receipt.Plan.ProviderID != request.ProviderID || receipt.Plan.AdapterID != request.AdapterID || receipt.Plan.ParticipantID != request.ParticipantID || receipt.Plan.ParticipantVersionID != request.ParticipantVersionID || receipt.Plan.RepositoryNodeID != request.RepositoryNodeID || receipt.Plan.Commit != request.Commit || receipt.Plan.ManifestPath != request.ManifestPath || receipt.Plan.ManifestEntryKind != request.ManifestEntryKind || receipt.Plan.ManifestRequestDigest != request.TypedPayloadDigest || receipt.Plan.RawManifestBytesDigest != request.RawManifestBytesDigest {
		return ErrInvalidExecution
	}
	switch receipt.Status {
	case ClosurePlanReceiptAccepted, ClosurePlanReceiptReplayed:
		return nil
	default:
		return ErrInvalidExecution
	}
}

type CandidateSourceFile struct {
	CanonicalPath string          `json:"canonicalPath"`
	EntryKind     SourceEntryKind `json:"entryKind"`
	Bytes         []byte          `json:"bytes"`
}

// CandidateClosureTransfer preserves the resolver-reported entry kind so a
// symlink/submodule cannot be flattened into regular-looking bytes. Retention
// accepts only regular entries declared by the plan.
type CandidateClosureTransfer struct {
	Files []CandidateSourceFile `json:"files"`
}

func DigestCandidateTransferredFiles(files []CandidateSourceFile) (ArtifactDigest, error) {
	if len(files) == 0 {
		return "", ErrInvalidGrant
	}
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(files)))
	parts := make([][]byte, 0, 1+len(files)*3)
	parts = append(parts, count[:])
	seen := map[string]bool{}
	for _, file := range files {
		if !validCanonicalSourcePath(file.CanonicalPath) || !validSourceEntryKind(file.EntryKind) || seen[file.CanonicalPath] {
			return "", ErrInvalidGrant
		}
		seen[file.CanonicalPath] = true
		parts = append(parts, []byte(file.CanonicalPath), []byte(file.EntryKind), file.Bytes)
	}
	return ArtifactDigest(digestParts(candidateTransferredBytesDigestDomain, parts...)), nil
}

type CandidateClosureRetentionRequestPayload struct {
	ProviderID                      ProviderID           `json:"providerID"`
	AdapterID                       AdapterID            `json:"adapterID"`
	CommandID                       CommandID            `json:"commandID"`
	ParticipantID                   ParticipantID        `json:"participantID"`
	ParticipantVersionID            ParticipantVersionID `json:"participantVersionID"`
	RepositoryNodeID                string               `json:"repositoryNodeID"`
	Commit                          string               `json:"commit"`
	ClosurePlanID                   ClosurePlanID        `json:"closurePlanID"`
	ClosurePlanDigest               PayloadDigest        `json:"closurePlanDigest"`
	CandidateTransferredBytesDigest ArtifactDigest       `json:"candidateTransferredBytesDigest"`
	AggregateByteLimit              uint64               `json:"aggregateByteLimit"`
}

// CandidateClosureRetentionRequest binds an exact provider plan and exact
// transient file transfer. The canonical ArtifactDigest intentionally does
// not exist until the provider accepts and retains the candidate.
type CandidateClosureRetentionRequest struct {
	ProviderID                      ProviderID           `json:"providerID"`
	AdapterID                       AdapterID            `json:"adapterID"`
	CommandID                       CommandID            `json:"commandID"`
	ParticipantID                   ParticipantID        `json:"participantID"`
	ParticipantVersionID            ParticipantVersionID `json:"participantVersionID"`
	RepositoryNodeID                string               `json:"repositoryNodeID"`
	Commit                          string               `json:"commit"`
	ClosurePlanID                   ClosurePlanID        `json:"closurePlanID"`
	ClosurePlanDigest               PayloadDigest        `json:"closurePlanDigest"`
	CandidateTransferredBytesDigest ArtifactDigest       `json:"candidateTransferredBytesDigest"`
	AggregateByteLimit              uint64               `json:"aggregateByteLimit"`
	TypedPayloadDigest              PayloadDigest        `json:"typedPayloadDigest"`
}

func NewCandidateClosureRetentionRequest(payload CandidateClosureRetentionRequestPayload) (CandidateClosureRetentionRequest, error) {
	digest, err := DigestCandidateClosureRetentionRequestPayload(payload)
	if err != nil {
		return CandidateClosureRetentionRequest{}, err
	}
	request := CandidateClosureRetentionRequest{
		ProviderID: payload.ProviderID, AdapterID: payload.AdapterID, CommandID: payload.CommandID,
		ParticipantID: payload.ParticipantID, ParticipantVersionID: payload.ParticipantVersionID,
		RepositoryNodeID: payload.RepositoryNodeID, Commit: payload.Commit,
		ClosurePlanID: payload.ClosurePlanID, ClosurePlanDigest: payload.ClosurePlanDigest,
		CandidateTransferredBytesDigest: payload.CandidateTransferredBytesDigest,
		AggregateByteLimit:              payload.AggregateByteLimit, TypedPayloadDigest: digest,
	}
	if err := ValidateCandidateClosureRetentionRequest(request); err != nil {
		return CandidateClosureRetentionRequest{}, err
	}
	return request, nil
}

func (r CandidateClosureRetentionRequest) Payload() CandidateClosureRetentionRequestPayload {
	return CandidateClosureRetentionRequestPayload{
		ProviderID: r.ProviderID, AdapterID: r.AdapterID, CommandID: r.CommandID,
		ParticipantID: r.ParticipantID, ParticipantVersionID: r.ParticipantVersionID,
		RepositoryNodeID: r.RepositoryNodeID, Commit: r.Commit,
		ClosurePlanID: r.ClosurePlanID, ClosurePlanDigest: r.ClosurePlanDigest,
		CandidateTransferredBytesDigest: r.CandidateTransferredBytesDigest,
		AggregateByteLimit:              r.AggregateByteLimit,
	}
}

func DigestCandidateClosureRetentionRequestPayload(payload CandidateClosureRetentionRequestPayload) (PayloadDigest, error) {
	return digestJSON(candidateClosureRetentionDigestDomain, payload)
}

func ValidateCandidateClosureRetentionRequest(value CandidateClosureRetentionRequest) error {
	if value.ProviderID == "" || value.AdapterID == "" || value.CommandID == "" || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || value.ClosurePlanID == "" || !validSHA256Digest(string(value.ClosurePlanDigest)) || !validSHA256Digest(string(value.CandidateTransferredBytesDigest)) || value.AggregateByteLimit == 0 || !validSHA256Digest(string(value.TypedPayloadDigest)) {
		return ErrInvalidGrant
	}
	digest, err := DigestCandidateClosureRetentionRequestPayload(value.Payload())
	if err != nil || digest != value.TypedPayloadDigest {
		return ErrInvalidGrant
	}
	return nil
}

// ValidateCandidateClosureInput binds plan, ordered paths, byte limits and
// transferred bytes before the provider may retain anything.
func ValidateCandidateClosureInput(request CandidateClosureRetentionRequest, plan ClosurePlan, transfer CandidateClosureTransfer) error {
	if ValidateCandidateClosureRetentionRequest(request) != nil || ValidateClosurePlan(plan) != nil || request.ProviderID != plan.ProviderID || request.AdapterID != plan.AdapterID || request.ParticipantID != plan.ParticipantID || request.ParticipantVersionID != plan.ParticipantVersionID || request.RepositoryNodeID != plan.RepositoryNodeID || request.Commit != plan.Commit || request.ClosurePlanID != plan.ClosurePlanID || request.ClosurePlanDigest != plan.ClosurePlanDigest || request.AggregateByteLimit != plan.AggregateByteLimit || len(transfer.Files) != len(plan.Files) {
		return ErrInvalidGrant
	}
	return validateTransferAgainstPlan(plan, transfer, request.CandidateTransferredBytesDigest, request.AggregateByteLimit)
}

func validateTransferAgainstPlan(plan ClosurePlan, transfer CandidateClosureTransfer, expectedDigest ArtifactDigest, aggregateByteLimit uint64) error {
	if len(transfer.Files) != len(plan.Files) {
		return ErrInvalidGrant
	}
	var total uint64
	for index, file := range transfer.Files {
		planned := plan.Files[index]
		size := uint64(len(file.Bytes))
		if file.CanonicalPath != planned.CanonicalPath || file.EntryKind != planned.EntryKind || file.EntryKind != SourceEntryRegular || size > planned.ByteLimit || size > aggregateByteLimit || total > aggregateByteLimit-size {
			return ErrInvalidGrant
		}
		total += size
	}
	digest, err := DigestCandidateTransferredFiles(transfer.Files)
	if err != nil || digest != expectedDigest {
		return ErrInvalidGrant
	}
	return nil
}

type ArtifactRetentionReceiptStatus string

const (
	ArtifactRetentionAccepted ArtifactRetentionReceiptStatus = "accepted"
	ArtifactRetentionReplayed ArtifactRetentionReceiptStatus = "replayed"
)

// ArtifactRetentionReceipt is the first contract fact allowed to carry the
// provider-authoritative canonical artifact digest.
type ArtifactRetentionReceipt struct {
	ReceiptID              ArtifactRetentionReceiptID     `json:"receiptID"`
	ProviderID             ProviderID                     `json:"providerID"`
	AdapterID              AdapterID                      `json:"adapterID"`
	CommandID              CommandID                      `json:"commandID"`
	ParticipantID          ParticipantID                  `json:"participantID"`
	ParticipantVersionID   ParticipantVersionID           `json:"participantVersionID"`
	ClosurePlanID          ClosurePlanID                  `json:"closurePlanID"`
	ClosurePlanDigest      PayloadDigest                  `json:"closurePlanDigest"`
	CandidateRequestDigest PayloadDigest                  `json:"candidateRequestDigest"`
	ArtifactDigest         ArtifactDigest                 `json:"artifactDigest"`
	Status                 ArtifactRetentionReceiptStatus `json:"status"`
}

func ValidateArtifactRetentionReceiptForRequest(receipt ArtifactRetentionReceipt, request CandidateClosureRetentionRequest) error {
	if ValidateCandidateClosureRetentionRequest(request) != nil || receipt.ReceiptID == "" || receipt.ProviderID != request.ProviderID || receipt.AdapterID != request.AdapterID || receipt.CommandID != request.CommandID || receipt.ParticipantID != request.ParticipantID || receipt.ParticipantVersionID != request.ParticipantVersionID || receipt.ClosurePlanID != request.ClosurePlanID || receipt.ClosurePlanDigest != request.ClosurePlanDigest || receipt.CandidateRequestDigest != request.TypedPayloadDigest || !validSHA256Digest(string(receipt.ArtifactDigest)) {
		return ErrInvalidExecution
	}
	switch receipt.Status {
	case ArtifactRetentionAccepted, ArtifactRetentionReplayed:
		return nil
	default:
		return ErrInvalidExecution
	}
}

type ArtifactPublicationRequestPayload struct {
	ProviderID           ProviderID                 `json:"providerID"`
	AdapterID            AdapterID                  `json:"adapterID"`
	CommandID            CommandID                  `json:"commandID"`
	ParticipantID        ParticipantID              `json:"participantID"`
	ParticipantVersionID ParticipantVersionID       `json:"participantVersionID"`
	RetentionReceiptID   ArtifactRetentionReceiptID `json:"retentionReceiptID"`
	ArtifactDigest       ArtifactDigest             `json:"artifactDigest"`
}

type ArtifactPublicationRequest struct {
	ProviderID           ProviderID                 `json:"providerID"`
	AdapterID            AdapterID                  `json:"adapterID"`
	CommandID            CommandID                  `json:"commandID"`
	ParticipantID        ParticipantID              `json:"participantID"`
	ParticipantVersionID ParticipantVersionID       `json:"participantVersionID"`
	RetentionReceiptID   ArtifactRetentionReceiptID `json:"retentionReceiptID"`
	ArtifactDigest       ArtifactDigest             `json:"artifactDigest"`
	TypedPayloadDigest   PayloadDigest              `json:"typedPayloadDigest"`
}

func NewArtifactPublicationRequest(payload ArtifactPublicationRequestPayload) (ArtifactPublicationRequest, error) {
	digest, err := DigestArtifactPublicationRequestPayload(payload)
	if err != nil {
		return ArtifactPublicationRequest{}, err
	}
	request := ArtifactPublicationRequest{
		ProviderID: payload.ProviderID, AdapterID: payload.AdapterID, CommandID: payload.CommandID,
		ParticipantID: payload.ParticipantID, ParticipantVersionID: payload.ParticipantVersionID,
		RetentionReceiptID: payload.RetentionReceiptID, ArtifactDigest: payload.ArtifactDigest,
		TypedPayloadDigest: digest,
	}
	if err := ValidateArtifactPublicationRequest(request); err != nil {
		return ArtifactPublicationRequest{}, err
	}
	return request, nil
}

func (r ArtifactPublicationRequest) Payload() ArtifactPublicationRequestPayload {
	return ArtifactPublicationRequestPayload{
		ProviderID: r.ProviderID, AdapterID: r.AdapterID, CommandID: r.CommandID,
		ParticipantID: r.ParticipantID, ParticipantVersionID: r.ParticipantVersionID,
		RetentionReceiptID: r.RetentionReceiptID, ArtifactDigest: r.ArtifactDigest,
	}
}

func DigestArtifactPublicationRequestPayload(payload ArtifactPublicationRequestPayload) (PayloadDigest, error) {
	return digestJSON(artifactPublicationDigestDomain, payload)
}

func ValidateArtifactPublicationRequest(value ArtifactPublicationRequest) error {
	if value.ProviderID == "" || value.AdapterID == "" || value.CommandID == "" || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RetentionReceiptID == "" || !validSHA256Digest(string(value.ArtifactDigest)) || !validSHA256Digest(string(value.TypedPayloadDigest)) {
		return ErrInvalidGrant
	}
	digest, err := DigestArtifactPublicationRequestPayload(value.Payload())
	if err != nil || digest != value.TypedPayloadDigest {
		return ErrInvalidGrant
	}
	return nil
}

type ArtifactPublicationReceiptStatus string

const (
	ArtifactPublicationAccepted ArtifactPublicationReceiptStatus = "accepted"
	ArtifactPublicationReplayed ArtifactPublicationReceiptStatus = "replayed"
)

type ArtifactPublicationReceipt struct {
	ReceiptID                ArtifactPublicationReceiptID     `json:"receiptID"`
	ProviderID               ProviderID                       `json:"providerID"`
	AdapterID                AdapterID                        `json:"adapterID"`
	CommandID                CommandID                        `json:"commandID"`
	ParticipantID            ParticipantID                    `json:"participantID"`
	ParticipantVersionID     ParticipantVersionID             `json:"participantVersionID"`
	RetentionReceiptID       ArtifactRetentionReceiptID       `json:"retentionReceiptID"`
	PublicationRequestDigest PayloadDigest                    `json:"publicationRequestDigest"`
	ArtifactDigest           ArtifactDigest                   `json:"artifactDigest"`
	PublishedAt              time.Time                        `json:"publishedAt"`
	PublicReference          PublicArtifactReference          `json:"publicReference"`
	Status                   ArtifactPublicationReceiptStatus `json:"status"`
}

func ValidateArtifactPublicationReceiptForRequest(receipt ArtifactPublicationReceipt, request ArtifactPublicationRequest) error {
	if ValidateArtifactPublicationRequest(request) != nil || receipt.ReceiptID == "" || receipt.ProviderID != request.ProviderID || receipt.AdapterID != request.AdapterID || receipt.CommandID != request.CommandID || receipt.ParticipantID != request.ParticipantID || receipt.ParticipantVersionID != request.ParticipantVersionID || receipt.RetentionReceiptID != request.RetentionReceiptID || receipt.PublicationRequestDigest != request.TypedPayloadDigest || receipt.ArtifactDigest != request.ArtifactDigest || receipt.PublishedAt.IsZero() || receipt.PublicReference == "" {
		return ErrInvalidExecution
	}
	switch receipt.Status {
	case ArtifactPublicationAccepted, ArtifactPublicationReplayed:
		return nil
	default:
		return ErrInvalidExecution
	}
}

// ManifestClosurePlanner receives only the exact manifest bytes authorized by
// the request/grant. It never receives a repository credential.
type ManifestClosurePlanner interface {
	PlanManifestClosure(context.Context, VerifiedOperationGrant, ManifestClosurePlanRequest, []byte) (ClosurePlanReceipt, error)
}

// CandidateClosureRetainer receives only ordered planned regular-file bytes.
// Provider acceptance is the point at which retention authority begins.
type CandidateClosureRetainer interface {
	ValidateAndRetainCandidate(context.Context, VerifiedOperationGrant, CandidateClosureRetentionRequest, CandidateClosureTransfer) (ArtifactRetentionReceipt, error)
}

type ArtifactPublisher interface {
	PublishArtifact(context.Context, VerifiedOperationGrant, ArtifactPublicationRequest) (ArtifactPublicationReceipt, error)
}

type SourceArtifactProvider interface {
	ManifestClosurePlanner
	CandidateClosureRetainer
	ArtifactPublisher
	ArtifactDisclosureVerifier
}

func validCanonicalSourcePath(value string) bool {
	if value == "" || value == "." || path.IsAbs(value) || path.Clean(value) != value || strings.Contains(value, "\\") || value == ".." || strings.HasPrefix(value, "../") {
		return false
	}
	for _, char := range value {
		if char <= '\x1f' || char == '\x7f' {
			return false
		}
	}
	return true
}

func validSourceEntryKind(value SourceEntryKind) bool {
	switch value {
	case SourceEntryRegular, SourceEntrySymlink, SourceEntrySubmodule:
		return true
	default:
		return false
	}
}

func digestJSON(domain string, value any) (PayloadDigest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestParts(domain, encoded), nil
}
