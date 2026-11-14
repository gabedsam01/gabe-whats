# AGENTS.md — Diretrizes do Projeto `gabe-whats`

Este arquivo orienta qualquer agente de código ou assistente (Antigravity, OpenClaw, Moltbot, Hermes Agent, Claude Code, Codex) que trabalhe neste repositório.

---

## 1. Propósito do Projeto

O **`gabe-whats`** é um gateway pessoal de **WhatsApp** construído em **Go (`go1.26+` / `go1.27`)** sobre [`go.mau.fi/whatsmeow`](https://github.com/tulir/whatsmeow).
- **Escopo:** Uso **pessoal** com **1 número/sessão** conectada como *companion device* (nunca para envio em massa/spam).
- **Exposição externa:** **100% via API REST JSON** (`/api/v1/*`) + **Servidor MCP** (`stdio`/`SSE`) + **Webhook HTTP POST / SSE**. O SQLite (`sqlstore`) existe apenas como armazenamento interno obrigatório do estado criptográfico Signal/sessão do `whatsmeow` — nunca exponha acesso SQL direto.

---

## 2. Regras Arquiteturais Obrigatórias

1. **Separação estrita entre `apps/api` e `apps/mcp`:**
   - **`apps/api`** é o único módulo que importa `go.mau.fi/whatsmeow`, abre o SQLite de sessão (`session.db`), mantém o WebSocket com o WhatsApp, baixa mídias e dispara Webhooks/SSE.
   - **`apps/mcp`** é **stateless** e **NUNCA** deve importar `whatsmeow` nem abrir o SQLite diretamente. Toda operação do servidor MCP deve passar por chamadas HTTP REST à `apps/api` usando `GABE_WHATS_API_URL` e `GABE_WHATS_API_KEY`.
2. **Fonte única de contratos (`shared/contracts`):**
   - Todos os DTOs de request/response e eventos de webhook residem em `shared/contracts/` (`common.go`, `session.go`, `contacts.go`, `messages.go`, `groups.go`, `webhooks.go`).
   - Ao alterar ou criar endpoints/tools, atualize primeiro as structs em `shared/contracts/` para manter `apps/api`, `apps/mcp` e a documentação OpenAPI sincronizados.
3. **Documentação da API (`/docs`) e contratos:**
   - O Huma v2 + Chi é o runtime (roteamento, binding e validação). A especificação **publicada** em `docs/openapi.yaml` é curada e servida em `/openapi.yaml`, `/openapi.json` e `/docs` (Scalar UI).
   - A paridade é garantida por teste (`apps/api/internal/http/openapi_test.go` compara a spec com as rotas registradas; `shared/contracts/contracts_test.go` exige `doc` em todo campo exportado). **Toda rota ou campo novo exige atualizar a spec no mesmo commit** — o CI quebra caso contrário.
   - Toda struct em `shared/contracts/` deve manter tags `json`, `doc`, `enum`, `required` (somente requests) e limites (`maxItems`, `maxLength`) atualizadas.
4. **Protocolo `whatsmeow` & Casos Especiais:**
   - **Botões e Listas (`SendButtons`, `SendList`):** No protocolo Multi-Device, mensagens interativas (`InteractiveMessage`/`NativeFlowMessage` e `ListMessage`) requerem atenção aos nós binários adicionais (`SendRequestExtra.AdditionalNodes` com `<biz>`). Mantenha **Enquetes (`BuildPollCreation` / `DecryptPollVote`)** como alternativa nativa 100% garantida para contas pessoais.
   - **Recebimento de Eventos:** Todo evento `*events.Message` recebido via `AddEventHandler` deve ser normalizado para `contracts.WebhookInboundEvent` (desembrulhando texto, baixando mídia/áudio PTT via `DownloadAny` quando habilitado, extraindo clique de botão e descriptografando votos de enquete via `DecryptPollVote`).
   - **Grupos:** Respeitar o limite de 25 caracteres imposto pelo WhatsApp para nome do grupo (`CreateGroup` / `SetGroupName`).
5. **Segurança para Agentes Pessoais:**
   - A API REST escuta localmente por padrão (`127.0.0.1:8080`) e exige `X-API-Key` ou `Authorization: Bearer` em `/api/v1/*`.
   - Webhooks devem respeitar `IgnoreFromMe` (para evitar loops de auto-resposta) e `AllowJIDs` quando configurados, além de assinar payloads com HMAC-SHA256 quando `GABE_WHATS_WEBHOOK_SECRET` estiver definido.

---

## 3. Mapa de Arquivos de Referência

Antes de implementar ou modificar qualquer componente, leia a documentação correspondente:

- Visão geral: `README.md`
- Pesquisa do `tulir/whatsmeow`: `docs/01-WHATSMEOW-RESEARCH.md`
- Arquitetura & diagramas Mermaid: `docs/02-ARCHITECTURE.md`
- Rotas REST & geração OpenAPI (`/docs`): `docs/03-API-SPEC-SWAGGER.md` e `docs/openapi.yaml`
- Servidor MCP & payload de Webhooks: `docs/04-MCP-AND-AGENTS.md`
- Contratos Go: `shared/contracts/README.md`
- API REST (implementação): `apps/api/README.md` e `apps/api/internal/whatsapp/`
- Blueprint do Servidor MCP: `apps/mcp/README.md` e `apps/mcp/internal/tools/blueprint.go`

---

## 4. Convenções de Trabalho & Verificação

- **Estado atual do repositório:** Fases S1 (contratos + OpenAPI), S2 (daemon), S3 (mensagens e eventos) e S4 (grupos, enquetes e interatividade) concluídas. `apps/api` está funcional ponta a ponta; `apps/mcp` continua blueprint até o S5. Não adicione dependências pesadas nem lógica completa fora do escopo pedido sem alinhamento prévio.
- **Comandos obrigatórios antes de concluir:** `gofmt -l .` (vazio), `go vet ./...` e `go test ./...`. O alvo `make check` roda todos.
- **Idioma:** Documentação, comentários de descrição OpenAPI (`doc:"..."`) e mensagens orientadas ao usuário devem ser escritos em **português (pt-BR)**; identificadores Go, nomes de campos JSON e símbolos seguem **inglês** padrão idiomático Go.
- **Antes de concluir mudanças em código Go:** Execute `gofmt -l .` (e `go test ./...` / `go vet ./...` após os módulos `go.mod` serem inicializados).
- **Segredos:** Nunca commite `.env`, tokens reais ou arquivos `.db`/chaves Signal de sessão (`session.db`, `data/`).
