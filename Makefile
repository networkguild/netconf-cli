OPERATOR_NAME := netconf

.PHONY: format check ensure test build-linux-amd64 build-darwin-amd64 build-darwin-arm64 build-all build

format:
	 goimports -e ./pkg/ ./cmd/

check:
	go install golang.org/x/tools/cmd/goimports@latest
	./goimports.sh

ensure:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor

test: ensure check
	GO111MODULE=on go test -mod vendor ./cmd/... ./pkg/...

build-linux-amd64:
	rm -f bin/$(OPERATOR_NAME)-linux-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod vendor -ldflags="-s -w" -o bin/$(OPERATOR_NAME)-linux-amd64 main.go

build-darwin-amd64:
	rm -f bin/$(OPERATOR_NAME)-darwin-amd64
	GOOS=darwin GOARCH=amd64 GO111MODULE=on CGO_ENABLED=0 go build -mod vendor -ldflags="-s -w" -v -o bin/$(OPERATOR_NAME)-darwin-amd64 main.go

build-darwin-arm64:
	rm -f bin/$(OPERATOR_NAME)-darwin-arm64
	GOOS=darwin GOARCH=arm64 GO111MODULE=on CGO_ENABLED=0 go build -mod vendor -ldflags="-s -w" -v -o bin/$(OPERATOR_NAME)-darwin-arm64 main.go

build-all: build-linux-amd64 build-darwin-amd64 build-darwin-arm64

build: ensure
	rm -f bin/$(OPERATOR_NAME)
	GO111MODULE=on go build -mod vendor -ldflags="-s -w" -v -o bin/$(OPERATOR_NAME) main.go
