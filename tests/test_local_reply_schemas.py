import unittest
from pathlib import Path

from artifact_contract import ArtifactError, check_shape, contract_registry, registry_for
from mf_canonical import parse_json, schema_numbers
from plugin_schema_conversion import convert_spec

ROOT = Path(__file__).resolve().parents[1]


def _validate(spec_rel: str, schema_file: str, instance: dict) -> None:
    converted = convert_spec((ROOT / spec_rel).read_bytes())
    base_schemas, _ = contract_registry()
    raw_schemas = {k: schema_numbers(parse_json(v)) for k, v in converted["schemas"].items()}
    reg_map = {**base_schemas}
    for key, schema in raw_schemas.items():
        reg_map["https://example.invalid/" + key] = {**schema, "$id": "https://example.invalid/" + key}
    registry = registry_for(reg_map)
    check_shape(instance, reg_map["https://example.invalid/" + schema_file], registry, "LOCAL_REPLY_TEST_INVALID")


class LocalReplySchemaTests(unittest.TestCase):
    def test_route_and_global_local_reply_policy_contracts(self):
        cases = [
            ("plugins/wasm-go/resources/plugins/key-auth/spec.yaml", "route.schema.json", {"allow": ["c1"]}),
            ("plugins/release/metadata/model-router/spec.yaml", "config.schema.json", {"modelToHeader": "x-higress-llm-model"}),
            ("plugins/wasm-go/resources/plugins/ai-quota/spec.yaml", "route.schema.json", {"path_scope": "bound_target"}),
            ("plugins/wasm-go/resources/plugins/ai-token-ratelimit/spec.yaml", "route.schema.json", {
                "rule_name": "r1", "global_threshold": {"token_per_minute": 100}, "redis": {"service_name": "r.static"},
            }),
            ("plugins/wasm-go/resources/plugins/cluster-key-rate-limit/spec.yaml", "route.schema.json", {
                "rule_name": "r1", "global_threshold": {"query_per_minute": 100}, "redis": {"service_name": "r.static"},
            }),
            ("plugins/wasm-go/resources/plugins/model-mapper/spec.yaml", "route.schema.json", {
                "capture_body_basis": True, "modelMapping": {"a": "b"},
            }),
            ("plugins/wasm-go/resources/plugins/ai-cache/spec.yaml", "route.schema.json", {"cache_policy_version": "1"}),
            ("plugins/wasm-go/resources/plugins/ai-history/spec.yaml", "route.schema.json", {"redis": {"serviceName": "r.dns"}}),
            ("plugins/wasm-go/resources/plugins/ai-memory/spec.yaml", "route.schema.json", {
                "path_scope": "bound_target", "client_protocol": "anthropic_messages",
                "operation_kind": "text_generate", "memory_mode": "semantic",
            }),
            ("plugins/wasm-go/resources/plugins/ai-billing/spec.yaml", "route.schema.json", {"path_scope": "bound_target"}),
        ]
        valid_policies = [
            {"input_protocol": "openai_chat"},
            {"input_protocol": "openai_completions"},
            {"input_protocol": "openai_responses"},
            {"input_protocol": "anthropic_messages"},
            {"input_protocol": "openai_chat", "response_profile": "deepseek"},
            {"input_protocol": "openai_chat", "response_profile": "qwen"},
            {"input_protocol": "openai_chat", "response_profile": "openai"},
            {"input_protocol": "openai_completions", "response_profile": "deepseek"},
            {"input_protocol": "openai_responses", "response_profile": "deepseek"},
            {"input_protocol": "openai_responses", "response_profile": "qwen"},
            {"input_protocol": "openai_responses", "response_profile": "openai"},
            {"input_protocol": "anthropic_messages", "response_profile": "deepseek"},
            {"input_protocol": "anthropic_messages", "response_profile": "qwen"},
        ]
        invalid_policies = [
            {},
            {"input_protocol": "anthropic_messages", "response_profile": "openai"},
            {"response_profile": "deepseek"},
            {"input_protocol": "openai_chat", "response_profile": ""},
            {"input_protocol": "openai_chat", "response_profile": None},
            {"input_protocol": "openai_chat", "response_profile": "unknown"},
            {"input_protocol": "openai_embeddings", "response_profile": "deepseek"},
            {"input_protocol": "openai_chat", "response_profile": "deepseek", "extra": True},
        ]
        for spec_rel, schema_file, base in cases:
            with self.subTest(spec=spec_rel):
                _validate(spec_rel, schema_file, base)
                for policy in valid_policies:
                    _validate(spec_rel, schema_file, {**base, "local_reply_policy": policy})
                for bad in invalid_policies:
                    with self.assertRaises(ArtifactError):
                        _validate(spec_rel, schema_file, {**base, "local_reply_policy": bad})

    def test_preroute_selector_mutual_exclusion_in_key_auth_and_model_router(self):
        for spec_rel, schema_file, base in [
            ("plugins/wasm-go/resources/plugins/key-auth/spec.yaml", "route.schema.json", {"allow": ["c1"]}),
            ("plugins/wasm-go/resources/plugins/key-auth/spec.yaml", "config.schema.json", {"global_auth": True}),
            ("plugins/release/metadata/model-router/spec.yaml", "config.schema.json", {"modelToHeader": "x-model"}),
        ]:
            with self.subTest(spec=spec_rel, schema=schema_file):
                _validate(spec_rel, schema_file, {
                    **base,
                    "local_reply_default": True,
                    "local_reply_entrances": [{
                        "domains": ["api.example.com"],
                        "path": {"match_type": "PRE", "match_value": "/custom/messages", "case_sensitive": True},
                        "local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "qwen"},
                    }],
                })
                with self.assertRaises(ArtifactError):
                    _validate(spec_rel, schema_file, {
                        **base,
                        "local_reply_default": True,
                        "local_reply_policy": {"input_protocol": "openai_chat", "response_profile": "deepseek"},
                    })
                with self.assertRaises(ArtifactError):
                    _validate(spec_rel, schema_file, {
                        **base,
                        "local_reply_entrances": [],
                        "local_reply_policy": {"input_protocol": "openai_chat", "response_profile": "deepseek"},
                    })

    def test_tenant_policies_schemas_accept_local_reply_policy(self):
        tenant_id = "11111111-1111-4111-8111-111111111111"
        _validate("plugins/wasm-go/resources/plugins/ai-history/spec.yaml", "route.schema.json", {
            "path_scope": "bound_target",
            "client_protocol": "anthropic_messages",
            "operation_kind": "text_generate",
            "public_model": "claude-sonnet",
            "capture_body_basis": True,
            "native_route": {"release_id": "r1", "target_id": "t1", "route_id": tenant_id},
            "local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "qwen"},
            "tenant_policies": {
                tenant_id: {
                    "redis": {"serviceName": "r.dns"},
                    "local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "qwen"},
                }
            },
        })
        _validate("plugins/wasm-go/resources/plugins/ai-memory/spec.yaml", "route.schema.json", {
            "path_scope": "bound_target",
            "client_protocol": "anthropic_messages",
            "operation_kind": "text_generate",
            "public_model": "claude-sonnet",
            "capture_body_basis": True,
            "native_route": {"release_id": "r1", "target_id": "t1", "route_id": tenant_id},
            "local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "deepseek"},
            "tenant_policies": {
                tenant_id: {
                    "path_scope": "bound_target",
                    "client_protocol": "anthropic_messages",
                    "operation_kind": "text_generate",
                    "memory_mode": "semantic",
                    "local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "deepseek"},
                }
            },
        })

    def test_rate_limit_limit_keys_minimum_zero_and_negative_rejection(self):
        for spec_rel, field in [
            ("plugins/wasm-go/resources/plugins/cluster-key-rate-limit/spec.yaml", "query_per_minute"),
            ("plugins/wasm-go/resources/plugins/ai-token-ratelimit/spec.yaml", "token_per_minute"),
        ]:
            for schema_file in ("config.schema.json", "route.schema.json"):
                with self.subTest(spec=spec_rel, schema=schema_file):
                    _validate(spec_rel, schema_file, {
                        "rule_name": "static-user-limit",
                        "redis": {"service_name": "redis.static"},
                        "rule_items": [{
                            "limit_by_per_header": "x-modelfusion-principal-id",
                            "limit_keys": [
                                {"key": "regexp:^unlimited$", field: 0},
                                {"key": "*", field: 10},
                            ],
                        }],
                    })
                    with self.assertRaises(ArtifactError):
                        _validate(spec_rel, schema_file, {
                            "rule_name": "static-user-limit",
                            "redis": {"service_name": "redis.static"},
                            "rule_items": [{
                                "limit_by_per_header": "x-modelfusion-principal-id",
                                "limit_keys": [{"key": "*", field: -1}],
                            }],
                        })


if __name__ == "__main__":
    unittest.main()
