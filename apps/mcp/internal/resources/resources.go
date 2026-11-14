// Package resources expõe os resources MCP de leitura (somente consulta).
package resources

import (
	"context"
	"encoding/json"

	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/apiclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register adiciona os resources de leitura ao servidor.
func Register(server *mcp.Server, client *apiclient.Client) {
	server.AddResource(&mcp.Resource{
		URI:         "whatsapp://session/status",
		Name:        "Status da sessão",
		Description: "Estado atual da conexão do WhatsApp (conectado, autenticado, JID e uptime).",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		status, err := client.Status(ctx)
		if err != nil {
			return nil, err
		}
		return resourceResult("whatsapp://session/status", status), nil
	})

	server.AddResource(&mcp.Resource{
		URI:         "whatsapp://groups/list",
		Name:        "Lista de grupos",
		Description: "Grupos dos quais o número participa, com metadados e participantes.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		groups, err := client.ListGroups(ctx)
		if err != nil {
			return nil, err
		}
		return resourceResult("whatsapp://groups/list", groups), nil
	})

	server.AddResource(&mcp.Resource{
		URI:         "whatsapp://messages/recent",
		Name:        "Mensagens recentes",
		Description: "Últimas mensagens recebidas (até 50), da mais nova para a mais antiga.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		recent, err := client.RecentMessages(ctx, 50, "")
		if err != nil {
			return nil, err
		}
		return resourceResult("whatsapp://messages/recent", recent), nil
	})
}

// resourceResult serializa o valor como conteúdo JSON de um resource.
func resourceResult(uri string, value any) *mcp.ReadResourceResult {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/plain", Text: err.Error()}}}
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(data)}}}
}
