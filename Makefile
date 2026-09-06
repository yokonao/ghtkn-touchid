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

.PHONY: all build test

all: build

build: $(BUILD_DIR)/ghtkn-touchid $(BUILD_DIR)/ghtkn-touchid-reset

$(BUILD_DIR):
	mkdir -p $@

$(BUILD_DIR)/ghtkn-touchid: Makefile $(UNLOCK) | $(BUILD_DIR)
	$(SWIFTC) $(FLAGS) -O $(UNLOCK) -o $@

$(BUILD_DIR)/ghtkn-touchid-reset: Makefile $(RESET) | $(BUILD_DIR)
	$(SWIFTC) $(FLAGS) -O $(RESET) -o $@

$(BUILD_DIR)/ghtkn-touchid-tests: Makefile $(TEST) | $(BUILD_DIR)
	$(SWIFTC) $(FLAGS) -Onone -D TESTING -D UNIT_TESTING $(TEST) -o $@

test: build $(BUILD_DIR)/ghtkn-touchid-tests
	$(BUILD_DIR)/ghtkn-touchid-tests
	@if strings $(BUILD_DIR)/ghtkn-touchid $(BUILD_DIR)/ghtkn-touchid-reset | grep -q GHTKN_TOUCHID_TEST; then \
		echo "test-only authentication override found in a production binary" >&2; exit 1; \
	fi
