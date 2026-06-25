package vector

import (
	"net/http"
	"testing"

	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func testPayloadMetadata() PayloadMetadata {
	return PayloadMetadata{
		CacheEntryKey: "higress-ai-cache:{aiscope:tenant}:entry:abc123",
		Scope: PayloadScope{
			Type:     "tenant",
			Tenant:   "tenant-a",
			Consumer: "consumer-a",
		},
		Route: PayloadRoute{
			ID:      "route-1",
			Name:    "chat-route",
			APIName: "chat-api",
		},
		Provider: PayloadProvider{
			Name:  "qwen",
			Model: "qwen-plus",
		},
		Model:         "qwen-plus",
		PolicyVersion: "pv-20260625",
		RequestFingerprint: PayloadRequestFingerprint{
			Hash: "request-fingerprint",
		},
		Semantic: PayloadSemantic{
			TextHash: "semantic-text-hash",
		},
		SoftExpiresAt: 1782378600,
		HardExpiresAt: 1782382200,
	}
}

func requirePayloadMetadataFields(t *testing.T, body []byte, prefix string) {
	t.Helper()
	require.Equal(t, "higress-ai-cache:{aiscope:tenant}:entry:abc123", gjson.GetBytes(body, prefix+"cache_entry_key").String())
	require.Equal(t, "tenant", gjson.GetBytes(body, prefix+"scope_type").String())
	require.Equal(t, "tenant-a", gjson.GetBytes(body, prefix+"scope_tenant").String())
	require.Equal(t, "consumer-a", gjson.GetBytes(body, prefix+"scope_consumer").String())
	require.Equal(t, "route-1", gjson.GetBytes(body, prefix+"route_id").String())
	require.Equal(t, "chat-route", gjson.GetBytes(body, prefix+"route_name").String())
	require.Equal(t, "chat-api", gjson.GetBytes(body, prefix+"route_api_name").String())
	require.Equal(t, "qwen", gjson.GetBytes(body, prefix+"provider_name").String())
	require.Equal(t, "qwen-plus", gjson.GetBytes(body, prefix+"provider_model").String())
	require.Equal(t, "qwen-plus", gjson.GetBytes(body, prefix+"model").String())
	require.Equal(t, "pv-20260625", gjson.GetBytes(body, prefix+"policy_version").String())
	require.Equal(t, "request-fingerprint", gjson.GetBytes(body, prefix+"request_fingerprint_hash").String())
	require.Equal(t, "semantic-text-hash", gjson.GetBytes(body, prefix+"semantic_text_hash").String())
	require.Equal(t, int64(1782378600), gjson.GetBytes(body, prefix+"soft_expires_at").Int())
	require.Equal(t, int64(1782382200), gjson.GetBytes(body, prefix+"hard_expires_at").Int())
}

func TestPayloadMetadataFieldsUseOpenSpecNames(t *testing.T) {
	fields := testPayloadMetadata().Fields()

	require.Equal(t, "higress-ai-cache:{aiscope:tenant}:entry:abc123", fields["cache_entry_key"])
	require.Equal(t, "tenant", fields["scope_type"])
	require.Equal(t, "tenant-a", fields["scope_tenant"])
	require.Equal(t, "consumer-a", fields["scope_consumer"])
	require.Equal(t, "route-1", fields["route_id"])
	require.Equal(t, "chat-route", fields["route_name"])
	require.Equal(t, "chat-api", fields["route_api_name"])
	require.Equal(t, "qwen", fields["provider_name"])
	require.Equal(t, "qwen-plus", fields["provider_model"])
	require.Equal(t, "qwen-plus", fields["model"])
	require.Equal(t, "pv-20260625", fields["policy_version"])
	require.Equal(t, "request-fingerprint", fields["request_fingerprint_hash"])
	require.Equal(t, "semantic-text-hash", fields["semantic_text_hash"])
	require.Equal(t, int64(1782378600), fields["soft_expires_at"])
	require.Equal(t, int64(1782382200), fields["hard_expires_at"])
}

func TestDashVectorUploadParametersIncludeCacheMetadata(t *testing.T) {
	provider := &DvProvider{config: ProviderConfig{collectionID: "collection", apiKey: "token"}}

	_, body, _, err := provider.constructUploadParametersWithMetadata([]float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "question", gjson.GetBytes(body, "docs.0.fields.query").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "docs.0.fields.answer").String())
	requirePayloadMetadataFields(t, body, "docs.0.fields.")
}

func TestDashVectorLegacyUploadParametersDoNotIncludeMetadata(t *testing.T) {
	provider := &DvProvider{config: ProviderConfig{collectionID: "collection", apiKey: "token"}}

	_, body, _, err := provider.constructUploadParameters([]float64{0.1, 0.2}, "question", "answer")

	require.NoError(t, err)
	require.Equal(t, "question", gjson.GetBytes(body, "docs.0.fields.query").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "docs.0.fields.answer").String())
	require.False(t, gjson.GetBytes(body, "docs.0.fields.cache_entry_key").Exists())
	require.False(t, gjson.GetBytes(body, "docs.0.fields.soft_expires_at").Exists())
	require.False(t, gjson.GetBytes(body, "docs.0.fields.hard_expires_at").Exists())
}

func TestDashVectorMetadataUploadFallsBackToLegacyPayloadOnRemoteReject(t *testing.T) {
	client := &recordingHTTPClient{postStatuses: []int{http.StatusBadRequest, http.StatusOK}}
	provider := &DvProvider{
		config: ProviderConfig{collectionID: "collection", apiKey: "token"},
		client: client,
	}

	err := provider.UploadAnswerAndEmbeddingWithMetadata("question", []float64{0.1, 0.2}, "answer", testPayloadMetadata(), nil, noopLog{}, func(ctx wrapper.HttpContext, log log.Log, err error) {})

	require.NoError(t, err)
	require.Len(t, client.posts, 2)
	require.Equal(t, "higress-ai-cache:{aiscope:tenant}:entry:abc123", gjson.GetBytes(client.posts[0].body, "docs.0.fields.cache_entry_key").String())
	require.False(t, gjson.GetBytes(client.posts[1].body, "docs.0.fields.cache_entry_key").Exists())
	require.False(t, gjson.GetBytes(client.posts[1].body, "docs.0.fields.soft_expires_at").Exists())
	require.Equal(t, "question", gjson.GetBytes(client.posts[1].body, "docs.0.fields.query").String())
	require.Equal(t, "answer", gjson.GetBytes(client.posts[1].body, "docs.0.fields.answer").String())
}

func TestQdrantUploadRequestIncludesCacheMetadata(t *testing.T) {
	body, err := qdrantUploadRequestBody([]float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "question", gjson.GetBytes(body, "points.0.payload.question").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "points.0.payload.answer").String())
	requirePayloadMetadataFields(t, body, "points.0.payload.")
}

func TestPineconeUploadRequestIncludesCacheMetadata(t *testing.T) {
	body, err := pineconeUploadRequestBody("namespace", []float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "namespace", gjson.GetBytes(body, "namespace").String())
	require.Equal(t, "question", gjson.GetBytes(body, "vectors.0.metadata.question").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "vectors.0.metadata.answer").String())
	requirePayloadMetadataFields(t, body, "vectors.0.metadata.")
}

func TestChromaUploadRequestIncludesCacheMetadata(t *testing.T) {
	body, err := chromaUploadRequestBody([]float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "question", gjson.GetBytes(body, "ids.0").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "documents.0").String())
	requirePayloadMetadataFields(t, body, "metadatas.0.")
}

func TestWeaviateUploadRequestIncludesCacheMetadata(t *testing.T) {
	body, err := weaviateUploadRequestBody("collection", []float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "collection", gjson.GetBytes(body, "class").String())
	require.Equal(t, "question", gjson.GetBytes(body, "properties.question").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "properties.answer").String())
	requirePayloadMetadataFields(t, body, "properties.")
}

func TestElasticsearchUploadRequestIncludesCacheMetadata(t *testing.T) {
	body, err := esUploadRequestBody([]float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "question", gjson.GetBytes(body, "question").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "answer").String())
	requirePayloadMetadataFields(t, body, "")
}

func TestMilvusUploadRequestIncludesCacheMetadata(t *testing.T) {
	body, err := milvusUploadRequestBody("collection", []float64{0.1, 0.2}, "question", "answer", testPayloadMetadata())

	require.NoError(t, err)
	require.Equal(t, "collection", gjson.GetBytes(body, "collectionName").String())
	require.Equal(t, "question", gjson.GetBytes(body, "data.0.question").String())
	require.Equal(t, "answer", gjson.GetBytes(body, "data.0.answer").String())
	requirePayloadMetadataFields(t, body, "data.0.")
}

type recordedHTTPPost struct {
	rawURL string
	body   []byte
}

type recordingHTTPClient struct {
	posts        []recordedHTTPPost
	postStatuses []int
}

func (c *recordingHTTPClient) Post(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	c.posts = append(c.posts, recordedHTTPPost{rawURL: rawURL, body: body})
	statusCode := http.StatusOK
	if len(c.postStatuses) > 0 {
		statusCode = c.postStatuses[0]
		c.postStatuses = c.postStatuses[1:]
	}
	if cb != nil {
		cb(statusCode, http.Header{}, nil)
	}
	return nil
}

func (c *recordingHTTPClient) Get(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Head(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Options(rawURL string, headers [][2]string, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Put(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Patch(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Delete(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Connect(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Trace(rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) Call(method, rawURL string, headers [][2]string, body []byte, cb wrapper.ResponseCallback, timeoutMillisecond ...uint32) error {
	return nil
}

func (c *recordingHTTPClient) ClusterName() string {
	return "test-cluster"
}

type noopLog struct{}

func (noopLog) Trace(msg string)                             {}
func (noopLog) Tracef(format string, args ...interface{})    {}
func (noopLog) Debug(msg string)                             {}
func (noopLog) Debugf(format string, args ...interface{})    {}
func (noopLog) Info(msg string)                              {}
func (noopLog) Infof(format string, args ...interface{})     {}
func (noopLog) Warn(msg string)                              {}
func (noopLog) Warnf(format string, args ...interface{})     {}
func (noopLog) Error(msg string)                             {}
func (noopLog) Errorf(format string, args ...interface{})    {}
func (noopLog) Critical(msg string)                          {}
func (noopLog) Criticalf(format string, args ...interface{}) {}
func (noopLog) ResetID(pluginID string)                      {}
