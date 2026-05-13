import argparse
from pathlib import Path
from urllib.parse import quote
from urllib.request import Request, urlopen


def build_upload_url(server, name, version):
    base = server.rstrip("/")
    safe_name = quote(name, safe="")
    safe_version = quote(version, safe="")
    return f"{base}/plugins/{safe_name}/{safe_version}/plugin.wasm"


def upload_plugin(server, token, plugin_file, name, version):
    plugin_path = Path(plugin_file)
    if not plugin_path.is_file():
        raise FileNotFoundError(str(plugin_path))

    url = build_upload_url(server, name, version)
    request = Request(
        url,
        data=plugin_path.read_bytes(),
        method="PUT",
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/wasm",
        },
    )

    with urlopen(request) as response:
        response_body = response.read().decode("utf-8", errors="replace")
        return response.status, response_body


def main():
    parser = argparse.ArgumentParser(description="Upload a built Higress Wasm plugin")
    parser.add_argument("--server", required=True, help="Plugin server base URL")
    parser.add_argument("--token", required=True, help="Upload bearer token")
    parser.add_argument("--file", required=True, help="Path to built plugin.wasm")
    parser.add_argument("--name", required=True, help="Plugin name")
    parser.add_argument("--version", required=True, help="Plugin version")
    args = parser.parse_args()

    status, body = upload_plugin(
        server=args.server,
        token=args.token,
        plugin_file=Path(args.file),
        name=args.name,
        version=args.version,
    )
    print(f"HTTP {status}: {body}", end="")


if __name__ == "__main__":
    main()
