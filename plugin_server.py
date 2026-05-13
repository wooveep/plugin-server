import argparse
import hashlib
import os
import re
from datetime import datetime
from http import HTTPStatus
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse


SEGMENT_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")


def calculate_md5_bytes(content):
    md5_hash = hashlib.md5()
    md5_hash.update(content)
    return md5_hash.hexdigest()


def parse_plugin_upload_path(path):
    parsed_path = unquote(urlparse(path).path)
    parts = parsed_path.strip("/").split("/")
    if len(parts) != 4 or parts[0] != "plugins" or parts[3] != "plugin.wasm":
        raise ValueError("expected /plugins/{name}/{version}/plugin.wasm")

    name = parts[1]
    version = parts[2]
    if not SEGMENT_PATTERN.match(name) or not SEGMENT_PATTERN.match(version):
        raise ValueError("plugin name and version must be path-safe")

    return name, version


def write_metadata(plugin_dir, plugin_name, version, wasm_path):
    stat_info = wasm_path.stat()
    md5_value = calculate_md5_bytes(wasm_path.read_bytes())
    metadata_path = plugin_dir / "metadata.txt"
    with metadata_path.open("w") as f:
        f.write(f"Plugin Name: {plugin_name}\n")
        f.write(f"Version: {version}\n")
        f.write(f"Size: {stat_info.st_size} bytes\n")
        f.write(f"Last Modified: {datetime.fromtimestamp(stat_info.st_mtime).isoformat()}\n")
        f.write(f"Created: {datetime.fromtimestamp(stat_info.st_ctime).isoformat()}\n")
        f.write(f"MD5: {md5_value}\n")


class PluginRequestHandler(SimpleHTTPRequestHandler):
    plugins_dir = Path("plugins")
    upload_token = None
    extensions_map = {
        **SimpleHTTPRequestHandler.extensions_map,
        ".wasm": "application/wasm",
    }

    @classmethod
    def create(cls, plugins_dir, upload_token):
        class ConfiguredPluginRequestHandler(cls):
            pass

        ConfiguredPluginRequestHandler.plugins_dir = Path(plugins_dir).resolve()
        ConfiguredPluginRequestHandler.upload_token = upload_token
        ConfiguredPluginRequestHandler.directory = str(
            ConfiguredPluginRequestHandler.plugins_dir.parent
        )
        return ConfiguredPluginRequestHandler

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(self.directory), **kwargs)

    def do_PUT(self):
        if not self.upload_token:
            self.send_error(HTTPStatus.FORBIDDEN, "upload API is disabled")
            return

        if self.headers.get("Authorization") != f"Bearer {self.upload_token}":
            self.send_error(HTTPStatus.UNAUTHORIZED, "missing or invalid token")
            return

        try:
            plugin_name, version = parse_plugin_upload_path(self.path)
        except ValueError as exc:
            self.send_error(HTTPStatus.BAD_REQUEST, str(exc))
            return

        content_length = self.headers.get("Content-Length")
        if content_length is None:
            self.send_error(HTTPStatus.LENGTH_REQUIRED, "Content-Length is required")
            return

        try:
            length = int(content_length)
        except ValueError:
            self.send_error(HTTPStatus.BAD_REQUEST, "invalid Content-Length")
            return

        content = self.rfile.read(length)
        plugin_dir = self.plugins_dir / plugin_name / version
        plugin_dir.mkdir(parents=True, exist_ok=True)
        wasm_path = plugin_dir / "plugin.wasm"
        wasm_path.write_bytes(content)
        write_metadata(plugin_dir, plugin_name, version, wasm_path)

        self.send_response(HTTPStatus.CREATED)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.end_headers()
        self.wfile.write(b"uploaded\n")

def run_server(host, port, plugins_dir, upload_token):
    handler = PluginRequestHandler.create(
        plugins_dir=plugins_dir,
        upload_token=upload_token,
    )
    server = ThreadingHTTPServer((host, port), handler)
    print(f"Serving plugins from {Path(plugins_dir).resolve()} on {host}:{port}")
    server.serve_forever()


def main():
    parser = argparse.ArgumentParser(description="Higress plugin upload/download server")
    parser.add_argument("--host", default=os.getenv("PLUGIN_SERVER_HOST", "0.0.0.0"))
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.getenv("PLUGIN_SERVER_PORT", "8080")),
    )
    parser.add_argument(
        "--plugins-dir",
        default=os.getenv("PLUGIN_SERVER_PLUGINS_DIR", "/usr/share/plugin-server/plugins"),
    )
    parser.add_argument(
        "--upload-token",
        default=os.getenv("PLUGIN_SERVER_UPLOAD_TOKEN"),
    )
    args = parser.parse_args()

    run_server(
        host=args.host,
        port=args.port,
        plugins_dir=args.plugins_dir,
        upload_token=args.upload_token,
    )


if __name__ == "__main__":
    main()
