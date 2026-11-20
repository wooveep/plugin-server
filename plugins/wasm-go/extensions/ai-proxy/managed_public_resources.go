package main

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	supplier "github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func managedResourceScope(s *managedExecutionState) resourceref.Scope {
	return resourceref.Scope{Tenant: s.identity.TenantID, Principal: s.identity.PrincipalID, SalesAsset: s.config.release.SalesAssetID, Passthrough: s.config.settings.ProtocolMode == "passthrough"}
}

func managedResourceOwner(c *managedExecutionConfig) resourceref.Owner {
	d := c.release.Descriptor
	account := d.ProviderAccountID
	if account == "" && c.release.CredentialRef != "" {
		// Account identity is optional in a valid execution descriptor. A
		// selected credential reference is an exact, narrower boundary; its
		// secret and generation do not belong in the resource mapping.
		account = "credential:" + c.release.CredentialRef
	}
	return resourceref.Owner{Provider: d.ProviderID, Account: account, Region: d.RegionEndpointRef, Model: d.UpstreamModel, Protocol: c.profile.UpstreamProtocol}
}

func managedResourceConfigMatches(c *managedExecutionConfig, owner resourceref.Owner) bool {
	if managedResourceOwner(c) == owner {
		return true
	}
	if c.release.Descriptor.ProviderAccountID != "" || !strings.HasPrefix(owner.Account, "credential:") {
		return false
	}
	ref := strings.TrimPrefix(owner.Account, "credential:")
	// A not-yet-selected v2 pool retains its complete verified reference
	// directory. Checking a copy here does not select or consume a member.
	for _, member := range c.release.Credentials {
		if member.CredentialRef == ref {
			copy := *c
			copy.release.CredentialRef = ref
			return managedResourceOwner(&copy) == owner
		}
	}
	return false
}

func managedResourceCredentialAllowed(s *managedExecutionState, ref string) bool {
	return s.resourceOwner == nil || !strings.HasPrefix(s.resourceOwner.Account, "credential:") || s.resourceOwner.Account == "credential:"+ref
}

func managedNativeResponses(c *managedExecutionConfig) bool {
	return c.profile.UpstreamProtocol == clientresponse.Responses && (c.vendor == nil || c.vendor.NativeResponseFormat() == "")
}

func persistManagedResourceAliases(ctx wrapper.HttpContext, s *managedExecutionState, done func(error)) bool {
	if !managedPublicText(s) || s.publicAliases == nil || s.resourcesSaved {
		return false
	}
	responses := s.config.profile.InputProtocol == clientresponse.Responses
	if !responses && !s.publicAliases.HasSignedToolCalls() {
		return false
	}
	if s.resourcesSaving {
		return true
	}
	aliases := s.publicAliases.Records()
	if len(aliases) == 0 {
		return false
	}
	owner := managedResourceOwner(s.config)
	records := make([]resourceref.Record, 0, len(aliases))
	for _, alias := range aliases {
		if !responses && alias.Kind != clientresponse.SignedToolCallKind {
			continue
		}
		// A locally missing ID can be replaced for presentation, but it does not
		// create a native resource that can legally be referenced later.
		if alias.Original == "" || alias.Original == s.callID {
			continue
		}
		record := resourceref.NewRecord(alias, owner, managedNativeResponses(s.config))
		record.Passthrough = s.config.settings.ProtocolMode == "passthrough"
		records = append(records, record)
	}
	if len(records) == 0 {
		return false
	}
	s.resourcesSaving = true
	resourceref.Save(s.config.referenceRedis, managedResourceScope(s), records, func(err error) {
		if managedTextStopped(s) {
			return
		}
		s.resourcesSaving, s.resourcesSaved = false, err == nil
		done(err)
	})
	return true
}

type managedResourceInput struct {
	value     gjson.Result
	alias     clientresponse.ResourceAlias
	reference bool
}

func restoreManagedResourceReferences(ctx wrapper.HttpContext, s *managedExecutionState, body []byte, ready func([]byte)) {
	protocol := s.config.profile.InputProtocol
	passthrough := s.config.settings.ProtocolMode == "passthrough"
	if !managedPublicText(s) || (protocol != clientresponse.Responses && passthrough) {
		ready(body)
		return
	}
	var inputs []managedResourceInput
	add := func(kind string, reference bool, value gjson.Result) bool {
		if !value.Exists() || value.Type == gjson.Null {
			return true
		}
		if !passthrough && kind == "call" && strings.HasPrefix(value.String(), clientresponse.SignedToolCallPrefix) {
			id, valid := clientresponse.SignedToolCallResourceID(value.String())
			if !valid {
				return false
			}
			inputs = append(inputs, managedResourceInput{
				value: value,
				alias: clientresponse.ResourceAlias{Kind: clientresponse.SignedToolCallKind, Public: id},
			})
			return true
		}
		if protocol != clientresponse.Responses {
			return true
		}
		if !passthrough && !resourceref.IsAlias(kind, value.String()) {
			return !reference
		}
		inputs = append(inputs, managedResourceInput{value, clientresponse.ResourceAlias{Kind: kind, Public: value.String()}, reference})
		return true
	}
	valid := true
	switch protocol {
	case clientresponse.Responses:
		valid = add("response", true, gjson.GetBytes(body, "previous_response_id"))
		if conversation := gjson.GetBytes(body, "conversation"); conversation.IsObject() {
			valid = add("conversation", true, conversation.Get("id")) && valid
		} else {
			valid = add("conversation", true, conversation) && valid
		}
		for _, item := range gjson.GetBytes(body, "input").Array() {
			valid = add("item", item.Get("type").String() == "item_reference", item.Get("id")) && valid
			valid = add("call", false, item.Get("call_id")) && valid
		}
	case clientresponse.Chat:
		for _, message := range gjson.GetBytes(body, "messages").Array() {
			for _, call := range message.Get("tool_calls").Array() {
				valid = add("call", false, call.Get("id")) && valid
			}
			valid = add("call", false, message.Get("tool_call_id")) && valid
		}
	case clientresponse.Messages:
		for _, message := range gjson.GetBytes(body, "messages").Array() {
			for _, block := range message.Get("content").Array() {
				switch block.Get("type").String() {
				case "tool_use":
					valid = add("call", false, block.Get("id")) && valid
				case "tool_result":
					valid = add("call", false, block.Get("tool_use_id")) && valid
				}
			}
		}
	}
	if !valid {
		rejectManagedPreparation(ctx, s, 400, "resource_reference_unavailable")
		return
	}
	if len(inputs) == 0 {
		ready(body)
		return
	}
	aliases := make([]clientresponse.ResourceAlias, len(inputs))
	for i, input := range inputs {
		aliases[i] = input.alias
	}
	s.resourcesRestoring = true
	resourceref.Lookup(s.config.referenceRedis, managedResourceScope(s), aliases, func(records []resourceref.Record, err error) {
		if managedTextStopped(s) {
			return
		}
		s.resourcesRestoring = false
		if err != nil {
			rejectManagedPreparation(ctx, s, 400, "resource_reference_unavailable")
			return
		}
		var owner *resourceref.Owner
		for i, record := range records {
			signed := record.Alias.Kind == clientresponse.SignedToolCallKind
			if inputs[i].reference && !record.Native {
				rejectManagedPreparation(ctx, s, 400, "resource_reference_incompatible")
				return
			}
			if record.Native || signed {
				if owner != nil && *owner != record.Owner {
					rejectManagedPreparation(ctx, s, 400, "resource_reference_incompatible")
					return
				}
				copy := record.Owner
				owner = &copy
			}
			if signed {
				id := inputs[i].value.String()
				if _, found := s.signedToolCalls[id]; !found {
					call, err := clientresponse.OpenSignedToolCall(record.Alias, id)
					if err != nil {
						rejectManagedPreparation(ctx, s, 400, "resource_reference_unavailable")
						return
					}
					if s.signedToolCalls == nil {
						s.signedToolCalls = make(map[string]clientresponse.SignedToolCall)
					}
					s.signedToolCalls[id] = call
				}
				continue
			}
			inputs[i].alias = record.Alias
		}
		if owner != nil {
			if err := selectManagedResourceOwner(s, *owner); err != nil {
				rejectManagedPreparation(ctx, s, 503, "resource_owner_unavailable")
				return
			}
			s.resourceOwner = owner
		}
		_ = managedPublicPolicy(s)
		slices.SortFunc(inputs, func(a, b managedResourceInput) int { return a.value.Index - b.value.Index })
		var restored []byte
		position := 0
		for _, input := range inputs {
			if input.alias.Kind == clientresponse.SignedToolCallKind {
				continue
			}
			if restored == nil {
				restored = make([]byte, 0, len(body))
			}
			restored = append(restored, body[position:input.value.Index]...)
			original, _ := json.Marshal(input.alias.Original)
			restored = append(restored, original...)
			position = input.value.Index + len(input.value.Raw)
			s.publicAliases.Seed(input.alias)
		}
		if restored != nil {
			body = append(restored, body[position:]...)
		}
		if owner != nil {
			body, err = sjson.SetBytes(body, "model", s.config.release.Descriptor.UpstreamModel)
			if err != nil {
				rejectManagedPreparation(ctx, s, 503, "resource_owner_unavailable")
				return
			}
		}
		ready(body)
	})
}

func managedResourceContinuationMatches(s *managedExecutionState, candidate *managedExecutionConfig, owner resourceref.Owner) bool {
	if !managedResourceConfigMatches(candidate, owner) {
		return false
	}
	if len(s.signedToolCalls) != 0 {
		return resolveManagedCanonicalTarget(candidate, false).WireFormat == supplier.WireGeminiGenerateContent
	}
	return managedNativeResponses(candidate)
}

// The only selectable peers are the SDK's verified original operation members.
// This happens before any send and never consumes a retry/fallback attempt.
func selectManagedResourceOwner(s *managedExecutionState, owner resourceref.Owner) error {
	if managedResourceContinuationMatches(s, s.config, owner) {
		return nil
	}
	var candidate *managedExecutionConfig
	for _, peer := range s.config.targets {
		if managedResourceContinuationMatches(s, peer, owner) {
			if candidate == nil || peer.release.Descriptor.TargetID < candidate.release.Descriptor.TargetID {
				candidate = peer
			}
		}
	}
	if candidate == nil {
		return errors.New("original resource owner is not published")
	}
	copy := *candidate
	s.config = &copy
	return nil
}
