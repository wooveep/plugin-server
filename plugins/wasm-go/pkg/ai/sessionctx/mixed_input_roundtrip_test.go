package sessionctx_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIMessageContentNumbersRoundTrip(t *testing.T) {
	for _, number := range []string{"9007199254740993", "9007199254740993.0000000001", "1e400"} {
		t.Run(number, func(t *testing.T) {
			body := []byte(`{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA","vendor_sequence":` + number + `}}]}`)
			var message sessionctx.OpenAIMessage
			require.NoError(t, json.Unmarshal(body, &message))
			rewritten, err := json.Marshal(message)
			require.NoError(t, err)
			require.Equal(t, number, gjson.GetBytes(rewritten, "content.0.image_url.vendor_sequence").Raw)
		})
	}
	t.Run("invalid trailing JSON stays rejected", func(t *testing.T) {
		var message sessionctx.OpenAIMessage
		require.Error(t, message.UnmarshalJSON([]byte(`{"role":"user","content":[]} {}`)))
	})
}

func TestTextProtocolMixedAttachmentsSurviveMemoryInjection(t *testing.T) {
	type memoryAdapter interface {
		InjectContext([]byte, protocol.InjectableContext) ([]byte, error)
		BuildCacheDigest(protocol.RequestParseInput) (protocol.CacheDigestResult, error)
	}
	for _, tc := range []struct {
		name, body, inputField, outputField, numberField string
		adapter                                          memoryAdapter
	}{
		{
			name:       "openai chat image and audio",
			body:       `{"model":"public","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"},"vendor_sequence":9007199254740993},{"type":"input_audio","input_audio":{"data":"AAA","format":"wav"}}]}],"tools":[{"type":"function","function":{"name":"lookup"}}],"opaque":{"keep":true}}`,
			inputField: "messages.0.content", outputField: "messages.1.content", numberField: "messages.1.content.1.vendor_sequence",
			adapter: protocol.ChatCompletionsAdapter{},
		},
		{
			name:       "anthropic image and document",
			body:       `{"model":"public","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"},"vendor_sequence":9007199254740993},{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"AAA"}}]}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}],"opaque":{"keep":true}}`,
			inputField: "messages", outputField: "messages", numberField: "messages.0.content.1.vendor_sequence",
			adapter: protocol.MessagesAdapter{},
		},
		{
			name:       "openai responses image and file",
			body:       `{"model":"public","input":[{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,AAA","vendor_sequence":9007199254740993},{"type":"input_file","filename":"sample.pdf","file_data":"AAA"}]}],"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],"opaque":{"keep":true}}`,
			inputField: "input", outputField: "input", numberField: "input.0.content.1.vendor_sequence",
			adapter: protocol.ResponsesAdapter{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte(tc.body)
			rewritten, err := tc.adapter.InjectContext(original, protocol.InjectableContext{Text: "retained memory"})
			require.NoError(t, err)
			require.Equal(t, tc.body, string(original), "injection must not mutate the caller's original bytes")
			require.Contains(t, string(rewritten), "retained memory")
			require.Equal(t, "9007199254740993", gjson.GetBytes(rewritten, tc.numberField).Raw)
			require.Equal(t, exactMixedJSON(t, gjson.Get(tc.body, tc.inputField).Raw), exactMixedJSON(t, gjson.GetBytes(rewritten, tc.outputField).Raw))
			for _, field := range []string{"tools", "opaque"} {
				require.Equal(t, exactMixedJSON(t, gjson.Get(tc.body, field).Raw), exactMixedJSON(t, gjson.GetBytes(rewritten, field).Raw))
			}
			before, err := tc.adapter.BuildCacheDigest(protocol.RequestParseInput{Body: original})
			require.NoError(t, err)
			for _, changed := range []string{
				strings.ReplaceAll(tc.body, "AAA", "BBB"),
				strings.Replace(tc.body, "9007199254740993", "9007199254740992", 1),
			} {
				after, err := tc.adapter.BuildCacheDigest(protocol.RequestParseInput{Body: []byte(changed)})
				require.NoError(t, err)
				require.NotEqual(t, before.Digest, after.Digest, "attachments and exact numeric differences must remain part of the request digest")
			}
		})
	}
}

func exactMixedJSON(t *testing.T, raw string) interface{} {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	var value interface{}
	require.NoError(t, decoder.Decode(&value))
	return value
}
