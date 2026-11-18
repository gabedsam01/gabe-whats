# Política de Segurança (`SECURITY.md`)

Este documento define as diretrizes de segurança operacional e de código do **gabe-whats**, um gateway pessoal de WhatsApp construído sobre [`go.mau.fi/whatsmeow`](https://github.com/tulir/whatsmeow).

---

## 1. Escopo e Modelo de Ameaça

O `gabe-whats` foi projetado para **uso pessoal** com **1 sessão/número** conectado como *companion device* (Multi-Device) para automações e agentes de IA. Ele **não** deve ser usado para disparo em massa/spam, nem exposto diretamente na internet sem camada de autenticação e TLS.

## 2. Proteção de Credenciais e Chaves Criptográficas

### 2.1 Banco de Sessão (`session.db` / `app.db`)
- O arquivo `data/session.db` armazena as chaves criptográficas do protocolo **Signal** da sua conta WhatsApp. Qualquer pessoa com acesso a esse arquivo assume a identidade da sessão.
- **Nunca** commite arquivos `.db`, `.db-shm`, `.db-wal`, `data/`, `media/` ou `.env` no Git.
- Nunca exponha acesso SQL direto externo: toda a superfície pública deve passar exclusivamente pela API REST (`apps/api`) ou pelo Servidor MCP (`apps/mcp`).

### 2.2 Tokens e Segredos Obrigatórios
- **`GABE_WHATS_API_KEY`:** obrigatória (mínimo 16 caracteres, diferente do valor de exemplo). Todas as rotas `/api/v1/*` da `apps/api` exigem `X-API-Key` ou `Authorization: Bearer` (comparação em tempo constante), com exceção apenas dos links pré-assinados de mídia `/api/v1/media/{id}` (ID aleatório de 128 bits com TTL configurável via `GABE_WHATS_MEDIA_URL_TTL`, padrão `15m`) e do stream `/api/v1/events/stream` com ticket efêmero emitido por `POST /api/v1/events/ticket`.
- **`GABE_WHATS_MCP_TOKEN`:** token dedicado exigido via `Authorization: Bearer <token>` (ou `X-API-Key`) pelo servidor MCP nos transportes `--transport http` (Streamable HTTP) e `--transport sse`. Quando não definido, o servidor reutiliza `GABE_WHATS_API_KEY` para que o listener HTTP **nunca** suba sem autenticação.
  - Em produção/nuvem, **sempre** defina um `GABE_WHATS_MCP_TOKEN` distinto e forte (`openssl rand -hex 32`).
- **`GABE_WHATS_WEBHOOK_SECRET`:** quando configurado, todos os `POST` de webhook são assinados com HMAC-SHA256 no header `X-GabeWhats-Signature: sha256=<hmac>`. Valide essa assinatura no consumidor antes de processar eventos.

## 3. Segurança do Conector MCP Remoto (`--transport http`)

Ao expor o servidor MCP para plataformas externas (como `agentes.chat`, OpenClaw ou Hermes Agent) em VPS, Nexus ou **Dokploy**:

1. **TLS (HTTPS) obrigatório:** nunca trafegue o token `Bearer` em HTTP puro pela internet. Deixe o proxy reverso (Traefik do Dokploy ou Caddy) encerrar o TLS em `https://<seu-subdominio>/mcp`.
2. **Isolamento de rede:** mantenha `gabe-whats-api` acessível apenas na rede interna Docker (`http://gabe-whats-api:8080`) sem publicar a porta `8080` publicamente, a menos que sua aplicação externa precise chamar a API REST diretamente com HTTPS + `GABE_WHATS_API_KEY`.
3. **Healthcheck público restrito:** apenas `GET /health` não exige token (retorna somente `status`, `version` e `transport`), enquanto `/mcp`, `/mcp/`, `/sse` e `/` exigem autenticação Bearer.
4. **Guardrails para Agentes:**
   - Mantenha `GABE_WHATS_WEBHOOK_IGNORE_FROM_ME=true` (padrão) para evitar loops de auto-resposta.
   - Use `GABE_WHATS_WEBHOOK_ALLOW_JIDS` para restringir quais chats/grupos disparam webhooks automáticos para o agente.

## 4. Como Reportar Vulnerabilidades

Se você identificar uma falha de segurança neste projeto:

- **Não** abra uma *issue* pública com detalhes exploráveis ou credenciais expostas.
- Entre em contato diretamente com o mantenedor **Gabriel Sampaio**:
  - GitHub: [@gabedsam01](https://github.com/gabedsam01)
  - Portfólio / Contato: [sampaiodev.com.br](https://sampaiodev.com.br)

Inclua passos para reproduzir, impacto estimado e versão/commit afetado para correção prioritária.
