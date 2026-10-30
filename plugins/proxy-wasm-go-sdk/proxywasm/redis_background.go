package proxywasm

import "github.com/higress-group/proxy-wasm-go-sdk/proxywasm/internal"

// DispatchRedisCallInBackground owns the Redis call and callback in the plugin
// context, so event delivery can finish after its originating HTTP request.
// The callback must not access or resume that request. The existing Redis
// timeout and plugin lifetime still bound the operation.
func DispatchRedisCallInBackground(cluster string, query []byte, callback func(status, responseSize int)) (uint32, error) {
	return internal.DispatchOnRootContext(func() (uint32, error) {
		return DispatchRedisCall(cluster, query, callback)
	})
}
