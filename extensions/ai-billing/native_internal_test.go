package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/tidwall/gjson"
)

func nativeInternalBillingFixture(t *testing.T, embedding bool) (BillingConfig, *nativeBillingTestContext, *nativeBillingState, executionfacts.Snapshot) {
	t.Helper()
	cfg, value := nativeBillingConfigFixture(t)
	value["native_execution"].(map[string]interface{})["call_identity_source"] = executionfacts.CallIdentitySourceInternal
	release := value["_modelfusion_execution"].(map[string]interface{})
	descriptor := release["descriptor"].(map[string]interface{})
	if embedding {
		descriptor["adapter_ref"] = "openai-embeddings.v1"
		descriptor["transport_contract"].(map[string]interface{})["path_adapter_ref"] = "openai-embeddings.v1"
		descriptor["capabilities"] = map[string]nativeBillingCapability{executionfacts.NativeEmbeddingOperation: {SendMode: "single_send", ResponseClassifierRef: "ai-proxy.native-embedding.v1", MaxAttempts: 1}}
		raw, _ := json.Marshal(descriptor)
		digest, err := nativeBillingDescriptorDigest(raw)
		if err != nil {
			t.Fatal(err)
		}
		descriptor["descriptor_digest"] = digest
	}
	raw, _ := json.Marshal(value)
	if err := configureNativeBilling(gjson.ParseBytes(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	ctx := newNativeBillingTestContext()
	state := &nativeBillingState{config: cfg.NativeExecution.(*nativeBillingConfig)}
	ctx.SetContext(nativeBillingContextKey, state)
	captureNativeBillingIdentity(ctx, state, [][2]string{{executionfacts.InternalCallReferenceHeader, "50000000-0000-4000-8000-000000000009"}, {balanceidentity.APIKeyHeader, nativeKeyFixture}, {balanceidentity.GenerationHeader, "9007199254740993"}})
	if !state.valid || state.identity.AccountID != "" || state.internal == nil {
		t.Fatal("internal capture required or invented a wallet")
	}
	fact := executionfacts.Envelope{ProducerID: state.config.producerID, Kind: "call_delivery", CallID: state.internal.CallID, ReleaseID: state.config.release.Descriptor.ReleaseID, RouteID: state.config.release.RouteID,
		TenantID: nativeTenantFixture, PrincipalID: "service/ai-memory/" + nativePrincipalFixture, PrincipalKind: "service", BillingMode: "internal_cost", CostOwner: &executionfacts.CostOwner{Kind: "tenant", OwnerID: nativeTenantFixture, SourceJobID: "original-memory-job"}, OperationKind: nativeBillingOperation(state.config.release), OccurredAt: "2026-10-29T00:00:01Z", ObservedAt: "2026-10-29T00:00:02Z", DeliveryState: "succeeded", ServedFrom: "upstream", PublicModel: state.config.release.PublicModel, Usage: executionfacts.MissingTokenUsage("provider_usage_missing")}
	var snapshot executionfacts.Snapshot
	var err error
	if embedding {
		snapshot, err = executionfacts.FreezeEmbedding(fact)
	} else {
		snapshot, err = executionfacts.Freeze(fact)
	}
	if err != nil {
		t.Fatal(err)
	}
	return cfg, ctx, state, snapshot
}

func TestNativeInternalBillingWritesOnlyProtectedOriginalDeliveryBytes(t *testing.T) {
	for _, embedding := range []bool{false, true} {
		name := "text"
		if embedding {
			name = "embedding"
		}
		t.Run(name, func(t *testing.T) {
			cfg, ctx, state, snapshot := nativeInternalBillingFixture(t, embedding)
			redis := &nativeBillingTestRedis{}
			cfg.redisClient = redis
			var value string
			var err error
			if embedding {
				value, err = executionfacts.EncodeEmbeddingDeliveryBridge(snapshot)
			} else {
				value, err = executionfacts.EncodeDeliveryBridge(snapshot)
			}
			if err != nil {
				t.Fatal(err)
			}
			consumeNativeBillingBridge(ctx, cfg, state, value)
			consumeNativeBillingBridge(ctx, cfg, state, value)
			if state.gap || state.delivery == nil || len(redis.commands) != 1 || !bytes.Equal([]byte(redis.commands[0][6].(string)), snapshot.Bytes()) {
				t.Fatal("internal delivery bytes changed or were double written")
			}
			if state.snapshot.Envelope().AccountID != "" || state.snapshot.Envelope().SalesAssetID != "" {
				t.Fatal("static asset metadata created a customer sale")
			}
		})
	}
}

func TestNativeInternalBillingRejectsChangedCallerOrFrozenRoute(t *testing.T) {
	for _, field := range []string{"call", "producer", "release", "route", "public_model", "customer", "attempt"} {
		t.Run(field, func(t *testing.T) {
			cfg, ctx, state, original := nativeInternalBillingFixture(t, false)
			redis := &nativeBillingTestRedis{}
			cfg.redisClient = redis
			fact := original.Envelope()
			switch field {
			case "call":
				fact.CallID = "foreign-call"
			case "producer":
				fact.ProducerID = "foreign-producer"
			case "release":
				fact.ReleaseID = "foreign-release"
			case "route":
				fact.RouteID = "foreign-route"
			case "public_model":
				fact.PublicModel = "foreign-model"
			case "customer":
				fact.BillingMode, fact.PrincipalKind, fact.CostOwner = "customer", "user", nil
				fact.AccountID, fact.SalesAssetID = nativeAccountFixture, nativeAssetFixture
			case "attempt":
				fact.AttemptID = "foreign-attempt"
			}
			fact.PayloadDigest = ""
			snapshot, err := executionfacts.Freeze(fact)
			if err != nil {
				t.Fatal(err)
			}
			value, err := executionfacts.EncodeDeliveryBridge(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			consumeNativeBillingBridge(ctx, cfg, state, value)
			if len(redis.commands) != 0 || !state.gap {
				t.Fatal("changed internal ownership was published")
			}
		})
	}
}
