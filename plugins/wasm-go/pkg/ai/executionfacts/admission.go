package executionfacts

import (
	"errors"
	"strings"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
)

// These private headers carry this call's original quota admission result.
// Quota clears client values before installing a successfully checked result.
// Parsing is not authentication: released filter order establishes the source.
const (
	// CallIdentitySourceAdmission is a frozen capability, never a request grant.
	CallIdentitySourceAdmission = "admission"

	AdmissionCallHeader     = "x-modelfusion-call-id"
	AdmissionReleaseHeader  = "x-modelfusion-admission-release-id"
	AdmissionDeadlineHeader = "x-modelfusion-admission-valid-until"
	AdmissionOccurredHeader = "x-modelfusion-admission-occurred-at"
)

// AdmittedCall retains a header-stage call identity without inventing an attempt.
// Production time comes from the original Redis-admitted gateway call and is
// persisted through its existing registration event. Validation retains its
// committed Console admission time.
type AdmittedCall struct {
	CallID     string
	ReleaseID  string
	OccurredAt string
	ValidUntil time.Time
}

// ReadAdmittedCall checks the exact current call result from the preceding quota.
// It never creates a call, extends a deadline, or accepts a client self-grant.
func ReadAdmittedCall(headers [][2]string, releaseID string, at time.Time) (AdmittedCall, error) {
	values := make(map[string]string, 4)
	for _, header := range headers {
		name := strings.ToLower(header[0])
		switch name {
		case AdmissionCallHeader, AdmissionReleaseHeader, AdmissionDeadlineHeader, AdmissionOccurredHeader:
			if _, exists := values[name]; exists {
				return AdmittedCall{}, errors.New("duplicate original call admission header")
			}
			values[name] = header[1]
		}
	}
	occurredRaw := values[AdmissionOccurredHeader]
	occurred, occurredErr := time.Parse(time.RFC3339Nano, occurredRaw)
	if !balanceidentity.ValidID(values[AdmissionCallHeader]) || !id(releaseID) ||
		values[AdmissionReleaseHeader] != releaseID || occurredErr != nil ||
		!strings.HasSuffix(occurredRaw, "Z") || occurred.Year() < 1 || occurred.After(at) {
		return AdmittedCall{}, errors.New("original current call admission is missing or invalid")
	}
	// Redis-admitted production calls retain identity and business time for this
	// HTTP context. Only an explicit bounded permit carries an expiry.
	var deadline time.Time
	if raw, bounded := values[AdmissionDeadlineHeader]; bounded {
		var err error
		deadline, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil || !strings.HasSuffix(raw, "Z") || !at.Before(deadline) || !occurred.Before(deadline) {
			return AdmittedCall{}, errors.New("original bounded call admission is expired or invalid")
		}
	}
	return AdmittedCall{CallID: values[AdmissionCallHeader], ReleaseID: releaseID,
		OccurredAt: occurredRaw, ValidUntil: deadline}, nil
}
