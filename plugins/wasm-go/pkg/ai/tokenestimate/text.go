package tokenestimate

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

// RequestText reads model input from the final provider request, excluding metadata.
func RequestText(body []byte) string {
	object := gjson.ParseBytes(body)
	var parts []string
	for _, key := range []string{"instructions", "system", "systemInstruction", "messages", "contents", "input", "prompt"} {
		appendText(&parts, object.Get(key))
	}
	// Tool schemas and tool-call arguments are model input even without text blocks.
	for _, key := range []string{"tools", "functions", "response_format"} {
		if value := object.Get(key); value.Exists() && value.Type != gjson.Null {
			parts = append(parts, value.Raw)
		}
	}
	return strings.Join(parts, "\n")
}

func appendText(parts *[]string, value gjson.Result) {
	switch {
	case value.Type == gjson.String:
		*parts = append(*parts, value.String())
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool { appendText(parts, item); return true })
	case value.IsObject():
		kind := value.Get("type").String()
		if kind != "" && kind != "text" && kind != "input_text" && kind != "output_text" && kind != "message" && kind != "function_call" && kind != "function_call_output" && kind != "tool_use" && kind != "tool_result" {
			return
		}
		for _, key := range []string{"text", "content", "parts", "arguments", "output", "messages", "contents"} {
			appendText(parts, value.Get(key))
		}
		for _, key := range []string{"tool_calls", "function_call", "input"} {
			if item := value.Get(key); item.Exists() {
				*parts = append(*parts, item.Raw)
			}
		}
	}
}

// Documents returns complete JSON objects from a normal response or buffered SSE.
// Incomplete trailing stream data cannot create output or provider usage.
func Documents(body []byte) []gjson.Result {
	if !utf8.Valid(body) {
		return nil
	}
	if gjson.ValidBytes(body) {
		return []gjson.Result{gjson.ParseBytes(body)}
	}
	var documents []gjson.Result
	for _, event := range bytes.Split(bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n")), []byte("\n\n")) {
		var data []byte
		for _, line := range bytes.Split(event, []byte("\n")) {
			if bytes.HasPrefix(line, []byte("data:")) {
				if len(data) != 0 {
					data = append(data, '\n')
				}
				data = append(data, bytes.TrimSpace(line[5:])...)
			}
		}
		if gjson.ValidBytes(data) {
			documents = append(documents, gjson.ParseBytes(data))
		}
	}
	return documents
}

// ResponseText collects only available model content, never provider error text.
func ResponseText(body []byte) string {
	var parts []string
	var responseDone []string
	var responseFinal gjson.Result
	for _, object := range Documents(body) {
		if object.Get("error").Exists() {
			continue
		}
		switch object.Get("type").String() {
		case "response.output_text.delta", "response.function_call_arguments.delta":
			appendText(&parts, object.Get("delta"))
			continue
		case "response.output_text.done":
			appendText(&responseDone, object.Get("text"))
			continue
		case "response.completed", "response.incomplete", "response.failed":
			if output := object.Get("response.output"); output.IsArray() && len(output.Array()) > 0 {
				responseFinal = output
			}
			continue
		}
		for _, key := range []string{"output_text", "output", "content", "delta.text", "delta.content"} {
			appendText(&parts, object.Get(key))
		}
		for _, choices := range []gjson.Result{object.Get("choices"), object.Get("output.choices")} {
			choices.ForEach(func(_, choice gjson.Result) bool {
				for _, key := range []string{"text", "message", "delta"} {
					appendText(&parts, choice.Get(key))
				}
				return true
			})
		}
		object.Get("candidates").ForEach(func(_, candidate gjson.Result) bool { appendText(&parts, candidate.Get("content")); return true })
	}
	if len(parts) == 0 {
		if responseFinal.Exists() {
			appendText(&parts, responseFinal)
		} else {
			parts = responseDone
		}
	}
	return strings.Join(parts, "")
}
