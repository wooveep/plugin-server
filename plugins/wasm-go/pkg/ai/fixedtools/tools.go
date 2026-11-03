// Package fixedtools identifies registered supplier tools, excluding client functions.
package fixedtools

// Tool is a stable capability and billing meter identifier.
type Tool string

const (
	WebSearch       Tool = "web_search"
	WebExtractor    Tool = "web_extractor"
	CodeInterpreter Tool = "code_interpreter"
	ImageSearch     Tool = "image_search"
	WebSearchImage  Tool = "web_search_image"
)

// Valid recognizes only the fixed server-tool catalog.
func Valid(tool Tool) bool {
	switch tool {
	case WebSearch, WebExtractor, CodeInterpreter, ImageSearch, WebSearchImage:
		return true
	default:
		return false
	}
}
