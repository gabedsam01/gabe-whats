package tools

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/apiclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connect sobe um servidor com as tools registradas e devolve uma sessão cliente.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "teste", Version: "0"}, nil)
	Register(server, apiclient.New("http://127.0.0.1:9", "chave"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = server.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "teste-cliente", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestRegistraAs22Tools(t *testing.T) {
	session := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("listar tools: %v", err)
	}
	if len(result.Tools) != 22 {
		t.Errorf("esperava 22 tools, obtive %d", len(result.Tools))
	}

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
		if tool.Description == "" {
			t.Errorf("tool %s sem descrição", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s sem input schema", tool.Name)
		}
	}
	sort.Strings(names)

	expected := []string{
		"whatsapp_check_numbers", "whatsapp_create_group", "whatsapp_edit_message",
		"whatsapp_get_group", "whatsapp_get_group_invite_link", "whatsapp_get_qr",
		"whatsapp_join_group", "whatsapp_leave_group", "whatsapp_list_groups",
		"whatsapp_mark_read", "whatsapp_pair_phone", "whatsapp_react_message",
		"whatsapp_revoke_message", "whatsapp_send_buttons", "whatsapp_send_list",
		"whatsapp_send_media", "whatsapp_send_poll", "whatsapp_send_text",
		"whatsapp_send_voice_ptt", "whatsapp_session_status",
		"whatsapp_update_group_participants", "whatsapp_update_group_settings",
	}
	if len(expected) != len(names) {
		t.Fatalf("catálogo de teste desatualizado: %d esperadas", len(expected))
	}
	for i := range expected {
		if expected[i] != names[i] {
			t.Errorf("posição %d: esperava %q, obtive %q", i, expected[i], names[i])
		}
	}
}
