package test

import "strings"

// BodyBasisContract is a fixed offline wire handoff for native host tests.
// Each real plugin handler must match the next stage's complete input. It is
// neither a business observer nor an alternative to deployed filter checks.
type BodyBasisContract struct {
	ClientProtocol string
	LargeHistory   bool
	PriorInputs    []string
	Original       string
	AfterHistory   string
	AfterMemory    string
}

func BodyBasisContracts() []BodyBasisContract {
	return []BodyBasisContract{
		{
			ClientProtocol: "openai_chat",
			Original:       `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"messages":[{"role":"system","content":"original-system"},{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA","detail":"high"},"vendor_sequence":9007199254740993}]}]}`,
			AfterHistory:   `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"messages":[{"role":"system","content":"original-system"},{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"},{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA","detail":"high"},"vendor_sequence":9007199254740993}]}]}`,
			AfterMemory:    `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"messages":[{"role":"system","content":"original-system"},{"role":"system","content":"memory-context"},{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"},{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA","detail":"high"},"vendor_sequence":9007199254740993}]}]}`,
		},
		{
			ClientProtocol: "anthropic_messages",
			Original:       `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"system":[{"type":"text","text":"original-system","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"},"vendor_sequence":9007199254740993}]}]}`,
			AfterHistory:   `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"system":[{"type":"text","text":"original-system","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"},{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"},"vendor_sequence":9007199254740993}]}]}`,
			AfterMemory:    `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"system":[{"type":"text","text":"original-system","cache_control":{"type":"ephemeral"}},{"type":"text","text":"system: memory-context"}],"messages":[{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"},{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"},"vendor_sequence":9007199254740993}]}]}`,
		},
		{
			ClientProtocol: "openai_responses",
			Original:       `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"instructions":"original-system","input":[{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,AAA","detail":"high","vendor_sequence":9007199254740993}]}]}`,
			AfterHistory:   `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"instructions":"original-system","input":[{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"},{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,AAA","detail":"high","vendor_sequence":9007199254740993}]}]}`,
			AfterMemory:    `{"model":"public-model","seed":9007199254740993,"opaque":{"decimal":9007199254740993.0000000001},"tools":[],"instructions":"original-system\n\nsystem: memory-context","input":[{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"},{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,AAA","detail":"high","vendor_sequence":9007199254740993}]}]}`,
		},
	}
}

// LargeHistoryBodyBasisContracts keeps the client below the original 1 MiB
// capture budget, then adds two actual stored history turns. Each attachment
// was individually small; their combined downstream request exceeds 1 MiB.
func LargeHistoryBodyBasisContracts() []BodyBasisContract {
	cases := BodyBasisContracts()
	const previous = `{"role":"user","content":"previous"},{"role":"assistant","content":"previous answer"}`
	for i := range cases {
		c := &cases[i]
		c.LargeHistory = true
		c.Original = strings.ReplaceAll(c.Original, "AAA", strings.Repeat("A", 400_000))
		c.AfterHistory = strings.ReplaceAll(c.AfterHistory, "AAA", strings.Repeat("A", 400_000))
		c.AfterMemory = strings.ReplaceAll(c.AfterMemory, "AAA", strings.Repeat("A", 400_000))
		var block string
		switch c.ClientProtocol {
		case "openai_chat":
			block = `{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("H", 400_000) + `"},"vendor_sequence":9007199254740993}`
		case "anthropic_messages":
			block = `{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + strings.Repeat("H", 400_000) + `"},"vendor_sequence":9007199254740993}`
		case "openai_responses":
			block = `{"type":"input_image","image_url":"data:image/png;base64,` + strings.Repeat("H", 400_000) + `","vendor_sequence":9007199254740993}`
		}
		textType := "text"
		if c.ClientProtocol == "openai_responses" {
			textType = "input_text"
		}
		prior := `{"role":"user","content":[{"type":"` + textType + `","text":"previous"},` + block + `]}`
		c.PriorInputs = []string{prior, prior}
		pair := prior + `,{"role":"assistant","content":"previous answer"}`
		c.AfterHistory = strings.Replace(c.AfterHistory, previous, pair+","+pair, 1)
		c.AfterMemory = strings.Replace(c.AfterMemory, previous, pair+","+pair, 1)
	}
	return cases
}
