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
| `POST` | `/api/v1/events/ticket` | Emite um **ticket efêmero** (uso único, 60s) para abrir o stream SSE sem colocar a chave da API na URL. |
| `GET` | `/api/v1/events/stream` | Stream **SSE (Server-Sent Events)** em tempo real; aceita `X-API-Key` ou `?ticket=`. |

### 2.5 Mídia (`/api/v1/media`)
| Método | Rota | Descrição |
| :--- | :--- | :--- |
| `GET` | `/api/v1/media/{id}` | Baixa um arquivo de mídia exposto em alguma resposta da API. As URLs são **temporárias** (`download_url` + `expires_at`, TTL padrão 15 min — ver seção 7). |

### 2.6 Documentação Automática
| Método | Rota | Descrição |
| :--- | :--- | :--- |
| `GET` | `/docs` | Interface interativa (Scalar UI, com opção de Swagger UI) para explorar e testar as rotas. |
| `GET` | `/openapi.json` | Especificação **OpenAPI 3.1** publicada, em JSON (ver seção 1). |
| `GET` | `/openapi.yaml` | Especificação **OpenAPI 3.1** publicada, em YAML. |

> Rotas de documentação (`/docs`, `/openapi.json`, `/openapi.yaml`) e `/health` **não exigem autenticação**; todo o restante de `/api/v1/*` exige `X-API-Key` ou `Authorization: Bearer`. Duas exceções deliberadas: o stream SSE aceita `?ticket=` (seção 9) e `/api/v1/media/{id}` é uma **URL de capacidade** — o ID aleatório de 128 bits, com TTL, é o próprio segredo.

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
Após `expires_at` a API responde **410 Gone** com `code: media_expired`. O campo `media_id` do evento permite pedir a mídia novamente ao remetente.

## 8. Entrega de Webhook

Quando uma mensagem é recebida, a API publica `WebhookInboundEvent` via `POST` na URL configurada em `PUT /api/v1/webhooks` (ou em `GABE_WHATS_WEBHOOK_URL` quando o banco ainda não tem configuração).

**Cabeçalhos enviados:**

| Header | Conteúdo |
| :--- | :--- |
| `X-GabeWhats-Signature` | `sha256=<hex>` — HMAC-SHA256 do corpo cru, usando o segredo configurado |
| `X-GabeWhats-Delivery` | ID do evento, útil para deduplicação |
| `X-GabeWhats-Event` | `event_type` do evento |

**Política de reentrega:** a primeira requisição sai imediatamente. Se o destino não responder com sucesso (2xx) em **2 segundos**, a tentativa é considerada falha e as próximas são agendadas após **5s, 15s, 25s, 45s e 60s** — ou seja, 1 requisição inicial + 5 reentregas. Esgotadas as tentativas, a entrega é descartada e registrada em log; o histórico de `GET /api/v1/messages/recent` continua disponível como rede de segurança.

**Filtros:** `ignore_from_me=true` (padrão) descarta `message.received` enviadas pelo próprio número — necessário para evitar loop de auto-resposta. `allow_jids`, quando preenchido, restringe `message.received` aos chats ou remetentes listados.

## 9. Stream SSE e Ticket Efêmero

O endpoint `GET /api/v1/events/stream` entrega os mesmos eventos do webhook em tempo real, com `Content-Type: text/event-stream` e o envelope JSON no `data`.

Como a API `EventSource` do navegador não permite enviar headers, a chave real nunca vai na URL. O fluxo é:

1. `POST /api/v1/events/ticket` com `X-API-Key` → devolve `{ticket, expires_at, stream_url}`.
2. Abrir `GET /api/v1/events/stream?ticket=<ticket>`.
3. O ticket é **de uso único**, vale **60 segundos** e é consumido na abertura da conexão — reuso responde **401**.

Para agentes que conseguem enviar headers (`curl`, MCP), o stream aceita `X-API-Key` diretamente e o ticket é desnecessário. O stream envia um comentário `: ping` a cada 25 segundos para manter a conexão viva.
## 10. Enquetes e o Cache de Hashes

Enquetes são o mecanismo de interatividade **100% compatível com conta pessoal** (botões e listas dependem do nó `<biz>` e podem ser restringidos pelo app do destinatário).

O protocolo entrega o voto de forma peculiar: `DecryptPollVote` devolve **hashes SHA-256** das opções escolhidas, não o texto. Para que o agente receba algo utilizável:

1. toda enquete **criada ou recebida** é gravada em `app.db` (`poll_cache`) com pergunta e opções em texto;
2. ao chegar um voto, o ID da mensagem da enquete é usado para recuperar esse registro;
3. cada hash é comparado com `SHA-256` de cada opção (`HashPollOptions`) e convertido no texto correspondente.

O evento carrega sempre `selected_hashes` e, quando o cache tem a enquete, também `selected_options` em texto. Se a enquete não estiver no cache (ex.: criada antes do gateway existir), o voto **não é perdido**: os hashes continuam disponíveis para correlação.
## 11. Botões e Listas (limitação observada do protocolo)

Botões (`InteractiveMessage`) e listas (`ListMessage`) são implementados com o nó
`<biz><interactive type="native_flow" v="1"><native_flow name="mixed"/></interactive></biz>`
injetado via `SendRequestExtra.AdditionalNodes`, conforme a documentação do Multi-Device.

**Medição em ambiente real (14/11/2026):** com uma conta **WhatsApp Business** pareada como
companion device, o servidor recusou ambas as mensagens:

| Endpoint | Erro do servidor | HTTP da API |
| :--- | :--- | :--- |
| `POST /api/v1/messages/buttons` | `server returned error 473` | 422 `interactive_unsupported` |
| `POST /api/v1/messages/list` | `server returned error 479` | 422 `interactive_unsupported` |

A recusa é do **canal Web/companion**, não do tipo de conta: contas Business também não
conseguem enviar NativeFlow por esse caminho (a alternativa oficial é a Cloud API, que é
outro produto). Por isso a API traduz a recusa em `interactive_unsupported` com mensagem
apontando a alternativa.

**Recomendação:** use `POST /api/v1/messages/poll` para interatividade — enquete nativa
funciona em qualquer conta e o voto é decodificado (seção 10). Os endpoints de botões e
lista continuam disponíveis caso o comportamento mude ou o destino seja outro tipo de conta.
## 12. Validação com Tráfego Real (14/11/2026)

Teste ponta a ponta executado com uma conta **WhatsApp Business** pareada como companion device:

| Fluxo | Evidência |
| :--- | :--- |
| Envio de texto | `message_id` devolvido e mensagem entregue |
| Envio de imagem | upload + `message_id` |
| Envio de enquete | `message_id` + registro em `poll_cache` |
| Recebimento de texto | evento `text` com `chat_jid` normalizado para o número |
| Recebimento de imagem | `DownloadAny` + arquivo em `data/media` + `GET /media/{id}` respondendo `200 image/jpeg` |
| Voto de enquete | `selected_options: ["Opcao A"]` (hash SHA-256 resolvido pelo cache) |
| Listagem de grupos | 7 grupos com participantes |
| Webhook | `session.connected/disconnected` entregues com `X-GabeWhats-Signature` |
| SSE | mesmo envelope, framing `event: message` |

Achados corrigidos nesta rodada: reconexão que não reconectava, eventos de protocolo publicados como `unknown`,
identidade `@lid` não traduzida para número e falha de boot tocando na sessão do WhatsApp.
## 13. Tipos de Mensagem Suportados

| Mensagem | `message_type` | Observações |
| :--- | :--- | :--- |
| Texto (simples e estendido) | `text` | inclui citação (`reply_to_id`) |
| Imagem | `image` | legenda preservada |
| Vídeo | `video` | inclui GIF (`is_animated`) |
| **Vídeo-nota (PTV)** | `video` | ver armadilha abaixo |
| Áudio / nota de voz | `audio` | `is_ptt` indica nota de voz |
| Documento | `document` | nome do arquivo preservado |
| Figurinha | `sticker` | animada ou estática |
| Localização | `location` | estática e **em tempo real** |
| Contato | `contact` | individual e lista de contatos |
| Botão / lista respondidos | `button_reply`, `list_reply` | inclui resposta NativeFlow |
| Enquete e voto | `poll`, `poll_vote` | voto chega decifrado (seção 10) |
| Reação | `reaction` | **inclusive cifrada** (`EncReactionMessage`) |
| Revogação | `revoke` | `target_message_id` |
| Qualquer outra coisa | — | descartada, com registro em `debug` |

Mensagens de protocolo (sincronização de histórico, app state, chaves de grupo) e tipos
ainda não mapeados **não são publicadas**: o normalizador registra
`mensagem ignorada por tipo não mapeado` em nível `debug` para que um tipo novo
apareça no log em vez de desaparecer em silêncio.

### Armadilha do vídeo-nota

O `whatsmeow` guarda o vídeo-nota no campo `PtvMessage`, e o helper `client.DownloadAny`
**não o reconhece** (só procura `ImageMessage`, `VideoMessage`, `AudioMessage`,
`DocumentMessage` e `StickerMessage`). Usar `DownloadAny` resulta em
`didn't find any attachments in message`. Por isso o normalizador baixa o PTV com
`client.Download(ctx, msg.GetPtvMessage())` diretamente.
### Templates de conta Business

Mensagens de contas Business (cobrança, atualização de pedido, OTP) chegam embrulhadas em
`TemplateMessage`. O normalizador desembrulha o conteúdo real — imagem, vídeo, documento,
localização ou texto — e publica o evento como se fosse uma mensagem comum, usando
`hydratedContentText` como texto de apoio quando não há legenda.

Antes desse tratamento a mensagem caía em "tipo não mapeado" e era descartada.

### Álbuns

O WhatsApp entrega um álbum como **várias mensagens de mídia independentes**, cada uma com
seu próprio `message_id`. Não existe agrupamento no envelope que chega (o `AlbumMessage`
não carrega os filhos). Na prática: 6 fotos enviadas juntas geram 6 eventos `image`
distintos, todos com download próprio.
## 14. Ritmo Humano (anti-assinatura de bot)

O gateway não envia instantaneamente. O princípio vem do combate a anti-bot em scraping:
**"bot é regular, pessoa mexendo é irregular"** — o que um servidor consegue medir com
facilidade não é o conteúdo, é o **tempo**.

Das camadas daquele contexto, apenas a de velocidade tem análogo aqui: cabeçalho, TLS,
proxy e browser fazem sentido para HTTP, não para o protocolo WhatsApp, onde a identidade
já é o dispositivo pareado.

### O que é aplicado

| Mecanismo | Comportamento |
| :--- | :--- |
| Indicador "digitando..." | Proporcional ao texto (~6 caracteres/s) com ±35% de variação, mínimo 600 ms e teto de 8 s |
| Intervalo por conversa | 1,2 s a 3,5 s sorteados entre duas mensagens para o mesmo chat; conversas diferentes não se esperam |
| Nota de voz | Presença de "gravando áudio" em vez de "digitando" |
| Retry | Até 3 tentativas em erro transitório (429/5xx/timeout), com espera exponencial e jitter |

### Configuração

```bash
GABE_WHATS_HUMANIZE=true      # liga/desliga
GABE_WHATS_TYPING_CPS=6       # caracteres por segundo
GABE_WHATS_SEND_GAP_MIN=1200ms
GABE_WHATS_SEND_GAP_MAX=3500ms
GABE_WHATS_TYPING_MAX=8s
```

### Efeitos colaterais (esperados)

O tempo faz parte do contrato e acontece **dentro da requisição**: o indicador é limitado
por `GABE_WHATS_TYPING_MAX` (8 s por padrão) e soma-se ao intervalo da conversa, então um
texto curto leva cerca de 1 s e um texto longo chega ao teto de ~11 s. Quem precisa de
velocidade desliga a humanização
(`GABE_WHATS_HUMANIZE=false`) ou, em `POST /messages/text`, envia
`simulate_typing_ms` negativo para pular o indicador daquela mensagem.
