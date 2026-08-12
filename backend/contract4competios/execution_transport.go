package contract4competios

import (
	"strings"
)

// ExecutionLaunchEnvelope is the exact authenticated transport a game issues
// for an execution launch. AccessToken is opaque outside the game. RawBody,
// content type and route facts are forwarded byte-for-byte so the receiving
// game can verify the grant against the request it actually decodes.
type ExecutionLaunchEnvelope struct {
	AccessToken EncodedAccessToken `json:"-"`
	Method      string             `json:"-"`
	Resource    string             `json:"-"`
	ContentType string             `json:"-"`
	RawBody     []byte             `json:"-"`
}

// CloneExecutionLaunchEnvelope copies the mutable raw body so provider and
// consumer cannot accidentally share authority-bearing transport bytes.
func CloneExecutionLaunchEnvelope(value ExecutionLaunchEnvelope) ExecutionLaunchEnvelope {
	value.RawBody = append([]byte(nil), value.RawBody...)
	return value
}

// ValidateExecutionLaunchEnvelope checks only the transient transport shape.
// It deliberately cannot validate the opaque token, decode the request, or
// impose a deployment-specific raw-body limit after allocation.
func ValidateExecutionLaunchEnvelope(value ExecutionLaunchEnvelope) error {
	if value.AccessToken == "" || value.Method == "" || len(value.Method) > 64 || strings.TrimSpace(value.Method) != value.Method || strings.ContainsAny(value.Method, "\r\n") ||
		value.Resource == "" || len(value.Resource) > 2048 || strings.ContainsAny(value.Resource, "\r\n") ||
		value.ContentType == "" || len(value.ContentType) > 256 || strings.TrimSpace(value.ContentType) != value.ContentType || strings.ContainsAny(value.ContentType, "\r\n") ||
		len(value.RawBody) == 0 {
		return ErrInvalidGrant
	}
	for _, value := range []string{value.Method, value.Resource, value.ContentType} {
		for _, char := range value {
			if char < 0x20 || char == 0x7f {
				return ErrInvalidGrant
			}
		}
	}
	for _, char := range value.Method {
		if char <= 0x20 || char >= 0x7f || strings.ContainsRune("()<>@,;:\\\"/[]?={}", char) {
			return ErrInvalidGrant
		}
	}
	return nil
}
