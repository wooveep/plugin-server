package main

import (
	"errors"
	"net/url"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

const maxRequestTargetDecodePasses = 6

var errUnsafeRequestTarget = errors.New("unsafe request target")

func validateSafeOriginFormTarget(rawTarget string) (string, error) {
	if rawTarget == "" || rawTarget[0] != '/' || strings.HasPrefix(rawTarget, "//") {
		return "", errUnsafeRequestTarget
	}
	for index := 0; index < len(rawTarget); index++ {
		character := rawTarget[index]
		if character < 0x20 || character == 0x7f || character == '\\' || character == '#' {
			return "", errUnsafeRequestTarget
		}
	}

	parsed, err := url.ParseRequestURI(rawTarget)
	if err != nil ||
		parsed.Scheme != "" ||
		parsed.Host != "" ||
		parsed.User != nil ||
		parsed.Opaque != "" ||
		parsed.Fragment != "" {
		return "", errUnsafeRequestTarget
	}

	rawPath, _, _ := strings.Cut(rawTarget, "?")
	if err := validateDecodedRequestPath(rawPath); err != nil {
		return "", err
	}
	return parsed.Path, nil
}

func validateDecodedRequestPath(rawPath string) error {
	current := rawPath
	for pass := 0; pass < maxRequestTargetDecodePasses; pass++ {
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return errUnsafeRequestTarget
		}
		if containsUnsafeRequestTargetByte(decoded) ||
			strings.HasPrefix(decoded, "//") ||
			strings.Contains(decoded, "\\") ||
			strings.Contains(decoded, "#") ||
			hasTraversalSegment(decoded) {
			return errUnsafeRequestTarget
		}
		if decoded == current {
			return nil
		}
		current = decoded
	}
	return errUnsafeRequestTarget
}

func containsUnsafeRequestTargetByte(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}

func hasTraversalSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func rejectUnsafeRequestTarget() types.Action {
	_ = proxywasm.SendHttpResponseWithDetail(
		400,
		"ai-proxy.invalid_request_target",
		[][2]string{{"content-type", "text/plain"}},
		[]byte("invalid request target"),
		-1,
	)
	return types.ActionContinue
}
