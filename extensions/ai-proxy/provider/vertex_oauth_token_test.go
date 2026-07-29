package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidOAuthBearerToken(t *testing.T) {
	testCases := []struct {
		name  string
		token string
		valid bool
	}{
		{name: "single_alpha", token: "A", valid: true},
		{name: "full_b64token_alphabet", token: "AZaz09-._~+/", valid: true},
		{name: "trailing_padding", token: "token===", valid: true},
		{name: "maximum_length", token: strings.Repeat("A", 8*1024), valid: true},
		{name: "empty", token: "", valid: false},
		{name: "nul", token: "unsafe\x00token", valid: false},
		{name: "unit_separator", token: "unsafe\x1ftoken", valid: false},
		{name: "del", token: "unsafe\x7ftoken", valid: false},
		{name: "leading_space", token: " token", valid: false},
		{name: "trailing_space", token: "token ", valid: false},
		{name: "embedded_space", token: "to ken", valid: false},
		{name: "tab", token: "to\tken", valid: false},
		{name: "newline", token: "to\nken", valid: false},
		{name: "non_ascii", token: "token-令牌", valid: false},
		{name: "invalid_punctuation", token: "token!", valid: false},
		{name: "padding_only", token: "==", valid: false},
		{name: "leading_padding", token: "=token", valid: false},
		{name: "padding_in_middle", token: "to=ken", valid: false},
		{name: "data_after_padding", token: "token==tail", valid: false},
		{name: "overlong", token: strings.Repeat("A", 8*1024+1), valid: false},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.valid, isValidOAuthBearerToken(testCase.token))
		})
	}
}

func TestIsValidOAuthBearerTokenRejectsEveryC0Octet(t *testing.T) {
	for octet := byte(0); octet <= 0x1f; octet++ {
		octet := octet
		t.Run(fmt.Sprintf("0x%02x", octet), func(t *testing.T) {
			require.False(t, isValidOAuthBearerToken("prefix"+string(octet)+"suffix"))
		})
	}
}
