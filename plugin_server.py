"""Authenticated complete-package commits and exact immutable HTTP readback."""
from __future__ import annotations

import argparse
import hmac
import mimetypes
import os
import sqlite3
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlsplit

from artifact_contract import json_bytes, load_policy, safe_path
from artifact_storage import ArtifactStore, SEGMENT, StorageConflict, StorageUnavailable
from mf_canonical import ContractError


def parse_plugin_upload_path(path):
    parts = unquote(urlsplit(path).path, errors="strict").strip("/").split("/")
    if len(parts) != 4 or parts[0] != "plugins" or parts[3] != "package.tgz":
        raise ValueError("expected /plugins/{name}/{version}/package.tgz")
    if not SEGMENT.fullmatch(parts[1]) or not SEGMENT.fullmatch(parts[2]):
        raise ValueError("plugin name and version must be path-safe")
    return parts[1], parts[2]


class PluginRequestHandler(BaseHTTPRequestHandler):
    upload_token = None
    created_by = "artifact-service"

    @classmethod
    def create(cls, plugins_dir, upload_token, *, public_base_url="http://localhost:8080",
               state_dir=None, policy=None, created_by="artifact-service"):
        class ConfiguredPluginRequestHandler(cls):
            pass
        handler = ConfiguredPluginRequestHandler
        handler.plugins_dir = Path(plugins_dir).resolve()
        handler.upload_token, handler.created_by = upload_token, created_by
        handler.store = ArtifactStore(handler.plugins_dir, state_dir=state_dir,
                                     public_base_url=public_base_url, policy=policy)
        return handler

    def _reply(self, status, raw, content_type="application/json"):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("Connection", "close")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.close_connection = True
        if self.command != "HEAD":
            self.wfile.write(raw)

    def _error(self, status, code):
        self._reply(status, json_bytes({"error": code}))

    def _authorized(self):
        if not self.upload_token:
            self._error(HTTPStatus.FORBIDDEN, "upload_api_disabled")
            return False
        supplied = self.headers.get("Authorization", "")
        if not hmac.compare_digest(supplied.encode(), ("Bearer " + self.upload_token).encode()):
            self._error(HTTPStatus.UNAUTHORIZED, "unauthorized")
            return False
        return True

    def do_PUT(self):
        if not self._authorized():
            return
        try:
            path = unquote(urlsplit(self.path).path, errors="strict")
            expected = None
            if path.startswith("/uploads/") and path.count("/") == 2:
                upload_id = path.removeprefix("/uploads/")
            else:
                expected = parse_plugin_upload_path(self.path)
                upload_id = self.headers.get("Idempotency-Key", "")
            if not upload_id:
                self._error(HTTPStatus.BAD_REQUEST, "upload_id_required")
                return
            lengths = self.headers.get_all("Content-Length", [])
            if not lengths:
                self._error(HTTPStatus.LENGTH_REQUIRED, "content_length_required")
                return
            if len(lengths) != 1 or not lengths[0].isdigit() or self.headers.get("Transfer-Encoding"):
                self._error(HTTPStatus.BAD_REQUEST, "invalid_content_length")
                return
            length = int(lengths[0])
            if length > self.store.policy.max_archive_bytes:
                self._error(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "ART_LIMIT")
                return
            self.connection.settimeout(30)
            content = self.rfile.read(length)
            if len(content) != length:
                self._error(HTTPStatus.BAD_REQUEST, "short_upload")
                return
            result = self.store.commit(upload_id, content, created_by=self.created_by, expected_identity=expected)
            self._reply(HTTPStatus.CREATED, json_bytes(result))
        except ContractError as exc:
            status = HTTPStatus.REQUEST_ENTITY_TOO_LARGE if exc.code in ("ART_LIMIT", "E_LIMIT") else HTTPStatus.UNPROCESSABLE_ENTITY
            self._error(status, exc.code)
        except StorageConflict as exc:
            self._error(HTTPStatus.CONFLICT, exc.code)
        except (StorageUnavailable, sqlite3.Error, OSError):
            self._error(HTTPStatus.SERVICE_UNAVAILABLE, "storage_unavailable")
        except (ValueError, UnicodeError):
            self._error(HTTPStatus.BAD_REQUEST, "invalid_upload_path")

    def do_GET(self):
        try:
            path = unquote(urlsplit(self.path).path, errors="strict")
            if path.startswith("/uploads/") and path.count("/") == 2:
                if not self._authorized():
                    return
                result = self.store.query(path.removeprefix("/uploads/"))
                if result is None:
                    self._error(HTTPStatus.NOT_FOUND, "upload_not_found")
                else:
                    self._reply(HTTPStatus.OK, json_bytes(result))
                return
            parts = path.split("/")
            if len(parts) < 5 or parts[0] or parts[1] != "plugins" or not SEGMENT.fullmatch(parts[2]) or not SEGMENT.fullmatch(parts[3]):
                self._error(HTTPStatus.NOT_FOUND, "artifact_not_found")
                return
            name, version, member = parts[2], parts[3], "/".join(parts[4:])
            if not safe_path(member):
                self._error(HTTPStatus.NOT_FOUND, "artifact_not_found")
                return
            manifest = self.store.manifest(name, version)
            if member == "manifest.json" and manifest is not None:
                self._reply(HTTPStatus.OK, json_bytes(manifest))
                return
            raw = self.store.read_file(name, version, member) if manifest else None
            if manifest is None and member in ("plugin.wasm", "metadata.txt"):
                # Old versions stay readable; partially published managed versions
                # must never fall through to this compatibility path.
                directory = self.plugins_dir / name / version
                target = directory / member
                if not (directory / "manifest.json").exists() and not (directory / "package.json").exists() and target.is_file() and not target.is_symlink() and target.resolve().is_relative_to(self.plugins_dir):
                    raw = target.read_bytes()
            if raw is None:
                self._error(HTTPStatus.NOT_FOUND, "artifact_not_found")
                return
            content_type = "application/wasm" if member.endswith(".wasm") else mimetypes.guess_type(member)[0] or "application/octet-stream"
            self._reply(HTTPStatus.OK, raw, content_type)
        except ContractError as exc:
            self._error(HTTPStatus.BAD_REQUEST, exc.code)
        except (StorageUnavailable, OSError, sqlite3.Error):
            self._error(HTTPStatus.SERVICE_UNAVAILABLE, "storage_unavailable")
        except (ValueError, UnicodeError):
            self._error(HTTPStatus.BAD_REQUEST, "invalid_path")

    def do_HEAD(self):
        self.do_GET()


def run_server(host, port, plugins_dir, upload_token, **kwargs):
    handler = PluginRequestHandler.create(plugins_dir, upload_token, **kwargs)
    server = ThreadingHTTPServer((host, port), handler)
    print(f"Serving immutable plugins from {Path(plugins_dir).resolve()} on {host}:{port}")
    server.serve_forever()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default=os.getenv("PLUGIN_SERVER_HOST", "0.0.0.0"))
    parser.add_argument("--port", type=int, default=int(os.getenv("PLUGIN_SERVER_PORT", "8080")))
    parser.add_argument("--plugins-dir", default=os.getenv("PLUGIN_SERVER_PLUGINS_DIR", "/usr/share/plugin-server/plugins"))
    parser.add_argument("--state-dir", type=Path, default=os.getenv("PLUGIN_SERVER_STATE_DIR"))
    parser.add_argument("--public-base-url", default=os.getenv("PLUGIN_SERVER_PUBLIC_BASE_URL", "http://higress-plugin-server.higress-system.svc:8080"))
    parser.add_argument("--upload-token", default=os.getenv("PLUGIN_SERVER_UPLOAD_TOKEN"))
    parser.add_argument("--policy", type=Path, default=os.getenv("PLUGIN_SERVER_POLICY_FILE"))
    args = parser.parse_args()
    policy = load_policy(args.policy)
    run_server(args.host, args.port, args.plugins_dir, args.upload_token, state_dir=args.state_dir,
               public_base_url=args.public_base_url, policy=policy)


if __name__ == "__main__":
    main()
