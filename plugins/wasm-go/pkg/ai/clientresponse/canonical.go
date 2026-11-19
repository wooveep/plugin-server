package clientresponse

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
)

// Zero-value default avoids generating a package-level runtime init instruction
// across sibling Wasm plugins importing clientresponse (B08).
var identityRandReader io.Reader

// SetIdentityEntropySource swaps the random byte source used to mint public
// resource identifiers (e.g., to strip crypto/rand syscall jitter in benchmarks)
// and returns a restore callback. Passing nil restores crypto/rand.Reader.
func SetIdentityEntropySource(r io.Reader) func() {
	prev := identityRandReader
	identityRandReader = r
	return func() {
		identityRandReader = prev
	}
}

// ProjectedItem pairs a sanitized canonical Item with its allocated public
// item and tool-call identifiers so client writers never parse JSON or mint IDs.
type ProjectedItem struct {
	Item         canonical.Item
	PublicItemID string
	PublicCallID string
}

// ProjectedChoice holds one sanitized candidate output branch and its finish metadata.
type ProjectedChoice struct {
	Index            int
	Items            []ProjectedItem
	Finish           canonical.FinishReason
	Logprobs         json.RawMessage
	IncompleteReason string
}

// CanonicalResponse carries a fully projected, profile-formatted non-streaming
// response ready for one-pass client wire serialization.
// For ProtocolOpenAIResponses, nil WireUsage indicates "usage": null on wire;
// for Chat and Messages, nil WireUsage omits the usage field.
type CanonicalResponse struct {
	Protocol         canonical.Protocol
	Profile          Profile
	PublicModel      string
	PublicID         string
	CreatedAt        int64
	ServiceTier      string
	Choices          []ProjectedChoice
	EchoFields       map[string]json.RawMessage
	WireUsage        json.RawMessage
	IncompleteReason string
	Error            json.RawMessage
}

// CanonicalEvent carries a projected, profile-formatted streaming event and its
// stable public resource identities ready for client SSE serialization.
type CanonicalEvent struct {
	Protocol         canonical.Protocol
	Profile          Profile
	Event            canonical.Event
	PublicID         string
	PublicItemID     string
	PublicCallID     string
	PublicModel      string
	CreatedAt        int64
	ServiceTier      string
	EchoFields       map[string]json.RawMessage
	WireUsage        json.RawMessage
	StartUsage       json.RawMessage
	FinalUsage       json.RawMessage
	IncompleteReason string
	Error            json.RawMessage
	FailureBody      json.RawMessage
	Finish           canonical.FinishReason
}

type projectedSlotIDs struct {
	itemID  string
	callID  string
	toolIdx int
}

// CanonicalStreamProjection maintains per-attempt public identity and profile
// state across the streaming events of a single response.
type CanonicalStreamProjection struct {
	policy             Policy
	protocol           canonical.Protocol
	initErr            error
	publicID           string
	salt               string
	createdAt          int64
	serviceTier        string
	emittedStart       bool
	lastFinish         canonical.FinishReason
	itemSlots          map[[2]int]string
	itemKinds          map[[2]int]canonical.ItemKind
	toolSlots          map[[2]int]projectedSlotIDs
	toolByItem         map[[2]int]projectedSlotIDs
	toolOpen           map[[2]int]bool
	sharedToolOnItem   map[[2]int]bool
	chatChoiceFinished map[int]bool
}

func normalizeCanonicalPolicy(policy Policy) (Policy, canonical.Protocol, error) {
	if policy.PublicModel == "" || (policy.Protocol != Chat && policy.Protocol != Responses && policy.Protocol != Messages) {
		return Policy{}, "", errors.New("public response requires its frozen model and protocol")
	}
	profile, err := ParseProfile(string(policy.Profile), policy.Protocol)
	if err != nil {
		return Policy{}, "", err
	}
	policy.Profile = profile
	if policy.Aliases == nil {
		policy.Aliases = &ResourceAliases{}
	}
	return policy, canonical.Protocol(policy.Protocol), nil
}

func topResourceKind(protocol canonical.Protocol) string {
	switch protocol {
	case canonical.ProtocolOpenAIResponses:
		return "response"
	case canonical.ProtocolAnthropicMessages:
		return "message"
	default:
		return "chat"
	}
}

func incompleteReasonForFinish(cat canonical.FinishCategory) string {
	switch cat {
	case canonical.FinishLength, canonical.FinishContextWindow:
		return "max_output_tokens"
	case canonical.FinishContentFilter:
		return "content_filter"
	default:
		return ""
	}
}

func cloneEchoFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]json.RawMessage, len(fields))
	for k, v := range fields {
		out[k] = bytes.Clone(v)
	}
	return out
}

func sanitizeLogprobs(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return bytes.Clone(raw)
}

func projectAnnotations(annotations []json.RawMessage) ([]json.RawMessage, error) {
	if len(annotations) == 0 {
		return nil, nil
	}
	out := make([]json.RawMessage, 0, len(annotations))
	for _, raw := range annotations {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || obj == nil {
			return nil, errors.New("invalid public annotation")
		}
		sanitized, err := json.Marshal(projectCitation(obj))
		if err != nil {
			return nil, err
		}
		out = append(out, sanitized)
	}
	return out, nil
}

func (a *ResourceAliases) aliasString(kind, original string) (string, error) {
	if a.values == nil {
		a.values = map[[2]string]string{}
	}
	key := [2]string{kind, original}
	public, exists := a.values[key]
	if a.native && !exists {
		public = original
		a.values[key] = public
		if original != "" {
			a.records = append(a.records, ResourceAlias{Kind: kind, Public: public, Original: original})
		}
		return public, nil
	}
	if !exists {
		var random [16]byte
		r := identityRandReader
		if r == nil {
			r = rand.Reader
		}
		if _, err := io.ReadFull(r, random[:]); err != nil {
			return "", errors.New("public resource identity unavailable")
		}
		prefix := map[string]string{
			"chat":         "chatcmpl",
			"response":     "resp",
			"message":      "msg",
			"item":         "item",
			"call":         "call",
			"conversation": "conv",
		}[kind]
		if prefix == "" {
			return "", errors.New("unknown resource identity kind")
		}
		public = prefix + "_pf_" + hex.EncodeToString(random[:])
		a.values[key] = public
		a.records = append(a.records, ResourceAlias{Kind: kind, Public: public, Original: original})
	}
	return public, nil
}

// ProjectCanonical projects a decoded supplier canonical.Response and observed
// original Usage into a client-ready CanonicalResponse with B02 salted slot aliases.
func ProjectCanonical(policy Policy, resp canonical.Response, usage Usage) (CanonicalResponse, error) {
	normPolicy, protocol, err := normalizeCanonicalPolicy(policy)
	if err != nil {
		return CanonicalResponse{}, err
	}

	// Top-level response ID falls back to bare RequestID when missing so
	// persistManagedResourceAliases (:76) skips persisting non-native response IDs.
	topOrigin := resp.ID
	if topOrigin == "" {
		topOrigin = normPolicy.RequestID
	}
	publicID, err := normPolicy.Aliases.aliasString(topResourceKind(protocol), topOrigin)
	if err != nil {
		return CanonicalResponse{}, err
	}

	// B02 per-response salt: prefer non-empty upstream resp.ID, else RequestID, else minted publicID.
	salt := resp.ID
	if salt == "" {
		if normPolicy.RequestID != "" {
			salt = normPolicy.RequestID
		} else {
			salt = publicID
		}
	}

	createdAt := resp.CreatedAt
	if protocol == canonical.ProtocolAnthropicMessages {
		createdAt = 0
	} else if createdAt <= 0 && normPolicy.CreatedAt > 0 {
		createdAt = normPolicy.CreatedAt
	}

	serviceTier := ""
	if normPolicy.ExposeServiceTier {
		serviceTier = resp.ServiceTier
	}

	var echoFields map[string]json.RawMessage
	if protocol == canonical.ProtocolOpenAIResponses {
		echoFields = cloneEchoFields(normPolicy.RequestFields)
	}

	clonedHolder := canonical.Request{Items: nil}
	projectedChoices := make([]ProjectedChoice, len(resp.Choices))
	var topIncomplete string

	for ci, choice := range resp.Choices {
		clonedHolder.Items = choice.Items
		clonedItems := clonedHolder.CloneForAttempt().Items
		projItems := make([]ProjectedItem, len(clonedItems))
		toolIdx := 0

		for ii, item := range clonedItems {
			for pi := range item.Parts {
				if len(item.Parts[pi].Annotations) > 0 {
					anns, err := projectAnnotations(item.Parts[pi].Annotations)
					if err != nil {
						return CanonicalResponse{}, err
					}
					item.Parts[pi].Annotations = anns
				}
			}

			var pubItemID, pubCallID string
			switch item.Kind {
			case canonical.ItemMessage:
				if protocol == canonical.ProtocolOpenAIResponses {
					orig := item.MessageID
					if orig == "" {
						orig = fmt.Sprintf("%s:item:%d:%d", salt, choice.Index, ii)
					}
					pubItemID, err = normPolicy.Aliases.aliasString("item", orig)
					if err != nil {
						return CanonicalResponse{}, err
					}
					item.MessageID = pubItemID
				} else if protocol == canonical.ProtocolOpenAIChat && item.MessageID != "" {
					pubItemID, err = normPolicy.Aliases.aliasString("message", item.MessageID)
					if err != nil {
						return CanonicalResponse{}, err
					}
					item.MessageID = pubItemID
				}
			case canonical.ItemReasoning:
				if item.Reasoning != nil && protocol == canonical.ProtocolOpenAIResponses {
					orig := item.Reasoning.ItemID
					if orig == "" {
						orig = fmt.Sprintf("%s:item:%d:%d", salt, choice.Index, ii)
					}
					pubItemID, err = normPolicy.Aliases.aliasString("item", orig)
					if err != nil {
						return CanonicalResponse{}, err
					}
					item.Reasoning.ItemID = pubItemID
				}
			case canonical.ItemFunctionCall:
				if item.Call != nil {
					origCall := item.Call.CallID
					if origCall == "" {
						origCall = fmt.Sprintf("%s:call:%d:%d", salt, choice.Index, toolIdx)
					}
					pubCallID, err = normPolicy.Aliases.aliasString("call", origCall)
					if err != nil {
						return CanonicalResponse{}, err
					}
					item.Call.CallID = pubCallID

					if protocol == canonical.ProtocolOpenAIResponses {
						origItem := item.Call.ItemID
						if origItem == "" {
							origItem = fmt.Sprintf("%s:item:%d:%d", salt, choice.Index, ii)
						}
						pubItemID, err = normPolicy.Aliases.aliasString("item", origItem)
						if err != nil {
							return CanonicalResponse{}, err
						}
						item.Call.ItemID = pubItemID
					} else if protocol == canonical.ProtocolAnthropicMessages {
						pubItemID = pubCallID
						item.Call.ItemID = pubCallID
					}
					toolIdx++
				}
			}

			projItems[ii] = ProjectedItem{
				Item:         item,
				PublicItemID: pubItemID,
				PublicCallID: pubCallID,
			}
		}

		incReason := incompleteReasonForFinish(choice.Finish.Category)
		if topIncomplete == "" && incReason != "" {
			topIncomplete = incReason
		}
		projectedChoices[ci] = ProjectedChoice{
			Index:            choice.Index,
			Items:            projItems,
			Finish:           choice.Finish,
			Logprobs:         sanitizeLogprobs(choice.Logprobs),
			IncompleteReason: incReason,
		}
	}

	return CanonicalResponse{
		Protocol:         protocol,
		Profile:          normPolicy.Profile,
		PublicModel:      normPolicy.PublicModel,
		PublicID:         publicID,
		CreatedAt:        createdAt,
		ServiceTier:      serviceTier,
		Choices:          projectedChoices,
		EchoFields:       echoFields,
		WireUsage:        usage.wireProfile(normPolicy.Protocol, normPolicy.Profile),
		IncompleteReason: topIncomplete,
	}, nil
}

// NewCanonicalStreamProjection creates a stateful per-attempt stream projection.
func NewCanonicalStreamProjection(policy Policy) *CanonicalStreamProjection {
	normPolicy, protocol, err := normalizeCanonicalPolicy(policy)
	return &CanonicalStreamProjection{
		policy:             normPolicy,
		protocol:           protocol,
		initErr:            err,
		itemSlots:          make(map[[2]int]string),
		itemKinds:          make(map[[2]int]canonical.ItemKind),
		toolSlots:          make(map[[2]int]projectedSlotIDs),
		toolByItem:         make(map[[2]int]projectedSlotIDs),
		toolOpen:           make(map[[2]int]bool),
		sharedToolOnItem:   make(map[[2]int]bool),
		chatChoiceFinished: make(map[int]bool),
	}
}

func (s *CanonicalStreamProjection) ensureEnvelopeIdentity(upstreamID string, createdAt int64, serviceTier string) error {
	if s.publicID == "" {
		topOrigin := upstreamID
		if topOrigin == "" {
			topOrigin = s.policy.RequestID
		}
		pubID, err := s.policy.Aliases.aliasString(topResourceKind(s.protocol), topOrigin)
		if err != nil {
			return err
		}
		s.publicID = pubID
		if upstreamID != "" {
			s.salt = upstreamID
		} else if s.policy.RequestID != "" {
			s.salt = s.policy.RequestID
		} else {
			s.salt = pubID
		}
	}
	if s.protocol != canonical.ProtocolAnthropicMessages {
		if createdAt > 0 && s.createdAt == 0 {
			s.createdAt = createdAt
		}
		if s.createdAt == 0 && s.policy.CreatedAt > 0 {
			s.createdAt = s.policy.CreatedAt
		}
	}
	if s.policy.ExposeServiceTier && serviceTier != "" {
		s.serviceTier = serviceTier
	}
	return nil
}

func (s *CanonicalStreamProjection) buildStartEvent(usage Usage) CanonicalEvent {
	s.emittedStart = true
	ev := canonical.Event{
		Kind:        canonical.EventResponseStart,
		ID:          s.publicID,
		Model:       s.policy.PublicModel,
		CreatedAt:   s.createdAt,
		ServiceTier: s.serviceTier,
		Role:        canonical.RoleAssistant,
	}
	out := CanonicalEvent{
		Protocol:    s.protocol,
		Profile:     s.policy.Profile,
		Event:       ev,
		PublicID:    s.publicID,
		PublicModel: s.policy.PublicModel,
		CreatedAt:   s.createdAt,
		ServiceTier: s.serviceTier,
	}
	switch s.protocol {
	case canonical.ProtocolOpenAIResponses:
		out.EchoFields = cloneEchoFields(s.policy.RequestFields)
	case canonical.ProtocolAnthropicMessages:
		start := usage
		start.Output, start.Total = json.RawMessage("0"), nil
		start.OutputDetails = nil
		wire := start.wireProfile(Messages, s.policy.Profile)
		if len(wire) > 0 && s.policy.Profile == Qwen {
			// M04: Qwen Messages message_start exposes only input_tokens & output_tokens;
			// cache_read_input_tokens and cache_creation_input_tokens are deferred to message_delta.
			var counts map[string]json.RawMessage
			if json.Unmarshal(wire, &counts) == nil && counts != nil {
				wire, _ = json.Marshal(retain(counts, "input_tokens output_tokens"))
			}
		}
		out.StartUsage = wire
	case canonical.ProtocolOpenAIChat:
		if s.policy.IncludeUsage {
			out.WireUsage = json.RawMessage("null")
		}
	}
	return out
}

func (s *CanonicalStreamProjection) resolveItemSlot(choiceIdx, itemIdx int, kind canonical.ItemKind, upstreamItemID string) (string, error) {
	slot := [2]int{choiceIdx, itemIdx}
	if _, recorded := s.itemKinds[slot]; !recorded {
		s.itemKinds[slot] = kind
	}
	if s.protocol != canonical.ProtocolOpenAIResponses {
		if s.protocol == canonical.ProtocolOpenAIChat && upstreamItemID != "" {
			return s.policy.Aliases.aliasString("message", upstreamItemID)
		}
		return "", nil
	}
	if existing, ok := s.itemSlots[slot]; ok {
		return existing, nil
	}
	orig := upstreamItemID
	if orig == "" {
		orig = fmt.Sprintf("%s:item:%d:%d", s.salt, choiceIdx, itemIdx)
	}
	pubID, err := s.policy.Aliases.aliasString("item", orig)
	if err != nil {
		return "", err
	}
	s.itemSlots[slot] = pubID
	return pubID, nil
}

func (s *CanonicalStreamProjection) resolveToolSlot(choiceIdx, itemIdx, toolIdx int, upstreamItemID, upstreamCallID string) (projectedSlotIDs, error) {
	slot := [2]int{choiceIdx, toolIdx}
	s.toolOpen[slot] = true
	itemKey := [2]int{choiceIdx, itemIdx}
	if existing, ok := s.toolSlots[slot]; ok {
		if priorKind, recorded := s.itemKinds[itemKey]; recorded && priorKind != canonical.ItemFunctionCall {
			s.sharedToolOnItem[itemKey] = true
		}
		return existing, nil
	}
	origCall := upstreamCallID
	if origCall == "" {
		origCall = fmt.Sprintf("%s:call:%d:%d", s.salt, choiceIdx, toolIdx)
	}
	pubCallID, err := s.policy.Aliases.aliasString("call", origCall)
	if err != nil {
		return projectedSlotIDs{}, err
	}
	var pubItemID string
	switch s.protocol {
	case canonical.ProtocolOpenAIResponses:
		origItem := upstreamItemID
		if origItem == "" {
			candidateIdx := itemIdx
			if candidateIdx == 0 && toolIdx > 0 {
				candidateIdx = toolIdx
			}
			if _, taken := s.itemKinds[[2]int{choiceIdx, candidateIdx}]; !taken {
				origItem = fmt.Sprintf("%s:item:%d:%d", s.salt, choiceIdx, candidateIdx)
			} else {
				origItem = fmt.Sprintf("%s:item:%d:call:%d", s.salt, choiceIdx, toolIdx)
			}
		}
		pubItemID, err = s.policy.Aliases.aliasString("item", origItem)
		if err != nil {
			return projectedSlotIDs{}, err
		}
	case canonical.ProtocolAnthropicMessages:
		pubItemID = pubCallID
	}
	ids := projectedSlotIDs{itemID: pubItemID, callID: pubCallID, toolIdx: toolIdx}
	s.toolSlots[slot] = ids

	if priorKind, recorded := s.itemKinds[itemKey]; !recorded {
		s.itemKinds[itemKey] = canonical.ItemFunctionCall
		s.toolByItem[itemKey] = ids
		if pubItemID != "" {
			s.itemSlots[itemKey] = pubItemID
		}
	} else if priorKind != canonical.ItemFunctionCall {
		// Upstream decoder reused a message/reasoning ItemIndex (typically 0) for tool calls.
		s.sharedToolOnItem[itemKey] = true
	}
	return ids, nil
}

// Project transforms a single supplier canonical.Event into zero or more
// projected CanonicalEvents. Protocol terminal framing is held until Finish or Fail.
func (s *CanonicalStreamProjection) Project(ev canonical.Event, usage Usage) ([]CanonicalEvent, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}

	if ev.Kind == canonical.EventPing {
		return []CanonicalEvent{{
			Protocol:    s.protocol,
			Profile:     s.policy.Profile,
			Event:       canonical.Event{Kind: canonical.EventPing},
			PublicModel: s.policy.PublicModel,
		}}, nil
	}

	if ev.Kind == canonical.EventProviderError {
		failure := ClassifyFailure(FailureInput{Body: ev.RawError, Client: s.policy.Protocol})
		return s.Fail(failure, usage)
	}

	if err := s.ensureEnvelopeIdentity(ev.ID, ev.CreatedAt, ev.ServiceTier); err != nil {
		return nil, err
	}

	if ev.Kind == canonical.EventResponseStart {
		if s.emittedStart {
			return nil, nil
		}
		return []CanonicalEvent{s.buildStartEvent(usage)}, nil
	}

	if ev.Kind == canonical.EventProtocolDone {
		return nil, nil
	}

	if ev.Kind == canonical.EventChoiceFinish {
		if ev.Finish.Category != canonical.FinishNone {
			s.lastFinish = ev.Finish
		}
		if s.protocol != canonical.ProtocolOpenAIChat {
			return nil, nil
		}
		var out []CanonicalEvent
		if !s.emittedStart {
			out = append(out, s.buildStartEvent(usage))
		}
		s.chatChoiceFinished[ev.ChoiceIndex] = true
		projEv := ev
		projEv.ID = s.publicID
		projEv.Model = s.policy.PublicModel
		projEv.CreatedAt = s.createdAt
		projEv.ServiceTier = s.serviceTier
		projEv.Logprobs = sanitizeLogprobs(ev.Logprobs)
		projEv.Annotations = nil
		var chatWireUsage json.RawMessage
		if s.policy.IncludeUsage {
			chatWireUsage = json.RawMessage("null")
		}
		out = append(out, CanonicalEvent{
			Protocol:         s.protocol,
			Profile:          s.policy.Profile,
			Event:            projEv,
			PublicID:         s.publicID,
			PublicModel:      s.policy.PublicModel,
			CreatedAt:        s.createdAt,
			ServiceTier:      s.serviceTier,
			WireUsage:        chatWireUsage,
			IncompleteReason: incompleteReasonForFinish(ev.Finish.Category),
			Finish:           ev.Finish,
		})
		return out, nil
	}

	var out []CanonicalEvent
	if !s.emittedStart {
		out = append(out, s.buildStartEvent(usage))
	}

	anns, err := projectAnnotations(ev.Annotations)
	if err != nil {
		return nil, err
	}

	var pubItemID, pubCallID string
	switch ev.Kind {
	case canonical.EventTextDelta, canonical.EventRefusalDelta:
		pubItemID, err = s.resolveItemSlot(ev.ChoiceIndex, ev.ItemIndex, canonical.ItemMessage, ev.ItemID)
		if err != nil {
			return nil, err
		}
	case canonical.EventReasoningDelta, canonical.EventReasoningSignatureDelta, canonical.EventReasoningRedacted:
		pubItemID, err = s.resolveItemSlot(ev.ChoiceIndex, ev.ItemIndex, canonical.ItemReasoning, ev.ItemID)
		if err != nil {
			return nil, err
		}
	case canonical.EventToolCallStart, canonical.EventToolCallArgsDelta:
		ids, err := s.resolveToolSlot(ev.ChoiceIndex, ev.ItemIndex, ev.ToolIndex, ev.ItemID, ev.CallID)
		if err != nil {
			return nil, err
		}
		pubItemID, pubCallID = ids.itemID, ids.callID
	case canonical.EventContentBlockStop:
		itemKey := [2]int{ev.ChoiceIndex, ev.ItemIndex}
		toolKey := [2]int{ev.ChoiceIndex, ev.ToolIndex}
		if ev.ToolIndex > 0 || ev.CallID != "" || ev.ToolName != "" {
			if ids, ok := s.toolSlots[toolKey]; ok {
				pubItemID, pubCallID = ids.itemID, ids.callID
				s.toolOpen[toolKey] = false
			}
		} else if ids, ok := s.toolByItem[itemKey]; ok {
			pubItemID, pubCallID = ids.itemID, ids.callID
			s.toolOpen[[2]int{ev.ChoiceIndex, ids.toolIdx}] = false
		} else if s.sharedToolOnItem[itemKey] && s.toolOpen[toolKey] {
			if ids, ok := s.toolSlots[toolKey]; ok {
				pubItemID, pubCallID = ids.itemID, ids.callID
				s.toolOpen[toolKey] = false
			}
		} else if kind := s.itemKinds[itemKey]; kind == canonical.ItemMessage || kind == canonical.ItemReasoning {
			pubItemID = s.itemSlots[itemKey]
		} else if ids, ok := s.toolSlots[toolKey]; ok {
			pubItemID, pubCallID = ids.itemID, ids.callID
			s.toolOpen[toolKey] = false
		}
	}

	projEv := ev
	projEv.ID = s.publicID
	projEv.ItemID = pubItemID
	projEv.CallID = pubCallID
	projEv.Model = s.policy.PublicModel
	projEv.CreatedAt = s.createdAt
	projEv.ServiceTier = s.serviceTier
	projEv.Annotations = anns
	projEv.Logprobs = sanitizeLogprobs(ev.Logprobs)

	var wireUsage json.RawMessage
	if s.protocol == canonical.ProtocolOpenAIChat && s.policy.IncludeUsage {
		wireUsage = json.RawMessage("null")
	}

	out = append(out, CanonicalEvent{
		Protocol:     s.protocol,
		Profile:      s.policy.Profile,
		Event:        projEv,
		PublicID:     s.publicID,
		PublicItemID: pubItemID,
		PublicCallID: pubCallID,
		PublicModel:  s.policy.PublicModel,
		CreatedAt:    s.createdAt,
		ServiceTier:  s.serviceTier,
		WireUsage:    wireUsage,
		Finish:       ev.Finish,
	})
	return out, nil
}

// Finish emits the verified terminal CanonicalEvent(s) carrying profile-projected
// final usage after transport completion and B01 resource-persistence checks.
func (s *CanonicalStreamProjection) Finish(finish canonical.FinishReason, usage Usage) ([]CanonicalEvent, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
	if err := s.ensureEnvelopeIdentity("", 0, ""); err != nil {
		return nil, err
	}
	effective := finish
	if effective.Category == canonical.FinishNone && s.lastFinish.Category != canonical.FinishNone {
		effective = s.lastFinish
	}
	if effective.Category == canonical.FinishNone {
		effective = canonical.FinishReason{Category: canonical.FinishStop, RawReason: "stop"}
	}

	var out []CanonicalEvent
	if !s.emittedStart {
		out = append(out, s.buildStartEvent(usage))
	}

	if s.protocol == canonical.ProtocolOpenAIChat && len(s.chatChoiceFinished) == 0 && finish.Category != canonical.FinishNone {
		var chatWireUsage json.RawMessage
		if s.policy.IncludeUsage {
			chatWireUsage = json.RawMessage("null")
		}
		s.chatChoiceFinished[0] = true
		out = append(out, CanonicalEvent{
			Protocol:    s.protocol,
			Profile:     s.policy.Profile,
			Event:       canonical.Event{Kind: canonical.EventChoiceFinish, ChoiceIndex: 0, ID: s.publicID, Model: s.policy.PublicModel, CreatedAt: s.createdAt, ServiceTier: s.serviceTier, Finish: effective},
			PublicID:    s.publicID,
			PublicModel: s.policy.PublicModel,
			CreatedAt:   s.createdAt,
			ServiceTier: s.serviceTier,
			WireUsage:   chatWireUsage,
			Finish:      effective,
		})
	}

	var finalUsage json.RawMessage
	if s.protocol != canonical.ProtocolOpenAIChat || s.policy.IncludeUsage {
		finalUsage = usage.wireProfile(s.policy.Protocol, s.policy.Profile)
	}

	var echoFields map[string]json.RawMessage
	if s.protocol == canonical.ProtocolOpenAIResponses {
		echoFields = cloneEchoFields(s.policy.RequestFields)
	}

	incReason := incompleteReasonForFinish(effective.Category)
	out = append(out, CanonicalEvent{
		Protocol:         s.protocol,
		Profile:          s.policy.Profile,
		Event:            canonical.Event{Kind: canonical.EventProtocolDone, ID: s.publicID, Model: s.policy.PublicModel, CreatedAt: s.createdAt, ServiceTier: s.serviceTier, Finish: effective},
		PublicID:         s.publicID,
		PublicModel:      s.policy.PublicModel,
		CreatedAt:        s.createdAt,
		ServiceTier:      s.serviceTier,
		EchoFields:       echoFields,
		FinalUsage:       finalUsage,
		IncompleteReason: incReason,
		Finish:           effective,
	})
	return out, nil
}

// Fail emits a sanitized terminal error CanonicalEvent without fabricating a
// successful completion frame. Streaming SSE errors always follow the platform's
// streaming error profile (OpenAI HTTP-only 503 E04 does not alter SSE frames).
func (s *CanonicalStreamProjection) Fail(failure Failure, usage Usage) ([]CanonicalEvent, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
	if err := s.ensureEnvelopeIdentity("", 0, ""); err != nil {
		return nil, err
	}

	streamErrorPolicy := s.policy
	streamErrorPolicy.Profile = DeepSeek

	var failureBody, resourceErr, finalUsage json.RawMessage
	var echoFields map[string]json.RawMessage

	switch s.protocol {
	case canonical.ProtocolOpenAIResponses:
		resourceErr = failure.ResourceError(Responses)
		finalUsage = usage.wireProfile(Responses, s.policy.Profile)
		echoFields = cloneEchoFields(s.policy.RequestFields)
		var env map[string]json.RawMessage
		_ = json.Unmarshal(failure.Body(streamErrorPolicy), &env)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(env["error"], &fields)
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
		fields["type"] = json.RawMessage(`"error"`)
		failureBody, _ = json.Marshal(fields)
	case canonical.ProtocolAnthropicMessages:
		failureBody = failure.Body(streamErrorPolicy)
	case canonical.ProtocolOpenAIChat:
		failureBody = failure.Body(streamErrorPolicy)
		if s.policy.IncludeUsage && s.emittedStart {
			finalUsage = usage.wireProfile(Chat, s.policy.Profile)
		}
	}

	return []CanonicalEvent{{
		Protocol:    s.protocol,
		Profile:     s.policy.Profile,
		Event:       canonical.Event{Kind: canonical.EventProviderError, ID: s.publicID, Model: s.policy.PublicModel, CreatedAt: s.createdAt, ServiceTier: s.serviceTier, RawError: failureBody},
		PublicID:    s.publicID,
		PublicModel: s.policy.PublicModel,
		CreatedAt:   s.createdAt,
		ServiceTier: s.serviceTier,
		EchoFields:  echoFields,
		FinalUsage:  finalUsage,
		Error:       resourceErr,
		FailureBody: failureBody,
	}}, nil
}

// ReadSupplierUsage extracts exact 128-bit presentation Usage directly from raw
// supplier JSON or SSE bytes without converting through intermediate Chat JSON
// or machine ints/floats.
//
// Supported format values: "openai_chat" (or ""), "openai_responses",
// "anthropic_messages", "qwen" / "dashscope_text" / "dashscope_multimodal",
// "gemini" / "gemini_generate_content" (following GE02), and "moonshot".
func ReadSupplierUsage(body []byte, format string, stream bool) Usage {
	switch format {
	case "":
		return ReadUsage(body, Chat, stream)
	case "dashscope_text", "dashscope_multimodal", "qwen":
		return ReadUsage(body, "qwen", stream)
	case "gemini", string(canonical.ProtocolGeminiGenerate):
		return readGeminiSupplierUsage(body, stream)
	case "moonshot":
		return readMoonshotSupplierUsage(body, stream)
	default:
		return ReadUsage(body, format, stream)
	}
}

func readGeminiSupplierUsage(body []byte, stream bool) Usage {
	var latest map[string]json.RawMessage
	replaceSnapshot := func(data []byte) {
		var root, update map[string]json.RawMessage
		if json.Unmarshal(data, &root) != nil || root == nil {
			return
		}
		if json.Unmarshal(root["usageMetadata"], &update) != nil || len(update) == 0 {
			return
		}
		// Align with managed_supplier_google.go:76 & managed_public_supplier_partial.go:90:
		// replace the entire usageMetadata snapshot per frame so stale partial fields
		// from earlier chunks cannot conflict with the terminal totalTokenCount.
		snapshot := make(map[string]json.RawMessage, len(update))
		for k, v := range update {
			if !bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				snapshot[k] = v
			}
		}
		if len(snapshot) > 0 {
			latest = snapshot
		}
	}

	if !stream {
		replaceSnapshot(body)
	} else {
		frames, err := Events(body)
		if err != nil {
			return Usage{}
		}
		for _, frame := range frames {
			if bytes.Equal(frame.Data, []byte("[DONE]")) {
				continue
			}
			replaceSnapshot(frame.Data)
		}
	}
	if len(latest) == 0 {
		return Usage{}
	}

	// GE02: promptTokenCount already includes cachedContentTokenCount (never add cached again);
	// toolUsePromptTokenCount adds to prompt_tokens; candidatesTokenCount + thoughtsTokenCount
	// form completion_tokens (never derive output via total - input).
	wire := map[string]json.RawMessage{
		"prompt_tokens":     latest["promptTokenCount"],
		"completion_tokens": latest["candidatesTokenCount"],
	}
	if total, exists := latest["totalTokenCount"]; exists {
		wire["total_tokens"] = total
	}
	for _, addition := range []struct {
		source      string
		destination string
	}{
		{"toolUsePromptTokenCount", "prompt_tokens"},
		{"thoughtsTokenCount", "completion_tokens"},
	} {
		if additional, exists := latest[addition.source]; exists {
			base, baseOK := number(wire[addition.destination])
			extra, extraOK := number(additional)
			if !baseOK || !extraOK {
				wire[addition.destination] = json.RawMessage("null")
			} else {
				sum := new(big.Int).Add(base, extra)
				if sum.BitLen() > 128 {
					wire[addition.destination] = json.RawMessage("null")
				} else {
					wire[addition.destination] = json.RawMessage(sum.String())
				}
			}
		}
	}
	if thoughts, ok := number(latest["thoughtsTokenCount"]); ok {
		wire["completion_tokens_details"], _ = json.Marshal(map[string]json.RawMessage{
			"reasoning_tokens": json.RawMessage(thoughts.String()),
		})
	}
	if cached, exists := latest["cachedContentTokenCount"]; exists {
		wire["prompt_tokens_details"], _ = json.Marshal(map[string]json.RawMessage{
			"cached_tokens": cached,
		})
	}
	raw, _ := json.Marshal(wire)
	return readUsageObject(raw, Chat)
}

func readMoonshotSupplierUsage(body []byte, stream bool) Usage {
	extractUsage := func(data []byte) map[string]json.RawMessage {
		var root map[string]json.RawMessage
		if json.Unmarshal(data, &root) != nil || root == nil {
			return nil
		}
		var usageObj map[string]json.RawMessage
		if raw, exists := root["usage"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			if json.Unmarshal(raw, &usageObj) == nil && len(usageObj) > 0 {
				return usageObj
			}
		}
		var choices []map[string]json.RawMessage
		if json.Unmarshal(root["choices"], &choices) == nil && len(choices) > 0 {
			if raw, exists := choices[0]["usage"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				if json.Unmarshal(raw, &usageObj) == nil && len(usageObj) > 0 {
					return usageObj
				}
			}
		}
		return nil
	}

	if !stream {
		usageObj := extractUsage(body)
		if len(usageObj) == 0 {
			return Usage{}
		}
		raw, _ := json.Marshal(usageObj)
		return readUsageObject(raw, Chat)
	}

	frames, err := Events(body)
	if err != nil {
		return Usage{}
	}
	var latest map[string]json.RawMessage
	for _, frame := range frames {
		if bytes.Equal(frame.Data, []byte("[DONE]")) {
			continue
		}
		update := extractUsage(frame.Data)
		if update == nil {
			continue
		}
		if latest == nil {
			latest = map[string]json.RawMessage{}
		}
		for k, v := range update {
			if !bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				latest[k] = v
			}
		}
	}
	if len(latest) == 0 {
		return Usage{}
	}
	raw, _ := json.Marshal(latest)
	return readUsageObject(raw, Chat)
}
