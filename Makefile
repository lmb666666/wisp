GO ?= go
WEB ?= pnpm

.PHONY: dev dev-api dev-web build build-web test test-go test-web test-e2e test-matrix load lint sqlc tidy release-snapshot clean

## 同时启动后端(:8080)与前端 dev server
dev:
	$(MAKE) -j2 dev-api dev-web

dev-api:
	$(GO) run ./cmd/wisp

dev-web:
	cd web && $(WEB) dev

## 构建：先打包前端，再编译嵌入了 web/dist 的单二进制
build: build-web
	CGO_ENABLED=0 $(GO) build -trimpath -o wisp ./cmd/wisp

build-web:
	cd web && $(WEB) build

test: test-go test-web

test-go:
	$(GO) test ./...

test-web:
	cd web && $(WEB) test

test-e2e:
	cd web && pnpm exec playwright test

test-matrix:
	python3 scripts/full_matrix.py

load:
	$(GO) test -tags load -run TestLoad10k ./internal/load/

release-snapshot: build-web
	goreleaser release --snapshot --clean

lint:
	$(GO) vet ./...
	cd web && $(WEB) lint

sqlc:
	sqlc generate

tidy:
	$(GO) mod tidy

clean:
	rm -f wisp
	rm -rf web/dist/* && touch web/dist/.gitkeep
