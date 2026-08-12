package grants

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sneat-co/ext-competios/backend/contract4competios"
)

// TokenLifetime is the fixed access-token lifetime, per Decision 0007.
const TokenLifetime = 5 * time.Minute

// Issuer issues at+jwt operation grants for exactly the purposes configured
// in its Direction. It never signs a purpose outside that set, regardless of
// what a caller requests -- IssueOperationGrant refuses before it ever builds
// a claim set.
type Issuer struct {
	direction Direction
}

// NewIssuer validates direction before returning a usable Issuer. A nil
// return always pairs with a non-nil error.
func NewIssuer(direction Direction) (*Issuer, error) {
	if err := direction.validateForIssue(); err != nil {
		return nil, err
	}
	return &Issuer{direction: direction}, nil
}

// IssueOperationGrant implements contract4competios.OperationGrantIssuer.
func (i *Issuer) IssueOperationGrant(_ context.Context, request contract4competios.OperationGrantRequest) (contract4competios.IssuedOperationAccessToken, error) {
	if i == nil {
		return contract4competios.IssuedOperationAccessToken{}, contract4competios.ErrInvalidGrant
	}
	if !i.direction.permits(request.Purpose) {
		return contract4competios.IssuedOperationAccessToken{}, ErrPurposeNotPermitted
	}
	if err := contract4competios.ValidateOperationGrantRequest(request); err != nil {
		return contract4competios.IssuedOperationAccessToken{}, err
	}

	tokenID, err := newTokenID()
	if err != nil {
		return contract4competios.IssuedOperationAccessToken{}, err
	}
	now := i.direction.now()
	grant := contract4competios.OperationGrant{
		Issuer: i.direction.Issuer, Subject: i.direction.Subject, Audience: i.direction.Audience,
		TokenType: contract4competios.GrantTokenTypeAccessJWT,
		Scope:     contract4competios.GrantScopeForPurpose(request.Purpose), Purpose: request.Purpose,
		KeyID: i.direction.Key.KeyID(), TokenID: tokenID,
		IssuedAt: now, NotBefore: now, ExpiresAt: now.Add(TokenLifetime),
		ProviderID: request.ProviderID, AdapterID: request.AdapterID,
		CompetitionID: request.CompetitionID, ContestID: request.ContestID, RequestID: request.RequestID,
		ProviderInstanceID: request.ProviderInstanceID, CommandID: request.CommandID,
		TypedPayloadDigest: request.TypedPayloadDigest, TransportContentType: request.TransportContentType,
		RawTransportDigest: request.RawTransportDigest, Method: request.Method, Resource: request.Resource,
		ParticipantID: request.ParticipantID, ParticipantVersionID: request.ParticipantVersionID,
		RepositoryNodeID: request.RepositoryNodeID, CommitOID: request.CommitOID, ManifestPath: request.ManifestPath,
		ManifestEntryKind:      request.ManifestEntryKind,
		RawManifestBytesDigest: request.RawManifestBytesDigest, ManifestByteLimit: request.ManifestByteLimit,
		ClosurePlanID: request.ClosurePlanID, ClosurePlanDigest: request.ClosurePlanDigest,
		CandidateTransferredBytesDigest:       request.CandidateTransferredBytesDigest,
		PublicCandidateTransferredBytesDigest: request.PublicCandidateTransferredBytesDigest,
		AggregateByteLimit:                    request.AggregateByteLimit, RetentionReceiptID: request.RetentionReceiptID,
		ArtifactDigest: request.ArtifactDigest, DisclosureReceiptID: request.DisclosureReceiptID,
		DisclosureRequestDigest: request.DisclosureRequestDigest,
	}
	if err := contract4competios.ValidateOperationGrant(grant); err != nil {
		return contract4competios.IssuedOperationAccessToken{}, err
	}

	token := jwt.NewWithClaims(i.direction.Key.Method(), claimsFromGrant(grant))
	token.Header["kid"] = grant.KeyID
	token.Header["typ"] = tokenTypeHeader
	signed, err := token.SignedString(i.direction.Key.Key())
	if err != nil {
		return contract4competios.IssuedOperationAccessToken{}, fmt.Errorf("grants: signing token: %w", err)
	}

	return contract4competios.IssuedOperationAccessToken{
		AccessToken: contract4competios.EncodedAccessToken(signed),
		TokenType:   contract4competios.GrantTokenTypeAccessJWT,
		ExpiresAt:   grant.ExpiresAt,
	}, nil
}

var _ contract4competios.OperationGrantIssuer = (*Issuer)(nil)

// newTokenID returns a fresh random jti: 16 bytes (128 bits) of CSPRNG
// output, hex-encoded. Freshness -- not structure -- is what a jti needs:
// ReplayStore rejects any second presentation of the same value.
func newTokenID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("grants: generating token ID: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
