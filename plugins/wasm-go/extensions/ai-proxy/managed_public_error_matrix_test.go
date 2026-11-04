package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPublicErrorOfficialBusinessCodeMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, code, message, errorType, publicCode, retry string
		status, publicStatus                              int
	}{
		{"invalid_parameter", "InvalidParameter", "max_tokens must be positive", "invalid_request_error", "invalid_parameter", "false", 400, 400},
		{"arrearage_is_not_bad_input", "Arrearage", "fixture-acc-7 in arrears", "api_error", "service_unavailable", "false", 400, 503},
		{"invalid_purchase_credential", "InvalidApiKey", "fixture-acc-7 key invalid", "api_error", "service_unavailable", "false", 401, 503},
		{"model_access_is_platform_configuration", "Model.AccessDenied", "fixture-acc-7 denied", "api_error", "service_unavailable", "false", 403, 503},
		{"rate_quota", "Throttling.RateQuota", "fixture-acc-7 rate exceeded", "rate_limit_error", "rate_limit_exceeded", "true", 429, 429},
		{"concurrency", "Throttling.Concurrency", "fixture-acc-7 concurrency exceeded", "rate_limit_error", "rate_limit_exceeded", "true", 429, 429},
		{"overload_is_not_account_quota", "Throttling.ServiceOverloaded", "fixture-acc-7 server busy", "overloaded_error", "server_is_overloaded", "true", 429, 529},
		{"allocation_is_not_short_rate", "Throttling.AllocationQuota", "fixture-acc-7 allocation exhausted", "api_error", "service_unavailable", "false", 429, 503},
		{"insufficient_quota", "insufficient_quota", "fixture-acc-7 free tier exhausted", "api_error", "service_unavailable", "false", 429, 503},
		{"prepaid_expired", "PrepaidBillOverdue", "fixture-acc-7 expired", "api_error", "service_unavailable", "false", 429, 503},
		{"budget_limit", "BudgetLimitExceeded", "fixture-acc-7 budget stop", "api_error", "service_unavailable", "false", 429, 503},
		{"internal_failure", "InternalError", "fixture-acc-7 internal failure", "api_error", "server_error", "true", 500, 500},
		{"temporary_unavailable", "ServiceUnavailable", "fixture-acc-7 temporarily unavailable", "overloaded_error", "server_is_overloaded", "true", 503, 529},
	} {
		for _, native := range []bool{false, true} {
			t.Run(tc.name+"/native="+strconv.FormatBool(native), func(t *testing.T) {
				provider := `{"type":"qwen","qwenEnableCompatible":true,"apiTokens":["controlled-upstream-key"]}`
				fields := map[string]string{"code": tc.code, "message": tc.message, "param": "max_tokens"}
				body, err := json.Marshal(map[string]map[string]string{"error": fields})
				require.NoError(t, err)
				if native {
					provider = managedQwenFixtureProvider
					body, err = json.Marshal(map[string]string{"code": tc.code, "message": tc.message, "request_id": "qwen-private-trace"})
					require.NoError(t, err)
				}
				h := publicFailureProbe(t, managedProtocolMessages, managedProtocolChat, provider, tc.status, false, string(body))
				response := h.host.GetSentLocalResponse(h.id)
				require.EqualValues(t, tc.publicStatus, response.StatusCode)
				require.Equal(t, tc.errorType, gjson.GetBytes(response.Data, "error.type").String())
				require.Equal(t, tc.publicCode, gjson.GetBytes(response.Data, "error.code").String())
				for _, hidden := range []string{"fixture-acc-7", "qwen", "upstream-model", "procurement", "supplier"} {
					require.NotContains(t, strings.ToLower(string(response.Data)), hidden)
				}
				retry, _ := managedSingleHeader(response.Headers, "x-should-retry")
				require.Equal(t, tc.retry, retry)
				wait, _ := managedSingleHeader(response.Headers, "retry-after")
				if tc.retry == "true" {
					require.Equal(t, "10", wait)
				} else {
					require.Empty(t, wait)
				}
				require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id), "no policy permits an extra business send")
			})
		}
	}
}

func TestPublicDeepSeekSevenHTTPStatesAndParameterOwnership(t *testing.T) {
	for _, tc := range []struct {
		status, publicStatus           int
		code, param, publicCode, retry string
	}{
		{400, 503, "invalid_request_error", "", "service_unavailable", "false"},
		{422, 400, "invalid_request_error", "max_tokens", "invalid_parameter", "false"},
		{401, 503, "invalid_api_key", "", "service_unavailable", "false"},
		{402, 503, "insufficient_balance", "", "service_unavailable", "false"},
		{429, 429, "rate_limit_exceeded", "", "rate_limit_exceeded", "true"},
		{500, 500, "server_error", "", "server_error", "true"},
		{503, 529, "server_error", "", "server_is_overloaded", "true"},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			body, err := json.Marshal(map[string]map[string]string{"error": {"code": tc.code, "param": tc.param, "message": "deepseek fixture-acc-7 diagnostic"}})
			require.NoError(t, err)
			h := publicFailureProbe(t, managedProtocolMessages, managedProtocolChat, `{"type":"deepseek","apiTokens":["controlled-upstream-key"]}`, tc.status, false, string(body))
			response := h.host.GetSentLocalResponse(h.id)
			require.EqualValues(t, tc.publicStatus, response.StatusCode)
			require.Equal(t, tc.publicCode, gjson.GetBytes(response.Data, "error.code").String())
			require.NotContains(t, string(response.Data), "fixture-acc-7")
			require.NotContains(t, string(response.Data), "deepseek")
			retry, _ := managedSingleHeader(response.Headers, "x-should-retry")
			require.Equal(t, tc.retry, retry)
		})
	}
}
