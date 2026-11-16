package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
)

// Regression benchmark: https://github.com/wooveep/plugin-server/issues/11.
func BenchmarkRestoreManagedResourceReferences(b *testing.B) {
	for _, count := range []int{545, 1090, 2180} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			var body strings.Builder
			body.WriteString(`{"model":"upstream-model","input":[`)
			for i := range count {
				if i != 0 {
					body.WriteByte(',')
				}
				fmt.Fprintf(&body, `{"type":"function_call_output","call_id":"native-call-%d","output":%q}`, i, strings.Repeat("x", 800))
			}
			body.WriteString(`]}`)
			raw := []byte(body.String())
			state := &managedExecutionState{config: &managedExecutionConfig{profile: managedProtocolProfile{InputProtocol: clientresponse.Responses, UpstreamProtocol: clientresponse.Responses}}}
			var restored []byte
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				restoreManagedResourceReferences(nil, state, raw, func(value []byte) { restored = value })
			}
			b.StopTimer()
			if !bytes.Equal(raw, restored) {
				b.Fatal("ordinary native tool history changed during reference inspection")
			}
		})
	}
}
