package grants

import (
	"errors"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewHMACKeyRejectsShortSecret(t *testing.T) {
	_, err := NewHMACKey("kid", []byte(strings.Repeat("x", MinimumHMACSecretLength-1)))
	if !errors.Is(err, ErrKeyMaterialInvalid) {
		t.Fatalf("err = %v, want ErrKeyMaterialInvalid", err)
	}
}

func TestNewHMACKeyRejectsEmptyKeyID(t *testing.T) {
	_, err := NewHMACKey("", []byte(strings.Repeat("x", MinimumHMACSecretLength)))
	if !errors.Is(err, ErrKeyMaterialInvalid) {
		t.Fatalf("err = %v, want ErrKeyMaterialInvalid", err)
	}
}

func TestNewHMACKeyAccepts32ByteSecret(t *testing.T) {
	key, err := NewHMACKey("kid-a", []byte(strings.Repeat("x", MinimumHMACSecretLength)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.KeyID() != "kid-a" {
		t.Fatalf("KeyID() = %q, want kid-a", key.KeyID())
	}
	if key.Method() != jwt.SigningMethodHS256 {
		t.Fatalf("Method() = %v, want HS256", key.Method())
	}
	secret, ok := key.Key().([]byte)
	if !ok || len(secret) != MinimumHMACSecretLength {
		t.Fatalf("Key() = %v, want %d-byte secret", key.Key(), MinimumHMACSecretLength)
	}
}

// TestHMACKeySecretIsCopied proves NewHMACKey does not alias the caller's
// backing array -- mutating the slice passed in must not change the key.
func TestHMACKeySecretIsCopied(t *testing.T) {
	secret := []byte(strings.Repeat("x", MinimumHMACSecretLength))
	key, err := NewHMACKey("kid", secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secret[0] = 'Z'
	stored := key.Key().([]byte)
	if stored[0] == 'Z' {
		t.Fatalf("HMACKey aliased the caller's secret slice")
	}
}
