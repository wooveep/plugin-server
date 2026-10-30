package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

type ScopedCacheKeyInput struct {
	KeyPrefix          string
	Tenant             string
	Consumer           string
	CacheScope         string
	Route              string
	Model              string
	Protocol           protocol.ProtocolKind
	RequestDigest      string
	CachePolicyVersion string
}

type ScopedCacheKeyMaterial struct {
	RedisKey           string
	Tenant             string
	Consumer           string
	CacheScope         string
	Route              string
	Model              string
	Protocol           protocol.ProtocolKind
	RequestDigest      string
	CachePolicyVersion string
}

func BuildScopedCacheKeyMaterial(input ScopedCacheKeyInput) (ScopedCacheKeyMaterial, error) {
	normalized := normalizeScopedCacheKeyInput(input)
	if err := validateScopedCacheKeyInput(normalized); err != nil {
		return ScopedCacheKeyMaterial{}, err
	}

	parts := []string{
		"v1",
		"s=" + normalized.CacheScope,
		"t=" + shortDigest(normalized.Tenant),
		"r=" + shortDigest(normalized.Route),
		"m=" + shortDigest(normalized.Model),
		"x=" + shortDigest(string(normalized.Protocol)),
		"d=" + shortDigest(normalized.RequestDigest),
		"p=" + shortDigest(normalized.CachePolicyVersion),
	}
	if normalized.CacheScope == config.CACHE_SCOPE_CONSUMER {
		parts = append(parts, "c="+shortDigest(normalized.Consumer))
	}
	sort.Strings(parts[1:])

	return ScopedCacheKeyMaterial{
		RedisKey:           normalized.KeyPrefix + strings.Join(parts, ":"),
		Tenant:             normalized.Tenant,
		Consumer:           normalized.Consumer,
		CacheScope:         normalized.CacheScope,
		Route:              normalized.Route,
		Model:              normalized.Model,
		Protocol:           normalized.Protocol,
		RequestDigest:      normalized.RequestDigest,
		CachePolicyVersion: normalized.CachePolicyVersion,
	}, nil
}

func BuildOpenAIRequestDigest(body []byte) (string, string, error) {
	fields, err := decodeJSONObject(body)
	if err != nil {
		return "", "", err
	}

	model := ""
	if raw, ok := fields["model"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &model); err != nil {
			return "", "", fmt.Errorf("model must be a string: %w", err)
		}
	}

	payload := make(map[string]interface{}, len(fields))
	for field, raw := range fields {
		if field == "stream" || len(raw) == 0 {
			continue
		}
		value, err := canonicalJSON(raw)
		if err != nil {
			return "", "", fmt.Errorf("canonicalize %s: %w", field, err)
		}
		payload[field] = value
	}

	digest, err := stableDigest(payload)
	if err != nil {
		return "", "", err
	}
	return model, digest, nil
}

func normalizeScopedCacheKeyInput(input ScopedCacheKeyInput) ScopedCacheKeyInput {
	input.KeyPrefix = strings.TrimSpace(input.KeyPrefix)
	input.Tenant = strings.TrimSpace(input.Tenant)
	input.Consumer = strings.TrimSpace(input.Consumer)
	input.CacheScope = strings.TrimSpace(input.CacheScope)
	input.Route = strings.TrimSpace(input.Route)
	input.Model = strings.TrimSpace(input.Model)
	input.RequestDigest = strings.TrimSpace(input.RequestDigest)
	input.CachePolicyVersion = strings.TrimSpace(input.CachePolicyVersion)
	return input
}

func validateScopedCacheKeyInput(input ScopedCacheKeyInput) error {
	if input.KeyPrefix == "" {
		return errors.New("materialized Redis key prefix is required")
	}
	if input.Tenant == "" {
		return errors.New("tenant is required")
	}
	if input.CacheScope != config.CACHE_SCOPE_TENANT && input.CacheScope != config.CACHE_SCOPE_CONSUMER {
		return fmt.Errorf("invalid cache scope: %s", input.CacheScope)
	}
	if input.CacheScope == config.CACHE_SCOPE_CONSUMER && input.Consumer == "" {
		return errors.New("consumer is required for consumer-scoped cache")
	}
	if input.Route == "" {
		return errors.New("route is required")
	}
	if input.Model == "" {
		return errors.New("model is required")
	}
	if !isSupportedThinProtocol(input.Protocol) {
		return errors.New("supported protocol is required")
	}
	if input.RequestDigest == "" {
		return errors.New("request digest is required")
	}
	if input.CachePolicyVersion == "" {
		return errors.New("cache policy version is required")
	}
	return nil
}

func shortDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func stableDigest(value interface{}) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func decodeJSONObject(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("request body must be a JSON object")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid JSON: multiple values")
		}
		return nil, err
	}
	return fields, nil
}

func canonicalJSON(body []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid JSON: multiple values")
		}
		return nil, err
	}
	return value, nil
}
