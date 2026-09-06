SHELL := /bin/bash

BUILD_DIR ?= .build
SDKROOT ?= $(shell xcrun --sdk macosx --show-sdk-path)
ARCH ?= $(shell uname -m)
TARGET ?= $(ARCH)-apple-macosx13.0
CLANG_MODULE_CACHE_PATH ?= $(BUILD_DIR)/ModuleCache
export CLANG_MODULE_CACHE_PATH

SWIFTC = xcrun swiftc
FLAGS = -sdk $(SDKROOT) -target $(TARGET) -swift-version 5 -parse-as-library \
	-framework LocalAuthentication -framework Security -warnings-as-errors \
	-Wwarning DeprecatedDeclaration
CORE = Sources/Base.swift Sources/PassphraseStore.swift
UNLOCK = $(CORE) Sources/AgentClient.swift Sources/UnlockMain.swift
RESET = $(CORE) Sources/PassphraseReset.swift Sources/ResetPTY.swift Sources/ResetMain.swift
TEST = $(CORE) Sources/AgentClient.swift Sources/ResetPTY.swift Sources/ResetMain.swift Tests/TestMain.swift
INTEGRATION = $(CORE) Sources/AgentClient.swift Sources/ResetPTY.swift Tests/IntegrationTestMain.swift

.PHONY: all build test test-integration

all: build

build: $(BUILD_DIR)/ghtkn-touchid $(BUILD_DIR)/ghtkn-touchid-reset

$(BUILD_DIR)/ghtkn-touchid: Makefile $(UNLOCK)
	@mkdir -p $(@D)
	$(SWIFTC) $(FLAGS) -O $(UNLOCK) -o $@

$(BUILD_DIR)/ghtkn-touchid-reset: Makefile $(RESET)
	@mkdir -p $(@D)
	$(SWIFTC) $(FLAGS) -O $(RESET) -o $@

$(BUILD_DIR)/ghtkn-touchid-tests: Makefile $(TEST)
	@mkdir -p $(@D)
	$(SWIFTC) $(FLAGS) -Onone -D TESTING -D UNIT_TESTING $(TEST) -o $@

$(BUILD_DIR)/ghtkn-touchid-integration: Makefile $(INTEGRATION)
	@mkdir -p $(@D)
	$(SWIFTC) $(FLAGS) -Onone $(INTEGRATION) -o $@

test: build $(BUILD_DIR)/ghtkn-touchid-tests
	$(BUILD_DIR)/ghtkn-touchid-tests
	@if strings $(BUILD_DIR)/ghtkn-touchid $(BUILD_DIR)/ghtkn-touchid-reset | grep -q GHTKN_TOUCHID_TEST; then \
		echo "test-only authentication override found in a production binary" >&2; exit 1; \
	fi

GHTKN_VERSION ?=
GHTKN_ARCH = $(if $(filter arm64,$(ARCH)),arm64,amd64)
GHTKN_DIR = $(abspath $(BUILD_DIR)/ghtkn-versions/$(GHTKN_VERSION))

test-integration: $(BUILD_DIR)/ghtkn-touchid-integration
ifdef GHTKN_VERSION
	@mkdir -p $(GHTKN_DIR)
	@test -x $(GHTKN_DIR)/ghtkn || { \
		curl -sL "https://github.com/suzuki-shunsuke/ghtkn/releases/download/$(GHTKN_VERSION)/ghtkn_darwin_$(GHTKN_ARCH).tar.gz" \
			| tar -xz -C $(GHTKN_DIR) ghtkn; \
		xattr -dr com.apple.quarantine $(GHTKN_DIR)/ghtkn 2>/dev/null || true; \
	}
	PATH="$(GHTKN_DIR):$$PATH" $(BUILD_DIR)/ghtkn-touchid-integration
else
	$(BUILD_DIR)/ghtkn-touchid-integration
endif
