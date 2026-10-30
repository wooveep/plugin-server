PYTHON ?= $(if $(wildcard .venv/bin/python),.venv/bin/python,python3)
IMAGE ?= plugin-server
VERSION ?= 1.0.0
TAG ?= $(IMAGE):$(VERSION)
REGISTRY ?= registry.lc.wooveep.com:9980/modelfusion
REMOTE_TAG ?= $(REGISTRY)/$(IMAGE):$(VERSION)
DOCKERFILE ?= Dockerfile
ALPINE_MIRROR ?=
PLATFORMS ?= linux/amd64
CLI_DIST ?= dist/upload-plugin
PLUGIN_SERVER_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
PLUGIN_ARTIFACTS_DIR ?= $(abspath $(PLUGIN_SERVER_ROOT)/dist/plugins)
PLUGIN_DIR ?= build/local-store/plugins
PLUGIN ?=
IMPLEMENTATION ?=
HOST_TOOLCHAIN ?= 0
TRUSTED_POLICY ?= build/trusted-policy.json
PUBLIC_BASE_URL ?= http://higress-plugin-server.higress-system.svc:80

SERVER ?= http://localhost:8080
TOKEN ?=
PLUGIN_FILE ?= ./plugin.tgz
UPLOAD_ID ?=
PLUGIN_NAME ?= demo
PLUGIN_VERSION ?= 1.0.0

.PHONY: help bootstrap update-go-locks test build-plugin build-all list-plugins prepare-packages package-plugins build-cli clean sync-plugin-artifacts docker-build docker-push docker-buildx-push run upload

help:
	@printf '%s\n' 'Targets:'
	@printf '  %-18s %s\n' 'bootstrap' 'Install locked Python release dependencies into .venv'
	@printf '  %-18s %s\n' 'list-plugins' 'List every selected formal plugin and declared version'
	@printf '  %-18s %s\n' 'update-go-locks' 'Explicitly reconcile module locks after source integration'
	@printf '  %-18s %s\n' 'build-plugin' 'Build PLUGIN=name; optionally IMPLEMENTATION=cpp/native-go/assemblyscript'
	@printf '  %-18s %s\n' 'build-all' 'Build the complete formal catalog with pinned local containers'
	@printf '  %-18s %s\n' 'prepare-packages' 'Prepare complete package candidates and provenance review data'
	@printf '  %-18s %s\n' 'package-plugins' 'Validate approved complete packages into dist/; requires TRUSTED_POLICY'
	@printf '  %-18s %s\n' 'test' 'Run Python unit tests'
	@printf '  %-18s %s\n' 'build-cli' 'Build local upload CLI at $(CLI_DIST)'
	@printf '  %-18s %s\n' 'sync-plugin-artifacts' 'Copy registered artifacts from $(PLUGIN_ARTIFACTS_DIR) into $(PLUGIN_DIR)'
	@printf '  %-18s %s\n' 'docker-build' 'Build Docker image $(TAG)'
	@printf '  %-18s %s\n' 'docker-push' 'Tag and push Docker image $(REMOTE_TAG)'
	@printf '  %-18s %s\n' 'docker-buildx-push' 'Build and push Docker image $(REMOTE_TAG) for $(PLATFORMS)'
	@printf '  %-18s %s\n' 'run' 'Run Docker image locally with PLUGIN_SERVER_UPLOAD_TOKEN'
	@printf '  %-18s %s\n' 'upload' 'Upload a wasm plugin with the local CLI'
	@printf '  %-18s %s\n' 'clean' 'Remove local build outputs'
	@printf '%s\n' 'Set HOST_TOOLCHAIN=1 for an explicit developer host build. plugins/ always contains source.'

bootstrap:
	python3 -m venv .venv
	.venv/bin/python -m pip install --require-hashes -r requirements.lock

list-plugins:
	$(PYTHON) plugin_catalog.py

update-go-locks:
	$(PYTHON) scripts/update-go-locks.py

build-plugin:
	@test -n "$(PLUGIN)" || (printf '%s\n' 'PLUGIN is required' && exit 1)
	$(PYTHON) scripts/build-plugins.py --plugin "$(PLUGIN)" $(if $(IMPLEMENTATION),--implementation "$(IMPLEMENTATION)",) $(if $(filter 1,$(HOST_TOOLCHAIN)),--host,)

build-all:
	$(PYTHON) scripts/build-plugins.py $(if $(filter 1,$(HOST_TOOLCHAIN)),--host,)

prepare-packages: build-all
	$(PYTHON) scripts/package-plugins.py prepare

package-plugins: prepare-packages
	$(PYTHON) scripts/package-plugins.py ingest --policy "$(TRUSTED_POLICY)" --base-url "$(PUBLIC_BASE_URL)"

test:
	$(PYTHON) -m unittest discover -s tests -v
	$(PYTHON) -m py_compile plugin_server.py upload_plugin.py pull_plugins.py generate_metadata.py artifact_contract.py artifact_storage.py artifact_import.py artifact_packaging.py plugin_schema_conversion.py mf_canonical.py
	PYTHON=$(PYTHON) bash tests/test_sync_plugin_artifacts.sh

build-cli:
	$(PYTHON) scripts/build-upload-cli.py --output $(CLI_DIST)
	@printf 'Built %s\n' '$(CLI_DIST)'

clean:
	rm -rf dist __pycache__ tests/__pycache__

sync-plugin-artifacts:
	$(PYTHON) scripts/seed-registered-store.py --source "$(PLUGIN_ARTIFACTS_DIR)" --destination "$(PLUGIN_DIR)"

docker-build: package-plugins
	docker build \
		-t $(TAG) \
		-f $(DOCKERFILE) \
		.

docker-push:
	docker tag $(TAG) $(REMOTE_TAG)
	docker push $(REMOTE_TAG)

docker-buildx-push: package-plugins
	docker buildx build \
		--platform $(PLATFORMS) \
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
	@test -n "$(UPLOAD_ID)" || (printf '%s\n' 'UPLOAD_ID is required and must be reused after unknown outcomes' && exit 1)
	$(PYTHON) $(CLI_DIST) \
		--server $(SERVER) \
		--token $(TOKEN) \
		--file $(PLUGIN_FILE) \
		--name $(PLUGIN_NAME) \
		--version $(PLUGIN_VERSION) \
		--upload-id $(UPLOAD_ID)
