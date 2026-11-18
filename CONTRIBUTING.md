# Guia de Contribuição (`CONTRIBUTING.md`)

Obrigado pelo interesse em contribuir com o **gabe-whats**!

---

## 1. Regras Arquiteturais do Projeto

Antes de enviar um Pull Request, leia [`AGENTS.md`](AGENTS.md) e respeite os limites entre os pacotes:

1. **`apps/api` (Runtime WhatsApp):** único módulo autorizado a importar `go.mau.fi/whatsmeow`, manipular `session.db` / `app.db`, manter o WebSocket, baixar/servir mídias e disparar Webhooks/SSE.
2. **`apps/mcp` (Servidor MCP Stateless):** **nunca** deve importar `whatsmeow` nem abrir o SQLite diretamente. Toda operação deve usar `internal/apiclient` chamando a API REST via `GABE_WHATS_API_URL` e `GABE_WHATS_API_KEY`.
3. **`shared/contracts` (Fonte Única da Verdade):** todo DTO de request/response/webhook vive em `shared/contracts/` com tags `json`, `doc` (em português pt-BR), `enum` e limites validados por `shared/contracts/contracts_test.go`.
4. **Paridade OpenAPI (`/docs`):** ao adicionar ou alterar rotas/campos em `apps/api/internal/http/router.go`, atualize `docs/openapi.yaml` no mesmo commit. O teste `apps/api/internal/http/openapi_test.go` falha em caso de divergência.

---

## 2. Verificações Obrigatórias

Antes de abrir seu PR, garanta que todos os checks passam sem erros:

```bash
make check
# ou individualmente:
gofmt -l .     # deve retornar vazio
go vet ./...
go test ./...
go build ./...
```

---

## 3. Licença das Contribuições (AGPL-3.0)

Este projeto é distribuído sob a licença **GNU Affero General Public License v3.0 (AGPL-3.0)** — veja [`LICENSE`](LICENSE).

Ao enviar uma contribuição (issue, patch ou Pull Request), você concorda que seu código seja licenciado sob a mesma **AGPL-3.0**, garantindo que o projeto e quaisquer modificações permaneçam 100% livres e de código aberto.
