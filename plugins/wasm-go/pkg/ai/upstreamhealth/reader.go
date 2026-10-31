package upstreamhealth

import (
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

// RedisReader is intentionally read-only. Existing wrapper.RedisClient satisfies
// it without modifying the shared interface, credentials or command timeout.
type RedisReader interface {
	Ready() bool
	HMGet(key string, fields []string, callback wrapper.RedisResponseCallback) error
}

// HashFields returns the eight fields written atomically by Console's publisher.
func HashFields() []string {
	return []string{"authority_id", "revision", "state_digest", "body", "observed_at", "valid_until", "projection_schema", "complete"}
}

type Reader struct {
	client RedisReader
	config Config
	key    string
	fence  revisionFence
	clock  func() time.Time
}

func NewReader(client RedisReader, config Config) (*Reader, error) {
	key, err := config.SnapshotKey()
	if err != nil {
		return nil, err
	}
	if client == nil || reflect.ValueOf(client).Kind() == reflect.Ptr && reflect.ValueOf(client).IsNil() {
		return nil, errors.New("health projection requires an initialized Redis reader")
	}
	return &Reader{client: client, config: config, key: key, clock: time.Now,
		fence: revisionFence{revision: config.MinimumRevision}}, nil
}

func (r *Reader) SnapshotKey() string {
	if r == nil {
		return ""
	}
	return r.key
}

// Lookup performs one atomic HMGET. It has no write, background probe, cache
// fallback, or business retry. Dispatch errors and duplicate callbacks complete
// the supplied callback once; raw Redis errors never become public diagnostics.
func (r *Reader) Lookup(callback func(ReadResult)) {
	if callback == nil {
		return
	}
	var once sync.Once
	finish := func(result func() ReadResult) { once.Do(func() { callback(result()) }) }
	if r == nil || r.client == nil {
		finish(func() ReadResult { return ReadResult{Status: ReadRedisUnavailable} })
		return
	}
	// Ready=false is not permanent: the existing wrapper HMGet retries its
	// configured connection authentication. Let that read return its own error;
	// refusing to call it here would strand a client after a transient Init fault.
	err := r.client.HMGet(r.key, HashFields(), func(value resp.Value) {
		finish(func() ReadResult { return r.decodeResponse(value, r.clock()) })
	})
	if err != nil {
		finish(func() ReadResult { return ReadResult{Status: ReadRedisUnavailable} })
	}
}

func (r *Reader) decodeResponse(value resp.Value, now time.Time) ReadResult {
	if value.Error() != nil {
		return ReadResult{Status: ReadRedisUnavailable}
	}
	if value.Type() != resp.Array || len(value.Array()) != len(HashFields()) {
		return ReadResult{Status: ReadInvalid}
	}
	values := value.Array()
	missing := 0
	fields := make([]string, len(values))
	for index, field := range values {
		if field.IsNull() {
			missing++
			continue
		}
		if field.Type() != resp.BulkString && field.Type() != resp.SimpleString {
			return ReadResult{Status: ReadInvalid}
		}
		fields[index] = field.String()
		if index != 3 && len(fields[index]) > 512 {
			return ReadResult{Status: ReadInvalid}
		}
	}
	if missing == len(values) {
		return ReadResult{Status: ReadMissing}
	}
	if missing != 0 {
		return ReadResult{Status: ReadIncomplete}
	}
	return r.acceptSnapshot(decodeHash(fields, r.config, now))
}

func (r *Reader) acceptSnapshot(result ReadResult) ReadResult {
	if result.Status != ReadReady {
		return result
	}
	snapshot := result.Snapshot
	r.fence.mu.Lock()
	defer r.fence.mu.Unlock()
	order := compareRevision(snapshot.metadata.Revision, r.fence.revision)
	if order < 0 {
		return ReadResult{Status: ReadStale}
	}
	if order == 0 && r.fence.digest != "" && r.fence.digest != snapshot.metadata.StateDigest {
		return ReadResult{Status: ReadConflict}
	}
	r.fence.revision, r.fence.digest = snapshot.metadata.Revision, snapshot.metadata.StateDigest
	snapshot.fence = &r.fence
	return result
}
