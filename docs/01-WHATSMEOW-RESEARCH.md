# Pesquisa Técnica: Biblioteca `tulir/whatsmeow` (`go.mau.fi/whatsmeow`)

Este documento consolida a análise do repositório oficial [`tulir/whatsmeow`](https://github.com/tulir/whatsmeow) e mapeia exatamente quais pacotes, structs e métodos da biblioteca darão suporte à API REST e ao Servidor MCP do **gabe-whats**.

---

## 1. Visão Geral da Biblioteca

- **Módulo Go:** `go.mau.fi/whatsmeow`
- **Versão de Go no upstream (`go.mod`):** `go 1.26.0` (compatível com `go1.27.3` instalado neste ambiente).
- **Propósito:** Implementação em Go puro do protocolo **WhatsApp Web Multi-Device** (WebSocket + Protobuf + criptografia Signal `go.mau.fi/libsignal`).
- **Uso previsto:** 1 número pessoal conectado como *companion device* (dispositivo vinculado via QR Code ou Pairing Code de 8 dígitos), mantendo a sessão persistida em SQLite local.

---

## 2. Mapeamento dos Arquivos-Chave no Repositório Upstream

| Arquivo / Pacote | Responsabilidade no `whatsmeow` | Uso no `gabe-whats` |
| :--- | :--- | :--- |
| `client.go` | Struct `Client`, ciclo de vida WebSocket (`Connect`, `ConnectContext`, `Disconnect`, `ResetConnection`, `Logout`, `IsLoggedIn`, `IsConnected`, `WaitForConnection`) e registro de handlers (`AddEventHandler`, `RemoveEventHandler`). | Gerenciar a conexão contínua do número pessoal e escutar eventos em tempo real. |
| `qrchan.go` | `GetQRChannel(ctx)` retorna canal `<-chan QRChannelItem` emitindo eventos `"code"`, `QRChannelSuccess`, `QRChannelTimeout`, `QRChannelClientOutdated`, etc. | Endpoint `GET /api/v1/session/qr` (QR Code em PNG/Base64/string). |
| `pair-code.go` | `PairPhone(ctx, phone, showPushNotification, clientType, clientDisplayName)` (ex: `PairClientChrome`, `"Chrome (Linux)"`). | Endpoint `POST /api/v1/session/pair-code` para parear sem precisar ler câmera. |
| `store/sqlstore/` | `sqlstore.New(ctx, dialect, address, log)` + `container.GetFirstDevice(ctx)` + `lidmap.go` (`LID <-> PN` mapping). | Persistir chaves Signal e sessão no SQLite local (sem precisar de QR Code a cada restart). **Atenção:** o SQLite é interno apenas para a sessão do `whatsmeow`; toda a exposição externa é via REST JSON. |
| `send.go` | `SendMessage(ctx, to, message, extra...)`, `GenerateMessageID()`, `BuildEdit()`, `BuildRevoke()`, `BuildReaction()`, `SetDisappearingTimer()`. | Endpoints de envio de texto, mídia, botões, edição, exclusão e reação. |
| `upload.go` | `Upload(ctx, plaintext, appInfo)` / `UploadReader(...)` com `MediaImage`, `MediaVideo`, `MediaAudio`, `MediaDocument`. Retorna `UploadResponse` (`URL`, `DirectPath`, `MediaKey`, `FileEncSHA256`, `FileSHA256`, `FileLength`). | Criptografar e enviar mídia/áudios antes de chamar `SendMessage`. |
| `download.go` | `DownloadAny(ctx, msg)` e `Download(ctx, msg)` para baixar e descriptografar mídia recebida. | Auto-download de áudios PTT, imagens e documentos recebidos para entrega imediata ao agente via Webhook/REST. |
| `group.go` | `CreateGroup`, `GetJoinedGroups`, `GetGroupInfo`, `UpdateGroupParticipants`, `SetGroupName`, `SetGroupDescription`, `SetGroupTopic`, `SetGroupPhoto`, `SetGroupAnnounce`, `SetGroupLocked`, `GetGroupInviteLink`, `JoinGroupWithLink`, `LeaveGroup`. | Gestão completa de grupos na API REST e nas Tools MCP. |
| `presence.go` & `receipt.go` | `SendPresence`, `SendChatPresence` (`composing`, `recording`, `paused`), `MarkRead`. | Marcar mensagens como lidas e simular digitação/gravação de áudio pelo agente. |
| `msgsecret.go` | `BuildPollCreation`, `BuildPollVote`, `DecryptPollVote`, `DecryptReaction`, `DecryptSecretEncryptedMessage`. | Enviar enquetes nativas e descriptografar votos/reações recebidos. |
| `types/events/events.go` | Definições dos eventos (`*events.Message`, `*events.Receipt`, `*events.ChatPresence`, `*events.GroupInfo`, `*events.JoinedGroup`, `*events.Connected`, `*events.Disconnected`, `*events.LoggedOut`). | Alimentar o normalizador de Webhook HTTP POST e SSE para OpenClaw, Moltbot, Hermes e Agent Zero. |

---

## 3. Detalhes Críticos de Protocolo para o `gabe-whats`

### 3.1 Recebimento de Mensagens (`*events.Message`)
No handler registrado via `cli.AddEventHandler(func(evt any) { ... })`:
- `evt.(*events.Message)` contém:
  - `Info` (`types.MessageInfo`): `ID`, `Chat` (`types.JID`), `Sender` (`types.JID`), `IsFromMe`, `IsGroup`, `Timestamp`, `PushName`.
  - `Message` (`*waE2E.Message`): mensagem já desembrulhada de wrappers (`IsEphemeral`, `IsViewOnce`, `IsEdit`, `IsDocumentWithCaption`).
  - `RawMessage`: payload bruto original (útil para inspecionar formatos desconhecidos).
- **Normalização necessária:** Uma mensagem recebida pode vir em `Conversation` (texto simples), `ExtendedTextMessage.Text` (texto com link/citação/menção), `ImageMessage`, `AudioMessage` (inclusive `PTT: true`), `VideoMessage`, `DocumentMessage`, `ButtonsResponseMessage`, `ListResponseMessage`, `InteractiveResponseMessage` ou `PollUpdateMessage` (que precisa de `cli.DecryptPollVote(ctx, evt)`).

### 3.2 Envio de Botões, Listas e Enquetes (`waE2E.Message`)
No protocolo Multi-Device atual do WhatsApp para contas pessoais, há **três mecanismos** que devem ser tratados de forma distinta:

1. **Botões Interativos (`InteractiveMessage` + `NativeFlowMessage`):**
   - Suporta botões `quick_reply` (resposta rápida), `cta_url` (abrir link) e `cta_copy` (copiar código) via `NativeFlowButton{Name, ButtonParamsJSON}`.
   - **Requisito crítico descoberto na pesquisa:** No `SendMessage`, o `whatsmeow` aceita `SendRequestExtra{AdditionalNodes: &nodes}`. Para que aplicativos móveis rendam mensagens interativas/botões em conexões Web Multi-Device, costuma ser necessário injetar o nó binário `<biz><interactive type="native_flow" v="1"><native_flow name="quick_reply"/></interactive></biz>`. Ainda assim, clientes iOS/Android recentes podem restringir botões `NativeFlow` fora de contas Business oficiais.

2. **Menus de Lista (`ListMessage`):**
   - Abre um botão que expande seções e linhas selecionáveis (`ListType: waE2E.ListMessage_SINGLE_SELECT`).
   - Também depende da injeção de `<biz>` via `AdditionalNodes` em contas não-oficiais e possui as mesmas restrições de renderização em versões recentes do app.

3. **Enquetes Nativas (`PollCreationMessage` via `cli.BuildPollCreation`):**
   - Funciona **100% nativamente** em qualquer conta pessoal (Android, iOS e Web) sem necessidade de `<biz>` ou nós extras.
   - Aceita pergunta, lista de opções e `selectableOptionCount` (`1` para escolha única estilo botão, `0` para múltipla escolha).
   - Quando o usuário toca em uma opção, o evento chega como `*events.Message` contendo `PollUpdateMessage`, e `cli.DecryptPollVote(ctx, evt)` retorna as opções selecionadas em texto claro para o agente.

> **Decisão arquitetural:** A API REST e o servidor MCP exporão endpoints/tools separados para `/messages/buttons`, `/messages/list` e `/messages/poll`, permitindo usar botões `NativeFlow` quando suportados pelo destinatário e `Poll` como alternativa garantida para menus interativos pessoais.

### 3.3 Gestão de Grupos (`group.go`)
Os principais métodos suportados nativamente:
- **Listar e Inspecionar:** `GetJoinedGroups(ctx)` e `GetGroupInfo(ctx, groupJID)`.
- **Criar:** `CreateGroup(ctx, ReqCreateGroup{Name, Participants, ...})` *(limite de 25 caracteres para `Name` imposto pelo servidor do WhatsApp; nomes maiores retornam erro `406`)*.
- **Participantes (`UpdateGroupParticipants`):** aceita `ParticipantChangeAdd` (`"add"`), `ParticipantChangeRemove` (`"remove"`), `ParticipantChangePromote` (`"promote"`), `ParticipantChangeDemote` (`"demote"`).
- **Configurações e Correção:** `SetGroupName`, `SetGroupDescription`, `SetGroupTopic`, `SetGroupPhoto`, `SetGroupAnnounce(ctx, jid, bool)` (somente admins enviam mensagens), `SetGroupLocked(ctx, jid, bool)` (somente admins editam info do grupo), `SetGroupJoinApprovalMode`, `SetGroupMemberAddMode`.
- **Convites e Saída:** `GetGroupInviteLink(ctx, jid, reset)`, `JoinGroupWithLink(ctx, code)`, `LeaveGroup(ctx, jid)`.

### 3.4 Mapeamento de Identidade (`LID` vs Número de Telefone)
O WhatsApp introduziu identificadores `LID` (`@lid`) além dos tradicionais `@s.whatsapp.net`. O `sqlstore` do `whatsmeow` já mantém o `CachedLIDMap` (`cli.Store.LIDs`), permitindo resolver `GetPNForLID` e `GetLIDForPN` de forma transparente ao normalizar remetentes para o agente.

---

## 4. Cuidados Operacionais (Uso Pessoal)
- `whatsmeow` emula um dispositivo WhatsApp Web. Embora seja estável e amplamente testado (usado no `mautrix-whatsapp` e Beeper), **não** deve ser usado para disparo em massa/spam.
- Para uso pessoal com agentes (OpenClaw, Moltbot, Hermes, Agent Zero), recomenda-se configurar uma **allowlist de JIDs/números** e filtro de disparo (ex: responder apenas contatos permitidos ou grupos específicos) no webhook/agente para evitar loops de resposta automática.
