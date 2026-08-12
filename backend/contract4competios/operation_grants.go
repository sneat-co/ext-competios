package contract4competios

import (
	"context"
	"time"
)

type GrantPurpose string
type GrantScope string
type GrantTokenType string

const (
	GrantPurposeManifestClosurePlan      GrantPurpose = "manifest-closure-plan"
	GrantPurposeCandidateValidateRetain  GrantPurpose = "candidate-closure-validate-retain"
	GrantPurposeArtifactPublish          GrantPurpose = "artifact-publish"
	GrantPurposeArtifactDisclosureVerify GrantPurpose = "artifact-disclosure-verify"
	GrantPurposeContestLaunch            GrantPurpose = "contest-launch"
	GrantPurposeContestStarted           GrantPurpose = "contest-started-event"
	GrantPurposeContestResultSubmit      GrantPurpose = "contest-result-event"

	GrantScopeManifestClosurePlan      GrantScope = "participant.version.manifest.plan"
	GrantScopeCandidateValidateRetain  GrantScope = "participant.version.validate-and-retain"
	GrantScopeArtifactPublish          GrantScope = "participant.version.publish"
	GrantScopeArtifactDisclosureVerify GrantScope = "participant.version.disclosure.match"
	GrantScopeContestLaunch            GrantScope = "competition.contest.launch"
	GrantScopeContestStarted           GrantScope = "competition.contest.started"
	GrantScopeContestResultSubmit      GrantScope = "competition.contest.result.submit"

	GrantTokenTypeAccessJWT GrantTokenType = "at+jwt"
)

// OperationGrant is a post-verification claim set. It has no bearer-token
// field: cryptographic parsing and HTTP extraction remain in owning services.
type OperationGrant struct {
	Issuer                                string                     `json:"issuer"`
	Subject                               string                     `json:"subject"`
	Audience                              string                     `json:"audience"`
	TokenType                             GrantTokenType             `json:"tokenType"`
	Scope                                 GrantScope                 `json:"scope"`
	Purpose                               GrantPurpose               `json:"purpose"`
	KeyID                                 string                     `json:"keyID"`
	TokenID                               string                     `json:"tokenID"`
	IssuedAt                              time.Time                  `json:"issuedAt"`
	NotBefore                             time.Time                  `json:"notBefore"`
	ExpiresAt                             time.Time                  `json:"expiresAt"`
	ProviderID                            ProviderID                 `json:"providerID"`
	AdapterID                             AdapterID                  `json:"adapterID"`
	CompetitionID                         CompetitionID              `json:"competitionID,omitempty"`
	ContestID                             ContestID                  `json:"contestID,omitempty"`
	RequestID                             ExecutionRequestID         `json:"requestID,omitempty"`
	ProviderInstanceID                    ProviderInstanceID         `json:"providerInstanceID,omitempty"`
	CommandID                             CommandID                  `json:"commandID"`
	TypedPayloadDigest                    PayloadDigest              `json:"typedPayloadDigest"`
	TransportContentType                  string                     `json:"transportContentType"`
	RawTransportDigest                    PayloadDigest              `json:"rawTransportDigest"`
	Method                                string                     `json:"method"`
	Resource                              string                     `json:"resource"`
	ParticipantID                         ParticipantID              `json:"participantID,omitempty"`
	ParticipantVersionID                  ParticipantVersionID       `json:"participantVersionID,omitempty"`
	RepositoryNodeID                      string                     `json:"repositoryNodeID,omitempty"`
	Commit                                string                     `json:"commit,omitempty"`
	ManifestPath                          string                     `json:"manifestPath,omitempty"`
	ManifestEntryKind                     SourceEntryKind            `json:"manifestEntryKind,omitempty"`
	RawManifestBytesDigest                ArtifactDigest             `json:"rawManifestBytesDigest,omitempty"`
	ManifestByteLimit                     uint64                     `json:"manifestByteLimit,omitempty"`
	ClosurePlanID                         ClosurePlanID              `json:"closurePlanID,omitempty"`
	ClosurePlanDigest                     PayloadDigest              `json:"closurePlanDigest,omitempty"`
	CandidateTransferredBytesDigest       ArtifactDigest             `json:"candidateTransferredBytesDigest,omitempty"`
	PublicCandidateTransferredBytesDigest ArtifactDigest             `json:"publicCandidateTransferredBytesDigest,omitempty"`
	AggregateByteLimit                    uint64                     `json:"aggregateByteLimit,omitempty"`
	RetentionReceiptID                    ArtifactRetentionReceiptID `json:"retentionReceiptID,omitempty"`
	ArtifactDigest                        ArtifactDigest             `json:"artifactDigest,omitempty"`
}

type VerifiedOperationGrant struct {
	Claims OperationGrant `json:"claims"`
}

// EncodedAccessToken is opaque to this module. Owning services choose and
// verify its encoding; the public contract never parses JWT/OAuth/JWKS data.
type EncodedAccessToken string

// OperationGrantRequest contains only the exact business operation requested
// by an authenticated client. The issuer—not the caller—selects issuer,
// subject, audience, token type, scope, signing key, token ID and lifetime.
type OperationGrantRequest struct {
	Purpose                               GrantPurpose               `json:"purpose"`
	ProviderID                            ProviderID                 `json:"providerID"`
	AdapterID                             AdapterID                  `json:"adapterID"`
	CompetitionID                         CompetitionID              `json:"competitionID,omitempty"`
	ContestID                             ContestID                  `json:"contestID,omitempty"`
	RequestID                             ExecutionRequestID         `json:"requestID,omitempty"`
	ProviderInstanceID                    ProviderInstanceID         `json:"providerInstanceID,omitempty"`
	CommandID                             CommandID                  `json:"commandID"`
	TypedPayloadDigest                    PayloadDigest              `json:"typedPayloadDigest"`
	TransportContentType                  string                     `json:"transportContentType"`
	RawTransportDigest                    PayloadDigest              `json:"rawTransportDigest"`
	Method                                string                     `json:"method"`
	Resource                              string                     `json:"resource"`
	ParticipantID                         ParticipantID              `json:"participantID,omitempty"`
	ParticipantVersionID                  ParticipantVersionID       `json:"participantVersionID,omitempty"`
	RepositoryNodeID                      string                     `json:"repositoryNodeID,omitempty"`
	Commit                                string                     `json:"commit,omitempty"`
	ManifestPath                          string                     `json:"manifestPath,omitempty"`
	ManifestEntryKind                     SourceEntryKind            `json:"manifestEntryKind,omitempty"`
	RawManifestBytesDigest                ArtifactDigest             `json:"rawManifestBytesDigest,omitempty"`
	ManifestByteLimit                     uint64                     `json:"manifestByteLimit,omitempty"`
	ClosurePlanID                         ClosurePlanID              `json:"closurePlanID,omitempty"`
	ClosurePlanDigest                     PayloadDigest              `json:"closurePlanDigest,omitempty"`
	CandidateTransferredBytesDigest       ArtifactDigest             `json:"candidateTransferredBytesDigest,omitempty"`
	PublicCandidateTransferredBytesDigest ArtifactDigest             `json:"publicCandidateTransferredBytesDigest,omitempty"`
	AggregateByteLimit                    uint64                     `json:"aggregateByteLimit,omitempty"`
	RetentionReceiptID                    ArtifactRetentionReceiptID `json:"retentionReceiptID,omitempty"`
	ArtifactDigest                        ArtifactDigest             `json:"artifactDigest,omitempty"`
}

type IssuedOperationAccessToken struct {
	AccessToken EncodedAccessToken `json:"-"`
	TokenType   GrantTokenType     `json:"tokenType"`
	ExpiresAt   time.Time          `json:"expiresAt"`
}

type OperationGrantIssuer interface {
	IssueOperationGrant(context.Context, OperationGrantRequest) (IssuedOperationAccessToken, error)
}

type OperationGrantVerifier interface {
	VerifyOperationGrant(context.Context, EncodedAccessToken) (VerifiedOperationGrant, error)
}

// OperationRouteBinding pins the stable route and trust-domain facts. TokenID
// and times deliberately remain outside it so a fresh short-lived token can
// retry the same durable command after an unknown outcome.
type OperationRouteBinding struct {
	Issuer               string
	Subject              string
	Audience             string
	TokenType            GrantTokenType
	Scope                GrantScope
	Purpose              GrantPurpose
	ProviderID           ProviderID
	AdapterID            AdapterID
	TransportContentType string
	RawTransportDigest   PayloadDigest
	Method               string
	Resource             string
}

func ValidateOperationGrant(value OperationGrant) error {
	if value.Issuer == "" || value.Subject == "" || value.Audience == "" || value.TokenType != GrantTokenTypeAccessJWT || value.Scope == "" || value.Purpose == "" || value.KeyID == "" || value.TokenID == "" || value.IssuedAt.IsZero() || value.NotBefore.IsZero() || value.ExpiresAt.IsZero() || value.NotBefore.Before(value.IssuedAt) || !value.ExpiresAt.After(value.NotBefore) || value.ProviderID == "" || value.AdapterID == "" || value.CommandID == "" || !validSHA256Digest(string(value.TypedPayloadDigest)) || value.TransportContentType == "" || !validSHA256Digest(string(value.RawTransportDigest)) || value.Method == "" || value.Resource == "" {
		return ErrInvalidGrant
	}
	if !validGrantPurposeScope(value.Purpose, value.Scope) {
		return ErrInvalidGrant
	}
	sourceFields := value.ParticipantID != "" || value.ParticipantVersionID != "" || value.RepositoryNodeID != "" || value.Commit != "" || value.ManifestPath != "" || value.ManifestEntryKind != "" || value.RawManifestBytesDigest != "" || value.ManifestByteLimit != 0 || value.ClosurePlanID != "" || value.ClosurePlanDigest != "" || value.CandidateTransferredBytesDigest != "" || value.PublicCandidateTransferredBytesDigest != "" || value.AggregateByteLimit != 0 || value.RetentionReceiptID != "" || value.ArtifactDigest != ""
	switch value.Purpose {
	case GrantPurposeManifestClosurePlan:
		if hasExecutionBinding(value) || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || value.ManifestPath == "" || value.ManifestEntryKind != SourceEntryRegular || !validSHA256Digest(string(value.RawManifestBytesDigest)) || value.ManifestByteLimit == 0 || value.ClosurePlanID != "" || value.ClosurePlanDigest != "" || value.CandidateTransferredBytesDigest != "" || value.PublicCandidateTransferredBytesDigest != "" || value.AggregateByteLimit != 0 || value.RetentionReceiptID != "" || value.ArtifactDigest != "" {
			return ErrInvalidGrant
		}
	case GrantPurposeCandidateValidateRetain:
		if hasExecutionBinding(value) || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || value.ManifestPath != "" || value.ManifestEntryKind != "" || value.RawManifestBytesDigest != "" || value.ManifestByteLimit != 0 || value.ClosurePlanID == "" || !validSHA256Digest(string(value.ClosurePlanDigest)) || !validSHA256Digest(string(value.CandidateTransferredBytesDigest)) || value.PublicCandidateTransferredBytesDigest != "" || value.AggregateByteLimit == 0 || value.RetentionReceiptID != "" || value.ArtifactDigest != "" {
			return ErrInvalidGrant
		}
	case GrantPurposeArtifactPublish:
		if hasExecutionBinding(value) || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID != "" || value.Commit != "" || value.ManifestPath != "" || value.ManifestEntryKind != "" || value.RawManifestBytesDigest != "" || value.ManifestByteLimit != 0 || value.ClosurePlanID != "" || value.ClosurePlanDigest != "" || value.CandidateTransferredBytesDigest != "" || value.PublicCandidateTransferredBytesDigest != "" || value.AggregateByteLimit != 0 || value.RetentionReceiptID == "" || !validSHA256Digest(string(value.ArtifactDigest)) {
			return ErrInvalidGrant
		}
	case GrantPurposeArtifactDisclosureVerify:
		if hasExecutionBinding(value) || value.ParticipantID == "" || value.ParticipantVersionID == "" || value.RepositoryNodeID == "" || value.Commit == "" || value.ManifestPath != "" || value.ManifestEntryKind != "" || value.RawManifestBytesDigest != "" || value.ManifestByteLimit != 0 || value.ClosurePlanID == "" || !validSHA256Digest(string(value.ClosurePlanDigest)) || value.CandidateTransferredBytesDigest != "" || !validSHA256Digest(string(value.PublicCandidateTransferredBytesDigest)) || value.AggregateByteLimit == 0 || value.RetentionReceiptID == "" || !validSHA256Digest(string(value.ArtifactDigest)) {
			return ErrInvalidGrant
		}
	case GrantPurposeContestLaunch:
		if value.CompetitionID == "" || value.ContestID == "" || value.RequestID == "" || value.ProviderInstanceID != "" || sourceFields {
			return ErrInvalidGrant
		}
	case GrantPurposeContestStarted, GrantPurposeContestResultSubmit:
		if value.CompetitionID == "" || value.ContestID == "" || value.RequestID == "" || value.ProviderInstanceID == "" || sourceFields {
			return ErrInvalidGrant
		}
	default:
		return ErrInvalidGrant
	}
	return nil
}

func (value OperationGrant) RequestedOperation() OperationGrantRequest {
	return OperationGrantRequest{
		Purpose: value.Purpose, ProviderID: value.ProviderID, AdapterID: value.AdapterID,
		CompetitionID: value.CompetitionID, ContestID: value.ContestID, RequestID: value.RequestID,
		ProviderInstanceID: value.ProviderInstanceID, CommandID: value.CommandID,
		TypedPayloadDigest: value.TypedPayloadDigest, TransportContentType: value.TransportContentType,
		RawTransportDigest: value.RawTransportDigest, Method: value.Method, Resource: value.Resource,
		ParticipantID: value.ParticipantID, ParticipantVersionID: value.ParticipantVersionID,
		RepositoryNodeID: value.RepositoryNodeID, Commit: value.Commit, ManifestPath: value.ManifestPath,
		ManifestEntryKind:      value.ManifestEntryKind,
		RawManifestBytesDigest: value.RawManifestBytesDigest, ManifestByteLimit: value.ManifestByteLimit,
		ClosurePlanID: value.ClosurePlanID, ClosurePlanDigest: value.ClosurePlanDigest,
		CandidateTransferredBytesDigest:       value.CandidateTransferredBytesDigest,
		PublicCandidateTransferredBytesDigest: value.PublicCandidateTransferredBytesDigest,
		AggregateByteLimit:                    value.AggregateByteLimit, RetentionReceiptID: value.RetentionReceiptID,
		ArtifactDigest: value.ArtifactDigest,
	}
}

func ValidateOperationGrantRequest(value OperationGrantRequest) error {
	now := time.Unix(1, 0).UTC()
	grant := OperationGrant{
		Issuer: "issuer", Subject: "subject", Audience: "audience",
		TokenType: GrantTokenTypeAccessJWT, Scope: grantScopeForPurpose(value.Purpose), Purpose: value.Purpose,
		KeyID: "key", TokenID: "token", IssuedAt: now, NotBefore: now, ExpiresAt: now.Add(time.Minute),
		ProviderID: value.ProviderID, AdapterID: value.AdapterID,
		CompetitionID: value.CompetitionID, ContestID: value.ContestID, RequestID: value.RequestID,
		ProviderInstanceID: value.ProviderInstanceID, CommandID: value.CommandID,
		TypedPayloadDigest: value.TypedPayloadDigest, TransportContentType: value.TransportContentType,
		RawTransportDigest: value.RawTransportDigest, Method: value.Method, Resource: value.Resource,
		ParticipantID: value.ParticipantID, ParticipantVersionID: value.ParticipantVersionID,
		RepositoryNodeID: value.RepositoryNodeID, Commit: value.Commit, ManifestPath: value.ManifestPath,
		ManifestEntryKind:      value.ManifestEntryKind,
		RawManifestBytesDigest: value.RawManifestBytesDigest, ManifestByteLimit: value.ManifestByteLimit,
		ClosurePlanID: value.ClosurePlanID, ClosurePlanDigest: value.ClosurePlanDigest,
		CandidateTransferredBytesDigest:       value.CandidateTransferredBytesDigest,
		PublicCandidateTransferredBytesDigest: value.PublicCandidateTransferredBytesDigest,
		AggregateByteLimit:                    value.AggregateByteLimit, RetentionReceiptID: value.RetentionReceiptID,
		ArtifactDigest: value.ArtifactDigest,
	}
	return ValidateOperationGrant(grant)
}

func ValidateIssuedOperationGrantForRequest(value OperationGrant, request OperationGrantRequest) error {
	if ValidateOperationGrant(value) != nil || ValidateOperationGrantRequest(request) != nil || value.Scope != grantScopeForPurpose(request.Purpose) || value.RequestedOperation() != request {
		return ErrInvalidGrant
	}
	return nil
}

func hasExecutionBinding(value OperationGrant) bool {
	return value.CompetitionID != "" || value.ContestID != "" || value.RequestID != "" || value.ProviderInstanceID != ""
}

func validGrantPurposeScope(purpose GrantPurpose, scope GrantScope) bool {
	return grantScopeForPurpose(purpose) == scope && scope != ""
}

func grantScopeForPurpose(purpose GrantPurpose) GrantScope {
	switch purpose {
	case GrantPurposeManifestClosurePlan:
		return GrantScopeManifestClosurePlan
	case GrantPurposeCandidateValidateRetain:
		return GrantScopeCandidateValidateRetain
	case GrantPurposeArtifactPublish:
		return GrantScopeArtifactPublish
	case GrantPurposeArtifactDisclosureVerify:
		return GrantScopeArtifactDisclosureVerify
	case GrantPurposeContestLaunch:
		return GrantScopeContestLaunch
	case GrantPurposeContestStarted:
		return GrantScopeContestStarted
	case GrantPurposeContestResultSubmit:
		return GrantScopeContestResultSubmit
	default:
		return ""
	}
}

// ValidateExactOperationGrant is the verifier-side equality guard. Every claim
// is comparable and bound, including token replay identity and time window.
func ValidateExactOperationGrant(actual, expected OperationGrant) error {
	if ValidateOperationGrant(actual) != nil || ValidateOperationGrant(expected) != nil || actual != expected {
		return ErrInvalidGrant
	}
	return nil
}

func validateRoute(value OperationGrant, route OperationRouteBinding) error {
	if route.Issuer == "" || route.Subject == "" || route.Audience == "" || route.TokenType == "" || route.Scope == "" || route.Purpose == "" || route.ProviderID == "" || route.AdapterID == "" || route.TransportContentType == "" || !validSHA256Digest(string(route.RawTransportDigest)) || route.Method == "" || route.Resource == "" {
		return ErrInvalidGrant
	}
	if value.Issuer != route.Issuer || value.Subject != route.Subject || value.Audience != route.Audience || value.TokenType != route.TokenType || value.Scope != route.Scope || value.Purpose != route.Purpose || value.ProviderID != route.ProviderID || value.AdapterID != route.AdapterID || value.TransportContentType != route.TransportContentType || value.RawTransportDigest != route.RawTransportDigest || value.Method != route.Method || value.Resource != route.Resource {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateLaunchGrantForRequest(grant VerifiedOperationGrant, route OperationRouteBinding, request ExecutionRequest) error {
	value := grant.Claims
	if ValidateOperationGrant(value) != nil || ValidateExecutionRequest(request) != nil || validateRoute(value, route) != nil || value.Purpose != GrantPurposeContestLaunch || value.Scope != GrantScopeContestLaunch || value.ProviderID != request.ProviderID || value.AdapterID != request.AdapterID || value.CompetitionID != request.CompetitionID || value.ContestID != request.ContestID || value.RequestID != request.ID || value.CommandID != request.CommandID || value.TypedPayloadDigest != request.TypedPayloadDigest {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateManifestClosurePlanGrantForRequest(grant VerifiedOperationGrant, route OperationRouteBinding, request ManifestClosurePlanRequest) error {
	value := grant.Claims
	if ValidateOperationGrant(value) != nil || ValidateManifestClosurePlanRequest(request) != nil || validateRoute(value, route) != nil || value.Purpose != GrantPurposeManifestClosurePlan || value.Scope != GrantScopeManifestClosurePlan || value.ProviderID != request.ProviderID || value.AdapterID != request.AdapterID || value.CommandID != request.CommandID || value.TypedPayloadDigest != request.TypedPayloadDigest || value.ParticipantID != request.ParticipantID || value.ParticipantVersionID != request.ParticipantVersionID || value.RepositoryNodeID != request.RepositoryNodeID || value.Commit != request.Commit || value.ManifestPath != request.ManifestPath || value.ManifestEntryKind != request.ManifestEntryKind || value.RawManifestBytesDigest != request.RawManifestBytesDigest || value.ManifestByteLimit != request.ManifestByteLimit {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateCandidateRetentionGrantForRequest(grant VerifiedOperationGrant, route OperationRouteBinding, request CandidateClosureRetentionRequest) error {
	value := grant.Claims
	if ValidateOperationGrant(value) != nil || ValidateCandidateClosureRetentionRequest(request) != nil || validateRoute(value, route) != nil || value.Purpose != GrantPurposeCandidateValidateRetain || value.Scope != GrantScopeCandidateValidateRetain || value.ProviderID != request.ProviderID || value.AdapterID != request.AdapterID || value.CommandID != request.CommandID || value.TypedPayloadDigest != request.TypedPayloadDigest || value.ParticipantID != request.ParticipantID || value.ParticipantVersionID != request.ParticipantVersionID || value.RepositoryNodeID != request.RepositoryNodeID || value.Commit != request.Commit || value.ClosurePlanID != request.ClosurePlanID || value.ClosurePlanDigest != request.ClosurePlanDigest || value.CandidateTransferredBytesDigest != request.CandidateTransferredBytesDigest || value.AggregateByteLimit != request.AggregateByteLimit {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateArtifactPublicationGrantForRequest(grant VerifiedOperationGrant, route OperationRouteBinding, request ArtifactPublicationRequest) error {
	value := grant.Claims
	if ValidateOperationGrant(value) != nil || ValidateArtifactPublicationRequest(request) != nil || validateRoute(value, route) != nil || value.Purpose != GrantPurposeArtifactPublish || value.Scope != GrantScopeArtifactPublish || value.ProviderID != request.ProviderID || value.AdapterID != request.AdapterID || value.CommandID != request.CommandID || value.TypedPayloadDigest != request.TypedPayloadDigest || value.ParticipantID != request.ParticipantID || value.ParticipantVersionID != request.ParticipantVersionID || value.RetentionReceiptID != request.RetentionReceiptID || value.ArtifactDigest != request.ArtifactDigest {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateArtifactDisclosureGrantForRequest(grant VerifiedOperationGrant, route OperationRouteBinding, request ArtifactDisclosureVerificationRequest) error {
	value := grant.Claims
	if ValidateOperationGrant(value) != nil || ValidateArtifactDisclosureVerificationRequest(request) != nil || validateRoute(value, route) != nil || value.Purpose != GrantPurposeArtifactDisclosureVerify || value.Scope != GrantScopeArtifactDisclosureVerify || value.ProviderID != request.ProviderID || value.AdapterID != request.AdapterID || value.CommandID != request.CommandID || value.TypedPayloadDigest != request.TypedPayloadDigest || value.ParticipantID != request.ParticipantID || value.ParticipantVersionID != request.ParticipantVersionID || value.RepositoryNodeID != request.RepositoryNodeID || value.Commit != request.Commit || value.ClosurePlanID != request.ClosurePlanID || value.ClosurePlanDigest != request.ClosurePlanDigest || value.PublicCandidateTransferredBytesDigest != request.PublicCandidateTransferredBytesDigest || value.AggregateByteLimit != request.AggregateByteLimit || value.RetentionReceiptID != request.RetentionReceiptID || value.ArtifactDigest != request.ArtifactDigest {
		return ErrInvalidGrant
	}
	return nil
}

func ValidateEventGrantForEvent(grant VerifiedOperationGrant, route OperationRouteBinding, event ExecutionEvent) error {
	value := grant.Claims
	if ValidateOperationGrant(value) != nil || ValidateExecutionEvent(event) != nil || validateRoute(value, route) != nil || value.ProviderID != event.ProviderID || value.AdapterID != event.AdapterID || value.CompetitionID != event.CompetitionID || value.ContestID != event.ContestID || value.RequestID != event.RequestID || value.ProviderInstanceID != event.ProviderInstanceID || value.CommandID != event.CommandID || value.TypedPayloadDigest != event.TypedPayloadDigest {
		return ErrInvalidGrant
	}
	wantPurpose, wantScope := eventGrantPurpose(event.Kind)
	if value.Purpose != wantPurpose || value.Scope != wantScope {
		return ErrInvalidGrant
	}
	return nil
}

func eventGrantPurpose(kind LifecycleEventKind) (GrantPurpose, GrantScope) {
	if kind == LifecycleEventStarted {
		return GrantPurposeContestStarted, GrantScopeContestStarted
	}
	if kind == LifecycleEventCompleted || kind == LifecycleEventFailed || kind == LifecycleEventCancelled {
		return GrantPurposeContestResultSubmit, GrantScopeContestResultSubmit
	}
	return "", ""
}
