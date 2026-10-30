package upstreamhealth

import (
	"bytes"
	"testing"
	"time"
)

func TestHTTPSnapshotUsesOriginalSourceDigestAndClosedJSONRules(t *testing.T) {
	fields := hashFixture(t, fixtureWire())
	if result := DecodeSnapshot([]byte(fields[3]), fixtureConfig(), fixtureNow); result.Status != ReadReady {
		t.Fatalf("original snapshot rejected: %s", result.Status)
	}
	for _, raw := range [][]byte{
		bytes.Replace([]byte(fields[3]), []byte(`"complete":true`), []byte(`"complete":true,"complete":true`), 1),
		bytes.Replace([]byte(fields[3]), []byte(`"revision":"9007199254740993"`), []byte(`"revision":9007199254740993`), 1),
		bytes.Replace([]byte(fields[3]), []byte(`"backoff_step":0`), []byte(`"backoff_step":1`), 1),
	} {
		if DecodeSnapshot(raw, fixtureConfig(), fixtureNow).Status == ReadReady {
			t.Fatal("HTTP path bypassed original JSON/digest validation")
		}
	}
}

func TestHTTPSnapshotSharesOriginalReadAndACKWatermarkWithNativeCaller(t *testing.T) {
	r := &gatewayRecorder{}
	client := gatewayClientFixture(t, r)
	wire := fixtureWire()
	now := time.Now().UTC()
	wire.ObservedAt, wire.ValidUntil = now.Add(-time.Second).Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano)
	var read ReadResult
	if err := client.Snapshot(fixtureConfig(), func(value ReadResult) { read = value }); err != nil {
		t.Fatal(err)
	}
	if r.calls[0].method != "GET" || r.calls[0].path != SnapshotPath || len(r.calls[0].body) != 0 {
		t.Fatal("snapshot read performed another command")
	}
	fields := hashFixture(t, wire)
	r.calls[0].callback(GatewayResponse{StatusCode: 200, HeadersComplete: true, BodyComplete: true, Body: []byte(fields[3])})
	if client.DecideNewCall(read, Candidate{Target: fixtureTarget()}, now).Kind != Eligible {
		t.Fatal("original HTTP snapshot was not eligible")
	}
	wire.Scopes[0].BackoffStep = 1
	fields = hashFixture(t, wire)
	if err := client.Snapshot(fixtureConfig(), func(value ReadResult) { read = value }); err != nil {
		t.Fatal(err)
	}
	r.calls[1].callback(GatewayResponse{StatusCode: 200, HeadersComplete: true, BodyComplete: true, Body: []byte(fields[3])})
	if client.DecideNewCall(read, Candidate{Target: fixtureTarget()}, now).ControlStatus != ReadConflict {
		t.Fatal("HTTP snapshot lost same-revision conflict protection")
	}
}
