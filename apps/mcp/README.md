# `apps/mcp` — Servidor MCP (Model Context Protocol)

Servidor **stateless** que expõe o `gabe-whats` como ferramentas MCP para agentes
(OpenClaw, Moltbot, Hermes Agent, Agent Zero e qualquer cliente compatível).

> **Regra de arquitetura:** este módulo **nunca** importa `go.mau.fi/whatsmeow` nem abre o
> `session.db`. Toda operação é uma chamada HTTP à API REST (`apps/api`) usando
> `GABE_WHATS_API_URL` e `GABE_WHATS_API_KEY`.

## Como rodar

```bash
go run ./apps/mcp/cmd/mcp-server --transport stdio
```

Transportes:

| Flag | Uso |
| :--- | :--- |
| `--transport stdio` | Padrão. O cliente MCP sobe o processo e conversa por stdin/stdout. |
| `--transport http` | Sobe um servidor **Streamable HTTP** (`/mcp`, com `/health` público) em `--http-addr` (fallback `--sse-addr`, padrão `127.0.0.1:8090`) exigindo `Authorization: Bearer <token>`. |
| `--transport sse` | Sobe um servidor HTTP SSE legado em `--sse-addr` (padrão `127.0.0.1:8090`) também protegido por Bearer token. |

Configuração por ambiente (flags da CLI têm precedência sobre o ambiente):

- `GABE_WHATS_API_URL` — URL da API (ex.: `http://127.0.0.1:8080`);
- `GABE_WHATS_API_KEY` — **obrigatória**; a mesma chave do `.env` da API;
- `GABE_WHATS_MCP_TOKEN` — token exigido em `Authorization: Bearer <token>` no modo `http` ou `sse` (quando vazio, usa `GABE_WHATS_API_KEY`);
- `GABE_WHATS_MCP_TRANSPORT` / `GABE_WHATS_MCP_ADDR` — padrões opcionais para `--transport` e endereço HTTP.

Exemplo de configuração de cliente em [`mcp-config.example.json`](mcp-config.example.json).

## Tools (22)

| Área | Tools |
| :--- | :--- |
| Sessão | `whatsapp_session_status`, `whatsapp_get_qr`, `whatsapp_pair_phone` |
| Contatos | `whatsapp_check_numbers` |
| Envio | `whatsapp_send_text`, `whatsapp_send_media`, `whatsapp_send_voice_ptt`, `whatsapp_send_poll`, `whatsapp_send_buttons`, `whatsapp_send_list` |
| Ações | `whatsapp_react_message`, `whatsapp_edit_message`, `whatsapp_revoke_message`, `whatsapp_mark_read` |
| Grupos | `whatsapp_list_groups`, `whatsapp_get_group`, `whatsapp_create_group`, `whatsapp_update_group_participants`, `whatsapp_update_group_settings`, `whatsapp_get_group_invite_link`, `whatsapp_join_group`, `whatsapp_leave_group` |

> **Botões e listas:** continuam expostos, mas o servidor do WhatsApp recusa esse tipo de
> mensagem no canal Web/companion (erros 473/479), inclusive em conta Business. Para
> interatividade garantida, use `whatsapp_send_poll`.

## Resources (3)

| URI | Conteúdo |
| :--- | :--- |
| `whatsapp://session/status` | Estado da conexão (conectado, autenticado, JID, uptime) |
| `whatsapp://groups/list` | Grupos com metadados e participantes |
| `whatsapp://messages/recent` | Últimas 50 mensagens recebidas |

## Camadas

| Arquivo | Responsabilidade |
| :--- | :--- |
| [`cmd/mcp-server/main.go`](cmd/mcp-server/main.go) | Flags, transporte (`stdio`/`http`/`sse`), auth Bearer e ciclo de vida |
| [`internal/apiclient/client.go`](internal/apiclient/client.go) | Cliente HTTP tipado nos contratos de `shared/contracts` |
| [`internal/tools/tools.go`](internal/tools/tools.go) | Registro das 22 tools |
| [`internal/resources/resources.go`](internal/resources/resources.go) | Registro dos 3 resources de leitura |
