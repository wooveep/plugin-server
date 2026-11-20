package clientresponse

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	SignedToolCallKind       = "signed_call"
	SignedToolCallPrefix     = "call_pf_s_"
	signedToolCallResourceID = len(SignedToolCallPrefix) + 32
	maxSignedToolCallBytes   = 16 * 1024
	gcmOverhead              = 12 + 16
)

var errSignedToolCallUnavailable = errors.New("signed tool call is unavailable")

// SignedToolCall is the Gemini context carried by the client. An empty
// ProviderID means the supplier omitted the optional function call ID.
type SignedToolCall struct {
	ProviderID string `json:"provider_id,omitempty"`
	Signature  string `json:"signature"`
}

func sealSignedToolCall(record *ResourceAlias, call SignedToolCall) (string, error) {
	plain, err := json.Marshal(call)
	if err != nil || len(plain) > maxSignedToolCallBytes {
		return "", errSignedToolCallUnavailable
	}
	// The random private association ID also keys its single envelope. The
	// registry receives the two IDs and the owner. The client carries the seal.
	var privateID [32]byte
	if _, err := rand.Read(privateID[:]); err != nil {
		return "", errSignedToolCallUnavailable
	}
	gcm, err := signedToolCallCipher(privateID[:])
	if err != nil {
		return "", err
	}
	record.Original = hex.EncodeToString(privateID[:])
	sealed := gcm.Seal(nil, nil, plain, []byte(record.Public))
	return record.Public + "_" + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// SignedToolCallResourceID extracts the short alias for the ID-only registry.
func SignedToolCallResourceID(value string) (string, bool) {
	if !strings.HasPrefix(value, SignedToolCallPrefix) ||
		len(value) <= signedToolCallResourceID+1 ||
		len(value) > signedToolCallResourceID+1+base64.RawURLEncoding.EncodedLen(maxSignedToolCallBytes+gcmOverhead) ||
		value[signedToolCallResourceID] != '_' {
		return "", false
	}
	return value[:signedToolCallResourceID], true
}

// OpenSignedToolCall verifies the client context using its resolved private
// association ID. The short public ID binds the seal to the association.
func OpenSignedToolCall(record ResourceAlias, value string) (SignedToolCall, error) {
	id, valid := SignedToolCallResourceID(value)
	if !valid || record.Kind != SignedToolCallKind || record.Public != id {
		return SignedToolCall{}, errSignedToolCallUnavailable
	}
	privateID, err := hex.DecodeString(record.Original)
	if err != nil || len(privateID) != 32 {
		return SignedToolCall{}, errSignedToolCallUnavailable
	}
	gcm, err := signedToolCallCipher(privateID)
	if err != nil {
		return SignedToolCall{}, err
	}
	sealed, err := base64.RawURLEncoding.Strict().DecodeString(value[signedToolCallResourceID+1:])
	if err != nil {
		return SignedToolCall{}, errSignedToolCallUnavailable
	}
	plain, err := gcm.Open(nil, nil, sealed, []byte(id))
	if err != nil {
		return SignedToolCall{}, errSignedToolCallUnavailable
	}
	var call SignedToolCall
	if json.Unmarshal(plain, &call) != nil || call.Signature == "" {
		return SignedToolCall{}, errSignedToolCallUnavailable
	}
	return call, nil
}

func signedToolCallCipher(privateID []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(privateID)
	if err != nil {
		return nil, errSignedToolCallUnavailable
	}
	return cipher.NewGCMWithRandomNonce(block)
}
