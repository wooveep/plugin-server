// Package mfjson implements the frozen mf-json.v1 exact JSON byte profile.
package mfjson

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// These limits match mf-common-2026-10-25.3's canonicalization-profile.json.
const (
	MaxInputBytes      = 1_048_576
	MaxOutputBytes     = 4_194_304
	MaxNumberChars     = 10_000
	MaxDepth           = 128
	MaxCollectionItems = 10_000
)

// Error reports a frozen canonicalization rejection code, without input data.
type Error struct {
	Code   string
	detail string
}

func (e *Error) Error() string { return e.Code + ": mf-json.v1 " + e.detail }

func reject(code, detail string) error { return &Error{Code: code, detail: detail} }

// Canonicalize validates the complete input before encoding mf-json.v1 bytes.
// It accepts every JSON root type, preserves exact decimals and array order,
// and never changes or retains an alias of raw. Object keys use Unicode scalar
// order, with no Unicode normalization. Duplicate keys at any depth, invalid
// UTF-8, isolated surrogates and resource limit violations are rejected.
func Canonicalize(raw []byte) ([]byte, error) {
	if len(raw) > MaxInputBytes {
		return nil, reject("E_LIMIT", "input byte limit exceeded")
	}
	if !utf8.Valid(raw) {
		return nil, reject("E_UTF8", "invalid UTF-8")
	}
	// encoding/json otherwise replaces isolated surrogate escapes with U+FFFD.
	if err := checkSurrogates(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := readValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, reject("E_JSON", "unexpected data after JSON value")
	}
	var output encoder
	if err := output.value(value); err != nil {
		return nil, err
	}
	return output.bytes, nil
}

// Digest returns sha256: followed by lowercase hex of the complete canonical
// JSON bytes. It has the same validation and budgets as Canonicalize.
func Digest(raw []byte) (string, error) {
	canonical, err := Canonicalize(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func readValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > MaxDepth {
		return nil, reject("E_LIMIT", "nesting limit exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, reject("E_JSON", "invalid JSON")
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		for decoder.More() {
			// Keys, as well as values, occupy the next structural depth.
			if depth == MaxDepth || len(object) == MaxCollectionItems {
				return nil, reject("E_LIMIT", "object limit exceeded")
			}
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok {
				return nil, reject("E_JSON", "invalid object key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, reject("E_DUPLICATE_KEY", "duplicate object key")
			}
			value, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
			return nil, reject("E_JSON", "incomplete object")
		}
		return object, nil
	case json.Delim('['):
		var items []any
		for decoder.More() {
			if len(items) == MaxCollectionItems {
				return nil, reject("E_LIMIT", "array item limit exceeded")
			}
			value, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
			return nil, reject("E_JSON", "incomplete array")
		}
		return items, nil
	default:
		switch token.(type) {
		case nil, bool, string, json.Number:
			return token, nil
		default:
			return nil, reject("E_JSON", "invalid JSON value")
		}
	}
}

// Check escapes before the standard decoder can replace malformed Unicode.
// Escaped backslashes and quotes are skipped, so literal "\\ud800" is valid.
func checkSurrogates(raw []byte) error {
	quoted := false
	for index := 0; index < len(raw); index++ {
		if raw[index] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[index] != '\\' {
			continue
		}
		if index+1 >= len(raw) || raw[index+1] != 'u' {
			index++
			continue
		}
		code, ok := hexCode(raw, index+2)
		if !ok {
			return reject("E_JSON", "invalid Unicode escape")
		}
		if code >= 0xd800 && code <= 0xdbff {
			low, valid := hexCode(raw, index+8)
			if index+7 >= len(raw) || raw[index+6] != '\\' || raw[index+7] != 'u' ||
				!valid || low < 0xdc00 || low > 0xdfff {
				return reject("E_SURROGATE", "isolated surrogate")
			}
			index += 11
		} else if code >= 0xdc00 && code <= 0xdfff {
			return reject("E_SURROGATE", "isolated surrogate")
		} else {
			index += 5
		}
	}
	return nil
}

func hexCode(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var code uint16
	for _, digit := range raw[start : start+4] {
		code <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			code += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			code += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			code += uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return code, true
}

func decimal(text string) (string, error) {
	coefficient, exponent := text, int64(0)
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		coefficient = text[:index]
		var err error
		exponent, err = strconv.ParseInt(text[index+1:], 10, 64)
		if err != nil {
			return "", reject("E_LIMIT", "number exponent limit exceeded")
		}
	}
	negative := strings.HasPrefix(coefficient, "-")
	coefficient = strings.TrimPrefix(coefficient, "-")
	fraction := 0
	if index := strings.IndexByte(coefficient, '.'); index >= 0 {
		fraction = len(coefficient) - index - 1
	}
	digits := strings.TrimLeft(strings.ReplaceAll(coefficient, ".", ""), "0")
	if digits == "" {
		return "0", nil
	}
	// These comparisons also exclude an overflowing exponent-fraction sum.
	if exponent < int64(fraction)-MaxNumberChars || exponent > int64(fraction)+MaxNumberChars {
		return "", reject("E_LIMIT", "expanded number limit exceeded")
	}
	shift := int(exponent - int64(fraction))
	absShift := shift
	if absShift < 0 {
		absShift = -absShift
	}
	if len(digits)+absShift > MaxNumberChars {
		return "", reject("E_LIMIT", "expanded number limit exceeded")
	}
	point := len(digits) + shift
	var number string
	switch {
	case point <= 0:
		number = "0." + strings.Repeat("0", -point) + digits
	case point >= len(digits):
		number = digits + strings.Repeat("0", point-len(digits))
	default:
		number = digits[:point] + "." + digits[point:]
	}
	if strings.Contains(number, ".") {
		number = strings.TrimRight(strings.TrimRight(number, "0"), ".")
	}
	if negative {
		number = "-" + number
	}
	return number, nil
}

type encoder struct{ bytes []byte }

func (e *encoder) append(text string) error {
	if len(text) > MaxOutputBytes-len(e.bytes) {
		return reject("E_LIMIT", "canonical output limit exceeded")
	}
	e.bytes = append(e.bytes, text...)
	return nil
}

func (e *encoder) quoted(value string) error {
	if err := e.append("\""); err != nil {
		return err
	}
	const hexDigits = "0123456789abcdef"
	start := 0
	for index, scalar := range value {
		if scalar >= 0x20 && scalar != '"' && scalar != '\\' {
			continue
		}
		if err := e.append(value[start:index]); err != nil {
			return err
		}
		escaped := "\\" + string(scalar)
		if scalar < 0x20 {
			escaped = string([]byte{'\\', 'u', '0', '0', hexDigits[scalar>>4], hexDigits[scalar&15]})
		}
		if err := e.append(escaped); err != nil {
			return err
		}
		start = index + 1 // Only single-byte ASCII characters are escaped.
	}
	if err := e.append(value[start:]); err != nil {
		return err
	}
	return e.append("\"")
}

func (e *encoder) value(value any) error {
	switch value := value.(type) {
	case nil:
		return e.append("null")
	case bool:
		return e.append(strconv.FormatBool(value))
	case string:
		return e.quoted(value)
	case json.Number:
		number, err := decimal(string(value))
		if err != nil {
			return err
		}
		return e.append(number)
	case []any:
		if err := e.append("["); err != nil {
			return err
		}
		for index, item := range value {
			if index > 0 {
				if err := e.append(","); err != nil {
					return err
				}
			}
			if err := e.value(item); err != nil {
				return err
			}
		}
		return e.append("]")
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		// For valid UTF-8, byte lexicographic order equals scalar value order.
		sort.Strings(keys)
		if err := e.append("{"); err != nil {
			return err
		}
		for index, key := range keys {
			if index > 0 {
				if err := e.append(","); err != nil {
					return err
				}
			}
			if err := e.quoted(key); err != nil {
				return err
			}
			if err := e.append(":"); err != nil {
				return err
			}
			if err := e.value(value[key]); err != nil {
				return err
			}
		}
		return e.append("}")
	default:
		return reject("E_JSON", "unsupported JSON value")
	}
}
