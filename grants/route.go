package grants

import "github.com/sneat-co/ext-competios/backend/contract4competios"

// RouteBinding builds the OperationRouteBinding a caller compares a verified
// grant against -- the last verification step Decision 0007 leaves to the
// caller: "route binding + payload digests". It exists so every caller
// derives OperationRouteBinding's twelve fields the same way from the same
// Direction that produced the Verifier, instead of re-deriving the field
// list by hand at each call site (see contract4competios.OperationRouteBinding
// and the per-purpose contract4competios.ValidateXGrantForRequest helpers,
// which this function feeds).
//
// It refuses to build a binding for a purpose the Direction does not permit,
// so a caller cannot accidentally construct a route that a Verifier built
// from the same Direction would never actually match.
func RouteBinding(
	direction Direction,
	purpose contract4competios.GrantPurpose,
	providerID contract4competios.ProviderID,
	adapterID contract4competios.AdapterID,
	contentType string,
	rawBody []byte,
	method, resource string,
) (contract4competios.OperationRouteBinding, error) {
	scope := contract4competios.GrantScopeForPurpose(purpose)
	if scope == "" || !direction.permits(purpose) {
		return contract4competios.OperationRouteBinding{}, ErrPurposeNotPermitted
	}
	return contract4competios.OperationRouteBinding{
		Issuer: direction.Issuer, Subject: direction.Subject, Audience: direction.Audience,
		TokenType: contract4competios.GrantTokenTypeAccessJWT, Scope: scope, Purpose: purpose,
		ProviderID: providerID, AdapterID: adapterID,
		TransportContentType: contentType,
		RawTransportDigest:   contract4competios.DigestRawTransportBody(contentType, rawBody),
		Method:               method, Resource: resource,
	}, nil
}
