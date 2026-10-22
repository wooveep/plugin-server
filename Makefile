PYTHON ?= python3
IMAGE ?= plugin-server
VERSION ?= 1.0.0
TAG ?= $(IMAGE):$(VERSION)
REGISTRY ?= registry.lc.wooveep.com:9980/modelfusion
REMOTE_TAG ?= $(REGISTRY)/$(IMAGE):$(VERSION)
DOCKERFILE ?= Dockerfile
ALPINE_MIRROR ?=
PLATFORMS ?= linux/amd64
CLI_DIST ?= dist/upload-plugin
PLUGIN_ARTIFACTS_DIR ?= /home/cloudyi/CodeWorkspace/higress-plugin/wasm-go/build
PLUGIN_DIR ?= plugins
SYNC_PLUGIN_SCRIPT ?= scripts/sync-plugin-artifacts.sh
SYNC_PLUGIN_ARGS ?=

SERVER ?= http://localhost:8080
TOKEN ?=
PLUGIN_FILE ?= ./plugin.wasm
PLUGIN_NAME ?= demo
PLUGIN_VERSION ?= 1.0.0

.PHONY: help test build-cli clean sync-plugin-artifacts docker-build docker-push docker-buildx-push run upload

help:
	@printf '%s\n' 'Targets:'
	@printf '  %-18s %s\n' 'test' 'Run Python unit tests'
	@printf '  %-18s %s\n' 'build-cli' 'Build local upload CLI at $(CLI_DIST)'
	@printf '  %-18s %s\n' 'sync-plugin-artifacts' 'Sync wasm-go build artifacts into $(PLUGIN_DIR)'
	@printf '  %-18s %s\n' 'docker-build' 'Build Docker image $(TAG)'
	@printf '  %-18s %s\n' 'docker-push' 'Tag and push Docker image $(REMOTE_TAG)'
	@printf '  %-18s %s\n' 'docker-buildx-push' 'Build and push Docker image $(REMOTE_TAG) for $(PLATFORMS)'
	@printf '  %-18s %s\n' 'run' 'Run Docker image locally with PLUGIN_SERVER_UPLOAD_TOKEN'
	@printf '  %-18s %s\n' 'upload' 'Upload a wasm plugin with the local CLI'
	@printf '  %-18s %s\n' 'clean' 'Remove local build outputs'

test:
	$(PYTHON) -m unittest discover -s tests -v
	$(PYTHON) -m py_compile plugin_server.py upload_plugin.py pull_plugins.py generate_metadata.py
	bash tests/test_sync_plugin_artifacts.sh

build-cli:
	@mkdir -p $(dir $(CLI_DIST))
	@printf '%s\n' '#!/usr/bin/env python3' > $(CLI_DIST)
	@sed '1{/^#!/d;}' upload_plugin.py >> $(CLI_DIST)
	@chmod +x $(CLI_DIST)
	@printf 'Built %s\n' '$(CLI_DIST)'

clean:
	rm -rf dist __pycache__ tests/__pycache__

sync-plugin-artifacts:
	$(SYNC_PLUGIN_SCRIPT) --source "$(PLUGIN_ARTIFACTS_DIR)" --destination "$(PLUGIN_DIR)" $(SYNC_PLUGIN_ARGS)

docker-build: sync-plugin-artifacts
	docker build \
		$(if $(ALPINE_MIRROR),--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR),) \
		-t $(TAG) \
		-f $(DOCKERFILE) \
		.

docker-push:
	docker tag $(TAG) $(REMOTE_TAG)
	docker push $(REMOTE_TAG)

docker-buildx-push: sync-plugin-artifacts
	docker buildx build \
		--platform $(PLATFORMS) \
		$(if $(ALPINE_MIRROR),--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR),) \
		-t $(REMOTE_TAG) \
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
