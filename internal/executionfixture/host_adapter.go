package executionfixture

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
)

// CalloutHost is the real SDK capability required by this thin adapter.
type CalloutHost interface {
	GetCalloutAttributesFromContext(uint32) []proxytest.HttpCalloutAttribute
	CallOnHttpCallResponse(uint32, [][2]string, [][2]string, []byte)
}
type Response struct {
	Headers  [][2]string
	Body     []byte
	Complete bool
	Read     string
}
type Record struct {
	SyntheticID, HostID uint32
	Snapshot            Snapshot
	Status              *int
	Response            Response
	Layer               string
}
type binding struct {
	hostID    uint32
	snapshot  Snapshot
	delivered bool
}

// HostAdapter keeps model lifecycle checks distinct from SDK callback delivery.
type HostAdapter struct {
	host       CalloutHost
	context    uint32
	bindings   map[uint32]*binding
	generation map[uint32]uint32
	closed     map[uint32]bool
	records    []Record
}

func NewHostAdapter(host CalloutHost, context uint32) *HostAdapter {
	return &HostAdapter{host: host, context: context, bindings: map[uint32]*binding{}, generation: map[uint32]uint32{}, closed: map[uint32]bool{}}
}
func fixtureError(code string) error { return fmt.Errorf("WG_FIXTURE_%s", code) }

// Bind captures a real pending CalloutID selected by its unique upstream label.
// It deliberately never uses the first attribute as a callback selector.
func (a *HostAdapter) Bind(synthetic uint32, upstream string, source Snapshot) error {
	if a.host == nil {
		return fixtureError("HOST_CAPABILITY_BLOCKED")
	}
	if _, exists := a.bindings[synthetic]; exists {
		return fixtureError("CALLOUT_DUPLICATE_BINDING")
	}
	var found []uint32
	for _, item := range a.host.GetCalloutAttributesFromContext(a.context) {
		if item.Upstream == upstream {
			found = append(found, item.CalloutID)
		}
	}
	if len(found) != 1 {
		return fixtureError("HOST_CALLOUT_CAPTURE_BLOCKED")
	}
	for _, previous := range a.bindings {
		if previous.hostID == found[0] {
			return fixtureError("CALLOUT_DUPLICATE_BINDING")
		}
	}
	a.bindings[synthetic] = &binding{hostID: found[0], snapshot: CaptureSnapshot(source)}
	a.generation[source.Context] = source.Generation
	return nil
}
func (a *HostAdapter) Open(context, generation uint32) {
	a.generation[context] = generation
	a.closed[context] = false
}
func (a *HostAdapter) Close(context uint32) { a.closed[context] = true }
func (a *HostAdapter) Pending() []uint32 {
	var result []uint32
	for id, item := range a.bindings {
		if !item.delivered {
			result = append(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
func (a *HostAdapter) CapturedIDs() map[uint32]uint32 {
	result := make(map[uint32]uint32, len(a.bindings))
	for synthetic, item := range a.bindings {
		result[synthetic] = item.hostID
	}
	return result
}
func (a *HostAdapter) Records() []Record {
	result := make([]Record, len(a.records))
	for i, record := range a.records {
		result[i] = record
		result[i].Snapshot = CaptureSnapshot(record.Snapshot)
		result[i].Response.Headers = append([][2]string(nil), record.Response.Headers...)
		result[i].Response.Body = append([]byte(nil), record.Response.Body...)
		if record.Status != nil {
			status := *record.Status
			result[i].Status = &status
		}
	}
	return result
}
func (a *HostAdapter) Release(id uint32, assertedSnapshot string, response Response) error {
	item, exists := a.bindings[id]
	if !exists {
		return fixtureError("CALLOUT_UNKNOWN")
	}
	if item.delivered {
		return fixtureError("CALLBACK_DUPLICATE")
	}
	if assertedSnapshot != "" && assertedSnapshot != item.snapshot.ID {
		return fixtureError("CALLOUT_SNAPSHOT_MISMATCH")
	}
	if a.generation[item.snapshot.Context] != item.snapshot.Generation {
		return fixtureError("CALLBACK_GENERATION_MISMATCH")
	}
	if a.closed[item.snapshot.Context] {
		return fixtureError("CONTEXT_CLOSED")
	}
	if response.Read != "ok" { // SDK cannot inject a GetHttpCallResponseBody error: record the model result only.
		item.delivered = true
		a.records = append(a.records, Record{SyntheticID: id, HostID: item.hostID, Snapshot: item.snapshot, Status: responseStatus(response), Response: response, Layer: "contract_fixture"})
		return fixtureError("CALLBACK_BODY_READ_FAILED")
	}
	a.host.CallOnHttpCallResponse(item.hostID, response.Headers, nil, response.Body)
	item.delivered = true
	a.records = append(a.records, Record{SyntheticID: id, HostID: item.hostID, Snapshot: item.snapshot, Status: responseStatus(response), Response: response, Layer: "host_emulator"})
	if !response.Complete {
		return fixtureError("CALLBACK_BODY_TRUNCATED")
	}
	return nil
}
func responseStatus(response Response) *int {
	for _, pair := range response.Headers {
		if pair[0] == ":status" {
			if value, err := strconv.Atoi(pair[1]); err == nil {
				return &value
			}
		}
	}
	return nil
}
