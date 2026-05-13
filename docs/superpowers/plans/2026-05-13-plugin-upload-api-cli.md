# Plugin Upload API CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an authenticated API for pushing built Wasm plugins to the plugin server, plus a local CLI that uploads plugin files.

**Architecture:** Add a small Python HTTP server that serves existing static plugin files and handles `PUT /plugins/{name}/{version}/plugin.wasm`. Store uploads under the same `plugins/<name>/<version>/plugin.wasm` layout and generate `metadata.txt` in the existing format. Add a dependency-free CLI using `urllib.request`.

**Tech Stack:** Python standard library, `unittest`, Docker Alpine Python runtime.

---

### Task 1: Upload Service

**Files:**
- Create: `plugin_server.py`
- Test: `tests/test_plugin_server.py`

- [ ] Write tests for authorized upload, missing token rejection, invalid path rejection, and static download.
- [ ] Run `python3 -m unittest tests.test_plugin_server -v` and verify the tests fail because `plugin_server` does not exist.
- [ ] Implement `PluginRequestHandler`, metadata generation, path validation, and `run_server`.
- [ ] Run `python3 -m unittest tests.test_plugin_server -v` and verify the tests pass.

### Task 2: Local CLI

**Files:**
- Create: `upload_plugin.py`
- Test: `tests/test_upload_plugin.py`

- [ ] Write tests for URL construction, token header, file upload body, and validation of missing files.
- [ ] Run `python3 -m unittest tests.test_upload_plugin -v` and verify the tests fail because `upload_plugin` does not exist.
- [ ] Implement dependency-free CLI arguments: `--server`, `--token`, `--file`, `--name`, `--version`.
- [ ] Run `python3 -m unittest tests.test_upload_plugin -v` and verify the tests pass.

### Task 3: Packaging and Docs

**Files:**
- Modify: `Dockerfile`
- Modify: `README.md`
- Modify: `deploy/higress-plugin-server.yaml`

- [ ] Update Docker runtime to copy `plugin_server.py`, keep prebuilt plugins, expose port 8080, and run the Python server.
- [ ] Document `PLUGIN_SERVER_UPLOAD_TOKEN`, upload API curl usage, and `upload_plugin.py` usage.
- [ ] Add the env var placeholder to the Kubernetes deployment manifest.
- [ ] Run full verification: `python3 -m unittest discover -s tests -v`.
- [ ] Run `mcp__gitnexus__.detect_changes({repo:"plugin-server", scope:"all"})`.
