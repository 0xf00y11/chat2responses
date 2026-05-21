# Makefile for chat2responses
#
# Copyright (c) 2026 fooyii.
#
# Created: 2026-05-22

BINARY     := chat2responses
VERSION    := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT     := $(shell git log -1 --format='%h' 2>/dev/null || echo "unknown")
BUILDTIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS    := -ldflags="-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILDTIME)"

.PHONY: all build build-all clean release

all: build

## build - 编译当前平台二进制
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/$(BINARY)

## build-linux - 编译 Linux AMD64 二进制
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o release/$(BINARY)-linux-amd64 ./cmd/$(BINARY)

## build-darwin - 编译 macOS (Intel) 二进制
build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o release/$(BINARY)-darwin-amd64 ./cmd/$(BINARY)

## build-darwin-arm64 - 编译 macOS (Apple Silicon) 二进制
build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o release/$(BINARY)-darwin-arm64 ./cmd/$(BINARY)

## build-windows - 编译 Windows AMD64 二进制
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/$(BINARY)-windows-amd64.exe ./cmd/$(BINARY)

## build-all - 编译所有平台二进制 (Linux / macOS Intel+Silicon / Windows)
build-all: release build-linux build-darwin build-darwin-arm64 build-windows
	@echo "\nâœ“ All binaries built in release/"
	@ls -lh release/

release:
	mkdir -p release

## clean - 清除编译产物
clean:
	$(RM) -r release/
	$(RM) $(BINARY)

## fmt - 格式化代码
fmt:
	gofumpt -l -w . 2>/dev/null || gofmt -l -w .

## vet - 代码审查
vet:
	go vet ./...

## test - 运行测试
test:
	go test ./... -v

## help - 显示所有目标
help:
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

