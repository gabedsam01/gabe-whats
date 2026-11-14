# gabe-whats

Gateway pessoal de **WhatsApp (`tulir/whatsmeow`)** em **Go**, estruturado de forma desacoplada com:
1. **`apps/api`**: API REST JSON com geração automática de **OpenAPI 3.1 + Swagger/Scalar UI (`/docs`)**, sessão única em SQLite, **Webhook HTTP POST** e **SSE (`/api/v1/events/stream`)**.
2. **`apps/mcp`**: Servidor **MCP (Model Context Protocol)** independente (`stdio` / `SSE`) para conectar seu WhatsApp ao **OpenClaw**, **Moltbot**, **Hermes Agent**, **Agent Zero** e clientes compatíveis.
3. **`shared/contracts`**: DTOs e contratos únicos compartilhados entre a API REST e o Servidor MCP.

> **Status Atual:** Projeto na fase **S3 — Mensagens e Eventos** concluída. `apps/api` sobe com autenticação por chave, `/docs`, `/health`, endpoints de sessão (status, QR Code, pareamento, reconexão, logout), envio de texto/mídia/nota de voz, histórico recente, webhook com HMAC e política de reentrega, stream SSE com ticket efêmero e mídia servida por URL temporária. As fases **S4** (grupos, enquetes e botões) e **S5** (servidor MCP) estão pendentes.

---

## Documentação e Desenho

- [Guia para Agentes (`AGENTS.md`)](file:///home/gabedsam01/meus_projetos/gabe-whats/AGENTS.md)
- [01 — Pesquisa Técnica do `tulir/whatsmeow`](file:///home/gabedsam01/meus_projetos/gabe-whats/docs/01-WHATSMEOW-RESEARCH.md)
- [02 — Arquitetura Geral & Diagramas Mermaid](file:///home/gabedsam01/meus_projetos/gabe-whats/docs/02-ARCHITECTURE.md)
- [03 — Especificação REST & Documentação Automática (`/docs`)](file:///home/gabedsam01/meus_projetos/gabe-whats/docs/03-API-SPEC-SWAGGER.md)
- [04 — Servidor MCP & Integração com Agentes](file:///home/gabedsam01/meus_projetos/gabe-whats/docs/04-MCP-AND-AGENTS.md)
- [Contrato OpenAPI 3.1 (`openapi.yaml`)](file:///home/gabedsam01/meus_projetos/gabe-whats/docs/openapi.yaml)

## Módulos do Desenho

- **Contratos Compartilhados:** [`shared/contracts/`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/README.md)
- **API REST (`whatsmeow`):** [`apps/api/`](file:///home/gabedsam01/meus_projetos/gabe-whats/apps/api/README.md)
- **Servidor MCP:** [`apps/mcp/`](file:///home/gabedsam01/meus_projetos/gabe-whats/apps/mcp/README.md)
- **Variáveis de Ambiente:** [`.env.example`](file:///home/gabedsam01/meus_projetos/gabe-whats/.env.example)
- **Modelo Go Workspace (OBSOLETO):** [`go.work.example`](file:///home/gabedsam01/meus_projetos/gabe-whats/go.work.example) — o projeto usa módulo Go único na raiz (ver `go.mod`); o arquivo permanece apenas como registro histórico.
