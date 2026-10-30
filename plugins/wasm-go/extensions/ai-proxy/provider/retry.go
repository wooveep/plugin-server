package provider

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	ctxRetryCount = "retryCount"
)

type retryOnFailure struct {
	// @Title zh-CN 是否启用请求重试
	enabled bool `required:"false" yaml:"enabled"`
	// @Title zh-CN 重试次数
	maxRetries int64 `required:"false" yaml:"maxRetries"`
	// @Title zh-CN 重试超时时间
	retryTimeout int64 `required:"false" yaml:"retryTimeout"`
	// @Title zh-CN 需要进行重试的原始请求的状态码，支持正则表达式匹配
	retryOnStatus []string `required:"false" yaml:"retryOnStatus"`
}

func (r *retryOnFailure) FromJson(json gjson.Result) {
	r.enabled = json.Get("enabled").Bool()
	r.maxRetries = json.Get("maxRetries").Int()
	if r.maxRetries == 0 {
		r.maxRetries = 1
	}
	r.retryTimeout = json.Get("retryTimeout").Int()
	if r.retryTimeout == 0 {
		r.retryTimeout = 60 * 1000
	}
	for _, status := range json.Get("retryOnStatus").Array() {
		r.retryOnStatus = append(r.retryOnStatus, status.String())
	}
	// If retryOnStatus is empty, default to retry on 4xx and 5xx
	if len(r.retryOnStatus) == 0 {
		r.retryOnStatus = []string{"4.*", "5.*"}
	}
}

func (c *ProviderConfig) IsRetryOnFailureEnabled() bool {
	return c.retryOnFailure.enabled
}

func (c *ProviderConfig) retryFailedRequest(activeProvider Provider, ctx wrapper.HttpContext, apiTokenInUse string, apiTokens []string) error {
	log.Infof("Retry failed request: provider=%s", activeProvider.GetProviderType())
	retryClient := createRetryClient()
	apiName, _ := ctx.GetContext(CtxKeyApiName).(ApiName)
	ctx.SetContext(ctxRetryCount, 1)
	return c.sendRetryRequest(ctx, apiName, activeProvider, retryClient, apiTokenInUse, apiTokens)
}

func (c *ProviderConfig) transformResponseHeadersAndBody(ctx wrapper.HttpContext, activeProvider Provider, apiName ApiName, headers http.Header, body []byte) ([][2]string, []byte) {
	if handler, ok := activeProvider.(TransformResponseHeadersHandler); ok {
		handler.TransformResponseHeaders(ctx, apiName, headers)
	} else {
		c.DefaultTransformResponseHeaders(ctx, headers)
	}

	if handler, ok := activeProvider.(TransformResponseBodyHandler); ok {
		var err error
		body, err = handler.TransformResponseBody(ctx, apiName, body)
		if err != nil {
			log.Errorf("Failed to transform response body: %v", err)
		}
	}

	return util.HeaderToSlice(headers), body
}

func (c *ProviderConfig) retryCall(
	ctx wrapper.HttpContext, activeProvider Provider,
	apiName ApiName, statusCode int, responseHeaders http.Header, responseBody []byte,
	retryClient *wrapper.ClusterClient[wrapper.RouteCluster],
	apiTokenInUse string, apiTokens []string) {

	retryCount := ctx.GetContext(ctxRetryCount).(int)
	log.Infof("Sent retry request: %d/%d", retryCount, c.retryOnFailure.maxRetries)

	if statusCode == 200 {
		log.Infof("Retry request succeeded")
		if ctx.GetBoolContext(CtxTransparentLifecycle, false) {
			proxywasm.SendHttpResponse(
				uint32(statusCode),
				util.HeaderToSlice(responseHeaders),
				responseBody,
				-1,
			)
			return
		}
		headers, body := c.transformResponseHeadersAndBody(ctx, activeProvider, apiName, responseHeaders, responseBody)
		proxywasm.SendHttpResponse(200, headers, body, -1)
		return
	} else {
		log.Infof("The retry request still failed, status: %d", statusCode)
	}

	retryCount++
	if retryCount <= int(c.retryOnFailure.maxRetries) {
		ctx.SetContext(ctxRetryCount, retryCount)
		err := c.sendRetryRequest(ctx, apiName, activeProvider, retryClient, apiTokenInUse, apiTokens)
		if err != nil {
			log.Errorf("sendRetryRequest failed, err:%v", err)
			proxywasm.ResumeHttpResponse()
			return
		}
	} else {
		log.Infof("Reached the maximum retry count: %d", c.retryOnFailure.maxRetries)
		proxywasm.ResumeHttpResponse()
		return
	}
}

func (c *ProviderConfig) sendRetryRequest(
	ctx wrapper.HttpContext, apiName ApiName, activeProvider Provider,
	retryClient *wrapper.ClusterClient[wrapper.RouteCluster],
	apiTokenInUse string, apiTokens []string) error {

	// Remove last failed token from retry apiTokens list
	apiTokens = removeApiTokenFromRetryList(apiTokens, apiTokenInUse)
	if len(apiTokens) == 0 {
		return errors.New("no more apiTokens to retry")
	}
	// Set apiTokenInUse for the retry request
	apiTokenInUse = GetRandomToken(apiTokens)
	log.Debugf("Retry request selected a replacement credential")
	ctx.SetContext(c.failover.ctxApiTokenInUse, apiTokenInUse)
	requestBody := ctx.GetByteSliceContext(CtxRequestBody, []byte(""))
	var (
		method          = http.MethodPost
		modifiedHeaders http.Header
		modifiedBody    []byte
	)
	if ctx.GetBoolContext(CtxTransparentLifecycle, false) {
		storedHeaders, _ := ctx.GetContext(CtxRequestHeaders).([][2]string)
		modifiedHeaders = c.BuildTransparentRequestHeaders(
			activeProvider,
			ctx,
			util.SliceToHeader(storedHeaders),
		)
		modifiedBody = append([]byte(nil), requestBody...)
		method = ctx.GetStringContext(CtxRequestMethod, http.MethodPost)
		var (
			action types.Action
			err    error
		)
		modifiedHeaders, action, err = c.BuildTransparentRequestBodyAuthHeaders(
			activeProvider,
			ctx,
			modifiedHeaders,
			modifiedBody,
		)
		if err != nil {
			return fmt.Errorf("failed to authenticate transparent retry request: %v", err)
		}
		if action != types.ActionContinue {
			return errors.New("transparent retry authentication requires asynchronous continuation")
		}
	} else {
		var err error
		modifiedHeaders, modifiedBody, err = c.transformRequestHeadersAndBody(ctx, activeProvider, [][2]string{
			{"content-type", "application/json"},
			{":authority", ctx.GetStringContext(CtxRequestHost, "")},
			{":path", ctx.GetStringContext(CtxRequestPath, "")},
		}, requestBody)
		if err != nil {
			return fmt.Errorf("sendRetryRequest failed to transform request headers and body: %v", err)
		}
	}

	err := dispatchRetryRequest(retryClient, method, modifiedHeaders, modifiedBody,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			c.retryCall(ctx, activeProvider, apiName, statusCode, responseHeaders, responseBody, retryClient, apiTokenInUse, apiTokens)
		}, uint32(c.retryOnFailure.retryTimeout))
	if err != nil {
		return fmt.Errorf("failed to send retry request: %v", err)
	}
	return nil
}

func dispatchRetryRequest(
	retryClient *wrapper.ClusterClient[wrapper.RouteCluster],
	method string,
	headers http.Header,
	body []byte,
	callback func(int, http.Header, []byte),
	timeout uint32,
) error {
	headers.Set(":method", method)
	_, err := proxywasm.DispatchHttpCall(
		retryClient.ClusterName(),
		util.HeaderToSlice(headers),
		body,
		nil,
		timeout,
		func(numHeaders, bodySize, numTrailers int) {
			responseBody, bodyErr := proxywasm.GetHttpCallResponseBody(0, bodySize)
			if bodyErr != nil && bodySize > 0 {
				log.Errorf("failed to read retry response body: %v", bodyErr)
			}
			responseHeaderSlice, headerErr := proxywasm.GetHttpCallResponseHeaders()
			if headerErr != nil {
				log.Errorf("failed to read retry response headers: %v", headerErr)
			}

			statusCode := http.StatusBadGateway
			responseHeaders := make(http.Header)
			for _, header := range responseHeaderSlice {
				responseHeaders.Add(header[0], header[1])
				if header[0] != ":status" {
					continue
				}
				parsedStatus, parseErr := strconv.Atoi(header[1])
				if parseErr != nil {
					log.Errorf("failed to parse retry response status")
					continue
				}
				statusCode = parsedStatus
			}
			callback(statusCode, responseHeaders, responseBody)
		},
	)
	return err
}

func createRetryClient() *wrapper.ClusterClient[wrapper.RouteCluster] {
	retryClient := wrapper.NewClusterClient(wrapper.RouteCluster{})
	return retryClient
}

func removeApiTokenFromRetryList(apiTokens []string, removedApiToken string) []string {
	var availableApiTokens []string
	for _, s := range apiTokens {
		if s != removedApiToken {
			availableApiTokens = append(availableApiTokens, s)
		}
	}
	log.Debugf("Removed failed credential from retry candidates")
	log.Debugf("Available retry credential count: %d", len(availableApiTokens))
	return availableApiTokens
}

func GetRandomToken(apiTokens []string) string {
	count := len(apiTokens)
	switch count {
	case 0:
		return ""
	case 1:
		return apiTokens[0]
	default:
		return apiTokens[rand.Intn(count)]
	}
}
