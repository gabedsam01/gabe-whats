# `apps/api` — Daemon REST & Gateway `whatsmeow`

Este serviço é o **único dono** da sessão do WhatsApp e dos arquivos SQLite locais: o `session.db` do `whatsmeow` (estado criptográfico Signal) e um `app.db` próprio (`GABE_WHATS_APP_DSN`, para cache de enquetes e configuração de webhooks). Ele permanece rodando em background para manter o WebSocket conectado, servir a API REST + documentação automática (`/docs`) e despachar eventos em tempo real para os agentes via **Webhook HTTP POST** e **SSE**.

## Camadas (estado atual — S1: apenas blueprints)

| Diretório | Responsabilidade | Estado |
| :--- | :--- | :--- |
| `cmd/server/` | Bootstrap da aplicação (carrega `.env`, inicializa `sqlstore` + `whatsmeow.Client`, sobe servidor HTTP e trata graceful shutdown). | Blueprint em `main.go` (ciclo de vida descrito em comentários); implementação em S2. |
| `internal/config/` | Parsing e validação das variáveis de ambiente reais (`GABE_WHATS_HTTP_ADDR`, `GABE_WHATS_API_KEY`, `GABE_WHATS_SQLITE_DSN`, `GABE_WHATS_APP_DSN`, `GABE_WHATS_MEDIA_URL_TTL`, `GABE_WHATS_WEBHOOK_URL`, etc. — ver [`.env.example`](../../.env.example)). | Planejado (S2). |
| `internal/whatsapp/` | Wrapper sobre `go.mau.fi/whatsmeow` (conexão, QR/PairCode, envio de texto/mídia/PTT, botões com `<biz>`, enquetes, grupos e download de mídia). | Interface `Service` + tipos em `blueprint.go`; implementação em S2. |
| `internal/http/` | Mapa de rotas REST com geração automática de **OpenAPI 3.1 + Scalar/Swagger UI** em `/docs`, `/openapi.json` e `/openapi.yaml`. | `router.go` com `PlannedRoutes`; handlers/middleware em S2. |
| `internal/http/middleware/` | Autenticação por `X-API-Key` / `Authorization: Bearer`, CORS local, logging estruturado e rate-limit leve. | Planejado (S2). |
| `internal/http/handlers/` | Handlers REST registrados no router (Huma v2 + Chi) a partir dos DTOs de `shared/contracts`. | Planejado (S2). |
| `internal/webhook/` | Normalizador de `*events.Message` / `*events.GroupInfo` para `contracts.WebhookInboundEvent`, fila assíncrona de entrega POST com assinatura HMAC e hub SSE. | Planejado (S2). |
