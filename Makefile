PYTHON ?= python3
IMAGE ?= higress-plugin-server
VERSION ?= 1.0.0
TAG ?= $(IMAGE):$(VERSION)
DOCKERFILE ?= Dockerfile
ALPINE_MIRROR ?=
PLATFORMS ?= linux/amd64,linux/arm64
CLI_DIST ?= dist/upload-plugin

SERVER ?= http://localhost:8080
TOKEN ?=
PLUGIN_FILE ?= ./plugin.wasm
PLUGIN_NAME ?= demo
PLUGIN_VERSION ?= 1.0.0

.PHONY: help test build-cli clean docker-build docker-buildx-push run upload

help:
	@printf '%s\n' 'Targets:'
	@printf '  %-18s %s\n' 'test' 'Run Python unit tests'
	@printf '  %-18s %s\n' 'build-cli' 'Build local upload CLI at $(CLI_DIST)'
	@printf '  %-18s %s\n' 'docker-build' 'Build Docker image $(TAG)'
	@printf '  %-18s %s\n' 'docker-buildx-push' 'Build and push multi-arch Docker image'
	@printf '  %-18s %s\n' 'run' 'Run Docker image locally with PLUGIN_SERVER_UPLOAD_TOKEN'
	@printf '  %-18s %s\n' 'upload' 'Upload a wasm plugin with the local CLI'
	@printf '  %-18s %s\n' 'clean' 'Remove local build outputs'

test:
	$(PYTHON) -m unittest discover -s tests -v
	$(PYTHON) -m py_compile plugin_server.py upload_plugin.py pull_plugins.py generate_metadata.py

build-cli:
	@mkdir -p $(dir $(CLI_DIST))
	@printf '%s\n' '#!/usr/bin/env python3' > $(CLI_DIST)
	@sed '1{/^#!/d;}' upload_plugin.py >> $(CLI_DIST)
	@chmod +x $(CLI_DIST)
	@printf 'Built %s\n' '$(CLI_DIST)'

clean:
	rm -rf dist __pycache__ tests/__pycache__

docker-build:
	docker build \
		$(if $(ALPINE_MIRROR),--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR),) \
		-t $(TAG) \
		-f $(DOCKERFILE) \
		.

docker-buildx-push:
	docker buildx build \
		--platform $(PLATFORMS) \
		$(if $(ALPINE_MIRROR),--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR),) \
		-t $(TAG) \
		-f $(DOCKERFILE) \
		--push \
		.

run:
	@test -n "$(TOKEN)" || (printf '%s\n' 'TOKEN is required, for example: make run TOKEN=change-me' && exit 1)
	docker run --rm -p 8080:8080 \
		-e PLUGIN_SERVER_UPLOAD_TOKEN=$(TOKEN) \
		$(TAG)

upload: build-cli
	@test -n "$(TOKEN)" || (printf '%s\n' 'TOKEN is required, for example: make upload TOKEN=change-me' && exit 1)
	$(CLI_DIST) \
		--server $(SERVER) \
		--token $(TOKEN) \
		--file $(PLUGIN_FILE) \
		--name $(PLUGIN_NAME) \
		--version $(PLUGIN_VERSION)
