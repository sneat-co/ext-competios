package contract4competios

import (
	"context"
	"time"
)

type GrantPurpose string

const (
	GrantPurposeParticipantVersionValidate GrantPurpose = "participant.version.validate"
	GrantPurposeContestLaunch              GrantPurpose = "competition.contest.launch"
	GrantPurposeContestStarted             GrantPurpose = "competition.contest.started"
	GrantPurposeContestResultSubmit        GrantPurpose = "competition.contest.result.submit"
)

// OperationGrant is the post-verification claim set. It deliberately has no
// bearer token/JWT field: cryptographic parsing and HTTP extraction remain in
// the owning service, which hands business code only verified facts.
type OperationGrant struct {
	Issuer               string               `json:"issuer"`
	Subject              string               `json:"subject"`
	Audience             string               `json:"audience"`
	Purpose              GrantPurpose         `json:"purpose"`
	KeyID                string               `json:"keyID"`
	TokenID              string               `json:"tokenID"`
	IssuedAt             time.Time            `json:"issuedAt"`
	NotBefore            time.Time            `json:"notBefore"`
	ExpiresAt            time.Time            `json:"expiresAt"`
	ProviderID           ProviderID           `json:"providerID"`
	AdapterID            AdapterID            `json:"adapterID"`
	CompetitionID        CompetitionID        `json:"competitionID,omitempty"`
	ContestID            ContestID            `json:"contestID,omitempty"`
	RequestID            ExecutionRequestID   `json:"requestID,omitempty"`
	ProviderInstanceID   ProviderInstanceID   `json:"providerInstanceID,omitempty"`
	CommandID            CommandID            `json:"commandID"`
	TransportContentType string               `json:"transportContentType"`
	RawTransportDigest   PayloadDigest        `json:"rawTransportDigest"`
	Method               string               `json:"method"`
	Resource             string               `json:"resource"`
	ParticipantID        ParticipantID        `json:"participantID,omitempty"`
	ParticipantVersionID ParticipantVersionID `json:"participantVersionID,omitempty"`
	RepositoryNodeID     string               `json:"repositoryNodeID,omitempty"`
	Commit               string               `json:"commit,omitempty"`
	Path                 string               `json:"path,omitempty"`
	ManifestDigest       ArtifactDigest       `json:"manifestDigest,omitempty"`
	ArtifactDigest       ArtifactDigest       `json:"artifactDigest,omitempty"`
	ByteLimit            uint64               `json:"byteLimit,omitempty"`
}

// VerifiedOperationGrant is produced by a service-owned verifier. The public
// type is a fact carrier for ports and conformance, not a cryptographic claim.
type VerifiedOperationGrant struct {
	Claims OperationGrant `json:"claims"`
}

type OperationGrantRequest struct {
	Claims OperationGrant `json:"claims"`
}

// OperationGrantIssuer models a service's issuance/acquisition boundary after
// confidential-client authentication has happened outside this module.
type OperationGrantIssuer interface {
	IssueOperationGrant(context.Context, OperationGrantRequest) (VerifiedOperationGrant, error)
}

// OperationGrantVerifier models post-JWT verification. Its input is claims
// supplied by the owning transport adapter, never a raw bearer credential.
type OperationGrantVerifier interface {
	VerifyOperationGrant(context.Context, OperationGrant) (VerifiedOperationGrant, error)
}

func ValidateOperationGrant(value OperationGrant) error {
	if value.Issuer == "" || value.Subject == "" || value.Audience == "" || value.Purpose == "" || value.KeyID == "" || value.TokenID == "" || value.IssuedAt.IsZero() || value.NotBefore.IsZero() || value.ExpiresAt.IsZero() || !value.ExpiresAt.After(value.NotBefore) || value.ProviderID == "" || value.AdapterID == "" || value.CommandID == "" || value.TransportContentType == "" || value.RawTransportDigest == "" || value.Method == "" || value.Resource == "" {
		return ErrInvalidGrant
	}
	if value.Purpose == GrantPurposeParticipantVersionValidate {
		if value.CompetitionID != "" || value.ContestID != "" || value.RequestID != "" || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || value.Path == "" || value.ManifestDigest == "" || value.ArtifactDigest == "" || value.ByteLimit == 0 || value.ProviderInstanceID != "" {
			return ErrInvalidGrant
		}
	} else if value.Purpose == GrantPurposeContestLaunch {
		if value.CompetitionID == "" || value.ContestID == "" || value.RequestID == "" || value.ProviderInstanceID != "" || value.ParticipantID != "" || value.ParticipantVersionID != "" || value.RepositoryNodeID != "" || value.ByteLimit != 0 {
			return ErrInvalidGrant
		}
	} else if value.Purpose == GrantPurposeContestStarted || value.Purpose == GrantPurposeContestResultSubmit {
		if value.CompetitionID == "" || value.ContestID == "" || value.RequestID == "" || value.ProviderInstanceID == "" || value.ParticipantID != "" || value.ParticipantVersionID != "" || value.RepositoryNodeID != "" || value.ByteLimit != 0 {
			return ErrInvalidGrant
		}
	} else {
		return ErrInvalidGrant
	}
	return nil
}
