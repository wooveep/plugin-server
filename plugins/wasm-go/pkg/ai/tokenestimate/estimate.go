// Package tokenestimate estimates text usage when a provider omits token counts.
package tokenestimate

import (
	"strings"
	"unicode/utf8"

	"github.com/tiktoken-go/tokenizer"
)

// Usage contains estimated input and output counts, excluding unseen output.
type Usage struct {
	InputToken, OutputToken, TotalToken int64
}

// EncodingForModel preserves the existing billing estimator's encoding policy.
func EncodingForModel(model string) tokenizer.Encoding {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, family := range []string{"gpt-4", "gpt-3.5", "gpt-35", "ft:gpt-4", "ft:gpt-3.5", "ft:davinci-002", "ft:babbage-002", "text-embedding-ada-002", "davinci-002", "babbage-002"} {
		if model == family || strings.HasPrefix(model, family+"-") || strings.HasPrefix(model, family+":") {
			return tokenizer.Cl100kBase
		}
	}
	return tokenizer.O200kBase
}

// Codec returns the shared tokenizer for a model's selected encoding.
func Codec(model string) (tokenizer.Codec, error) {
	return tokenizer.Get(EncodingForModel(model))
}

// Count counts available text; an empty model output has zero tokens.
func Count(model, text string) (int64, error) {
	if text == "" {
		return 0, nil
	}
	codec, err := Codec(model)
	if err != nil {
		return 0, err
	}
	// Long unbroken inputs make BPE merge work quadratic. Bounded UTF-8
	// chunks keep estimation practical within the existing 2-MiB body budget.
	// Boundary differences remain explicitly estimated usage.
	const chunkBytes = 1024
	var total int64
	for len(text) > 0 {
		end := len(text)
		if end > chunkBytes {
			end = chunkBytes
			for end > 0 && !utf8.RuneStart(text[end]) {
				end--
			}
		}
		count, err := codec.Count(text[:end])
		if err != nil {
			return 0, err
		}
		total += int64(count)
		text = text[end:]
	}
	return total, nil
}

// Text retains the existing billing estimator's nonempty-input contract.
func Text(model, input, output string) (Usage, bool) {
	if input == "" {
		return Usage{}, false
	}
	in, err := Count(model, input)
	if err != nil {
		return Usage{}, false
	}
	out, err := Count(model, output)
	if err != nil {
		return Usage{}, false
	}
	return Usage{InputToken: in, OutputToken: out, TotalToken: in + out}, in+out > 0
}
