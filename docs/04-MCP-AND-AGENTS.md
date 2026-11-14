# Servidor MCP & Integração com Agentes (OpenClaw, Moltbot, Hermes, Agent Zero)

Este documento define o desenho do **`apps/mcp`**: um servidor **Model Context Protocol (MCP)** independente que traduz ações de agentes de IA para chamadas REST na `apps/api`, além do contrato de **Webhook / SSE** para que o agente receba mensagens do WhatsApp em tempo real.

---

## 1. Como o Agente Conecta nas Duas Pontas

Todo agente autônomo precisa de **duas vias** para operar o WhatsApp:
1. **Via Ativa (Agente $\rightarrow$ WhatsApp): MCP (`apps/mcp`)**
   - Você registra o binário do `gabe-whats-mcp` na configuração de servidores MCP do **OpenClaw / Moltbot / Hermes Agent / Claude / Cursor**.
   - Assim que o MCP sobe (`stdio` ou `SSE`), o agente passa a enxergar todas as **Tools** e **Resources** do seu WhatsApp pessoal.
2. **Via Reativa (WhatsApp $\rightarrow$ Agente): Webhook HTTP POST ou SSE (`apps/api`)**
   - Como o protocolo MCP é orientado a chamadas iniciadas pelo cliente/modelo, o recebimento instantâneo de mensagens vindas de contatos/grupos é entregue pela `apps/api` via **Webhook HTTP POST** para o endpoint do seu agente (ou lido em stream contínuo via **SSE `/api/v1/events/stream`**).

---

## 2. Catálogo de Tools MCP (`apps/mcp`)

Todas as tools possuem prefixo `whatsapp_` e retornam JSON estruturado (incluindo `message_id` ou mensagens de erro acionáveis para o LLM corrigir parâmetros).

### 2.1 Sessão & Contatos
| Tool MCP | Parâmetros Principais | Endpoint REST Chamado |
| :--- | :--- | :--- |
| `whatsapp_session_status` | *(nenhum)* | `GET /api/v1/session/status` |
| `whatsapp_get_qr` | *(nenhum)* | `GET /api/v1/session/qr` |
| `whatsapp_pair_phone` | `phone` | `POST /api/v1/session/pair-code` |
| `whatsapp_check_numbers` | `phones[]` | `POST /api/v1/contacts/check` |

### 2.2 Mensagens & Interatividade
| Tool MCP | Parâmetros Principais | Endpoint REST Chamado |
| :--- | :--- | :--- |
| `whatsapp_send_text` | `to`, `text`, `reply_to_id?`, `mentions?[]`, `simulate_typing_ms?` | `POST /api/v1/messages/text` |
| `whatsapp_send_media` | `to`, `media_type` (`image`\|`video`\|`document`), `source` (URL/path/base64), `caption?`, `filename?` | `POST /api/v1/messages/media` |
| `whatsapp_send_voice_ptt` | `to`, `source` (URL/path/base64 OGG/Opus) | `POST /api/v1/messages/voice` |
| `whatsapp_send_buttons` | `to`, `body`, `header?`, `footer?`, `buttons[]` (`type`, `label`, `id`, `value?`) | `POST /api/v1/messages/buttons` |
| `whatsapp_send_list` | `to`, `title`, `description`, `button_text`, `sections[]` | `POST /api/v1/messages/list` |
| `whatsapp_send_poll` | `to`, `question`, `options[]`, `max_selections?` | `POST /api/v1/messages/poll` |
| `whatsapp_react_message` | `chat_jid`, `message_id`, `sender_jid?`, `emoji` | `POST /api/v1/messages/reaction` |
| `whatsapp_edit_message` | `chat_jid`, `message_id`, `new_text` | `PUT /api/v1/messages/{id}` |
| `whatsapp_revoke_message` | `chat_jid`, `message_id` | `DELETE /api/v1/messages/{id}` |
| `whatsapp_mark_read` | `chat_jid`, `sender_jid?`, `message_ids[]` | `POST /api/v1/messages/read` |

### 2.3 Gestão de Grupos
| Tool MCP | Parâmetros Principais | Endpoint REST Chamado |
| :--- | :--- | :--- |
| `whatsapp_list_groups` | *(nenhum)* | `GET /api/v1/groups` |
| `whatsapp_get_group` | `group_jid` | `GET /api/v1/groups/{jid}` |
| `whatsapp_create_group` | `name` (max 25 chars), `participants[]` | `POST /api/v1/groups` |
| `whatsapp_update_group_participants` | `group_jid`, `action` (`add`\|`remove`\|`promote`\|`demote`), `participants[]` | `POST /api/v1/groups/{jid}/participants` |
| `whatsapp_update_group_settings` | `group_jid`, `name?`, `description?`, `announce?`, `locked?` | `PATCH /api/v1/groups/{jid}` |
| `whatsapp_get_group_invite_link` | `group_jid`, `reset?` | `GET /api/v1/groups/{jid}/invite-link` |
| `whatsapp_join_group` | `invite_code_or_url` | `POST /api/v1/groups/join` |
| `whatsapp_leave_group` | `group_jid` | `POST /api/v1/groups/{jid}/leave` |

---

## 3. Catálogo de Resources MCP (Leitura de Contexto)

| URI do Resource | Conteúdo Retornado |
| :--- | :--- |
| `whatsapp://session/status` | Estado atual da conexão, número conectado e uptime. |
| `whatsapp://groups/list` | Lista JSON resumida de todos os grupos (`jid`, `name`, `participant_count`, `is_announce`). |
| `whatsapp://messages/recent` | Buffer em memória das últimas mensagens recebidas (útil caso o agente consulte histórico recente via MCP sem depender só de webhook). |

---

## 4. Payload Padronizado de Webhook para Agentes

Quando uma mensagem ou evento chega no WhatsApp, a `apps/api` normaliza o evento Protobuf do `whatsmeow` em um JSON limpo e envia `POST <WEBHOOK_URL>` com assinatura `X-GabeWhats-Signature: sha256=<hmac>`:

```json
{
  "event_id": "evt_01J...",
  "event_type": "message.received",
  "timestamp": "2026-11-14T01:05:00-03:00",
  "chat_jid": "5511999999999@s.whatsapp.net",
  "sender_jid": "5511999999999@s.whatsapp.net",
  "sender_name": "Contato",
  "is_group": false,
  "is_from_me": false,
  "message_id": "3EB0A1B2C3D4...",
  "message_type": "text",
  "text": "Oi! Pode me mandar o resumo de hoje?",
  "reply_to_id": null,
  "button_reply": null,
  "poll_vote": null,
  "media": null
}
```

### Exemplos de Payloads Especiais para Agentes
- **Clique em Botão (`message_type: "button_reply"`):**
  ```json
  "button_reply": {
    "selected_id": "confirmar_agenda",
    "selected_text": "Confirmar"
  }
  ```
- **Voto em Enquete (`message_type: "poll_vote"`):** *(descriptografado via `cli.DecryptPollVote`)*
  ```json
  "poll_vote": {
    "poll_message_id": "3EB0...",
    "selected_options": ["Opção 1 - Sim"]
  }
  ```
- **Áudio / Imagem (`message_type: "audio"` ou `"image"`):** *(baixado via `cli.DownloadAny`)*
  ```json
  "media": {
    "mime_type": "audio/ogg; codecs=opus",
    "is_ptt": true,
    "file_size": 18420,
    "local_path": "./data/media/3EB0A1B2.ogg",
    "base64": "T2dnUwAC..."
  }
  ```

---

## 5. Exemplo de Configuração no OpenClaw / Hermes / Clientes MCP

Veja o modelo pronto em [`mcp-config.example.json`](file:///home/gabedsam01/meus_projetos/gabe-whats/apps/mcp/mcp-config.example.json):

```json
{
  "mcpServers": {
    "gabe-whats": {
      "command": "/usr/local/bin/gabe-whats-mcp",
      "args": ["--transport", "stdio"],
      "env": {
        "GABE_WHATS_API_URL": "http://127.0.0.1:8080",
        "GABE_WHATS_API_KEY": "troque-por-sua-chave-secreta"
      }
    }
  }
}
```

### Guardrails Recomendados para Uso Pessoal com Agentes
- **`WEBHOOK_ALLOW_JIDS` / `WEBHOOK_IGNORE_FROM_ME`:** Evita que o agente responda a desconhecidos ou entre em loop infinito respondendo às próprias mensagens.
- **Modo "Self-Chat" opcional:** Permite conversar com o seu agente enviando mensagens no chat com o seu próprio número ou em um grupo privado dedicado.
