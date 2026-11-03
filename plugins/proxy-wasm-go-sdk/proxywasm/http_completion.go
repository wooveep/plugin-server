package proxywasm

import "github.com/higress-group/proxy-wasm-go-sdk/proxywasm/internal"

// DispatchHttpCallWithCompletion retains the result of an already dispatched call.
// An alive request receives the usual context. After deletion, the callback runs
// in the plugin context with requestAlive=false and must never resume or respond
// to the deleted request. Other HTTP callers retain their cancellation behavior.
func DispatchHttpCallWithCompletion(cluster string, headers [][2]string, body []byte, trailers [][2]string, timeoutMillisecond uint32, callback func(numHeaders, bodySize, numTrailers int, requestAlive bool)) (uint32, error) {
	// The ABI dispatch and callback registration complete in the same VM turn.
	id, err := DispatchHttpCall(cluster, headers, body, trailers, timeoutMillisecond, func(int, int, int) {})
	if err == nil {
		internal.RegisterHttpCalloutCompletion(id, callback)
	}
	return id, err
}
