package grants

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// tokenTypeHeader is the fixed RFC 9068 access-token JOSE "typ" value.
// contract4competios.GrantTokenTypeAccessJWT already fixes this at the
// contract layer; this constant is the literal JWT header spelling.
const tokenTypeHeader = "at+jwt"

// ErrMalformedClaims is returned when a token's claims cannot be mapped onto
// contract4competios.OperationGrant (e.g. a non-singular audience). It is
// always wrapped under contract4competios.ErrInvalidGrant.
var ErrMalformedClaims = errors.New("grants: claims cannot be decoded into an operation grant")

// accessTokenClaims maps contract4competios.OperationGrant onto RFC 9068
// access-token JWT claims one-to-one:
//
//   - registered claims (iss/sub/aud/jti/iat/nbf/exp) come from the
//     correspondingly named grant fields -- OperationGrant.TokenID becomes
//     the JWT "jti" (jwt.RegisteredClaims.ID);
//   - OperationGrant.Scope becomes the JWT "scope" claim;
//   - OperationGrant.KeyID travels in the JOSE header "kid", not as a claim,
//     because a verifier must resolve it to a key BEFORE the signed claims
//     can be trusted;
//   - OperationGrant.TokenType is not carried as a claim either: it is
//     already the JOSE "typ" header value, verified before claims are even
//     parsed (see Verifier.VerifyOperationGrant) -- carrying it twice would
//     let the header and the claim disagree;
//   - every remaining business-binding field is a namespaced private claim
//     (RFC 7519 SS4.3), prefixed "cg_" (Competios Grant) so it cannot collide
//     with a registered or public claim name.
type accessTokenClaims struct {
	jwt.RegisteredClaims

	Scope contract4competios.GrantScope `json:"scope"`

	Purpose                               contract4competios.GrantPurpose                            `json:"cg_purpose"`
	ProviderID                            contract4competios.ProviderID                              `json:"cg_provider_id"`
	AdapterID                             contract4competios.AdapterID                               `json:"cg_adapter_id"`
	CompetitionID                         contract4competios.CompetitionID                           `json:"cg_competition_id,omitempty"`
	ContestID                             contract4competios.ContestID                               `json:"cg_contest_id,omitempty"`
	RequestID                             contract4competios.ExecutionRequestID                      `json:"cg_request_id,omitempty"`
	ProviderInstanceID                    contract4competios.ProviderInstanceID                      `json:"cg_provider_instance_id,omitempty"`
	CommandID                             contract4competios.CommandID                               `json:"cg_command_id"`
	TypedPayloadDigest                    contract4competios.PayloadDigest                           `json:"cg_typed_payload_digest"`
	TransportContentType                  string                                                     `json:"cg_transport_content_type"`
	RawTransportDigest                    contract4competios.PayloadDigest                           `json:"cg_raw_transport_digest"`
	Method                                string                                                     `json:"cg_method"`
	Resource                              string                                                     `json:"cg_resource"`
	ParticipantID                         contract4competios.ParticipantID                           `json:"cg_participant_id,omitempty"`
	ParticipantVersionID                  contract4competios.ParticipantVersionID                    `json:"cg_participant_version_id,omitempty"`
	RepositoryNodeID                      string                                                     `json:"cg_repository_node_id,omitempty"`
	CommitOID                             contract4competios.SourceObjectID                          `json:"cg_commit_oid,omitempty"`
	ManifestPath                          string                                                     `json:"cg_manifest_path,omitempty"`
	ManifestEntryKind                     contract4competios.SourceEntryKind                         `json:"cg_manifest_entry_kind,omitempty"`
	RawManifestBytesDigest                contract4competios.ArtifactDigest                          `json:"cg_raw_manifest_bytes_digest,omitempty"`
	ManifestByteLimit                     uint64                                                     `json:"cg_manifest_byte_limit,omitempty"`
	ClosurePlanID                         contract4competios.ClosurePlanID                           `json:"cg_closure_plan_id,omitempty"`
	ClosurePlanDigest                     contract4competios.PayloadDigest                           `json:"cg_closure_plan_digest,omitempty"`
	CandidateTransferredBytesDigest       contract4competios.ArtifactDigest                          `json:"cg_candidate_transferred_bytes_digest,omitempty"`
	PublicCandidateTransferredBytesDigest contract4competios.ArtifactDigest                          `json:"cg_public_candidate_transferred_bytes_digest,omitempty"`
	AggregateByteLimit                    uint64                                                     `json:"cg_aggregate_byte_limit,omitempty"`
	RetentionReceiptID                    contract4competios.ArtifactRetentionReceiptID              `json:"cg_retention_receipt_id,omitempty"`
	ArtifactDigest                        contract4competios.ArtifactDigest                          `json:"cg_artifact_digest,omitempty"`
	DisclosureReceiptID                   contract4competios.ArtifactDisclosureVerificationReceiptID `json:"cg_disclosure_receipt_id,omitempty"`
	DisclosureRequestDigest               contract4competios.PayloadDigest                           `json:"cg_disclosure_request_digest,omitempty"`
}

// claimsFromGrant builds the exact wire claims for an already-assembled,
// already-ValidateOperationGrant-checked grant. It is the issuer's only path
// to a signable claim set.
func claimsFromGrant(g contract4competios.OperationGrant) accessTokenClaims {
	return accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.Issuer,
			Subject:   g.Subject,
			Audience:  jwt.ClaimStrings{g.Audience},
			ID:        g.TokenID,
			IssuedAt:  jwt.NewNumericDate(g.IssuedAt),
			NotBefore: jwt.NewNumericDate(g.NotBefore),
			ExpiresAt: jwt.NewNumericDate(g.ExpiresAt),
		},
		Scope:                                 g.Scope,
		Purpose:                               g.Purpose,
		ProviderID:                            g.ProviderID,
		AdapterID:                             g.AdapterID,
		CompetitionID:                         g.CompetitionID,
		ContestID:                             g.ContestID,
		RequestID:                             g.RequestID,
		ProviderInstanceID:                    g.ProviderInstanceID,
		CommandID:                             g.CommandID,
		TypedPayloadDigest:                    g.TypedPayloadDigest,
		TransportContentType:                  g.TransportContentType,
		RawTransportDigest:                    g.RawTransportDigest,
		Method:                                g.Method,
		Resource:                              g.Resource,
		ParticipantID:                         g.ParticipantID,
		ParticipantVersionID:                  g.ParticipantVersionID,
		RepositoryNodeID:                      g.RepositoryNodeID,
		CommitOID:                             g.CommitOID,
		ManifestPath:                          g.ManifestPath,
		ManifestEntryKind:                     g.ManifestEntryKind,
		RawManifestBytesDigest:                g.RawManifestBytesDigest,
		ManifestByteLimit:                     g.ManifestByteLimit,
		ClosurePlanID:                         g.ClosurePlanID,
		ClosurePlanDigest:                     g.ClosurePlanDigest,
		CandidateTransferredBytesDigest:       g.CandidateTransferredBytesDigest,
		PublicCandidateTransferredBytesDigest: g.PublicCandidateTransferredBytesDigest,
		AggregateByteLimit:                    g.AggregateByteLimit,
		RetentionReceiptID:                    g.RetentionReceiptID,
		ArtifactDigest:                        g.ArtifactDigest,
		DisclosureReceiptID:                   g.DisclosureReceiptID,
		DisclosureRequestDigest:               g.DisclosureRequestDigest,
	}
}

// grantFromClaims decodes a verifier-side claim set back into an
// OperationGrant. keyID is the KID that resolved to the KeyMaterial the
// signature was actually checked against (never trust a "kid" claim inside
// the payload itself, only the JOSE header value the verifier already used).
// The caller (Verifier) always runs contract4competios.ValidateOperationGrant
// on the result before trusting it.
func grantFromClaims(c accessTokenClaims, keyID string) (contract4competios.OperationGrant, error) {
	if len(c.Audience) != 1 || c.Audience[0] == "" {
		return contract4competios.OperationGrant{}, ErrMalformedClaims
	}
	return contract4competios.OperationGrant{
		Issuer:                                c.Issuer,
		Subject:                               c.Subject,
		Audience:                              c.Audience[0],
		TokenType:                             contract4competios.GrantTokenTypeAccessJWT,
		Scope:                                 c.Scope,
		Purpose:                               c.Purpose,
		KeyID:                                 keyID,
		TokenID:                               c.ID,
		IssuedAt:                              numericDateTime(c.IssuedAt),
		NotBefore:                             numericDateTime(c.NotBefore),
		ExpiresAt:                             numericDateTime(c.ExpiresAt),
		ProviderID:                            c.ProviderID,
		AdapterID:                             c.AdapterID,
		CompetitionID:                         c.CompetitionID,
		ContestID:                             c.ContestID,
		RequestID:                             c.RequestID,
		ProviderInstanceID:                    c.ProviderInstanceID,
		CommandID:                             c.CommandID,
		TypedPayloadDigest:                    c.TypedPayloadDigest,
		TransportContentType:                  c.TransportContentType,
		RawTransportDigest:                    c.RawTransportDigest,
		Method:                                c.Method,
		Resource:                              c.Resource,
		ParticipantID:                         c.ParticipantID,
		ParticipantVersionID:                  c.ParticipantVersionID,
		RepositoryNodeID:                      c.RepositoryNodeID,
		CommitOID:                             c.CommitOID,
		ManifestPath:                          c.ManifestPath,
		ManifestEntryKind:                     c.ManifestEntryKind,
		RawManifestBytesDigest:                c.RawManifestBytesDigest,
		ManifestByteLimit:                     c.ManifestByteLimit,
		ClosurePlanID:                         c.ClosurePlanID,
		ClosurePlanDigest:                     c.ClosurePlanDigest,
		CandidateTransferredBytesDigest:       c.CandidateTransferredBytesDigest,
		PublicCandidateTransferredBytesDigest: c.PublicCandidateTransferredBytesDigest,
		AggregateByteLimit:                    c.AggregateByteLimit,
		RetentionReceiptID:                    c.RetentionReceiptID,
		ArtifactDigest:                        c.ArtifactDigest,
		DisclosureReceiptID:                   c.DisclosureReceiptID,
		DisclosureRequestDigest:               c.DisclosureRequestDigest,
	}, nil
}

func numericDateTime(value *jwt.NumericDate) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.Time.UTC()
}
