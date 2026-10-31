package upstreamhealth

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

var fixtureNow = time.Date(2026, 10, 27, 0, 0, 10, 500000000, time.UTC)

func pointer[T any](value T) *T { return &value }

func fixtureConfig() Config {
	return Config{Prefix: "modelfusion:e2", AuthorityID: "10000000-0000-0000-0000-000000000001", MinimumRevision: "0"}
}

func fixtureWire() snapshotWire {
	return snapshotWire{ProjectionSchema: ProjectionSchema, AuthorityID: fixtureConfig().AuthorityID, Revision: "9007199254740993", Complete: true,
		Credentials: []CredentialControl{{Identity: CredentialIdentity{CredentialRef: "upstream:original-key", ProviderAccountID: "account:original", OwnerKind: "platform", OwnerID: "platform:original"}, Generation: "1", Revision: "2", Enabled: true}},
		Scopes:      []ScopeState{{Identity: ScopeIdentity{ScopeID: "scope:model", Kind: "model_operation", ProviderAccountID: "account:original", RegionEndpointRef: pointer("region:original"), UpstreamModel: pointer("model:original"), OperationKind: pointer("text_generate")}, Revision: "4", FailureGeneration: "2", RecoveryEpoch: "3"}},
		ObservedAt:  "2026-10-27T00:00:00Z", ValidUntil: "2026-10-27T00:00:30Z"}
}

func fixtureTarget() TargetReference {
	return TargetReference{TargetID: "target:original", BindingID: "binding:original", ProviderAccountID: "account:original", RegionEndpointRef: "region:original",
		UpstreamModel: "model:original", OperationKind: "text_generate", CredentialRef: "upstream:original-key", CredentialGeneration: "1", ScopeIDs: []string{"scope:model"}}
}

func hashFixture(t *testing.T, wire snapshotWire) []string {
	t.Helper()
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parseExact(raw)
	if err != nil {
		t.Fatal(err)
	}
	object := tree.(map[string]interface{})
	wire.StateDigest, err = stateDigest([]interface{}{wire.AuthorityID, string(wire.Revision), object["credentials"], object["scopes"]})
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	return []string{wire.AuthorityID, string(wire.Revision), wire.StateDigest, string(raw), wire.ObservedAt, wire.ValidUntil, wire.ProjectionSchema, "true"}
}

func readyFixture(t *testing.T, wire snapshotWire) ReadResult {
	t.Helper()
	result := decodeHash(hashFixture(t, wire), fixtureConfig(), fixtureNow)
	if result.Status != ReadReady {
		t.Fatalf("fixture was not ready: %s", result.Status)
	}
	return result
}

func redisValues(t *testing.T, values []resp.Value) resp.Value {
	t.Helper()
	var raw bytes.Buffer
	if err := resp.NewWriter(&raw).WriteArray(values); err != nil {
		t.Fatal(err)
	}
	result, _, err := resp.NewReader(&raw).ReadValue()
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func redisHash(t *testing.T, fields []string) resp.Value {
	t.Helper()
	values := make([]resp.Value, len(fields))
	for index, value := range fields {
		values[index] = resp.StringValue(value)
	}
	return redisValues(t, values)
}

type fakeReader struct {
	ready     bool
	err       error
	keys      []string
	fields    [][]string
	callbacks []wrapper.RedisResponseCallback
	inline    *resp.Value
}

func (f *fakeReader) Ready() bool { return f.ready }
func (f *fakeReader) HMGet(key string, fields []string, callback wrapper.RedisResponseCallback) error {
	f.keys = append(f.keys, key)
	f.fields = append(f.fields, append([]string(nil), fields...))
	f.callbacks = append(f.callbacks, callback)
	if f.inline != nil {
		callback(*f.inline)
	}
	return f.err
}

func newFixtureReader(t *testing.T, client *fakeReader) *Reader {
	t.Helper()
	reader, err := NewReader(client, fixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	reader.clock = func() time.Time { return fixtureNow }
	return reader
}

func TestControlledDestinationAndExactNativeRevision(t *testing.T) {
	key, err := fixtureConfig().SnapshotKey()
	if err != nil || key != "modelfusion:e2:upstream-health:{10000000-0000-0000-0000-000000000001}:snapshot" {
		t.Fatalf("Console destination mismatch: %s %v", key, err)
	}
	for _, prefix := range []string{"", "unsafe{slot}", "space here", "line\nbreak", strings.Repeat("a", 129)} {
		config := fixtureConfig()
		config.Prefix = prefix
		if _, err := config.SnapshotKey(); err == nil {
			t.Fatalf("ambiguous destination accepted: %q", prefix)
		}
	}
	for _, version := range []string{"0", "1", "9007199254740993", "9223372036854775807"} {
		if actual, err := ParseRevision(version); err != nil || string(actual) != version {
			t.Fatalf("native version changed: %s %v", actual, err)
		}
	}
	for _, version := range []string{"", "-1", "01", "+1", "1.0", "1e3", "9223372036854775808"} {
		if _, err := ParseRevision(version); err == nil {
			t.Fatalf("invalid native version accepted: %s", version)
		}
	}
	if _, err := NewReader((*fakeReader)(nil), fixtureConfig()); err == nil {
		t.Fatal("typed nil Redis reader accepted")
	}
}

func TestReaderReturnsMissingIncompleteAndDependencyFailures(t *testing.T) {
	allMissing := make([]resp.Value, len(HashFields()))
	for index := range allMissing {
		allMissing[index] = resp.NullValue()
	}
	partial := append([]resp.Value(nil), allMissing...)
	partial[0] = resp.StringValue(fixtureConfig().AuthorityID)
	for _, tc := range []struct {
		name   string
		ready  bool
		err    error
		value  resp.Value
		status ReadStatus
	}{
		{"not initialized", false, errors.New("not initialized"), resp.Value{}, ReadRedisUnavailable},
		{"dispatch failure", true, errors.New("owned dependency unavailable"), resp.Value{}, ReadRedisUnavailable},
		{"missing", true, nil, redisValues(t, allMissing), ReadMissing},
		{"partial", true, nil, redisValues(t, partial), ReadIncomplete},
		{"wrong RESP type", true, nil, resp.StringValue("healthy"), ReadInvalid},
		{"wrong RESP count", true, nil, redisValues(t, partial[:2]), ReadInvalid},
		{"Redis error", true, nil, resp.ErrorValue(errors.New("owned Redis unavailable")), ReadRedisUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeReader{ready: tc.ready, err: tc.err}
			reader := newFixtureReader(t, client)
			var results []ReadResult
			reader.Lookup(func(result ReadResult) { results = append(results, result) })
			if tc.ready && tc.err == nil {
				client.callbacks[0](tc.value)
				client.callbacks[0](redisHash(t, hashFixture(t, fixtureWire())))
			} else if tc.err != nil {
				client.callbacks[0](redisHash(t, hashFixture(t, fixtureWire())))
			}
			if len(results) != 1 || results[0].Status != tc.status || results[0].Snapshot != nil || results[0].DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow).Kind != Denied {
				t.Fatalf("missing health became available: %#v", results)
			}
		})
	}
}

func TestLookupCompletesOnceAndKeepsExistingAuthenticationRecovery(t *testing.T) {
	value := redisHash(t, hashFixture(t, fixtureWire()))
	// Ready=false may be a previous transient Init failure. The production
	// wrapper's HMGet owns reauthentication and can still return a valid read.
	client := &fakeReader{ready: false, inline: &value, err: errors.New("late dispatch error")}
	reader := newFixtureReader(t, client)
	var results []ReadResult
	reader.Lookup(func(result ReadResult) { results = append(results, result) })
	changed := fixtureWire()
	changed.Revision, changed.Credentials[0].Enabled = "9007199254740994", false
	client.callbacks[0](redisHash(t, hashFixture(t, changed)))
	if len(results) != 1 || results[0].Status != ReadReady || results[0].DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow).Kind != Eligible {
		t.Fatal("late duplicate/error changed a completed lookup or stranded connection recovery")
	}
}

func TestReaderFencesOutOfOrderAndConflictingSnapshots(t *testing.T) {
	client := &fakeReader{ready: true}
	reader := newFixtureReader(t, client)
	var results []ReadResult
	reader.Lookup(func(result ReadResult) { results = append(results, result) })
	reader.Lookup(func(result ReadResult) { results = append(results, result) })
	current := fixtureWire()
	current.Revision, current.Credentials[0].Enabled, current.Credentials[0].Revision = "9007199254740994", false, "3"
	client.callbacks[1](redisHash(t, hashFixture(t, current)))
	client.callbacks[0](redisHash(t, hashFixture(t, fixtureWire())))
	if len(results) != 2 || results[0].Status != ReadReady || results[1].Status != ReadStale || results[0].DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow).Reason != "credential_unavailable" {
		t.Fatalf("out-of-order view revived a revoked key: %#v", results)
	}
	reader.Lookup(func(result ReadResult) { results = append(results, result) })
	current.Credentials[0].Enabled = true
	client.callbacks[2](redisHash(t, hashFixture(t, current)))
	if results[2].Status != ReadConflict {
		t.Fatal("same revision with changed state accepted")
	}
	reader.Lookup(func(result ReadResult) { results = append(results, result) })
	client.callbacks[3](redisHash(t, hashFixture(t, current)))
	if results[3].Status != ReadConflict {
		t.Fatal("conflict silently replaced the accepted watermark digest")
	}
}

func TestHeldSnapshotRetainsStateAndCannotOverrideNewerRevocation(t *testing.T) {
	client := &fakeReader{ready: true}
	reader := newFixtureReader(t, client)
	var held, current ReadResult
	reader.Lookup(func(result ReadResult) { held = result })
	client.callbacks[0](redisHash(t, hashFixture(t, fixtureWire())))
	if held.DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow).Kind != Eligible {
		t.Fatal("ordinary complete healthy control was not eligible")
	}
	if result := held.DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow.Add(24*time.Hour)); result.ControlStatus != ReadReady || result.Kind != Eligible {
		t.Fatal("healthy persisted control expired during a Console outage")
	}
	changed := fixtureWire()
	changed.Revision, changed.Credentials[0].Enabled = "9007199254740994", false
	reader.Lookup(func(result ReadResult) { current = result })
	client.callbacks[1](redisHash(t, hashFixture(t, changed)))
	if current.Status != ReadReady || held.DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow).ControlStatus != ReadStale {
		t.Fatal("previous snapshot escaped the reader's newer control watermark")
	}
}

func TestAgedControlRetainsExplicitDenials(t *testing.T) {
	for _, reason := range []string{"credential_unavailable", "scope_permanently_blocked", "scope_cooling_down"} {
		t.Run(reason, func(t *testing.T) {
			wire := fixtureWire()
			switch reason {
			case "credential_unavailable":
				wire.Credentials[0].Enabled = false
			case "scope_permanently_blocked":
				wire.Scopes[0].PermanentReason = pointer("confirmed_account_debt")
			case "scope_cooling_down":
				wire.Scopes[0].BlockedUntil = pointer(fixtureNow.Add(48 * time.Hour).Format(time.RFC3339Nano))
			}
			at := fixtureNow.Add(24 * time.Hour)
			read := decodeHash(hashFixture(t, wire), fixtureConfig(), at)
			decision := read.DecideNewCall(Candidate{Target: fixtureTarget()}, at)
			if read.Status != ReadReady || decision.Kind != Denied || decision.Reason != reason {
				t.Fatalf("persisted denial lost its meaning during a Console outage: %#v", decision)
			}
		})
	}
}

func TestDecodeRejectsWrongAuthorityExpiryCorruptionAndAmbiguousOriginalBytes(t *testing.T) {
	for name, mutate := range map[string]func([]string){
		"authority":     func(fields []string) { fields[0] = "20000000-0000-0000-0000-000000000001" },
		"schema":        func(fields []string) { fields[6] = "upstream-health.v2" },
		"incomplete":    func(fields []string) { fields[7] = "false" },
		"digest":        func(fields []string) { fields[2] = "sha256:" + strings.Repeat("0", 64) },
		"body identity": func(fields []string) { fields[3] = strings.Replace(fields[3], `"enabled":true`, `"enabled":false`, 1) },
		"missing field": func(fields []string) {
			fields[3] = strings.Replace(fields[3], `"quota_group_ref":null`, `"different":null`, 1)
		},
		"unknown field": func(fields []string) {
			fields[3] = strings.Replace(fields[3], `"enabled":true`, `"enabled":true,"healthy":true`, 1)
		},
		"case alias": func(fields []string) { fields[3] = strings.Replace(fields[3], `"enabled":true`, `"Enabled":true`, 1) },
		"duplicate key": func(fields []string) {
			fields[3] = strings.Replace(fields[3], `"enabled":true`, `"enabled":true,"enabled":true`, 1)
		},
		"numeric revision": func(fields []string) {
			fields[3] = strings.Replace(fields[3], `"revision":"9007199254740993"`, `"revision":9007199254740993`, 1)
		},
		"nullable enabled": func(fields []string) { fields[3] = strings.Replace(fields[3], `"enabled":true`, `"enabled":null`, 1) },
		"nullable backoff": func(fields []string) {
			fields[3] = strings.Replace(fields[3], `"backoff_step":0`, `"backoff_step":null`, 1)
		},
		"header timestamp mismatch": func(fields []string) { fields[4] = "2026-10-27T00:00:01Z" },
		"lone surrogate":            func(fields []string) { fields[3] = strings.Replace(fields[3], `"platform:original"`, `"\ud800"`, 1) },
		"invalid UTF8":              func(fields []string) { fields[3] += string([]byte{0xff}) },
		"truncated":                 func(fields []string) { fields[3] = fields[3][:len(fields[3])-1] },
		"trailing JSON":             func(fields []string) { fields[3] += `{}` },
	} {
		t.Run(name, func(t *testing.T) {
			fields := hashFixture(t, fixtureWire())
			mutate(fields)
			if result := decodeHash(fields, fixtureConfig(), fixtureNow); result.Status == ReadReady || result.Snapshot != nil {
				t.Fatalf("malformed original projection accepted: %s", name)
			}
		})
	}
	fields := hashFixture(t, fixtureWire())
	fields[4], fields[5] = "2026-10-27T00:00:00.000000000Z", "2026-10-27T00:00:30.000000000Z"
	if result := decodeHash(fields, fixtureConfig(), fixtureNow); result.Status != ReadReady {
		t.Fatal("publisher's Nanos hash timestamps did not match serde AutoSi body times")
	}
	for _, tc := range []struct {
		wire   snapshotWire
		now    time.Time
		status ReadStatus
	}{
		{fixtureWire(), fixtureNow.Add(24 * time.Hour), ReadReady},
		{fixtureWire(), fixtureNow.Add(-time.Minute), ReadFuture},
	} {
		if result := decodeHash(hashFixture(t, tc.wire), fixtureConfig(), tc.now); result.Status != tc.status {
			t.Fatalf("freshness classification: %s expected %s", result.Status, tc.status)
		}
	}
	config := fixtureConfig()
	config.MinimumRevision = "9007199254740994"
	if result := decodeHash(hashFixture(t, fixtureWire()), config, fixtureNow); result.Status != ReadStale {
		t.Fatal("configured minimum authority revision ignored")
	}
}

func TestCandidateQualificationPreservesConsoleControlSemantics(t *testing.T) {
	for name, mutate := range map[string]func(*snapshotWire, *Candidate){
		"eligible": func(wire *snapshotWire, candidate *Candidate) {
			wire.Scopes[0].RecoveredAt = pointer("2000-01-01T00:00:00Z")
		},
		"credential_unavailable":           func(wire *snapshotWire, candidate *Candidate) { wire.Credentials[0].Enabled = false },
		"generation":                       func(wire *snapshotWire, candidate *Candidate) { candidate.Target.CredentialGeneration = "0" },
		"credential_control_missing":       func(wire *snapshotWire, candidate *Candidate) { wire.Credentials = []CredentialControl{} },
		"health_scope_missing_or_misbound": func(wire *snapshotWire, candidate *Candidate) { candidate.Target.ScopeIDs = []string{"scope:missing"} },
		"credential_control_revision_mismatch": func(wire *snapshotWire, candidate *Candidate) {
			candidate.ExpectedCredentialControlRevision = pointer(Revision("1"))
		},
		"local_health_failure_pending": func(wire *snapshotWire, candidate *Candidate) { candidate.PendingLocalVeto = true },
		"scope_permanently_blocked": func(wire *snapshotWire, candidate *Candidate) {
			wire.Scopes[0].PermanentReason = pointer("account_funding_required")
		},
		"scope_cooling_down": func(wire *snapshotWire, candidate *Candidate) {
			wire.Scopes[0].BlockedUntil = pointer("2026-10-27T00:00:12Z")
		},
		"recovery_required": func(wire *snapshotWire, candidate *Candidate) {
			wire.Scopes[0].BlockedUntil = pointer("2026-10-27T00:00:10.500000000Z")
			wire.Scopes[0].ActiveClaim = pointer("20000000-0000-0000-0000-000000000001")
		},
	} {
		t.Run(name, func(t *testing.T) {
			wire, candidate := fixtureWire(), Candidate{Target: fixtureTarget()}
			mutate(&wire, &candidate)
			decision := readyFixture(t, wire).DecideNewCall(candidate, fixtureNow)
			switch name {
			case "eligible":
				if decision.Kind != Eligible || decision.CredentialControlRevision != "2" || decision.ScopeVersions["scope:model"].FailureGeneration != "2" {
					t.Fatalf("idle candidate: %#v", decision)
				}
			case "recovery_required":
				if decision.Kind != RecoveryRequired || decision.ScopeEpochs["scope:model"] != "3" {
					t.Fatalf("expired cooldown automatically opened: %#v", decision)
				}
			case "generation":
				if decision.Kind != Denied || decision.Reason != "credential_unavailable" {
					t.Fatalf("generation: %#v", decision)
				}
			default:
				if decision.Kind != Denied || decision.Reason != name {
					t.Fatalf("decision: %#v expected %s", decision, name)
				}
			}
			if name == "scope_cooling_down" && (decision.RetryAfterSeconds == nil || *decision.RetryAfterSeconds != 2) {
				t.Fatal("retry delay did not round the original absolute deadline upward")
			}
		})
	}
}

func TestSnapshotRejectsDuplicateAndMisboundControlRows(t *testing.T) {
	for name, mutate := range map[string]func(*snapshotWire){
		"duplicate credential":                func(wire *snapshotWire) { wire.Credentials = append(wire.Credentials, wire.Credentials[0]) },
		"duplicate scope":                     func(wire *snapshotWire) { wire.Scopes = append(wire.Scopes, wire.Scopes[0]) },
		"unknown ownership":                   func(wire *snapshotWire) { wire.Credentials[0].Identity.OwnerKind = "client" },
		"wrong credential revision":           func(wire *snapshotWire) { wire.Credentials[0].Revision = "01" },
		"model without region":                func(wire *snapshotWire) { wire.Scopes[0].Identity.RegionEndpointRef = nil },
		"credential scope without generation": func(wire *snapshotWire) { wire.Scopes[0].Identity.Kind = "credential" },
	} {
		t.Run(name, func(t *testing.T) {
			wire := fixtureWire()
			mutate(&wire)
			if result := decodeHash(hashFixture(t, wire), fixtureConfig(), fixtureNow); result.Status != ReadInvalid {
				t.Fatalf("invalid typed control rows accepted: %s", result.Status)
			}
		})
	}
}

func TestStructuralAccountAndModelBlocksCannotBeBypassedByNewKeys(t *testing.T) {
	wire := fixtureWire()
	wire.Credentials[0].Generation = "2"
	target := fixtureTarget()
	target.CredentialGeneration = "2"
	wire.Scopes = append(wire.Scopes, ScopeState{Identity: ScopeIdentity{ScopeID: "scope:account", Kind: "account", ProviderAccountID: target.ProviderAccountID},
		Revision: "5", FailureGeneration: "4", RecoveryEpoch: "6", BlockedUntil: pointer("2026-10-27T00:01:00Z")})
	decision := readyFixture(t, wire).DecideNewCall(Candidate{Target: target}, fixtureNow)
	if decision.Reason != "scope_cooling_down" || decision.RetryAfterSeconds == nil || *decision.RetryAfterSeconds != 50 || len(decision.ScopeVersions) != 2 {
		t.Fatalf("unlisted structural account scope bypassed: %#v", decision)
	}
	// A different operation/model outside the blocked structural identity is not
	// classified by URL. Account blocks still apply across those operations.
	wire.Scopes[1].BlockedUntil = nil
	wire.Scopes[0].BlockedUntil = pointer("2026-10-27T00:01:00Z")
	queryScope := ScopeState{Identity: ScopeIdentity{ScopeID: "scope:query", Kind: "model_operation", ProviderAccountID: target.ProviderAccountID,
		RegionEndpointRef: pointer(target.RegionEndpointRef), UpstreamModel: pointer(target.UpstreamModel), OperationKind: pointer("media_query")}, Revision: "1", FailureGeneration: "0", RecoveryEpoch: "0"}
	wire.Scopes = append(wire.Scopes, queryScope)
	target.OperationKind, target.ScopeIDs = "media_query", []string{"scope:query"}
	if decision := readyFixture(t, wire).DecideNewCall(Candidate{Target: target}, fixtureNow); decision.Kind != Eligible {
		t.Fatalf("generation cooling incorrectly blocked a different query scope: %#v", decision)
	}
}

func TestQuotaGroupsRequireExplicitMembershipAndNoCandidateIsElected(t *testing.T) {
	wire := fixtureWire()
	wire.Scopes = append(wire.Scopes, ScopeState{Identity: ScopeIdentity{ScopeID: "scope:quota", Kind: "quota_group", ProviderAccountID: "account:original", QuotaGroupRef: pointer("quota:confirmed")},
		Revision: "1", FailureGeneration: "1", RecoveryEpoch: "1", BlockedUntil: pointer("2026-10-27T00:01:00Z")})
	result := readyFixture(t, wire)
	member, other := fixtureTarget(), fixtureTarget()
	member.ScopeIDs = append(member.ScopeIDs, "scope:quota")
	decisions := result.FilterNewCallCandidates([]Candidate{{Target: member}, {Target: other}}, fixtureNow)
	if len(decisions) != 2 || decisions[0].Reason != "scope_cooling_down" || decisions[1].Kind != Eligible || len(other.ScopeIDs) != 1 {
		t.Fatalf("quota membership or candidate order changed: %#v", decisions)
	}
}

func TestSnapshotObservationsAreImmutableAndAcceptedOperationsRequireOwnAuthority(t *testing.T) {
	result := readyFixture(t, fixtureWire())
	control, _ := result.Snapshot.Credential(fixtureTarget().CredentialRef)
	control.Enabled = false
	raw := result.Snapshot.RawBody()
	raw[0] = 'x'
	decision := result.DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow)
	decision.ScopeVersions["scope:model"] = ScopeVersion{Revision: "999"}
	again := result.DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow)
	if again.Kind != Eligible || again.ScopeVersions["scope:model"].Revision != "4" || result.Snapshot.RawBody()[0] != '{' {
		t.Fatal("caller mutation changed the validated snapshot")
	}
	for _, status := range []ReadStatus{ReadReady, ReadMissing, ReadExpired, ReadRedisUnavailable} {
		if decision := (ReadResult{Status: status}).DecideAcceptedOperation("operation:original"); decision.Kind != ExternalOperationAuthorityRequired {
			t.Fatal("new-call policy gave permission to recreate or cancel an accepted operation")
		}
	}
	if decision := (*Snapshot)(nil).DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow); decision.Kind != Denied {
		t.Fatal("nil/zero snapshot became healthy")
	}
}

// Fixed projection data follows the native Console delivery test's original
// credential, revocation and >2^53 revision. Canonical bytes/hash are independent
// Python hashlib expectations; running this test is not a PG/Redis live receipt.
func TestConsoleProjectionCanonicalGolden(t *testing.T) {
	const body = `{"projection_schema":"upstream-health.v1","authority_id":"10000000-0000-0000-0000-000000000001","revision":"9007199254740993","complete":true,"credentials":[{"identity":{"credential_ref":"upstream:original-key","provider_account_id":"account:original","owner_kind":"platform","owner_id":"platform:original"},"generation":"1","revision":"2","enabled":false}],"scopes":[],"observed_at":"2026-10-27T00:00:00Z","valid_until":"2026-10-27T00:00:30Z","state_digest":"sha256:9d1c63f65d43a4cc8895cc2c8416dcff6447421e4bb3c08f276109b2abff3265"}`
	const canonical = `["10000000-0000-0000-0000-000000000001","9007199254740993",[{"enabled":false,"generation":"1","identity":{"credential_ref":"upstream:original-key","owner_id":"platform:original","owner_kind":"platform","provider_account_id":"account:original"},"revision":"2"}],[]]`
	tree, err := parseExact([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	object := tree.(map[string]interface{})
	payload := []interface{}{object["authority_id"], object["revision"], object["credentials"], object["scopes"]}
	actual, err := canonicalState(payload)
	if err != nil || string(actual) != canonical {
		t.Fatalf("Console canonical projection mismatch: %s %v", actual, err)
	}
	digest, err := stateDigest(payload)
	if err != nil || digest != object["state_digest"] {
		t.Fatalf("Console state digest mismatch: %s %v", digest, err)
	}
	fields := []string{fixtureConfig().AuthorityID, "9007199254740993", digest, body, "2026-10-27T00:00:00.000000000Z", "2026-10-27T00:00:30.000000000Z", ProjectionSchema, "true"}
	if result := decodeHash(fields, fixtureConfig(), fixtureNow); result.Status != ReadReady {
		t.Fatalf("native golden decode: %s", result.Status)
	}
}

func TestClosedCanonicalProjectionPreservesUnicodeNullAndIntegerValues(t *testing.T) {
	value, err := parseExact([]byte(`{"\ud83d\ude00":null,"\ue000":["é","é","\u0000\n\t","/",9007199254740993]}`))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := canonicalState(value)
	const expected = `{"":["é","é","\u0000\u000a\u0009","/",9007199254740993],"😀":null}`
	if err != nil || string(actual) != expected {
		t.Fatalf("mf-json.v1 closed-tree mismatch: %s %v", actual, err)
	}
	for _, raw := range []string{`{"a":1,"\u0061":2}`, `{"x":"\ud800"}`, `{"x":"\udc00"}`} {
		if _, err := parseExact([]byte(raw)); err == nil {
			t.Fatalf("ambiguous JSON accepted: %s", raw)
		}
	}
	for _, number := range []string{"1.0", "1e3", "-0", "18446744073709551616"} {
		if _, err := canonicalState(json.Number(number)); err == nil {
			t.Fatalf("non-typed backoff number accepted: %s", number)
		}
	}
}
