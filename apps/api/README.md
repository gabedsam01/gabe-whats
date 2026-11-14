# `apps/api` — Daemon REST & Gateway `whatsmeow`

Este serviço é o **único dono** da sessão do WhatsApp e dos arquivos SQLite locais: o `session.db` do `whatsmeow` (estado criptográfico Signal) e um `app.db` próprio (`GABE_WHATS_APP_DSN`, para cache de enquetes e configuração de webhooks). Ele permanece rodando em background para manter o WebSocket conectado, servir a API REST com documentação (`/docs`) e — a partir do S3 — despachar eventos para os agentes via **Webhook HTTP POST** e **SSE**.

## Camadas (estado atual — S2)

| Diretório / arquivo | Responsabilidade | Estado |
| :--- | :--- | :--- |
| [`cmd/server/main.go`](cmd/server/main.go) | Bootstrap: carrega configuração, abre a sessão, sobe o HTTP e trata encerramento gracioso. | **Implementado** |
| [`internal/config/config.go`](internal/config/config.go) | Parsing e validação das variáveis de ambiente (com `.env` opcional). | **Implementado** + testes |
| [`internal/whatsapp/manager.go`](internal/whatsapp/manager.go) | Conexão, status, QR Code, código de pareamento, reconexão e logout. | **Implementado** + testes de sessão via HTTP |
| [`internal/whatsapp/`](internal/whatsapp/) | Envio (texto, mídia, PTT, botões, listas, enquetes), ações em mensagens, grupos e normalização de eventos. | **Implementado** |
| [`internal/http/server.go`](internal/http/server.go) | Chi + Huma, autenticação por chave, `/health`, `/docs` e spec publicada. | **Implementado** |
| [`internal/http/errors.go`](internal/http/errors.go) | Erros no padrão RFC 9457 (`application/problem+json`). | **Implementado** |
| [`internal/http/session.go`](internal/http/session.go) | Endpoints de sessão e pareamento. | **Implementado** |
| [`internal/http/router.go`](internal/http/router.go) | `PlannedRoutes`: mapa de rotas que serve de contrato para os testes. | **Implementado** |
| `internal/http/handlers/` e `internal/http/middleware/` | Subdiretórios previstos no desenho. | Não foram necessários: handlers e autenticação vivem em `internal/http` por enquanto |
| [`internal/appstore/`](internal/appstore/store.go) | SQLite próprio (`app.db`): configuração de webhook e registro de mídias. | **Implementado** + testes |
| [`internal/media/`](internal/media/store.go) | Gravação de mídia baixada e URL temporária com TTL (padrão 15 min). | **Implementado** + testes |
| [`internal/webhook/`](internal/webhook/dispatcher.go) | Entrega HTTP POST com HMAC-SHA256, timeout por tentativa e 1 + 5 reentregas (5s, 15s, 25s, 45s, 60s). | **Implementado** + testes |
| [`internal/sse/`](internal/sse/hub.go) | Hub de Server-Sent Events e tickets efêmeros de uso único (60s). | **Implementado** + testes |
| [`internal/events/`](internal/events/bus.go) | Barramento: histórico recente, SSE e webhook com filtros (`ignore_from_me`, `allow_jids`). | **Implementado** + testes |

## Como rodar

```bash
cp .env.example .env          # ajuste GABE_WHATS_API_KEY (mínimo 16 caracteres)
go run ./apps/api/cmd/server
```

- Documentação interativa: <http://127.0.0.1:8080/docs>
- Especificação: `/openapi.json` e `/openapi.yaml`
- Healthcheck (sem autenticação): `/health`
- Todas as rotas `/api/v1/*` exigem `X-API-Key` ou `Authorization: Bearer`.

Primeiro pareamento:

```bash
curl -H "X-API-Key: $GABE_WHATS_API_KEY" http://127.0.0.1:8080/api/v1/session/qr   # leia o QR
# ou, sem câmera:
curl -X POST -H "X-API-Key: $GABE_WHATS_API_KEY" -H 'Content-Type: application/json' \
     -d '{"phone":"5511999999999"}' http://127.0.0.1:8080/api/v1/session/pair-code
```

Fechar o processo encerra a conexão via `SIGINT`/`SIGTERM` sem corromper o `session.db`.
