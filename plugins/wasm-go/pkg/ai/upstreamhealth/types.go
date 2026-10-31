// Package upstreamhealth reads Console's upstream-health.v1 projection. It
// qualifies frozen candidates; it never elects targets or changes health state.
package upstreamhealth

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
)

const ProjectionSchema = "upstream-health.v1"

// Revision is Console's nonnegative native i64, transported without a float.
type Revision string

func ParseRevision(value string) (Revision, error) {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < 0 || strconv.FormatInt(number, 10) != value {
		return "", errors.New("health revision requires a canonical nonnegative i64 string")
	}
	return Revision(value), nil
}

func compareRevision(left, right Revision) int {
	if len(left) < len(right) || len(left) == len(right) && left < right {
		return -1
	}
	if left == right {
		return 0
	}
	return 1
}

// Config is server-controlled discovery, not a pool or credential revision.
// Redis connection/authentication/timeout remain the caller's existing client.
type Config struct {
	Prefix          string   `json:"prefix"`
	AuthorityID     string   `json:"authority_id"`
	MinimumRevision Revision `json:"minimum_revision"`
}

func (c Config) validate() error {
	if len(c.Prefix) == 0 || len(c.Prefix) > 128 || !balanceidentity.ValidID(c.AuthorityID) {
		return errors.New("invalid health Redis destination")
	}
	for _, char := range c.Prefix {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("_-:", char)) {
			return errors.New("invalid health Redis prefix")
		}
	}
	_, err := ParseRevision(string(c.MinimumRevision))
	return err
}

// SnapshotKey exactly mirrors upstream_health_delivery::redis::Destination.
func (c Config) SnapshotKey() (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}
	return c.Prefix + ":upstream-health:{" + c.AuthorityID + "}:snapshot", nil
}

type CredentialIdentity struct {
	CredentialRef     string `json:"credential_ref"`
	ProviderAccountID string `json:"provider_account_id"`
	OwnerKind         string `json:"owner_kind"`
	OwnerID           string `json:"owner_id"`
}

type CredentialControl struct {
	Identity   CredentialIdentity `json:"identity"`
	Generation Revision           `json:"generation"`
	Revision   Revision           `json:"revision"`
	Enabled    bool               `json:"enabled"`
}

type ScopeIdentity struct {
	ScopeID              string    `json:"scope_id"`
	Kind                 string    `json:"kind"`
	ProviderAccountID    string    `json:"provider_account_id"`
	RegionEndpointRef    *string   `json:"region_endpoint_ref"`
	CredentialRef        *string   `json:"credential_ref"`
	CredentialGeneration *Revision `json:"credential_generation"`
	UpstreamModel        *string   `json:"upstream_model"`
	OperationKind        *string   `json:"operation_kind"`
	QuotaGroupRef        *string   `json:"quota_group_ref"`
}

type ScopeState struct {
	Identity          ScopeIdentity `json:"identity"`
	Revision          Revision      `json:"revision"`
	FailureGeneration Revision      `json:"failure_generation"`
	RecoveryEpoch     Revision      `json:"recovery_epoch"`
	BlockedUntil      *string       `json:"blocked_until"`
	BackoffStep       uint64        `json:"backoff_step"`
	PermanentReason   *string       `json:"permanent_reason"`
	RecoveredAt       *string       `json:"recovered_at"`
	ActiveClaim       *string       `json:"active_claim"`
	LastEventID       *string       `json:"last_event_id"`
}

// TargetReference is the existing Console runtime-registration mapping. Neither
// HTTP path, provider brand, nor client-supplied model constructs this mapping.
type TargetReference struct {
	TargetID             string   `json:"target_id"`
	BindingID            string   `json:"binding_id"`
	ProviderAccountID    string   `json:"provider_account_id"`
	RegionEndpointRef    string   `json:"region_endpoint_ref"`
	UpstreamModel        string   `json:"upstream_model"`
	OperationKind        string   `json:"operation_kind"`
	CredentialRef        string   `json:"credential_ref"`
	CredentialGeneration Revision `json:"credential_generation"`
	ScopeIDs             []string `json:"scope_ids"`
}

// Candidate adds local fences to the immutable release mapping. A control
// revision is not a secret generation, pool revision or global health revision.
type Candidate struct {
	Target                            TargetReference
	ExpectedCredentialControlRevision *Revision
	// The outcome owner clears this only after the original failure is durably
	// acknowledged; loading/reloading a snapshot cannot clear a local veto.
	PendingLocalVeto bool
}

type ReadStatus string

const (
	ReadReady            ReadStatus = "ready"
	ReadMissing          ReadStatus = "snapshot_missing"
	ReadRedisUnavailable ReadStatus = "redis_unavailable"
	ReadInvalid          ReadStatus = "snapshot_invalid"
	ReadWrongAuthority   ReadStatus = "authority_mismatch"
	ReadSchemaMismatch   ReadStatus = "schema_mismatch"
	ReadIncomplete       ReadStatus = "snapshot_incomplete"
	ReadExpired          ReadStatus = "control_expired"
	ReadFuture           ReadStatus = "observation_in_future"
	ReadStale            ReadStatus = "revision_stale"
	ReadConflict         ReadStatus = "same_revision_conflict"
)

type ReadResult struct {
	Status   ReadStatus
	Snapshot *Snapshot
}

type DecisionKind string

const (
	Eligible         DecisionKind = "eligible"
	RecoveryRequired DecisionKind = "recovery_required"
	Denied           DecisionKind = "denied"
	// This result grants nothing. Existing operation/query/Consume authority must
	// evaluate the original task, owner, account and dispatch independently.
	ExternalOperationAuthorityRequired DecisionKind = "operation_authority_required"
)

type ScopeVersion struct {
	Revision          Revision
	FailureGeneration Revision
	RecoveryEpoch     Revision
}

// Decision reports health qualification only, never a dispatch/claim grant.
type Decision struct {
	Kind                      DecisionKind
	Reason                    string
	ControlStatus             ReadStatus
	RetryAfterSeconds         *uint64
	ScopeEpochs               map[string]Revision
	AuthorityID               string
	SnapshotRevision          Revision
	CredentialGeneration      Revision
	CredentialControlRevision Revision
	ScopeVersions             map[string]ScopeVersion
}

type Metadata struct {
	AuthorityID string
	Revision    Revision
	StateDigest string
	ObservedAt  time.Time
	ValidUntil  time.Time
}

type snapshotWire struct {
	ProjectionSchema string              `json:"projection_schema"`
	AuthorityID      string              `json:"authority_id"`
	Revision         Revision            `json:"revision"`
	Complete         bool                `json:"complete"`
	Credentials      []CredentialControl `json:"credentials"`
	Scopes           []ScopeState        `json:"scopes"`
	ObservedAt       string              `json:"observed_at"`
	ValidUntil       string              `json:"valid_until"`
	StateDigest      string              `json:"state_digest"`
}

// Snapshot has no mutable exported state. Returned observations are copies.
// A reader watermark can veto an older held snapshot, never grant permission.
type Snapshot struct {
	wire            snapshotWire
	metadata        Metadata
	raw             []byte
	fence           *revisionFence
	validated       bool
	credentialIndex map[string]int
	scopeIndex      map[string]int
	accountScopes   map[string][]int
}

type revisionFence struct {
	mu       sync.Mutex
	revision Revision
	digest   string
}

func (s *Snapshot) Metadata() Metadata {
	if s == nil {
		return Metadata{}
	}
	return s.metadata
}

func (s *Snapshot) RawBody() []byte {
	if s == nil {
		return nil
	}
	return append([]byte(nil), s.raw...)
}

func (s *Snapshot) Credential(ref string) (CredentialControl, bool) {
	if s != nil {
		if index, found := s.credentialIndex[ref]; found {
			return s.wire.Credentials[index], true
		}
	}
	return CredentialControl{}, false
}

func identity(value string) bool {
	return value != "" && len(value) <= 512 && utf8.ValidString(value) && strings.TrimSpace(value) != ""
}

func utc(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	_, offset := parsed.Zone()
	return parsed, err == nil && offset == 0 && !strings.HasSuffix(value, "-00:00")
}

func (s *Snapshot) current(now time.Time) ReadStatus {
	if s == nil || !s.validated || now.IsZero() {
		return ReadInvalid
	}
	if s.metadata.ObservedAt.After(now) {
		return ReadFuture
	}
	if s.fence != nil {
		s.fence.mu.Lock()
		defer s.fence.mu.Unlock()
		if compareRevision(s.metadata.Revision, s.fence.revision) < 0 {
			return ReadStale
		}
		if s.metadata.Revision == s.fence.revision && s.metadata.StateDigest != s.fence.digest {
			return ReadConflict
		}
	}
	return ReadReady
}
