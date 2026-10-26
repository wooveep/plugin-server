"""mf-json.v1 production adapter, promoted without semantic changes from MF01.

Source: 753704b29a6ea40815d45dcbbce16cc0d91bace2, validate_bundle.py.
The frozen D0 source and golden corpus remain unchanged.
"""
from __future__ import annotations

import hashlib
import json
from decimal import Decimal
from typing import Any

MAX_INPUT_BYTES = 1_048_576
MAX_OUTPUT_BYTES = 4_194_304
MAX_NUMBER_CHARS = 10_000
MAX_DEPTH = 128
MAX_ITEMS = 10_000

class ContractError(Exception):
    def __init__(self, code: str, detail: str, stage: str = 'parse', pointer: str = ''):
        super().__init__(detail)
        self.code, self.stage, self.pointer = code, stage, pointer


def parse_json(raw: bytes) -> Any:
    if len(raw) > MAX_INPUT_BYTES:
        raise ContractError('E_LIMIT', 'input byte limit exceeded')
    try:
        text = raw.decode('utf-8')
    except UnicodeDecodeError as error:
        raise ContractError('E_UTF8', 'invalid UTF-8') from error

    def pairs(items: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in items:
            if key in result:
                raise ContractError('E_DUPLICATE_KEY', 'duplicate object key')
            result[key] = value
        return result

    def invalid_constant(value: str) -> Any:
        raise ContractError('E_JSON', 'non-JSON number')

    try:
        result = json.loads(text, parse_float=Decimal, parse_int=Decimal,
                            parse_constant=invalid_constant, object_pairs_hook=pairs)
    except (ValueError, RecursionError) as error:
        raise ContractError('E_JSON' if isinstance(error, ValueError) else 'E_LIMIT',
                            'invalid JSON or excessive nesting') from error

    def check(value: Any, depth: int = 0) -> None:
        if depth > MAX_DEPTH:
            raise ContractError('E_LIMIT', 'nesting limit exceeded')
        if isinstance(value, str) and any(0xD800 <= ord(c) <= 0xDFFF for c in value):
            raise ContractError('E_SURROGATE', 'isolated surrogate')
        if isinstance(value, (list, dict)) and len(value) > MAX_ITEMS:
            raise ContractError('E_LIMIT', 'container item limit exceeded')
        if isinstance(value, dict):
            for key, item in value.items():
                check(key, depth + 1)
                check(item, depth + 1)
        elif isinstance(value, list):
            for item in value:
                check(item, depth + 1)
    check(result)
    return result


def canonicalize(raw: bytes) -> bytes:
    """mf-json.v1, with exact decimal parsing and bounded exponent expansion."""
    chunks: list[bytes] = []
    size = 0

    def append(value: str) -> None:
        nonlocal size
        chunk = value.encode('utf-8')
        size += len(chunk)
        if size > MAX_OUTPUT_BYTES:
            raise ContractError('E_LIMIT', 'canonical output limit exceeded')
        chunks.append(chunk)

    def encode(value: Any) -> None:
        if value is None:
            append('null')
        elif isinstance(value, bool):
            append('true' if value else 'false')
        elif isinstance(value, Decimal):
            if not value:
                append('0')
            else:
                _, digits, exponent = value.as_tuple()
                if not isinstance(exponent, int) or len(digits) + abs(exponent) > MAX_NUMBER_CHARS:
                    raise ContractError('E_LIMIT', 'expanded number limit exceeded')
                number = format(value, 'f')
                append(number.rstrip('0').rstrip('.') if '.' in number else number)
        elif isinstance(value, str):
            append('"' + ''.join('\\u%04x' % ord(c) if ord(c) < 32 else
                                  '\\' + c if c in ('"', '\\') else c for c in value) + '"')
        elif isinstance(value, list):
            append('[')
            for index, item in enumerate(value):
                if index:
                    append(',')
                encode(item)
            append(']')
        else:
            append('{')
            for index, key in enumerate(sorted(value)):
                if index:
                    append(',')
                encode(key)
                append(':')
                encode(value[key])
            append('}')
    encode(parse_json(raw))
    return b''.join(chunks)


def sha256(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()



def schema_numbers(value: Any) -> Any:
    """The schema engine requires native integral keyword values; never use floats."""
    if isinstance(value, Decimal) and value == value.to_integral_value():
        if abs(value.adjusted()) > MAX_NUMBER_CHARS:
            raise ContractError('E_LIMIT', 'schema integer limit exceeded')
        return int(value)
    if isinstance(value, dict):
        return {key: schema_numbers(item) for key, item in value.items()}
    if isinstance(value, list):
        return [schema_numbers(item) for item in value]
    return value


