package tokenestimate

import (
	"github.com/stretchr/testify/require"
	"github.com/tiktoken-go/tokenizer"
	"testing"
)

func TestTokenizerEncodingForModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  tokenizer.Encoding
	}{
		{name: "empty model falls back to o200k", model: "", want: tokenizer.O200kBase},
		{name: "unknown model falls back to o200k", model: "provider-new-model", want: tokenizer.O200kBase},
		{name: "non openai compatible model falls back to o200k", model: "qwen-plus", want: tokenizer.O200kBase},
		{name: "newer openai model uses o200k", model: "gpt-4o-mini", want: tokenizer.O200kBase},
		{name: "newly matched openai model uses o200k", model: "gpt-4.1", want: tokenizer.O200kBase},
		{name: "older gpt-4 base uses cl100k", model: "gpt-4", want: tokenizer.Cl100kBase},
		{name: "older gpt-4 dated model uses cl100k", model: "gpt-4-0613", want: tokenizer.Cl100kBase},
		{name: "older gpt-3.5 model uses cl100k", model: "gpt-3.5-turbo", want: tokenizer.Cl100kBase},
		{name: "azure gpt-35 alias uses cl100k", model: "gpt-35-turbo-16k", want: tokenizer.Cl100kBase},
		{name: "fine tuned older model uses cl100k", model: "ft:gpt-3.5-turbo:org:suffix", want: tokenizer.Cl100kBase},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, EncodingForModel(tc.model))
		})
	}
}

func TestTokenizerForModelUsesSelectedEncoding(t *testing.T) {
	codec, err := Codec("provider-new-model")
	require.NoError(t, err)
	require.Equal(t, string(tokenizer.O200kBase), codec.GetName())

	count, err := codec.Count("hello")
	require.NoError(t, err)
	require.Greater(t, count, 0)
}
