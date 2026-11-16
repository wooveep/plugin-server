package main

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

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
	if !managedPublicText(s) || s.config.profile.InputProtocol != clientresponse.Responses || s.publicAliases == nil || s.resourcesSaved {
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
	if !managedPublicText(s) || s.config.profile.InputProtocol != clientresponse.Responses {
		ready(body)
		return
	}
	var inputs []managedResourceInput
	add := func(kind string, reference bool, value gjson.Result) bool {
		if !value.Exists() || value.Type == gjson.Null {
			return true
		}
		if s.config.settings.ProtocolMode != "passthrough" && !resourceref.IsAlias(kind, value.String()) {
			return !reference
		}
		inputs = append(inputs, managedResourceInput{value, clientresponse.ResourceAlias{Kind: kind, Public: value.String()}, reference})
		return true
	}
	valid := add("response", true, gjson.GetBytes(body, "previous_response_id"))
	if conversation := gjson.GetBytes(body, "conversation"); conversation.IsObject() {
		valid = add("conversation", true, conversation.Get("id")) && valid
	} else {
		valid = add("conversation", true, conversation) && valid
	}
	for _, item := range gjson.GetBytes(body, "input").Array() {
		valid = add("item", item.Get("type").String() == "item_reference", item.Get("id")) && valid
		valid = add("call", false, item.Get("call_id")) && valid
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
			if inputs[i].reference && !record.Native {
				rejectManagedPreparation(ctx, s, 400, "resource_reference_incompatible")
				return
			}
			if record.Native {
				if owner != nil && *owner != record.Owner {
					rejectManagedPreparation(ctx, s, 400, "resource_reference_incompatible")
					return
				}
				copy := record.Owner
				owner = &copy
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
		restored := make([]byte, 0, len(body))
		position := 0
		for _, input := range inputs {
			restored = append(restored, body[position:input.value.Index]...)
			original, _ := json.Marshal(input.alias.Original)
			restored = append(restored, original...)
			position = input.value.Index + len(input.value.Raw)
			s.publicAliases.Seed(input.alias)
		}
		body = append(restored, body[position:]...)
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

// The only selectable peers are the SDK's verified original operation members.
// This happens before any send and never consumes a retry/fallback attempt.
func selectManagedResourceOwner(s *managedExecutionState, owner resourceref.Owner) error {
	if managedResourceConfigMatches(s.config, owner) && managedNativeResponses(s.config) {
		return nil
	}
	var candidate *managedExecutionConfig
	for _, peer := range s.config.targets {
		if managedResourceConfigMatches(peer, owner) && managedNativeResponses(peer) {
			if candidate == nil || peer.release.Descriptor.TargetID < candidate.release.Descriptor.TargetID {
				candidate = peer
			}
		}
	}
	if candidate == nil {
		return errors.New("original native resource owner is not published")
	}
	copy := *candidate
	s.config = &copy
	return nil
}
