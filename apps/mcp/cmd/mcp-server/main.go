// Blueprint do entrypoint do Servidor MCP (`apps/mcp`).
// Este processo roda separado da API REST (`apps/api`) para que agentes
// (OpenClaw, Moltbot, Hermes Agent, Agent Zero) possam iniciar/parar via stdio
// sem derrubar a sessão WebSocket do WhatsApp.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	transport := flag.String("transport", "stdio", "Transporte MCP: stdio ou sse")
	sseAddr := flag.String("sse-addr", "127.0.0.1:8090", "Endereço HTTP quando transport=sse")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Passo 1 (Config & HTTP Client):
	//   - Ler GABE_WHATS_API_URL (default: http://127.0.0.1:8080) e GABE_WHATS_API_KEY
	//   - Instanciar internal/apiclient apontando para apps/api

	// Passo 2 (Registrar Tools & Resources):
	//   - Criar instância MCP Server ("gabe-whats-mcp", "0.1.0")
	//   - Registrar todas as tools definidas em internal/tools/blueprint.go
	//   - Registrar resources: whatsapp://session/status, whatsapp://groups/list, whatsapp://messages/recent

	// Passo 3 (Subir Transporte):
	//   - Se *transport == "stdio": servir MCP via stdin/stdout (padrão OpenClaw/Hermes)
	//   - Se *transport == "sse": servir MCP via SSE em *sseAddr

	log.Printf("Blueprint gabe-whats MCP Server pronto (transport=%s, sseAddr=%s)", *transport, *sseAddr)
	<-ctx.Done()
}
