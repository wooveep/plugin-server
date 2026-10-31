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
	deadlineRaw, occurredRaw := values[AdmissionDeadlineHeader], values[AdmissionOccurredHeader]
	deadline, deadlineErr := time.Parse(time.RFC3339Nano, deadlineRaw)
	occurred, occurredErr := time.Parse(time.RFC3339Nano, occurredRaw)
	if !balanceidentity.ValidID(values[AdmissionCallHeader]) || !id(releaseID) ||
		values[AdmissionReleaseHeader] != releaseID || deadlineErr != nil || occurredErr != nil ||
		!strings.HasSuffix(deadlineRaw, "Z") || !strings.HasSuffix(occurredRaw, "Z") ||
		occurred.Year() < 1 || !at.Before(deadline) || occurred.After(at) || !occurred.Before(deadline) {
		return AdmittedCall{}, errors.New("original current call admission is missing or invalid")
	}
	return AdmittedCall{CallID: values[AdmissionCallHeader], ReleaseID: releaseID,
		OccurredAt: occurredRaw, ValidUntil: deadline}, nil
}
