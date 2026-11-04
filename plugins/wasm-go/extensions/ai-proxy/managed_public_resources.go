package main

import (
	"errors"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func managedResourceScope(s *managedExecutionState) resourceref.Scope {
	return resourceref.Scope{Tenant: s.identity.TenantID, Principal: s.identity.PrincipalID, SalesAsset: s.config.release.SalesAssetID}
}

func managedResourceOwner(c *managedExecutionConfig) resourceref.Owner {
	d := c.release.Descriptor
	return resourceref.Owner{Provider: d.ProviderID, Account: d.ProviderAccountID, Region: d.RegionEndpointRef, Model: d.UpstreamModel, Protocol: c.profile.UpstreamProtocol}
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
		records = append(records, resourceref.NewRecord(alias, owner, managedNativeResponses(s.config)))
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
	path      string
	alias     clientresponse.ResourceAlias
	reference bool
}

func restoreManagedResourceReferences(ctx wrapper.HttpContext, s *managedExecutionState, body []byte, ready func([]byte)) {
	if !managedPublicText(s) || s.config.profile.InputProtocol != clientresponse.Responses {
		ready(body)
		return
	}
	var inputs []managedResourceInput
	add := func(path, kind string, reference bool) bool {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			return true
		}
		if !resourceref.IsAlias(kind, value.String()) {
			return !reference
		}
		inputs = append(inputs, managedResourceInput{path, clientresponse.ResourceAlias{Kind: kind, Public: value.String()}, reference})
		return true
	}
	valid := add("previous_response_id", "response", true)
	if conversation := gjson.GetBytes(body, "conversation"); conversation.IsObject() {
		valid = add("conversation.id", "conversation", true) && valid
	} else {
		valid = add("conversation", "conversation", true) && valid
	}
	for i, item := range gjson.GetBytes(body, "input").Array() {
		path := "input." + strconv.Itoa(i)
		valid = add(path+".id", "item", item.Get("type").String() == "item_reference") && valid
		valid = add(path+".call_id", "call", false) && valid
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
			body, err = sjson.SetBytes(body, inputs[i].path, record.Alias.Original)
			if err != nil {
				rejectManagedPreparation(ctx, s, 400, "resource_reference_incompatible")
				return
			}
			_ = managedPublicPolicy(s)
			s.publicAliases.Seed(record.Alias)
		}
		if owner != nil {
			if err := selectManagedResourceOwner(s, *owner); err != nil {
				rejectManagedPreparation(ctx, s, 503, "resource_owner_unavailable")
				return
			}
			body, err = sjson.SetBytes(body, "model", s.config.release.Descriptor.UpstreamModel)
			if err != nil {
				rejectManagedPreparation(ctx, s, 503, "resource_owner_unavailable")
				return
			}
			s.resourceOwner = owner
		}
		ready(body)
	})
}

// The only selectable peers are the SDK's verified original operation members.
// This happens before any send and never consumes a retry/fallback attempt.
func selectManagedResourceOwner(s *managedExecutionState, owner resourceref.Owner) error {
	if managedResourceOwner(s.config) == owner && managedNativeResponses(s.config) {
		return nil
	}
	var candidate *managedExecutionConfig
	for _, peer := range s.config.targets {
		if managedResourceOwner(peer) == owner && managedNativeResponses(peer) {
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
