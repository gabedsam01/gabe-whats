package resources

import (
	"context"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/apiclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegistraOsTresResources(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "teste", Version: "0"}, nil)
	Register(server, apiclient.New("http://127.0.0.1:9", "chave"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = server.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "teste-cliente", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("listar resources: %v", err)
	}
	if len(result.Resources) != 3 {
		t.Fatalf("esperava 3 resources, obtive %d", len(result.Resources))
	}

	uris := map[string]bool{}
	for _, resource := range result.Resources {
		uris[resource.URI] = true
	}
	for _, expected := range []string{"whatsapp://session/status", "whatsapp://groups/list", "whatsapp://messages/recent"} {
		if !uris[expected] {
			t.Errorf("resource ausente: %s", expected)
		}
	}
}
