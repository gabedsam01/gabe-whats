// Blueprint do entrypoint da API REST (`apps/api`).
// Este arquivo representa o desenho arquitetural do ciclo de vida do servidor.
// A implementação completa será adicionada após aprovação da fase de arquitetura.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Passo 1 (Config): carregar apps/api/internal/config
	//   - HTTPAddr (ex: 127.0.0.1:8080)
	//   - APIKey (para proteger todas as rotas /api/v1/*)
	//   - SQLiteDSN (ex: file:./data/session.db?_foreign_keys=on)
	//   - WebhookConfig (URL, Secret, IgnoreFromMe, AllowJIDs, AutoDownloadMedia)

	// Passo 2 (WhatsApp Store & Client): inicializar whatsmeow via internal/whatsapp
	//   - container, err := sqlstore.New(ctx, "sqlite3", cfg.SQLiteDSN, logger)
	//   - deviceStore, err := container.GetFirstDevice(ctx)
	//   - client := whatsmeow.NewClient(deviceStore, logger)

	// Passo 3 (Event Handler -> Webhook/SSE): registrar client.AddEventHandler(...)
	//   - Desembrulhar *events.Message (texto, mídia via DownloadAny, clique de botão, voto via DecryptPollVote)
	//   - Publicar WebhookInboundEvent para internal/webhook (POST assíncrono + SSE stream)

	// Passo 4 (Conexão): se deviceStore.ID != nil -> client.Connect()
	//   - Caso contrário, aguarda chamada em GET /api/v1/session/qr ou POST /api/v1/session/pair-code

	// Passo 5 (HTTP + Auto-Docs /docs): registrar internal/http/router.go
	//   - Huma v2 + Chi expondo /api/v1/*, /openapi.json, /openapi.yaml e /docs (Scalar / Swagger UI)

	<-ctx.Done()
	log.Println("Blueprint gabe-whats API: encerrando graciosamente (client.Disconnect)")
}
