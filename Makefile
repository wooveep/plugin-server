EXTENSIONS_DIR ?= extensions
BUILD_DIR ?= build/plugins
PLUGIN_VERSION ?=
PLUGIN ?=

WASM_BUILDER ?= go
GO ?= go
TINYGO ?= tinygo
GO_BUILD_FLAGS ?= -buildmode=c-shared -trimpath
TINYGO_FLAGS ?= -scheduler=none -target=wasi -gc=custom
TINYGO_BASE_TAGS ?= custommalloc nottinygc_finalizer
EXTRA_TAGS ?=
TINYGO_TAGS := $(strip $(TINYGO_BASE_TAGS) $(EXTRA_TAGS))

PLUGIN_DIRS := $(foreach mod,$(wildcard $(EXTENSIONS_DIR)/*/go.mod),$(if $(wildcard $(dir $(mod))main.go),$(patsubst %/,%,$(dir $(mod)))))
PLUGINS := $(sort $(notdir $(PLUGIN_DIRS)))

.DEFAULT_GOAL := build

.PHONY: build all build-all build-plugin metadata list-plugins clean

build: build-all

all: build-all

list-plugins:
	@printf '%s\n' $(PLUGINS)

build-all:
	@if [ -z "$(PLUGINS)" ]; then \
		echo "No plugins found under $(EXTENSIONS_DIR)" >&2; \
		exit 2; \
	fi
	@set -e; \
	for plugin in $(PLUGINS); do \
		$(MAKE) --no-print-directory build-plugin PLUGIN=$$plugin; \
	done

build-plugin:
	@if [ -z "$(PLUGIN)" ]; then \
		echo "PLUGIN is required. Usage: make build-plugin PLUGIN=<name>" >&2; \
		exit 2; \
	fi
	@found=0; \
	for plugin in $(PLUGINS); do \
		if [ "$$plugin" = "$(PLUGIN)" ]; then found=1; fi; \
	done; \
	if [ "$$found" -ne 1 ]; then \
		echo "Unknown plugin '$(PLUGIN)'. Discovered plugins: $(PLUGINS)" >&2; \
		exit 2; \
	fi
	@plugin_version="$(PLUGIN_VERSION)"; \
	if [ -z "$$plugin_version" ]; then \
		version_file="$(EXTENSIONS_DIR)/$(PLUGIN)/VERSION"; \
		if [ ! -f "$$version_file" ]; then \
			echo "Missing version file: $$version_file" >&2; \
			exit 2; \
		fi; \
		plugin_version=$$(tr -d '[:space:]' < "$$version_file"); \
		if [ -z "$$plugin_version" ]; then \
			echo "Empty version file: $$version_file" >&2; \
			exit 2; \
		fi; \
	fi; \
	out_dir="$(abspath $(BUILD_DIR))/$(PLUGIN)/$$plugin_version"; \
	wasm="$$out_dir/plugin.wasm"; \
	mkdir -p "$$out_dir"; \
	echo "Building $(PLUGIN) -> $$wasm"; \
	case "$(WASM_BUILDER)" in \
		go) \
			(cd "$(EXTENSIONS_DIR)/$(PLUGIN)" && GOOS=wasip1 GOARCH=wasm $(GO) build $(GO_BUILD_FLAGS) -o "$$wasm" ./) ;; \
		tinygo) \
			(cd "$(EXTENSIONS_DIR)/$(PLUGIN)" && $(TINYGO) build $(TINYGO_FLAGS) -tags='$(TINYGO_TAGS)' -o "$$wasm" main.go) ;; \
		*) \
			echo "Unsupported WASM_BUILDER '$(WASM_BUILDER)'. Use 'go' or 'tinygo'." >&2; \
			exit 2 ;; \
	esac
	@$(MAKE) --no-print-directory metadata PLUGIN=$(PLUGIN)

metadata:
	@if [ -z "$(PLUGIN)" ]; then \
		echo "PLUGIN is required. Usage: make metadata PLUGIN=<name>" >&2; \
		exit 2; \
	fi
	@plugin_version="$(PLUGIN_VERSION)"; \
	if [ -z "$$plugin_version" ]; then \
		version_file="$(EXTENSIONS_DIR)/$(PLUGIN)/VERSION"; \
		if [ ! -f "$$version_file" ]; then \
			echo "Missing version file: $$version_file" >&2; \
			exit 2; \
		fi; \
		plugin_version=$$(tr -d '[:space:]' < "$$version_file"); \
		if [ -z "$$plugin_version" ]; then \
			echo "Empty version file: $$version_file" >&2; \
			exit 2; \
		fi; \
	fi; \
	wasm="$(abspath $(BUILD_DIR))/$(PLUGIN)/$$plugin_version/plugin.wasm"; \
	out_dir=$$(dirname "$$wasm"); \
	if [ ! -f "$$wasm" ]; then \
		echo "Missing wasm artifact: $$wasm" >&2; \
		exit 2; \
	fi; \
	size=$$(wc -c < "$$wasm" | tr -d ' '); \
	last_modified=$$(date -r "$$wasm" "+%Y-%m-%dT%H:%M:%S"); \
	created=$$(date "+%Y-%m-%dT%H:%M:%S.%6N"); \
	md5=$$(md5sum "$$wasm" | awk '{print $$1}'); \
	{ \
		printf 'Plugin Name: %s\n' "$(PLUGIN)"; \
		printf 'Size: %s bytes\n' "$$size"; \
		printf 'Last Modified: %s\n' "$$last_modified"; \
		printf 'Created: %s\n' "$$created"; \
		printf 'MD5: %s\n' "$$md5"; \
	} > "$$out_dir/metadata.txt"; \
	echo "metadata: $$out_dir/metadata.txt"

clean:
	rm -rf "$(BUILD_DIR)"
