package mediaexecution

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/tidwall/sjson"
)

const (
	QwenImage20ProSync    = "qwen_image20_pro_sync"
	QwenImage30NativeSync = "qwen_image30_native_sync"
	QwenImage30Compatible = "qwen_image30_compatible_sync"
	Wan30Text             = "wan30_text"
	Wan30FirstFrame       = "wan30_first_frame"
	nativeImageSyncPath   = "/api/v1/services/aigc/multimodal-generation/generation"
	compatibleImagePath   = "/compatible-mode/v1/images/generations"
	nativeVideoCreatePath = "/api/v1/services/aigc/video-generation/video-synthesis"
)

type profileDefinition struct {
	name, model, operation, path         string
	asynchronous, compatible, firstFrame bool
}

// Protocol exposes this implementation's exact static wire contract. It is
// descriptive metadata, never registration authority or permission to send.
type Protocol struct {
	Model                  string
	OperationKind          string
	CreatePath             string
	Asynchronous           bool
	OutputIdentityContract string
}

func LookupProfile(name string) (Protocol, error) {
	p, err := registeredProfile(name)
	if err != nil {
		return Protocol{}, err
	}
	return Protocol{Model: p.model, OperationKind: p.operation, CreatePath: p.path,
		Asynchronous: p.asynchronous, OutputIdentityContract: p.outputIdentityContract()}, nil
}

type nativeText struct {
	Text string `json:"text"`
}
type nativeMessage struct {
	Role    string       `json:"role"`
	Content []nativeText `json:"content"`
}

// Protocol paths and wire shapes are from the retained Qwen3 and Wan3 official
// API snapshots, not inferred from the provider name or a client request path.
func registeredProfile(name string) (profileDefinition, error) {
	p := profileDefinition{name: name, model: Model, operation: "image_generate"}
	switch name {
	case Profile:
		p.path, p.asynchronous = CreatePath, true
	case QwenImage20ProSync:
		p.path, p.model = nativeImageSyncPath, "qwen-image-2.0-pro"
	case QwenImage30NativeSync:
		p.path = nativeImageSyncPath
	case QwenImage30Compatible:
		p.path, p.compatible = compatibleImagePath, true
	case Wan30Text, Wan30FirstFrame:
		p.path, p.model, p.operation, p.asynchronous = nativeVideoCreatePath, "wan3.0-video-prime", "video_generate", true
		p.firstFrame = name == Wan30FirstFrame
	default:
		return profileDefinition{}, ErrUnsupported
	}
	return p, nil
}

func (p profileDefinition) prepareBody(original []byte) ([]byte, error) {
	root, err := object(original)
	if err != nil {
		return nil, ErrUnsupported
	}
	if p.compatible {
		// This independently registered profile is Images T2I only. An image,
		// stream, edit, multipart body, or tool-specific request is not substituted.
		for key := range root {
			switch key {
			case "model", "prompt", "n", "size", "negative_prompt", "prompt_extend", "watermark", "seed", "response_format":
			default:
				return nil, ErrUnsupported
			}
		}
		if strings.TrimSpace(field(root, "prompt")) == "" {
			return nil, ErrUnsupported
		}
		return sjson.SetBytes(original, "model", p.model)
	}
	for key := range root {
		if key != "model" && key != "input" && key != "parameters" && !(p.firstFrame && key == "first_frame_url") {
			return nil, ErrUnsupported
		}
	}
	input, err := object(root["input"])
	if err != nil {
		return nil, ErrUnsupported
	}
	body := append([]byte(nil), original...)
	if p.operation == "image_generate" {
		if prompt := field(input, "prompt"); len(input) == 1 && strings.TrimSpace(prompt) != "" {
			// The platform's explicit T2I convenience shape is mapped to the
			// documented native single-user text message; the original basis stays
			// unchanged. Parameters retain their original exact bytes and numbers.
			native, marshalErr := json.Marshal(struct {
				Messages []nativeMessage `json:"messages"`
			}{Messages: []nativeMessage{{Role: "user", Content: []nativeText{{Text: prompt}}}}})
			if marshalErr != nil {
				return nil, ErrUnsupported
			}
			body, err = sjson.SetRawBytes(body, "input", native)
		} else {
			var messages []map[string]json.RawMessage
			if len(input) != 1 || decode(input["messages"], &messages) != nil || len(messages) != 1 ||
				len(messages[0]) != 2 || field(messages[0], "role") != "user" {
				return nil, ErrUnsupported
			}
			var content []map[string]json.RawMessage
			if decode(messages[0]["content"], &content) != nil || len(content) != 1 || len(content[0]) != 1 ||
				strings.TrimSpace(field(content[0], "text")) == "" {
				return nil, ErrUnsupported
			}
		}
	} else {
		if strings.TrimSpace(field(input, "prompt")) == "" || (!p.firstFrame && len(input) != 1) {
			return nil, ErrUnsupported
		}
		if p.firstFrame {
			frame := field(root, "first_frame_url")
			if input["media"] != nil {
				var media []map[string]json.RawMessage
				if frame != "" || len(input) != 2 || decode(input["media"], &media) != nil || len(media) != 1 || len(media[0]) != 2 || field(media[0], "type") != "first_frame" {
					return nil, ErrUnsupported
				}
				// The kernel has retained this independently registered native form.
				// Validate its one original frame and keep its exact bytes below.
				frame = field(media[0], "url")
			} else if len(input) != 1 {
				return nil, ErrUnsupported
			}
			parsed, parseErr := url.Parse(frame)
			if parseErr != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
				return nil, ErrUnsupported
			}
			media, marshalErr := json.Marshal([]struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			}{{Type: "first_frame", URL: frame}})
			if marshalErr != nil {
				return nil, ErrUnsupported
			}
			if input["media"] == nil {
				body, err = sjson.SetRawBytes(body, "input.media", media)
				if err == nil {
					body, err = sjson.DeleteBytes(body, "first_frame_url")
				}
			}
		}
	}
	if err != nil {
		return nil, ErrUnsupported
	}
	return sjson.SetBytes(body, "model", p.model)
}

// Selection proposes the original media D0 frame; it grants no provider send.
// A query attempt comes from the original query command namespace, independently
// of the sole generation attempt. Claims must be actual Console allocations.
func (e *Executor) Selection(live Invocation, queryAttempt string, claims []string) ([]byte, error) {
	if !identity(live.ID) || !decimalID(live.Generation) || (queryAttempt != "" && (!e.profile.asynchronous || !identity(queryAttempt) || queryAttempt == e.original.Dispatch.AttemptID)) {
		return nil, ErrIdentity
	}
	fields := make(map[string]json.RawMessage, len(e.candidate)+8)
	for key, value := range e.candidate {
		fields[key] = append([]byte(nil), value...)
	}
	delete(fields, "selection_digest")
	delete(fields, "recovery_claim_refs")
	for key, value := range map[string]string{
		"attempt_id": e.original.Dispatch.AttemptID, "dispatch_id": e.original.Dispatch.ID,
		"descriptor_digest": field(e.descriptor, "descriptor_digest"), "selection_canonicalization": "selection-json.v1",
		"invocation_id": live.ID, "context_generation": live.Generation, "gateway_boot_id": e.gateway.BootID,
	} {
		fields[key], _ = json.Marshal(value)
	}
	if queryAttempt != "" {
		delete(fields, "dispatch_id")
		for key, value := range map[string]string{"attempt_id": queryAttempt, "operation_kind": "task_query", "mechanism": "task_query", "original_body_digest": bodybasis.DigestBytes(nil)} {
			fields[key], _ = json.Marshal(value)
		}
	}
	if len(claims) > 0 {
		seen := make(map[string]bool, len(claims))
		for _, ref := range claims {
			if !identity(ref) || seen[ref] {
				return nil, ErrIdentity
			}
			seen[ref] = true
		}
		fields["recovery_claim_refs"], _ = json.Marshal(claims)
	}
	unsigned, err := json.Marshal(fields)
	if err != nil {
		return nil, ErrIdentity
	}
	digest, err := mfjson.Digest(unsigned)
	if err != nil {
		return nil, err
	}
	fields["selection_digest"], _ = json.Marshal(digest)
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, ErrIdentity
	}
	return mfjson.Canonicalize(raw)
}

func (e *Executor) DispatchID() string       { return e.original.Dispatch.ID }
func (e *Executor) OperationID() string      { return e.history.OperationID }
func (e *Executor) RuntimeConfigID() string  { return e.original.Runtime }
func (e *Executor) ConfigRevision() string   { return e.original.Revision }
func (e *Executor) DispatchRevision() string { return e.original.Dispatch.Revision }
func (e *Executor) DispatchState() string    { return e.original.Dispatch.State }
func (e *Executor) Descriptor() []byte       { return append([]byte(nil), e.original.Descriptor...) }
func (e *Executor) OriginalHistory() []byte  { return append([]byte(nil), e.original.History...) }
