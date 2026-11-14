# Especificação da API REST & Documentação Automática (`/docs`)

> Sobre o que você mencionou ("aquela biblioteca que aparece toda a documentação e protocolo automaticamente, estilo Feathers/FastAPI"): no ecossistema REST isso se chama **OpenAPI 3.1 + Swagger UI / Scalar UI**.

---

## 1. Como funciona a Documentação da API

A API usa **Huma v2 (`github.com/danielgtaylor/huma/v2`) + Chi (`go-chi/chi/v5`)** para roteamento, binding e validação de entrada. A documentação segue uma regra explícita para não haver drift:

1. **A especificação publicada é curada e versionada** em [`openapi.yaml`](openapi.yaml) — é ela que responde em `GET /openapi.json` e `GET /openapi.yaml`, e é ela que o `/docs` renderiza.
2. **O Huma valida em runtime** usando as structs de `shared/contracts`, que carregam `doc:"..."`, `required:"true"`, `enum:"..."`, `maxItems:"..."` etc.
3. **Testes travam a paridade**: `apps/api/internal/http/openapi_test.go` compara a especificação com o mapa de rotas e `shared/contracts/contracts_test.go` exige `doc` em todo campo exportado. Se o código e a especificação divergirem, o CI quebra.

Por que não gerar a especificação automaticamente: as descrições em português, os exemplos e as restrições negociadas nas fases de contrato seriam substituídas pelo texto mínimo derivado das tags. Mantendo o arquivo curado como fonte publicada e um teste de paridade, ganhamos documentação rica **e** garantia de sincronia.

- `GET /docs` — interface interativa (Scalar UI) apontando para `/openapi.json`;
- `GET /openapi.json` e `GET /openapi.yaml` — a especificação OpenAPI 3.1 publicada;
- `GET /health` — healthcheck, sem autenticação.

> Se `GABE_WHATS_OPENAPI_PATH` não estiver legível no boot, a API serve a especificação gerada pelo Huma como fallback e registra um aviso.

---

## 2. Mapa Completo de Endpoints REST (`/api/v1`)

Todos os endpoints (exceto `/health`, `/docs`, `/openapi.json` e `/openapi.yaml`) exigem o header `X-API-Key: <API_KEY>` (ou `Authorization: Bearer <API_KEY>`).

### 2.1 Sessão & Pareamento (`/api/v1/session`)
| Método | Rota | Descrição | Método `whatsmeow` subjacente |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/session/status` | Retorna estado (`connected`, `logged_in`, `jid`, `push_name`, `platform`). | `IsConnected()`, `IsLoggedIn()`, `Store.ID` |
| `GET` | `/api/v1/session/qr` | Retorna o QR Code atual (string raw + PNG base64) para pareamento. | `GetQRChannel(ctx)` |
| `POST` | `/api/v1/session/pair-code` | Gera código numérico/alfanumérico de 8 caracteres para vincular pelo número. | `PairPhone(ctx, phone, true, PairClientChrome, ...)` |
| `POST` | `/api/v1/session/reconnect` | Força reconexão do WebSocket sem deslogar. | `Disconnect()` + `Connect()` |
| `POST` | `/api/v1/session/logout` | Encerra a sessão vinculada no WhatsApp e limpa o dispositivo. | `Logout(ctx)` |

### 2.2 Mensagens & Interatividade (`/api/v1/messages`)
| Método | Rota | Descrição | Detalhe `whatsmeow` |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/messages/text` | Envia texto simples, link preview, citação (`reply_to_id`) e menções (`mentions`). | `SendMessage` (`Conversation` / `ExtendedTextMessage`) |
| `POST` | `/api/v1/messages/media` | Envia imagem, vídeo ou documento (URL, Base64 ou path local) com legenda. | `Upload` + `SendMessage` (`ImageMessage`/`VideoMessage`/`DocumentMessage`) |
| `POST` | `/api/v1/messages/voice` | Envia áudio como nota de voz gravada (`PTT: true`, formato OGG/Opus). | `Upload(..., MediaAudio)` + `AudioMessage{PTT: true}` |
| `POST` | `/api/v1/messages/buttons` | Envia botões interativos (`quick_reply`, `cta_url`, `cta_copy`). | `InteractiveMessage` + `NativeFlowMessage` + nó `<biz>` |
| `POST` | `/api/v1/messages/list` | Envia menu de lista expansível com seções e linhas selecionáveis. | `ListMessage` + nó `<biz>` |
| `POST` | `/api/v1/messages/poll` | Envia enquete interativa nativa (100% compatível com WhatsApp pessoal). | `BuildPollCreation` + `SendMessage` |
| `POST` | `/api/v1/messages/reaction` | Reage com emoji (ou `""` para remover reação) a uma mensagem. | `BuildReaction` + `SendMessage` |
| `PUT` | `/api/v1/messages/{id}` | Edita o texto de uma mensagem já enviada. | `BuildEdit` + `SendMessage` |
| `DELETE` | `/api/v1/messages/{id}` | Apaga/revoga uma mensagem para todos no chat (corpo `RevokeMessageRequest`). | `RevokeMessage` |
| `POST` | `/api/v1/messages/read` | Marca mensagens recebidas como lidas (tique azul). | `MarkRead` |
| `POST` | `/api/v1/messages/presence` | Envia estado `typing` (digitando), `recording` (gravando áudio) ou `paused`. | `SendChatPresence` |
| `GET` | `/api/v1/messages/recent` | Lista as mensagens mais recentes. Query: `limit` (padrão `50`, máx. `200`) e `chat_jid` (opcional, filtra um chat). | Histórico local normalizado a partir dos eventos `*events.Message` |

### 2.3 Gestão de Grupos (`/api/v1/groups`)
| Método | Rota | Descrição | Método `whatsmeow` |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/groups` | Lista todos os grupos em que o número participa. | `GetJoinedGroups(ctx)` |
| `POST` | `/api/v1/groups` | Cria um novo grupo (nome até 25 chars + lista de participantes). | `CreateGroup(ctx, ReqCreateGroup)` |
| `GET` | `/api/v1/groups/{jid}` | Retorna metadados, tópico/descrição, configurações e participantes do grupo. | `GetGroupInfo(ctx, jid)` |
| `PATCH` | `/api/v1/groups/{jid}` | Corrige/edita nome, descrição, modo anúncio (`announce`) ou trava (`locked`). | `SetGroupName`, `SetGroupDescription`, `SetGroupAnnounce`, `SetGroupLocked` |
| `PUT` | `/api/v1/groups/{jid}/photo` | Atualiza a foto de perfil do grupo via `multipart/form-data`. | `SetGroupPhoto(ctx, jid, bytes)` |
| `POST` | `/api/v1/groups/{jid}/participants` | Adiciona, remove, promove a admin ou rebaixa participantes (`add`, `remove`, `promote`, `demote`). | `UpdateGroupParticipants` |
| `GET` | `/api/v1/groups/{jid}/invite-link` | Obtém (ou reseta via `?reset=true`) o link de convite do grupo. | `GetGroupInviteLink` |
| `POST` | `/api/v1/groups/join` | Entra em um grupo usando código ou URL de convite. | `JoinGroupWithLink` |
| `POST` | `/api/v1/groups/{jid}/leave` | Sai de um grupo. | `LeaveGroup` |

### 2.4 Contatos, Webhooks & Stream de Eventos (`/api/v1/contacts`, `/api/v1/webhooks`, `/api/v1/events`)
| Método | Rota | Descrição |
| :--- | :--- | :--- |
| `POST` | `/api/v1/contacts/check` | Verifica se telefones possuem WhatsApp e retorna seus JIDs (`IsOnWhatsApp`). |
| `GET` | `/api/v1/webhooks` | Lista destinos de webhook configurados e filtros ativos. |
| `PUT` | `/api/v1/webhooks` | Configura/atualiza URL de webhook, secret HMAC, allowlist e auto-download de mídia. |
| `GET` | `/api/v1/events/stream` | Stream **SSE (Server-Sent Events)** em tempo real para agentes locais sem precisar expor porta HTTP de webhook. |

### 2.5 Mídia (`/api/v1/media`)
| Método | Rota | Descrição |
| :--- | :--- | :--- |
| `GET` | `/api/v1/media/{id}` | Baixa um arquivo de mídia exposto em alguma resposta da API. As URLs são **temporárias** (`download_url` + `expires_at`, TTL padrão 15 min — ver seção 7). |

### 2.6 Documentação Automática
| Método | Rota | Descrição |
| :--- | :--- | :--- |
| `GET` | `/docs` | Interface interativa (Scalar UI, com opção de Swagger UI) para explorar e testar as rotas. |
| `GET` | `/openapi.json` | Especificação **OpenAPI 3.1** gerada automaticamente (JSON). |
| `GET` | `/openapi.yaml` | Especificação **OpenAPI 3.1** gerada automaticamente (YAML). |

> Rotas de documentação (`/docs`, `/openapi.json`, `/openapi.yaml`) e `/health` **não exigem autenticação**; todo o restante de `/api/v1/*` exige `X-API-Key` ou `Authorization: Bearer`.

---

## 3. Especificação OpenAPI Completa

O contrato completo já desenhado em OpenAPI 3.1 encontra-se em [`openapi.yaml`](file:///home/gabedsam01/meus_projetos/gabe-whats/docs/openapi.yaml).

---

## 4. Formato de Erro — RFC 9457 (`application/problem+json`)

Toda resposta de erro da API usa **RFC 9457 (Problem Details)** com `Content-Type: application/problem+json`, com os campos:

- `type` — URI identificando o tipo do problema;
- `title` — resumo curto do problema;
- `status` — código HTTP numérico;
- `detail` — descrição legível do que falhou (ex.: limite violado);
- `instance` — rota/recursos onde o problema ocorreu.

Exemplo ilustrativo de uma violação de limite (decisão da seção 5):

```json
{
  "type": "https://gabe-whats.local/errors/whatsapp-limit",
  "title": "Limite do WhatsApp violado",
  "status": 400,
  "detail": "enquete deve ter entre 2 e 12 opções",
  "instance": "/api/v1/messages/poll"
}
```

As respostas de **sucesso não usam envelope**: retornam o array ou objeto direto (nada de `{ "data": ... }`).

## 5. Limites do WhatsApp (validação explícita → `4xx`)

Os limites impostos pelo próprio WhatsApp viram **validação explícita** da API: uma requisição que os viole é rejeitada com erro `4xx` (`application/problem+json`, seção 4) **antes** de qualquer envio ao WhatsApp.

| Recurso | Limite |
| :--- | :--- |
| Botões (`POST /api/v1/messages/buttons`) | no máximo **3** botões |
| Enquete (`POST /api/v1/messages/poll`) | de **2 a 12** opções |
| Lista (`POST /api/v1/messages/list`) | no máximo **10** seções e **10** linhas por seção |
| Nome de grupo (`POST /api/v1/groups`, `PATCH /api/v1/groups/{jid}`) | no máximo **25** caracteres |

## 6. Normalização de JID (`to` / `chat_jid`)

Os campos `to` e `chat_jid` aceitam qualquer uma das formas abaixo, e a API normaliza para a forma canônica do WhatsApp antes de operar:

- número com **DDI**, apenas dígitos (ex.: `5511999999999`);
- JID completo de usuário: `5511999999999@s.whatsapp.net`;
- JID de grupo: `...@g.us`;
- JID LID: `...@lid`.

Nas respostas, `to` / `chat_jid` aparecem sempre **normalizados** (número com DDI + sufixo do servidor correspondente).

## 7. URL Temporária de Mídia (`download_url` + `expires_at`)

Toda mídia exposta em alguma resposta da API (foto de grupo, mídia de mensagens etc.) inclui:

- `download_url` — URL temporária do arquivo;
- `expires_at` — instante em que a URL deixa de valer.

O conteúdo é servido por `GET /api/v1/media/{id}` (seção 2.5) com **TTL padrão de 15 minutos**, configurável por `GABE_WHATS_MEDIA_URL_TTL` (ver [`.env.example`](file:///home/gabedsam01/meus_projetos/gabe-whats/.env.example)).

Fluxo esperado do agente: receber a resposta → baixar o arquivo via `download_url` → guardar a mídia no próprio contexto/armazenamento antes do vencimento. Após `expires_at`, a URL expira e a requisição recebe erro `4xx` (`application/problem+json`); o agente deve então obter a mídia novamente para receber uma URL nova.
