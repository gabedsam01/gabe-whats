# `apps/mcp` — Servidor MCP (`Model Context Protocol`) para Agentes

Este módulo expõe toda a API REST do `gabe-whats` como **Tools** e **Resources** no padrão **MCP**, permitindo plugar seu WhatsApp pessoal diretamente no **OpenClaw**, **Moltbot**, **Hermes Agent**, **Agent Zero**, **Claude Desktop** ou **Antigravity/IDE**.

## Características do Desenho

- **Stateless e Leve:** Não abre SQLite e não importa `whatsmeow`. Ele se conecta à `apps/api` via HTTP (`GABE_WHATS_API_URL`) usando `GABE_WHATS_API_KEY`.
- **Modos de Transporte Planejados:**
  - `--transport stdio` (padrão para agentes locais como OpenClaw / Hermes / Claude / Antigravity)
  - `--transport sse` (para agentes rodando em outro container ou VPS), com endereço definido pela flag `--sse-addr` (ex.: `--sse-addr 127.0.0.1:8090`)

- **Biblioteca Recomendada na Implementação:** SDK oficial Go do MCP (`github.com/modelcontextprotocol/go-sdk`) ou `github.com/mark3labs/mcp-go`.

> **Nota:** o transporte do servidor MCP é configurado **somente por flags** (`--transport` e `--sse-addr`); não existem variáveis de ambiente para isso (ver [`.env.example`](../../.env.example)).

## Estrutura Interna

| Diretório | Responsabilidade |
| :--- | :--- |
| `cmd/mcp-server/` | Inicializa o servidor MCP (`stdio` ou `sse`), instancia o `apiclient` e registra tools + resources. |
| `internal/apiclient/` | Cliente HTTP tipado usando os DTOs de `shared/contracts` e injetando `X-API-Key`. |
| `internal/tools/` | Registro das 22 Tools MCP (`whatsapp_send_text`, `whatsapp_send_buttons`, `whatsapp_send_poll`, `whatsapp_create_group`, etc.). |
| `internal/resources/` | Registro dos Resources de leitura (`whatsapp://session/status`, `whatsapp://groups/list`, `whatsapp://messages/recent`). |
