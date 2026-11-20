package clientresponse

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenSignedToolCallRequiresTheOriginalAliasAndUnmodifiedContext(t *testing.T) {
	record := ResourceAlias{Kind: SignedToolCallKind, Public: "call_pf_s_0123456789abcdef0123456789abcdef"}
	expected := SignedToolCall{ProviderID: "native-id", Signature: "fixture-thought-signature"}
	id, err := sealSignedToolCall(&record, expected)
	require.NoError(t, err)
	actual, err := OpenSignedToolCall(record, id)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.NotContains(t, id, record.Original)

	start := len(record.Public) + 1
	ciphertext, err := base64.RawURLEncoding.DecodeString(id[start:])
	require.NoError(t, err)
	ciphertext[len(ciphertext)-1] ^= 1
	_, err = OpenSignedToolCall(record, id[:start]+base64.RawURLEncoding.EncodeToString(ciphertext))
	require.ErrorIs(t, err, errSignedToolCallUnavailable)

	other := ResourceAlias{Kind: SignedToolCallKind, Public: "call_pf_s_abcdef0123456789abcdef0123456789"}
	_, err = sealSignedToolCall(&other, expected)
	require.NoError(t, err)
	_, err = OpenSignedToolCall(other, other.Public+"_"+id[start:])
	require.ErrorIs(t, err, errSignedToolCallUnavailable)
}
