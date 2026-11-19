#!/usr/bin/env python3
"""Offline T25 consumer verification for Canonical IR C client outputs.

Uses the locally installed official OpenAI Python SDK (via httpx.MockTransport,
zero network calls) for OpenAI Chat Completions (JSON + SSE) and OpenAI
Responses (JSON + SSE completed/incomplete/failed), and a strict Pydantic v2
(extra='forbid') + httpx.MockTransport SSE state-machine parser for Anthropic
Messages 2023-06-01 when the official `anthropic` Python package is absent on
the offline host.
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.metadata
import json
import platform
from pathlib import Path
from typing import Annotated, Any, Dict, List, Literal, Optional, Union

import httpx
import openai
import pydantic
from openai import OpenAI
from pydantic import BaseModel, ConfigDict, Field, TypeAdapter


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class AnthropicUsage(StrictModel):
    input_tokens: int
    output_tokens: int
    cache_read_input_tokens: Optional[int] = None
    cache_creation_input_tokens: Optional[int] = None
    server_tool_use: Optional[Dict[str, int]] = None


class ThinkingBlock(StrictModel):
    type: Literal["thinking"]
    thinking: str
    signature: Optional[str] = None


class RedactedThinkingBlock(StrictModel):
    type: Literal["redacted_thinking"]
    data: str


class TextBlock(StrictModel):
    type: Literal["text"]
    text: str


class ToolUseBlock(StrictModel):
    type: Literal["tool_use"]
    id: str
    name: str
    input: Dict[str, Any]


AnthropicContentBlock = Annotated[
    Union[ThinkingBlock, RedactedThinkingBlock, TextBlock, ToolUseBlock],
    Field(discriminator="type"),
]


class AnthropicMessage(StrictModel):
    id: str
    type: Literal["message"]
    role: Literal["assistant"]
    model: str
    content: List[AnthropicContentBlock]
    stop_reason: Optional[str]
    stop_sequence: Optional[str]
    usage: Optional[AnthropicUsage] = None


class ThinkingDelta(StrictModel):
    type: Literal["thinking_delta"]
    thinking: str


class SignatureDelta(StrictModel):
    type: Literal["signature_delta"]
    signature: str


class TextDelta(StrictModel):
    type: Literal["text_delta"]
    text: str


class InputJSONDelta(StrictModel):
    type: Literal["input_json_delta"]
    partial_json: str


AnthropicBlockDelta = Annotated[
    Union[ThinkingDelta, SignatureDelta, TextDelta, InputJSONDelta],
    Field(discriminator="type"),
]


class MessageStartEvent(StrictModel):
    type: Literal["message_start"]
    message: AnthropicMessage


class ContentBlockStartEvent(StrictModel):
    type: Literal["content_block_start"]
    index: int
    content_block: AnthropicContentBlock


class ContentBlockDeltaEvent(StrictModel):
    type: Literal["content_block_delta"]
    index: int
    delta: AnthropicBlockDelta


class ContentBlockStopEvent(StrictModel):
    type: Literal["content_block_stop"]
    index: int


class MessageDeltaPayload(StrictModel):
    stop_reason: Optional[str] = None
    stop_sequence: Optional[str] = None


class MessageDeltaEvent(StrictModel):
    type: Literal["message_delta"]
    delta: MessageDeltaPayload
    usage: Optional[AnthropicUsage] = None


class MessageStopEvent(StrictModel):
    type: Literal["message_stop"]


class PingEvent(StrictModel):
    type: Literal["ping"]


AnthropicStreamEvent = Annotated[
    Union[
        MessageStartEvent,
        ContentBlockStartEvent,
        ContentBlockDeltaEvent,
        ContentBlockStopEvent,
        MessageDeltaEvent,
        MessageStopEvent,
        PingEvent,
    ],
    Field(discriminator="type"),
]

STREAM_EVENT_ADAPTER = TypeAdapter(AnthropicStreamEvent)


def make_openai_mock_client(raw: bytes, content_type: str, counter: List[str]) -> OpenAI:
    def serve(request: httpx.Request) -> httpx.Response:
        counter.append(f"{request.method} {request.url.path}")
        return httpx.Response(200, headers={"content-type": content_type}, content=raw)

    return OpenAI(
        api_key="controlled-local-sdk-fixture",
        base_url="http://in-process-sdk-fixture.invalid/v1",
        max_retries=0,
        http_client=httpx.Client(transport=httpx.MockTransport(serve)),
    )


def verify_chat_json(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "chat-json.json").read_bytes()
    reqs: List[str] = []
    client = make_openai_mock_client(raw, "application/json", reqs)
    try:
        comp = client.chat.completions.create(
            model="public-model",
            messages=[{"role": "user", "content": "Execute parallel tools"}],
        )
    finally:
        client.close()

    assert reqs == ["POST /v1/chat/completions"]
    assert comp.object == "chat.completion"
    assert comp.model == "public-model"
    assert comp.id.startswith("chatcmpl_pf_")
    choice = comp.choices[0]
    assert choice.finish_reason == "tool_calls"
    assert choice.message.content is None
    assert getattr(choice.message, "reasoning_content", None) == "Analyzing parallel parameters."
    assert choice.message.tool_calls is not None and len(choice.message.tool_calls) == 2
    tc0, tc1 = choice.message.tool_calls
    assert tc0.id.startswith("call_pf_") and tc0.function.name == "first_tool"
    assert json.loads(tc0.function.arguments) == {"value": 9007199254740993}
    assert tc1.id.startswith("call_pf_") and tc1.function.name == "second_tool"
    assert json.loads(tc1.function.arguments) == {"city": "Shanghai"}
    assert comp.usage is not None
    assert comp.usage.prompt_tokens == 9007199254740997
    assert comp.usage.completion_tokens == 8
    assert comp.usage.total_tokens == 9007199254741005

    return {
        "case": "openai_chat_json",
        "fixture": "chat-json.json",
        "protocol": "openai_chat",
        "mode": "json",
        "consumer": f"openai=={openai.__version__} OpenAI.chat.completions.create(stream=False)",
        "result": "PASS",
        "finish_reason": choice.finish_reason,
        "tool_calls": len(choice.message.tool_calls),
        "large_int_preserved": json.loads(tc0.function.arguments)["value"] == 9007199254740993,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_chat_stream(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "chat-stream.sse").read_bytes()
    reqs: List[str] = []
    client = make_openai_mock_client(raw, "text/event-stream", reqs)
    chunks = []
    reasoning_parts: List[str] = []
    tool_args: Dict[int, str] = {}
    tool_names: Dict[int, str] = {}
    finish_reason: Optional[str] = None
    terminal_usage = None
    try:
        stream = client.chat.completions.create(
            model="public-model",
            messages=[{"role": "user", "content": "Stream parallel tools"}],
            stream=True,
        )
        for chunk in stream:
            chunks.append(chunk)
            if len(chunk.choices) == 0 and chunk.usage is not None:
                terminal_usage = chunk.usage
                continue
            assert chunk.usage is None
            ch = chunk.choices[0]
            rc = getattr(ch.delta, "reasoning_content", None)
            if rc:
                reasoning_parts.append(rc)
            if ch.delta.tool_calls:
                for tc in ch.delta.tool_calls:
                    if tc.function and tc.function.name:
                        tool_names[tc.index] = tc.function.name
                    if tc.function and tc.function.arguments:
                        tool_args[tc.index] = tool_args.get(tc.index, "") + tc.function.arguments
            if ch.finish_reason:
                finish_reason = ch.finish_reason
    finally:
        client.close()

    assert reqs == ["POST /v1/chat/completions"]
    assert "".join(reasoning_parts) == "Step 1: dispatching."
    assert finish_reason == "tool_calls"
    assert tool_names == {0: "first_tool", 1: "second_tool"}
    assert json.loads(tool_args[0]) == {"value": 9007199254740993}
    assert json.loads(tool_args[1]) == {"city": "Shanghai"}
    assert terminal_usage is not None and terminal_usage.total_tokens == 112

    return {
        "case": "openai_chat_sse",
        "fixture": "chat-stream.sse",
        "protocol": "openai_chat",
        "mode": "sse",
        "consumer": f"openai=={openai.__version__} OpenAI.chat.completions.create(stream=True)",
        "result": "PASS",
        "sdk_chunks": len(chunks),
        "finish_reason": finish_reason,
        "tool_calls": len(tool_names),
        "large_int_preserved": json.loads(tool_args[0])["value"] == 9007199254740993,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_responses_json(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "responses-json.json").read_bytes()
    reqs: List[str] = []
    client = make_openai_mock_client(raw, "application/json", reqs)
    try:
        resp = client.responses.create(model="public-model", input="Run audit")
    finally:
        client.close()

    assert reqs == ["POST /v1/responses"]
    assert resp.object == "response"
    assert resp.status == "completed"
    assert resp.model == "public-model"
    assert resp.id.startswith("resp_pf_")
    assert resp.instructions == "Keep precision exact."
    assert [it.type for it in resp.output] == ["reasoning", "message", "function_call", "function_call"]
    assert resp.output[0].content[0].text == "Deliberating before tool calls."
    assert resp.output[1].id.startswith("item_pf_")
    assert resp.output[1].content[0].text == "Checking both systems."
    assert resp.output[2].id != resp.output[2].call_id
    assert json.loads(resp.output[2].arguments) == {"value": 9007199254740993}
    assert json.loads(resp.output[3].arguments) == {"city": "Shanghai"}
    assert resp.usage is not None and resp.usage.input_tokens == 9007199254740997

    return {
        "case": "openai_responses_json",
        "fixture": "responses-json.json",
        "protocol": "openai_responses",
        "mode": "json",
        "consumer": f"openai=={openai.__version__} OpenAI.responses.create(stream=False)",
        "result": "PASS",
        "status": resp.status,
        "output_items": len(resp.output),
        "large_int_preserved": json.loads(resp.output[2].arguments)["value"] == 9007199254740993,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_responses_stream_completed(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "responses-stream-completed.sse").read_bytes()
    wire = [json.loads(line[6:]) for line in raw.decode().splitlines() if line.startswith("data: ")]
    assert [ev["sequence_number"] for ev in wire] == list(range(len(wire)))

    reqs: List[str] = []
    client = make_openai_mock_client(raw, "text/event-stream", reqs)
    events = []
    try:
        with client.responses.stream(model="public-model", input="Run streaming audit") as stream:
            for ev in stream:
                events.append(ev)
            final_resp = stream.get_final_response()
    finally:
        client.close()

    assert reqs == ["POST /v1/responses"]
    event_types = [ev.type for ev in events]
    assert event_types[0] == "response.created"
    assert event_types[1] == "response.in_progress"
    assert "response.reasoning_text.delta" in event_types
    assert "response.reasoning_text.done" in event_types
    assert event_types[-1] == "response.completed"
    assert final_resp.status == "completed"
    assert [it.type for it in final_resp.output] == ["reasoning", "function_call", "message", "function_call"]
    assert json.loads(final_resp.output[1].arguments) == {"value": 9007199254740993}
    assert final_resp.output[2].content[0].text == "checking"
    assert json.loads(final_resp.output[3].arguments) == {"city": "Shanghai"}
    assert final_resp.usage is not None and final_resp.usage.input_tokens == 9007199254740997

    return {
        "case": "openai_responses_sse_completed",
        "fixture": "responses-stream-completed.sse",
        "protocol": "openai_responses",
        "mode": "sse",
        "consumer": f"openai=={openai.__version__} OpenAI.responses.stream() + get_final_response()",
        "result": "PASS",
        "sdk_events": len(events),
        "wire_events": len(wire),
        "status": final_resp.status,
        "output_items": len(final_resp.output),
        "large_int_preserved": json.loads(final_resp.output[1].arguments)["value"] == 9007199254740993,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_responses_stream_incomplete(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "responses-stream-incomplete.sse").read_bytes()
    wire = [json.loads(line[6:]) for line in raw.decode().splitlines() if line.startswith("data: ")]
    assert [ev["sequence_number"] for ev in wire] == list(range(len(wire)))

    reqs: List[str] = []
    client = make_openai_mock_client(raw, "text/event-stream", reqs)
    events = []
    try:
        stream = client.responses.create(model="public-model", input="Run streaming audit", stream=True)
        for ev in stream:
            events.append(ev)
    finally:
        client.close()

    assert reqs == ["POST /v1/responses"]
    terminal = events[-1]
    assert terminal.type == "response.incomplete"
    assert terminal.response.status == "incomplete"
    assert terminal.response.incomplete_details is not None
    assert terminal.response.incomplete_details.reason == "max_output_tokens"
    assert len(terminal.response.output) == 1
    assert terminal.response.output[0].id.startswith("item_pf_")
    assert terminal.response.output[0].status == "incomplete"

    return {
        "case": "openai_responses_sse_incomplete",
        "fixture": "responses-stream-incomplete.sse",
        "protocol": "openai_responses",
        "mode": "sse",
        "consumer": f"openai=={openai.__version__} OpenAI.responses.create(stream=True)",
        "result": "PASS",
        "sdk_events": len(events),
        "wire_events": len(wire),
        "status": terminal.response.status,
        "incomplete_reason": terminal.response.incomplete_details.reason,
        "output_items": len(terminal.response.output),
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_responses_stream_failed(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "responses-stream-failed.sse").read_bytes()
    wire = [json.loads(line[6:]) for line in raw.decode().splitlines() if line.startswith("data: ")]
    assert [ev["sequence_number"] for ev in wire] == list(range(len(wire)))

    reqs: List[str] = []
    client = make_openai_mock_client(raw, "text/event-stream", reqs)
    events = []
    try:
        stream = client.responses.create(model="public-model", input="Run streaming audit", stream=True)
        for ev in stream:
            events.append(ev)
    finally:
        client.close()

    assert reqs == ["POST /v1/responses"]
    event_types = [ev.type for ev in events]
    assert "response.completed" not in event_types
    assert "response.function_call_arguments.done" not in event_types
    terminal = events[-1]
    assert terminal.type == "response.failed"
    assert terminal.response.status == "failed"
    assert terminal.response.error is not None
    assert len(terminal.response.output) == 3
    assert [it.status for it in terminal.response.output] == ["incomplete", "incomplete", "incomplete"]
    assert terminal.response.output[2].arguments == '{"value":9007199'

    return {
        "case": "openai_responses_sse_failed",
        "fixture": "responses-stream-failed.sse",
        "protocol": "openai_responses",
        "mode": "sse",
        "consumer": f"openai=={openai.__version__} OpenAI.responses.create(stream=True)",
        "result": "PASS",
        "sdk_events": len(events),
        "wire_events": len(wire),
        "status": terminal.response.status,
        "output_items": len(terminal.response.output),
        "all_open_items_marked_incomplete": True,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_messages_json(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "messages-json.json").read_bytes()
    reqs: List[str] = []

    def serve(request: httpx.Request) -> httpx.Response:
        reqs.append(f"{request.method} {request.url.path}")
        return httpx.Response(200, headers={"content-type": "application/json"}, content=raw)

    with httpx.Client(transport=httpx.MockTransport(serve), base_url="http://in-process-sdk-fixture.invalid") as http:
        res = http.post("/v1/messages", json={"model": "public-model"})
        msg = AnthropicMessage.model_validate_json(res.content)

    assert reqs == ["POST /v1/messages"]
    assert msg.id.startswith("msg_pf_")
    assert msg.model == "public-model"
    assert msg.stop_reason == "tool_use"
    assert msg.stop_sequence is None
    assert [b.type for b in msg.content] == ["thinking", "text", "tool_use", "tool_use"]
    assert isinstance(msg.content[0], ThinkingBlock)
    assert msg.content[0].thinking == "Verifying ledger invariants."
    assert msg.content[0].signature == "sig_anth_compat_01"
    assert isinstance(msg.content[2], ToolUseBlock)
    assert msg.content[2].name == "first_tool" and msg.content[2].input == {"value": 9007199254740993}
    assert isinstance(msg.content[3], ToolUseBlock)
    assert msg.content[3].name == "second_tool" and msg.content[3].input == {"city": "Shanghai"}
    assert msg.usage is not None and msg.usage.input_tokens == 9007199254740995 and msg.usage.cache_read_input_tokens == 2

    return {
        "case": "anthropic_messages_json",
        "fixture": "messages-json.json",
        "protocol": "anthropic_messages",
        "mode": "json",
        "consumer": f"pydantic=={pydantic.__version__} strict extra=forbid AnthropicMessage + httpx=={httpx.__version__} MockTransport",
        "result": "PASS",
        "stop_reason": msg.stop_reason,
        "content_blocks": len(msg.content),
        "signature_preserved": msg.content[0].signature == "sig_anth_compat_01",
        "large_int_preserved": msg.content[2].input["value"] == 9007199254740993,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def verify_messages_stream(evidence_dir: Path) -> Dict[str, Any]:
    raw = (evidence_dir / "messages-stream.sse").read_bytes()
    reqs: List[str] = []

    def serve(request: httpx.Request) -> httpx.Response:
        reqs.append(f"{request.method} {request.url.path}")
        return httpx.Response(200, headers={"content-type": "text/event-stream"}, content=raw)

    parsed_events: List[AnthropicStreamEvent] = []
    raw_start_usage_keys: List[str] = []
    with httpx.Client(transport=httpx.MockTransport(serve), base_url="http://in-process-sdk-fixture.invalid") as http:
        with http.stream("POST", "/v1/messages", json={"model": "public-model", "stream": True}) as res:
            body_text = res.read().decode("utf-8")
            assert "[DONE]" not in body_text
            for block in body_text.split("\n\n"):
                if not block.strip():
                    continue
                ev_name = None
                data_lines: List[str] = []
                for line in block.splitlines():
                    if line.startswith("event: "):
                        ev_name = line[7:]
                    elif line.startswith("data: "):
                        data_lines.append(line[6:])
                payload_str = "\n".join(data_lines)
                ev_obj = STREAM_EVENT_ADAPTER.validate_json(payload_str)
                assert ev_obj.type == ev_name
                if ev_name == "message_start":
                    raw_start_usage_keys = sorted(json.loads(payload_str)["message"]["usage"].keys())
                parsed_events.append(ev_obj)

    assert reqs == ["POST /v1/messages"]
    assert isinstance(parsed_events[0], MessageStartEvent)
    assert isinstance(parsed_events[-1], MessageStopEvent)
    # M04 Qwen profile crops message_start.usage to input_tokens & output_tokens only.
    assert raw_start_usage_keys == ["input_tokens", "output_tokens"]

    open_index: Optional[int] = None
    blocks: Dict[int, Dict[str, Any]] = {}
    stop_reason: Optional[str] = None
    final_usage: Optional[AnthropicUsage] = None

    for ev in parsed_events:
        if isinstance(ev, ContentBlockStartEvent):
            assert open_index is None, "overlapping content_block_start before content_block_stop"
            assert ev.index == len(blocks)
            open_index = ev.index
            blocks[ev.index] = {"block": ev.content_block, "partial_json": ""}
        elif isinstance(ev, ContentBlockDeltaEvent):
            assert open_index == ev.index
            entry = blocks[ev.index]
            blk = entry["block"]
            if isinstance(ev.delta, ThinkingDelta):
                assert isinstance(blk, ThinkingBlock)
                blk.thinking += ev.delta.thinking
            elif isinstance(ev.delta, SignatureDelta):
                assert isinstance(blk, ThinkingBlock)
                blk.signature = (blk.signature or "") + ev.delta.signature
            elif isinstance(ev.delta, TextDelta):
                assert isinstance(blk, TextBlock)
                blk.text += ev.delta.text
            elif isinstance(ev.delta, InputJSONDelta):
                assert isinstance(blk, ToolUseBlock)
                entry["partial_json"] += ev.delta.partial_json
        elif isinstance(ev, ContentBlockStopEvent):
            assert open_index == ev.index
            entry = blocks[ev.index]
            blk = entry["block"]
            if isinstance(blk, ToolUseBlock) and entry["partial_json"]:
                blk.input = json.loads(entry["partial_json"])
            open_index = None
        elif isinstance(ev, MessageDeltaEvent):
            assert open_index is None
            stop_reason = ev.delta.stop_reason
            final_usage = ev.usage

    assert open_index is None
    assert len(blocks) == 4
    b0 = blocks[0]["block"]
    b1 = blocks[1]["block"]
    b2 = blocks[2]["block"]
    b3 = blocks[3]["block"]
    assert isinstance(b0, ThinkingBlock) and b0.thinking == "Step-by-step reasoning." and b0.signature == "sig_stream_an04"
    assert isinstance(b1, TextBlock) and b1.text == "Running tools now."
    assert isinstance(b2, ToolUseBlock) and b2.name == "first_tool" and b2.input == {"value": 9007199254740993}
    assert isinstance(b3, ToolUseBlock) and b3.name == "second_tool" and b3.input == {"city": "Shanghai"}
    assert stop_reason == "tool_use"
    assert final_usage is not None and final_usage.cache_read_input_tokens == 2 and final_usage.output_tokens == 8

    return {
        "case": "anthropic_messages_sse",
        "fixture": "messages-stream.sse",
        "protocol": "anthropic_messages",
        "mode": "sse",
        "consumer": f"pydantic=={pydantic.__version__} strict extra=forbid AnthropicStreamEvent + httpx=={httpx.__version__} stream MockTransport",
        "result": "PASS",
        "stream_events": len(parsed_events),
        "content_blocks": len(blocks),
        "signature_delta_verified": b0.signature == "sig_stream_an04",
        "m04_start_usage_cropped": raw_start_usage_keys == ["input_tokens", "output_tokens"],
        "large_int_preserved": b2.input["value"] == 9007199254740993,
        "stop_reason": stop_reason,
        "fixture_sha256": hashlib.sha256(raw).hexdigest(),
        "in_process_mock_requests": len(reqs),
        "network_calls": 0,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Offline T25 SDK/consumer verification for Canonical IR C")
    parser.add_argument("evidence_dir", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    evidence_dir = args.evidence_dir.resolve()
    try:
        anthropic_ver = importlib.metadata.version("anthropic")
    except importlib.metadata.PackageNotFoundError:
        anthropic_ver = None

    cases = [
        verify_chat_json(evidence_dir),
        verify_chat_stream(evidence_dir),
        verify_responses_json(evidence_dir),
        verify_responses_stream_completed(evidence_dir),
        verify_responses_stream_incomplete(evidence_dir),
        verify_responses_stream_failed(evidence_dir),
        verify_messages_json(evidence_dir),
        verify_messages_stream(evidence_dir),
    ]

    receipt = {
        "check_id": "T25",
        "date": "2026-11-19",
        "status": "PASS",
        "source_generator": {
            "package": "github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy",
            "test_entrypoint": "TestCanonicalSDKExport",
            "file": "plugins/wasm-go/extensions/ai-proxy/managed_canonical_sdk_export_test.go",
            "go_container": "golang:1.26.3-bookworm@sha256:3bf5b04541eb4a37fe62aa1bc9c98a1dec09db9d2e79c1d2eb54e3c9d08dbca9",
            "pipeline": "Actual proxywasm managedHost sole-sender Canonical IR C pipeline (DecodeCanonicalRequest -> EncodeCanonicalRequest -> DecodeCanonicalResponse/CanonicalEventDecoder -> ProjectCanonical/CanonicalStreamProjection -> EncodeCanonicalClientResponse/CanonicalStreamWriter)",
        },
        "environment": {
            "python_version": platform.python_version(),
            "python_executable": "/home/cloudyi/CodeWorkspace/UrltraKB/.venv/bin/python",
            "installed_sdks": {
                "openai": openai.__version__,
                "httpx": httpx.__version__,
                "pydantic": pydantic.__version__,
                "anthropic": anthropic_ver if anthropic_ver else "NOT_INSTALLED_ON_OFFLINE_HOST",
            },
            "anthropic_consumer_note": (
                "Official anthropic Python package is not installed in the offline host environment; "
                "messages-json.json and messages-stream.sse are consumed through httpx.MockTransport and "
                "strict Pydantic v2 (extra='forbid') Anthropic Messages 2023-06-01 wire/SSE schemas with "
                "sequential content_block open/close state-machine reassembly."
            ),
            "transport": "httpx.MockTransport (in-process, max_retries=0, 0 network or external supplier calls)",
        },
        "summary": {
            "total_cases": len(cases),
            "passed_cases": sum(1 for c in cases if c["result"] == "PASS"),
            "failed_cases": sum(1 for c in cases if c["result"] != "PASS"),
            "protocols_covered": ["openai_chat", "openai_responses", "anthropic_messages"],
            "modes_covered": ["json", "sse"],
        },
        "cases": cases,
    }

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": receipt["status"], "summary": receipt["summary"]}, indent=2))


if __name__ == "__main__":
    main()
