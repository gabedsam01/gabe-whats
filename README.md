# gabe-whats

Gateway pessoal de **WhatsApp (`go.mau.fi/whatsmeow`)** em **Go**, estruturado de forma desacoplada e pronto para automações, agentes de IA e deploy em container (**Docker Compose / Dokploy**).

> **Aviso de escopo:** Projetado para **uso pessoal** com **1 número/sessão** conectada como *companion device* (Multi-Device). Nunca utilize para envio em massa/spam.

---

## 1. Arquitetura

1. **`apps/api` (`gabe-whats-api`):**
   - Único módulo que importa `go.mau.fi/whatsmeow` e persiste o estado Signal em SQLite (`data/session.db` + `data/app.db`).
   - Expõe API REST JSON (`/api/v1/*`) com Huma v2 + Chi, documentação interativa em **`/docs`** (Scalar UI) e especificação **OpenAPI 3.1** (`/openapi.json`, `/openapi.yaml`).
   - Envio de texto (com simulação de digitação e ritmo humano configurável), mídia (`image`/`video`/`document`), nota de voz PTT, enquetes nativas, botões, listas, reações, edição, revogação, marcação de lido, gestão de grupos, histórico recente (`/api/v1/messages/recent`), stream SSE com ticket efêmero (`/api/v1/events/stream`) e **Webhook HTTP POST** com assinatura HMAC-SHA256.
2. **`apps/mcp` (`gabe-whats-mcp`):**
   - Servidor **MCP (Model Context Protocol) 100% stateless** — nunca importa `whatsmeow` nem abre o SQLite; comunica-se exclusivamente via REST com `apps/api`.
   - Expõe **22 tools** e **3 resources** através de três transportes:
     - `--transport stdio` (local / CLI / sandbox)
     - `--transport http` (**Streamable HTTP** em `/mcp`, protegido por `Authorization: Bearer <token>`, ideal para conectores remotos como `agentes.chat`)
     - `--transport sse` (SSE legado, também protegido por Bearer token)
3. **`shared/contracts`:**
   - Fonte única da verdade para os DTOs de request, response e eventos de webhook.

---

## 2. Execução Rápida Local

```bash
cp .env.example .env
# Defina GABE_WHATS_API_KEY (mín. 16 chars) e GABE_WHATS_MCP_TOKEN no .env

# Compila bin/gabe-whats-api e bin/gabe-whats-mcp
make bin

# Sobe a API REST (http://127.0.0.1:8080/docs)
./bin/gabe-whats-api

# Em outro terminal, sobe o MCP em Streamable HTTP (http://127.0.0.1:8090/mcp)
set -a && source .env && set +a
./bin/gabe-whats-mcp --transport http --http-addr 127.0.0.1:8090
```

Para parear o WhatsApp pela primeira vez:
- Via código de 8 dígitos: `POST /api/v1/session/pair-code`
- Via QR Code: `GET /api/v1/session/qr`

---

## 3. Deploy com Docker Compose & Dokploy

O repositório já inclui [`Dockerfile`](Dockerfile) multi-stage e [`docker-compose.yml`](docker-compose.yml) com os dois serviços (`gabe-whats-api` + `gabe-whats-mcp` em modo Streamable HTTP) e volume persistente `./data:/app/data`.

### Subindo localmente via Docker Compose

```bash
cp .env.example .env
# Edite .env com GABE_WHATS_API_KEY e GABE_WHATS_MCP_TOKEN fortes
docker compose up -d --build
```

### Deploy no Dokploy

1. No painel do **Dokploy**, crie um novo serviço do tipo **Compose** apontando para este repositório Git (`branch: main`, `Compose Path: ./docker-compose.yml`).
2. Na aba **Environment**, defina pelo menos:
   - `GABE_WHATS_API_KEY` — chave forte (mín. 16 caracteres; gera com `openssl rand -hex 32`)
   - `GABE_WHATS_MCP_TOKEN` — token Bearer exclusivo para o conector MCP (se omitido, reutiliza `GABE_WHATS_API_KEY`)
   - *(Opcional)* `GABE_WHATS_WEBHOOK_URL`, `GABE_WHATS_WEBHOOK_SECRET`, `GABE_WHATS_WEBHOOK_ALLOW_JIDS`
3. Na aba **Domains**, adicione o domínio HTTPS apontando para o servidor MCP:
   - **Service Name:** `gabe-whats-mcp`
   - **Container Port:** `8090`
   - **HTTPS:** ativado (Let's Encrypt)
   - *(Opcional)* Para acessar `/docs` ou parear via REST externamente, adicione um domínio separado para `gabe-whats-api` na porta `8080` com HTTPS (ou faça o pareamento inicial via `docker exec` / túnel SSH).
4. Faça o **Deploy** e pareie sua sessão WhatsApp.

### Conectando no `agentes.chat` (Conector Personalizado)

1. No `agentes.chat`, acesse **Conectores › Personalizado**.
2. Preencha:
   - **URL:** `https://<seu-subdominio>/mcp`
   - **Transporte:** Streamable HTTP
   - **Header:** `Authorization: Bearer <seu-GABE_WHATS_MCP_TOKEN>`
3. Salve e selecione quais agentes têm acesso a cada uma das 22 ferramentas do WhatsApp.

---

## 4. Documentação e Referências

- [Guia para Agentes (`AGENTS.md`)](AGENTS.md)
- [Segurança (`SECURITY.md`)](SECURITY.md)
- [Contribuição (`CONTRIBUTING.md`)](CONTRIBUTING.md)
- [01 — Pesquisa Técnica do `tulir/whatsmeow`](docs/01-WHATSMEOW-RESEARCH.md)
- [02 — Arquitetura Geral & Diagramas Mermaid](docs/02-ARCHITECTURE.md)
- [03 — Especificação REST & Documentação Automática (`/docs`)](docs/03-API-SPEC-SWAGGER.md)
- [04 — Servidor MCP & Integração com Agentes](docs/04-MCP-AND-AGENTS.md)
- [Contrato OpenAPI 3.1 (`docs/openapi.yaml`)](docs/openapi.yaml)
- [Contratos Compartilhados (`shared/contracts/`)](shared/contracts/README.md)
- [API REST (`apps/api/`)](apps/api/README.md)
- [Servidor MCP (`apps/mcp/`)](apps/mcp/README.md)
- [Variáveis de Ambiente (`.env.example`)](.env.example)

---

## 5. Licença e Termos Legais (AGPL-3.0)

Copyright © 2026 **Gabriel Sampaio** ([@gabedsam01](https://github.com/gabedsam01) · [sampaiodev.com.br](https://sampaiodev.com.br)).

Este projeto é software livre licenciado sob a **GNU Affero General Public License v3.0 (AGPL-3.0)** — leia os termos completos em [`LICENSE`](LICENSE).

- **Uso comercial permitido com copyleft forte:** você pode usar este projeto inclusive comercialmente, **desde que** todo o código-fonte (original e qualquer modificação, integração derivada ou disponibilização em rede/SaaS, conforme a **Seção 13 da AGPL-3.0**) seja mantido **100% gratuito, acessível e distribuído em código aberto sob a mesma licença AGPL-3.0**, preservando integralmente os avisos de direitos autorais e créditos de autoria.
- **Tolerância zero contra fechamento ou apropriação indevida:** é expressamente proibido fechar o código, comercializar versões modificadas de forma proprietária, distribuir binários/serviços em rede sem disponibilizar o código-fonte completo correspondente ou remover os avisos de autoria. Conforme a **Seção 8 da AGPL-3.0**, qualquer violação destas condições **rescinde automaticamente seus direitos de licença** e sujeita o infrator à reivindicação formal de direitos autorais, exigência de remoção/adequação imediata e às **medidas judiciais cabíveis** (cíveis e penais).
