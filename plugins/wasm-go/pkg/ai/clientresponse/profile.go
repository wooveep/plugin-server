package clientresponse

import "fmt"

// Profile selects presentation rules independently of the wire protocol.
type Profile string

const (
	DeepSeek Profile = "deepseek"
	Qwen     Profile = "qwen"
	OpenAI   Profile = "openai"
)

// ParseProfile validates a frozen response policy at the configuration boundary.
func ParseProfile(value, protocol string) (Profile, error) {
	if protocol != Chat && protocol != Responses && protocol != Messages {
		return "", fmt.Errorf("response profile requires a registered client text protocol: %s", protocol)
	}
	profile := Profile(value)
	if profile == "" {
		profile = DeepSeek
	}
	if profile != DeepSeek && profile != Qwen && profile != OpenAI || protocol == Messages && profile == OpenAI {
		return "", fmt.Errorf("unsupported response profile %q for %s", value, protocol)
	}
	return profile, nil
}
