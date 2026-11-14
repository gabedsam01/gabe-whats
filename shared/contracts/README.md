# `shared/contracts` — Contratos Compartilhados (DTOs)

Este pacote contém exclusivamente as definições de dados (structs Go com tags JSON e tags de documentação OpenAPI) compartilhadas entre:
- **`apps/api`**: usa estas structs para validar payloads HTTP e gerar automaticamente a documentação OpenAPI 3.1 / Swagger UI / Scalar UI (`/docs`).
- **`apps/mcp`**: usa exatamente as mesmas structs para serializar chamadas HTTP à API REST e descrever os schemas JSON das Tools MCP.

## Arquivos
- [`messages.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/messages.go): Envio de texto, mídia, áudio PTT, botões interativos, listas, enquetes, reações, edição, revogação, leitura, presença e consulta do histórico recente.
- [`groups.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/groups.go): Criação, leitura, atualização de configurações/correção, gestão de participantes (com resultado por participante), foto e link de convite de grupos.
- [`session.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/session.go): Estado da conexão da sessão, QR Code de pareamento e pareamento por código de 8 dígitos.
- [`contacts.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/contacts.go): Verificação de quais números possuem conta no WhatsApp, com resultado por número consultado.
- [`webhooks.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/webhooks.go): Envelope normalizado (`WebhookInboundEvent`) enviado via Webhook HTTP POST / SSE para OpenClaw, Moltbot, Hermes e Agent Zero, incluindo os objetos de mensagem, mídia, voto de enquete, `group` e `session`, além da configuração do webhook.
- [`common.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/common.go): Tipos comuns a todas as rotas — erro padrão RFC 9457 (`Problem`/`ProblemField`), healthcheck (`HealthResponse`), confirmação simples (`AckResponse`) e referência de mídia com URL temporária (`MediaRef`).
- [`contracts_test.go`](file:///home/gabedsam01/meus_projetos/gabe-whats/shared/contracts/contracts_test.go): Testes que auditam o próprio pacote (tag `doc` obrigatória, `json` em snake_case, contrato de erro e cobertura do registro de structs).
