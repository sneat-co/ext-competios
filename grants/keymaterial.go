package grants

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

// ErrKeyMaterialInvalid is returned by a KeyMaterial constructor when its
// input cannot produce safe signatures (e.g. a secret shorter than 32 bytes).
var ErrKeyMaterialInvalid = errors.New("grants: key material is invalid")

// KeyMaterial is the entire cryptographic seam behind every Issuer and
// Verifier in this package. HMACKey (Season 1) implements it over a shared
// secret; swapping to Ed25519 later means writing ONE new implementation of
// this three-method interface -- claim mapping, verification order, replay,
// and purpose enforcement are all unchanged. This is the "honest contract and
// transport" requirement recorded in Decision 0007's Season 1 amendment.
type KeyMaterial interface {
	// KeyID is carried in the JOSE "kid" header. A verifier resolves an
	// incoming token's kid to the trusted KeyMaterial to check it against;
	// an issuer stamps its own KeyID into every token it signs.
	KeyID() string

	// Method is the JOSE "alg" this key produces/accepts. HS256 for Season 1.
	Method() jwt.SigningMethod

	// Key is the golang-jwt key value passed to Token.SignedString (issuer
	// side) or returned from a parser's Keyfunc (verifier side). For a
	// symmetric method such as HMAC this is the same secret both sides hold;
	// for an asymmetric method it would be a private key for signing and the
	// matching public key for verification -- two different KeyMaterial
	// values sharing one KeyID, still satisfying this same interface.
	Key() any
}

// MinimumHMACSecretLength is the smallest secret NewHMACKey accepts: 32
// bytes gives HMAC-SHA256 a full-strength key per RFC 2104. Callers decode a
// base64 environment secret to this length or more before constructing a key
// -- see ext-competios/grants' consumers for the exact env-var contract.
const MinimumHMACSecretLength = 32

// HMACKey is the Season 1 KeyMaterial: HMAC-SHA256 over a shared secret. The
// same HMACKey value is used by both the issuer that signs and the verifier
// that checks the signature for one direction, one kid.
type HMACKey struct {
	keyID  string
	secret []byte
}

// NewHMACKey validates keyID and secret before returning a usable key. It
// never returns a key an Issuer or Verifier would accept for signing weaker
// than MinimumHMACSecretLength bytes -- callers do not need to re-check the
// secret length themselves.
func NewHMACKey(keyID string, secret []byte) (HMACKey, error) {
	if keyID == "" || len(secret) < MinimumHMACSecretLength {
		return HMACKey{}, ErrKeyMaterialInvalid
	}
	return HMACKey{keyID: keyID, secret: append([]byte(nil), secret...)}, nil
}

func (k HMACKey) KeyID() string             { return k.keyID }
func (k HMACKey) Method() jwt.SigningMethod { return jwt.SigningMethodHS256 }
func (k HMACKey) Key() any                  { return k.secret }

var _ KeyMaterial = HMACKey{}
