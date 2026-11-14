# Makefile do gabe-whats — atalhos de desenvolvimento e verificação.
# Requer Go 1.26+ e gofmt disponíveis no PATH.

.DEFAULT_GOAL := help
.PHONY: help fmt fmt-check vet test build bin openapi check

help:
	@echo "Alvos disponíveis:"
	@echo "  make fmt        Formata todos os arquivos Go com gofmt -w"
	@echo "  make fmt-check  Falha se gofmt -l encontrar arquivos não formatados"
	@echo "  make vet        Executa go vet ./..."
	@echo "  make test       Executa go test ./..."
	@echo "  make build      Executa go build ./..."
	@echo "  make bin        Gera bin/gabe-whats-api e bin/gabe-whats-mcp"
	@echo "  make openapi    Executa o teste de paridade entre router.go e docs/openapi.yaml"
	@echo "  make check      fmt-check + vet + test + build"

fmt:
	gofmt -w .

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt encontrou arquivos não formatados:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

build:
	go build ./...

# Binários prontos para uso (o MCP é registrado no cliente por caminho absoluto).
bin:
	mkdir -p bin
	go build -o bin/gabe-whats-api ./apps/api/cmd/server
	go build -o bin/gabe-whats-mcp ./apps/mcp/cmd/mcp-server

openapi:
	go test ./apps/api/internal/http -run 'TestPlannedRoutesMatchOpenAPI|TestOpenAPIOperationIDs|TestOpenAPISecurity' -count=1 -v

check: fmt-check vet test build
