# Review S1 — Auditoria adversarial dos contratos (shared/contracts + docs/openapi.yaml)

- **Revisor:** agente auditor em par com o autor dos contratos.
- **Data:** 2026-11-14
- **Escopo lido:** shared/contracts/{messages,groups,session,contacts,common,webhooks}.go, shared/contracts/README.md, apps/api/internal/http/router.go, apps/api/internal/whatsapp/blueprint.go, docs/openapi.yaml (completo), AGENTS.md, docs/01-WHATSMEOW-RESEARCH.md, README.md.
- **Restrições respeitadas:** nenhum arquivo de contrato ou da spec foi editado; único arquivo escrito é este review. Sem código de runtime, sem git, sem tocar em go.mod.
- **Severidades:** blocker = impede aprovar S1; major = corrigir antes de S2; minor = consistência/polimento.

---

## 1. Validação dos achados do agente pai

| ID pai | Achado | Veredito | Evidência |
|---|---|---|---|
| A | Falta UpdateGroupParticipantsResponse | **CONFIRMADO** | router.go:55 expõe POST /groups/{jid}/participants; openapi.yaml:989 responde GroupInfoResponse, que só retrata o estado final do grupo. O resultado do whatsmeow é por participante, com erro próprio (403 proibido / 409 conflito — semântica do IQ de participação, cf. docs/01-WHATSMEOW-RESEARCH.md:66). blueprint.go:45 agrava: UpdateGroupParticipants(...) error já descarta o resultado por participante na assinatura. Sem DTO próprio, o agente não descobre **qual JID falhou e por quê**. |
| B | Phones/Participants sem maxItems | **CONFIRMADO (expandido)** | contacts.go:5 e groups.go:8 têm minItems:"1" e nenhum maxItems. O mesmo vale em groups.go:28 (UpdateGroupParticipantsRequest.Participants) e messages.go:127 (MarkReadRequest.MessageIDs). |
| C | README não lista common.go, session.go, contacts.go | **CONFIRMADO** | shared/contracts/README.md:7-10 lista apenas messages.go, groups.go e webhooks.go. |
| D | ReactionRequest.Emoji sem required; semântica de "" não explícita | **CONFIRMADO EM PARTE / REFUTADO EM PARTE** | Confirmado: messages.go:108 não tem required:"true", mas openapi.yaml:1666 exige emoji — divergência real Go↔spec. **Refutado:** a semântica "string vazia remove a reação" **está** explícita nos dois lados (messages.go:108 doc: "...ou string vazia para remover a reação"; openapi.yaml:1679). O problema é exclusivamente a divergência de required. |
| E | AckResponse não existe, mas a spec referencia | **CONFIRMADO** | Referenciada em openapi.yaml:645 (markRead), 679 (presence) e 1089 (leave); schema definido só na spec (2076-2090, que admite "DTO ainda não existente em shared/contracts"). Grep em shared/contracts: zero ocorrências. |

---

## 2. Tabela de achados

| ID | Severidade | Arquivo:linha | Problema | Correção sugerida |
|---|---|---|---|---|
| R1 | blocker | webhooks.go:39 vs openapi.yaml:2142,2147 | Nome de campo divergente no voto de enquete: o Go serializa question; a spec declara poll_question (e o exige em required). Consumidor gerado de um lado quebra do outro — é o drift que a "fonte única" (AGENTS.md §2) existe para impedir. | Escolher um nome e aplicar nos dois lados. Sugestão: poll_question no Go (mantém a required list da spec intacta); alternativa simétrica válida é mudar a spec para question. |
| R2 | blocker | webhooks.go:41-42 vs openapi.yaml:2136-2154 | A spec de WebhookPollVote omite selected_hashes e voter_jid, presentes no Go. Sem voter_jid, quem lê só a spec não sabe quem votou — a decisão do dono 6 fica incompleta na documentação. | Adicionar selected_hashes (array de string) e voter_jid (string) à spec, com required alinhado ao Go. |
| R3 | blocker | webhooks.go:51-52 vs openapi.yaml:2156-2180 | A spec de WebhookMediaPayload omite download_url e expires_at — exatamente a exposição de mídia com URL temporária TTL 15 min (decisão do dono 4) no caminho do webhook/SSE. | Adicionar download_url (string) e expires_at (date-time) à spec, com descrição do TTL de 15 min. |
| R4 | blocker | webhooks.go:65-67,73-74 vs openapi.yaml:2182-2235 | A spec de WebhookInboundEvent omite 5 campos do Go: platform, server_id, verified_name, is_edit, edited_text. A spec publicada mente sobre o payload real do webhook. | Adicionar os 5 campos à spec (strings; is_edit boolean). |
| R5 | major | openapi.yaml:645,679,1089 + ausência em shared/contracts | AckResponse referenciada por 3 rotas (read, presence, leave) mas inexistente no pacote de contratos (achado E). Quebra a regra da fonte única e trava apps/mcp para essas operações. | Criar AckResponse em shared/contracts/common.go: ok bool (required:"true") + detail string (omitempty); a spec já está coerente com isso. |
| R6 | major | blueprint.go:45 + openapi.yaml:977-989 | Resposta de participantes expõe só GroupInfoResponse; o resultado por participante do whatsmeow (403/409 por JID) é descartado — nem o blueprint tem onde retorná-lo (achado A). Violação de limite de lista (decisão 9) é 4xx de request; erro por participante é **resultado parcial** e precisa de DTO próprio. | Criar UpdateGroupParticipantsResponse{Action, Results []GroupParticipantResult{JID, Error int, Message string}, Group *GroupInfoResponse} em groups.go e apontar a spec para ele; ajustar o blueprint para retorná-lo. |
| R7 | major | messages.go:100 vs openapi.yaml:1656-1661 | max_selections: omitir o campo produz zero-value 0, que a própria doc define como "múltipla escolha" (protocolo: selectableOptionCount 0 = múltipla, docs/01-WHATSMEOW-RESEARCH.md:57). A spec declara default: 1. Ou seja: **omitir = múltipla escolha**, contradizendo o default documentado. | Normalizar na validação (0 → 1) e corrigir a doc para "ausente/0 = escolha única (padrão); N = máximo de seleções", ou trocar para *int com nil = 1. Alinhar Go e spec. |
| R8 | major | múltiplos (ver correção) | Divergência sistêmica de required: structs de **resposta** não têm required:"true" no Go, mas a spec marca campos como required. Se a spec for gerada via Huma a partir das structs (padrão do projeto, AGENTS.md §3), todas essas listas required somem. | Adicionar required:"true" nos campos correspondentes: common.go:20-21,26-28 (ProblemField.Field/Message; HealthResponse.status/version/uptime_seconds); session.go:7-8,12,17,30-31 (SessionStatus, QRCodeResponse.event, PairCodeResponse); messages.go:10-13,151-158,164 (MessageResponse, MessageSummary, RecentMessagesResponse.messages); groups.go:41-44,49-54,65,70,82-85,90-92 (GroupParticipantInfo, GroupInfoResponse, JoinGroupResponse, GroupsResponse.groups, GroupPhotoResponse, InviteLinkResponse); contacts.go:10,15,17 (CheckNumbersResponse.results, NumberCheckResult); webhooks.go:32,38-42,47,50,59-63,68-69,111,113,115-116 (WebhookButtonReply, WebhookPollVote, WebhookMediaPayload, WebhookInboundEvent, WebhookConfig.url/ignore_from_me/auto_download_media/include_base64). |
| R9 | major | webhooks.go:82-107 vs openapi.yaml (ausente) | WebhookGroupChange, WebhookGroupEvent e WebhookSessionEvent não têm schema na spec. Pior: WebhookInboundEvent.event_type (webhooks.go:60) aceita group.updated/session.*, mas o DTO não tem campos para changes/reason. Ambiguidade de wire format: que forma um evento group.updated assume no webhook POST? | Decidir e documentar: ou schemas próprios na spec para os 3 DTOs, ou campos extras em WebhookInboundEvent — e remover o DTO órfão que ficar sem uso. |
| R10 | major | messages.go (ausente) vs openapi.yaml:701-716 | Query params de GET /messages/recent (limit default 50/máx 200; chat_jid — decisão do dono 5) existem só na spec; não há struct Go em shared/contracts para carregar query:"limit" default:"50" maximum:"200". A decisão 5 não tem lar no lado Go. | Criar RecentMessagesQuery em messages.go com as tags de validação equivalentes. |
| R11 | major | messages.go:108 vs openapi.yaml:1666 | ReactionRequest.Emoji: sem required no Go, required na spec (achado D). Como "" é valor válido (remover), exigir presença é aceitável — mas os dois lados precisam casar antes da geração Huma. | Adicionar required:"true" em messages.go:108; manter a doc que já explica "" = remover (correta nos dois lados). |
| R12 | minor | contacts.go:5; groups.go:8,28; messages.go:127 | Sem maxItems em Phones, Participants (create e participants) e MessageIDs (achado B, expandido). Arrays sem teto num gateway pessoal aceitam payloads arbitrariamente grandes; grupos têm limite de 1024 participantes no WhatsApp. | maxItems:"100" (phones), "1024" (participants) ou um cap pessoal (ex.: 100), "50" (message_ids). |
| R13 | minor | shared/contracts/README.md:7-10 | README do pacote não lista common.go, session.go nem contacts.go (achado C). | Adicionar os 3 arquivos à seção "## Arquivos" com uma linha descritiva cada. |
| R14 | minor | messages.go:13; session.go:17; common.go:26 | Valores de enum sem tipo nomeado/constantes: MessageResponse.Status (sent/edited/revoked/reacted), QRCodeResponse.Event (code/success/timeout — tem enum na tag mas é string crua), HealthResponse.Status (ok/degraded). O padrão do pacote (ButtonType, PresenceState, GroupParticipantAction têm constantes) não é seguido. | Criar tipos nomeados + constantes (MessageStatus, QRCodeEvent, HealthStatus) e usá-los nos campos. |
| R15 | minor | messages.go:19,22; session.go:25; groups.go:7; groups.go:32-37 | Validações pontuais ausentes: SendTextRequest.Text sem minLength (string vazia passa em ambos os lados); SimulateTypingMs sem maximum (a spec só tem minimum: 0, openapi.yaml:1481); PairCodeRequest.Phone sem pattern de dígitos em nenhum lado; CreateGroupRequest.Name sem minLength:"1" no Go (a spec tem, openapi.yaml:1850); UpdateGroupSettingsRequest sem minProperties: 1 (PATCH vazio é aceito). | Adicionar minLength/maximum/pattern/minProperties equivalentes nos dois lados. |
| R16 | minor | messages.go:156 vs webhooks.go:71 | Vocabulário de message_type divergente: MessageSummary lista poll/sticker; WebhookInboundEvent lista poll_vote e omite sticker. Consumidor que ramifica sobre message_type trocando histórico REST por webhook quebra. | Padronizar a lista de tipos nos dois doc tags (ou tipar como enum compartilhado). |
| R17 | minor | groups.go:76-78 vs openapi.yaml:1960-1965 | SetGroupPhotoRequest: a parte file (required, binary) existe só na spec; o DTO Go não a carrega (comentário em groups.go:73-75 assume isso). Na geração Huma a partir do DTO, o required de file se perde. | Ou incluir o campo no DTO multipart (form:"file"), ou garantir explicitamente que o gerador preserve a parte binária documentada. |
| R18 | minor | webhooks.go:49 | WebhookMediaPayload.IsPTT usa omitempty enquanto os demais booleanos do pacote (is_group, is_from_me, is_edit, ignore_from_me...) não usam. Serialização inconsistente de booleanos no mesmo payload. | Remover omitempty de is_ptt (e manter a spec tratando o campo como opcional, como hoje). |

---

## 3. Cobertura rota → DTO (35 rotas de router.go)

| Rota | Request | Response | Status |
|---|---|---|---|
| GET /health | — | HealthResponse | OK |
| GET /docs | — | HtmlDocument (só na spec) | Justificado (servido pelo framework) |
| GET /openapi.json | — | OpenAPISpec (só na spec) | Justificado |
| GET /openapi.yaml | — | OpenAPISpec (só na spec) | Justificado |
| GET /api/v1/session/status | — | SessionStatus | OK |
| GET /api/v1/session/qr | — | QRCodeResponse | OK |
| POST /api/v1/session/pair-code | PairCodeRequest | PairCodeResponse | OK |
| POST /api/v1/session/reconnect | — | SessionStatus | OK |
| POST /api/v1/session/logout | — | SessionStatus | OK |
| POST /api/v1/messages/text | SendTextRequest | MessageResponse | OK |
| POST /api/v1/messages/media | SendMediaRequest | MessageResponse | OK |
| POST /api/v1/messages/voice | SendVoiceRequest | MessageResponse | OK |
| POST /api/v1/messages/buttons | SendButtonsRequest | MessageResponse | OK |
| POST /api/v1/messages/list | SendListRequest | MessageResponse | OK |
| POST /api/v1/messages/poll | SendPollRequest | MessageResponse | OK |
| POST /api/v1/messages/reaction | ReactionRequest | MessageResponse | OK |
| PUT /api/v1/messages/{id} | EditMessageRequest | MessageResponse | OK |
| DELETE /api/v1/messages/{id} | RevokeMessageRequest | MessageResponse | OK (decisão 3) |
| POST /api/v1/messages/read | MarkReadRequest | AckResponse | **Sem DTO Go (R5)** |
| POST /api/v1/messages/presence | PresenceRequest | AckResponse | **Sem DTO Go (R5)** |
| GET /api/v1/messages/recent | — (query sem DTO Go) | RecentMessagesResponse | **Parcial (R10)** |
| GET /api/v1/media/{id} | — (path param) | binário (MediaBinary na spec; MediaRef cobre metadados) | OK |
| GET /api/v1/groups | — | GroupsResponse | OK |
| POST /api/v1/groups | CreateGroupRequest | GroupInfoResponse | OK |
| GET /api/v1/groups/{jid} | — | GroupInfoResponse | OK |
| PATCH /api/v1/groups/{jid} | UpdateGroupSettingsRequest | GroupInfoResponse | OK |
| PUT /api/v1/groups/{jid}/photo | SetGroupPhotoRequest (multipart; file só na spec) | GroupPhotoResponse | **Parcial (R17)** |
| POST /api/v1/groups/{jid}/participants | UpdateGroupParticipantsRequest | GroupInfoResponse | **Response inadequado (R6)** |
| GET /api/v1/groups/{jid}/invite-link | — (query reset sem DTO Go, openapi.yaml:1013-1019) | InviteLinkResponse | Parcial (menor) |
| POST /api/v1/groups/join | JoinGroupRequest | JoinGroupResponse | OK |
| POST /api/v1/groups/{jid}/leave | — | AckResponse | **Sem DTO Go (R5)** |
| POST /api/v1/contacts/check | CheckNumbersRequest | CheckNumbersResponse | OK |
| GET /api/v1/webhooks | — | WebhookConfig | OK |
| PUT /api/v1/webhooks | WebhookConfig | WebhookConfig | OK (decisão 7) |
| GET /api/v1/events/stream | — | WebhookInboundEvent (SSE) | OK (ver R9) |

**Resumo:** 35 rotas — 27 completas; 3 sem DTO Go no response (read, presence, leave); 4 parciais (recent e invite-link sem DTO de query; photo sem file no DTO; participants com response insuficiente). DTOs de contracts sem schema na spec: WebhookGroupChange, WebhookGroupEvent, WebhookSessionEvent (R9).

---

## 4. Verificações que passaram (sem achado)

- **Tags doc em pt-BR:** todos os campos exportados dos 6 arquivos de contrato têm doc:"..." em português — verificado campo a campo, nenhuma exceção.
- **snake_case:** todos os JSON tags estão em snake_case (uptime_seconds, download_url, max_selections, is_super_admin...). Nenhuma exceção.
- **Decisão 1 (sem envelope):** respostas são objetos diretos; GroupsResponse/RecentMessagesResponse são objetos nomeados de domínio, não envelope {"data":...}.
- **Decisão 2 (RFC 9457):** Problem/ProblemField (common.go:8-22) e todas as respostas de erro como application/problem+json (openapi.yaml:1259-1314).
- **Decisão 3 (DELETE com corpo):** router.go:41 + openapi.yaml:599-604 + RevokeMessageRequest (messages.go:117-121) consistentes.
- **Decisão 4 (foto multipart + download_url TTL 15 min):** openapi.yaml:941-947, groups.go:73-86, MediaRef (common.go:31-40), TTL citado em openapi.yaml:746 e 1309.
- **Decisão 5 (recent limit 50/200 + chat_jid):** correta na spec (openapi.yaml:701-716); falta apenas o DTO Go (R10).
- **Decisão 6 (voto com pergunta + opções em texto):** presente nos dois lados (webhooks.go:37-43; openapi.yaml:2139-2152, com menção ao cache SQLite) — mas com os drifts R1/R2.
- **Decisão 7 (PUT /webhooks persiste):** openapi.yaml:1162-1194 descreve persistência e não-read-only.
- **Decisão 8 (sem multi-sessão):** grep confirma zero session_id em shared/contracts; a spec só menciona a ausência (openapi.yaml:29).
- **Decisão 9 (limites → 4xx):** botões 1..3 (messages.go:69 / openapi.yaml:1568-1569); enquete 2..12 (messages.go:99 / 1652-1653); lista 10 seções × 10 linhas (messages.go:82,92 / 1601-1602,1631-1632); nome 25 chars (groups.go:7,33 / 1851,1868); violação documentada como 422 (openapi.yaml:1284-1298).
- **Enums de request:** ButtonItem.Type, SendMediaRequest.MediaType, PresenceRequest.State, UpdateGroupParticipantsRequest.Action têm tipo nomeado + constantes + enum na tag, espelhados na spec.
- **Ponteiro vs valor:** coerente no geral. Ponteiros corretos onde a ausência é semântica (UpdateGroupSettingsRequest todo ponteiro para PATCH; MarkReadRequest.Timestamp; QRCodeResponse.ExpiresAt). Nenhum campo obrigatório virou ponteiro por engano. Booleanos obrigatórios sem omitempty (presentes sempre) — exceto o ponto R18.

---

## 5. Risco de compatibilidade

Fase S1, sem runtime e sem consumidores: **hoje a correção é barata; depois de S2, cada item vira breaking change**. Prioridade de congelamento: (1) nomes de campo JSON (R1) — impossível migrar sem quebra; (2) required (R8, R11) — adicionar required depois rejeita payloads hoje aceitos; (3) formato dos eventos de grupo/sessão (R9) — define o contrato do webhook para sempre. Adições de campo (R2-R4) são retrocompatíveis, mas só se feitas antes de alguém validar contra a spec atual.

---

## 6. Veredito

O desenho dos DTOs é sólido e as 9 decisões do dono estão representadas nos dois lados; snake_case, tags doc pt-BR e enums de request estão corretos.
Porém a spec OpenAPI não está em sinc com shared/contracts: 4 blockers de divergência campo a campo (R1-R4), AckResponse e UpdateGroupParticipantsResponse ausentes (R5-R6), required sistemicamente ausente no Go (R8) e max_selections com default contraditório (R7).
**Veredito: APROVADO COM RESSALVAS** — aprovar o desenho; condicionar o fechamento de S1 à correção de R1-R11 (blockers + majors); minors (R12-R18) podem migrar para o backlog de S2.
