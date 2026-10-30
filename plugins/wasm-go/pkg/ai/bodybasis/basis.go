// Package bodybasis captures the original JSON request before gateway rewrites.
package bodybasis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

const (
	DigestHeader      = "x-modelfusion-original-body-digest"
	PublicModelHeader = "x-modelfusion-original-public-model"
)

// Snapshot holds values, not aliases of the body or a mutable header map.
// Its headers are trustworthy only when a gateway stage has cleared client
// values and captured the original body before any rewrite.
type Snapshot struct {
	digest      string
	publicModel string
}

// HeaderNames returns the reserved request headers.
func HeaderNames() [2]string {
	return [2]string{DigestHeader, PublicModelHeader}
}

// DigestBytes hashes the received bytes without parsing or reserializing them.
func DigestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ValidateModel checks the string can also be carried intact in a header.
func ValidateModel(model string) error {
	if !utf8.ValidString(model) || strings.TrimSpace(model) == "" {
		return errors.New("model must be a nonblank UTF-8 string")
	}
	for _, r := range model {
		if r < 0x20 || r == 0x7f {
			return errors.New("model contains a header control character")
		}
	}
	return nil
}

// Capture captures the original root model and its mf-json.v1 digest. It does
// not change raw; subsequent model mapping and sending retain their raw bytes.
// The complete input is validated, including nested duplicate keys and numbers.
func Capture(raw []byte) (Snapshot, error) {
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil {
		return Snapshot{}, err
	}
	if canonical[0] != '{' {
		return Snapshot{}, errors.New("request must be a JSON object")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &object); err != nil {
		return Snapshot{}, errors.New("invalid JSON object")
	}
	var model string
	if err := json.Unmarshal(object["model"], &model); err != nil {
		return Snapshot{}, errors.New("model must be a string")
	}
	if err := ValidateModel(model); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{digest: DigestBytes(canonical), publicModel: model}, nil
}

// Read validates exactly one occurrence of each reserved header. It does not
// authenticate their origin; the caller must enforce the gateway capture order.
func Read(headers [][2]string) (Snapshot, error) {
	var snapshot Snapshot
	var digests, models int
	for _, header := range headers {
		switch {
		case strings.EqualFold(header[0], DigestHeader):
			digests++
			snapshot.digest = header[1]
		case strings.EqualFold(header[0], PublicModelHeader):
			models++
			snapshot.publicModel = header[1]
		}
	}
	if digests != 1 || models != 1 {
		return Snapshot{}, errors.New("body basis requires exactly one digest and public model")
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s Snapshot) Digest() string { return s.digest }

func (s Snapshot) PublicModel() string { return s.publicModel }

// Validate requires the canonical sha256 representation and a header-safe model.
func (s Snapshot) Validate() error {
	const prefix = "sha256:"
	if len(s.digest) != len(prefix)+sha256.Size*2 || !strings.HasPrefix(s.digest, prefix) {
		return errors.New("body digest must be canonical sha256")
	}
	for _, c := range s.digest[len(prefix):] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return errors.New("body digest must use lowercase hexadecimal")
		}
	}
	return ValidateModel(s.publicModel)
}

// Headers returns a new slice; mutating it cannot change the snapshot.
func (s Snapshot) Headers() [][2]string {
	return [][2]string{{DigestHeader, s.digest}, {PublicModelHeader, s.publicModel}}
}
