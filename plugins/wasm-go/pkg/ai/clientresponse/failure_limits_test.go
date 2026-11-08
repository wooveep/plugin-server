package clientresponse

import (
	"encoding/json"
	"testing"
)

func TestInputLengthFailuresWithoutSupplierParameter(t *testing.T) {
	for _, protocol := range []string{Chat, Responses, Messages} {
		for _, tc := range []struct{ code, message, expected string }{
			{"context_length_exceeded", "private-account context too long", "context_length_exceeded"},
			{"InvalidParameter", "Range of input length should be [1, 1024]; private-account", "invalid_input_length"},
			{"InternalError.Algo.InvalidParameter", "Range of input length should be [1, 1024]", "invalid_input_length"},
			{"InvalidInputLength", "private-account input length invalid", "invalid_input_length"},
		} {
			t.Run(protocol+"/"+tc.code, func(t *testing.T) {
				wire, err := json.Marshal(map[string]map[string]string{"error": {"code": tc.code, "message": tc.message, "type": "invalid_request_error"}})
				if err != nil {
					t.Fatal(err)
				}
				f := ClassifyFailure(FailureInput{Status: 400, Client: protocol, Body: wire, Request: []byte(`{"model":"public","messages":[{"role":"user","content":"hello"}],"input":"hello"}`)})
				response := f.public(protocol)
				if response.status != 400 || response.code != tc.expected || f.Retryable || f.RetryCredential || f.AllowFallback {
					t.Fatalf("input rejection classified as recoverable service failure: %+v / %+v", f, response)
				}
			})
		}
	}
}
