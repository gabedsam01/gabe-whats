package contracts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// contractStructs é o registro dos structs exportados auditados por reflect.
// O pacote reflect não enumera tipos por nome, por isso a lista é explícita.
// TestExportedFieldsHaveDocTag confere via AST que ela cobre exatamente todos
// os structs exportados dos arquivos .go do pacote, falhando quando um novo
// struct for adicionado sem entrar aqui.
var contractStructs = []any{
	// common.go
	Problem{}, ProblemField{}, HealthResponse{}, AckResponse{}, MediaRef{},
	// events.go
	SSETicketResponse{},
	// session.go
	SessionStatus{}, QRCodeResponse{}, PairCodeRequest{}, PairCodeResponse{},
	// contacts.go
	CheckNumbersRequest{}, CheckNumbersResponse{}, NumberCheckResult{},
	// messages.go
	MessageResponse{}, SendTextRequest{}, SendMediaRequest{}, SendVoiceRequest{},
	ButtonItem{}, SendButtonsRequest{}, ListRow{}, ListSection{}, SendListRequest{},
	SendPollRequest{}, ReactionRequest{}, EditMessageRequest{}, RevokeMessageRequest{},
	MarkReadRequest{}, PresenceRequest{}, MessageSummary{}, RecentMessagesQuery{}, RecentMessagesResponse{},
	// groups.go
	CreateGroupRequest{}, UpdateGroupParticipantsRequest{}, GroupParticipantResult{}, UpdateGroupParticipantsResponse{}, UpdateGroupSettingsRequest{},
	GroupParticipantInfo{}, GroupInfoResponse{}, JoinGroupRequest{}, JoinGroupResponse{},
	GroupsResponse{}, SetGroupPhotoRequest{}, GroupPhotoResponse{}, InviteLinkResponse{},
	// webhooks.go
	WebhookButtonReply{}, WebhookPollVote{}, WebhookMediaPayload{}, WebhookReaction{},
	WebhookRevoke{}, WebhookPollCreation{}, WebhookInboundEvent{},
	WebhookGroupChange{}, WebhookGroupEvent{}, WebhookSessionEvent{},
	WebhookConfig{}, WebhookConfigResponse{},
}

// snakeCaseJSONTag é a forma exigida para nomes de campo JSON: minúsculas,
// dígitos e underscores simples entre segmentos (sem underscore inicial,
// final ou duplo).
var snakeCaseJSONTag = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// TestExportedFieldsHaveDocTag garante que todo campo exportado com tag json
// também declara uma tag doc com texto não vazio. Campos sem tag json são
// ignorados conforme a premissa de que nenhum existe no contrato.
func TestExportedFieldsHaveDocTag(t *testing.T) {
	checkRegistryCoverage(t)

	for _, value := range contractStructs {
		typ := reflect.TypeOf(value)
		if typ == nil || typ.Kind() != reflect.Struct {
			t.Errorf("contractStructs contém um valor que não é struct: %T", value)
			continue
		}

		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if !field.IsExported() {
				continue
			}
			if _, ok := field.Tag.Lookup("json"); !ok {
				continue
			}
			if strings.TrimSpace(field.Tag.Get("doc")) == "" {
				t.Errorf("%s.%s: campo exportado sem tag doc com texto não vazio", typ.Name(), field.Name)
			}
		}
	}
}

// TestJSONTagsAreSnakeCase garante que toda tag json de campo exportado usa
// snake_case, ignorando opções como ",omitempty".
func TestJSONTagsAreSnakeCase(t *testing.T) {
	for _, value := range contractStructs {
		typ := reflect.TypeOf(value)
		if typ == nil || typ.Kind() != reflect.Struct {
			continue
		}

		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if !field.IsExported() {
				continue
			}
			tag, ok := field.Tag.Lookup("json")
			if !ok {
				continue
			}

			name := strings.Split(tag, ",")[0]
			if name == "" {
				t.Errorf("%s.%s: tag json sem nome (%q)", typ.Name(), field.Name, tag)
				continue
			}
			if !snakeCaseJSONTag.MatchString(name) {
				t.Errorf("%s.%s: tag json %q não é snake_case (esperado %s)", typ.Name(), field.Name, name, snakeCaseJSONTag.String())
			}
		}
	}
}

// TestErrorContract valida por reflect que Problem expõe os cinco membros
// padrão da RFC 9457 e que ProblemField expõe field e message.
func TestErrorContract(t *testing.T) {
	assertJSONMembers(t, reflect.TypeOf(Problem{}), []string{"type", "title", "status", "detail", "instance"})
	assertJSONMembers(t, reflect.TypeOf(ProblemField{}), []string{"field", "message"})
}

// assertJSONMembers falha listando os nomes JSON ausentes no struct.
func assertJSONMembers(t *testing.T, typ reflect.Type, want []string) {
	t.Helper()

	if typ == nil || typ.Kind() != reflect.Struct {
		t.Fatalf("esperado struct para validar o contrato de erro, obtido %v", typ)
	}

	have := make(map[string]bool, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if !field.IsExported() {
			continue
		}
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			have[name] = true
		}
	}

	var missing []string
	for _, name := range want {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s: campos JSON obrigatórios ausentes: %s", typ.Name(), strings.Join(missing, ", "))
	}
}

// checkRegistryCoverage compara contractStructs com os structs exportados
// encontrados via AST nos arquivos .go do pacote (ignorando _test.go). Sem essa
// checagem, um struct novo ficaria silenciosamente fora da auditoria.
func checkRegistryCoverage(t *testing.T) {
	t.Helper()

	discovered := exportedStructNames(t)
	registered := make(map[string]bool, len(contractStructs))
	for _, value := range contractStructs {
		typ := reflect.TypeOf(value)
		if typ == nil || typ.Kind() != reflect.Struct {
			continue
		}
		registered[typ.Name()] = true
	}

	var missing, stale []string
	for name := range discovered {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	for name := range registered {
		if _, ok := discovered[name]; !ok {
			stale = append(stale, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)

	if len(missing) > 0 {
		t.Errorf("structs exportados ausentes de contractStructs (adicione-os para auditar as tags): %s", strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		t.Errorf("contractStructs lista structs que não existem mais no pacote: %s", strings.Join(stale, ", "))
	}
}

// exportedStructNames devolve nome -> arquivo de cada struct exportado
// declarado nos arquivos de produção do pacote.
func exportedStructNames(t *testing.T) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("não foi possível listar o diretório do pacote contracts: %v", err)
	}

	fset := token.NewFileSet()
	found := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("não foi possível analisar %s: %v", name, err)
		}
		for _, declaration := range file.Decls {
			genDecl, ok := declaration.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !ast.IsExported(typeSpec.Name.Name) {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); !ok {
					continue
				}
				found[typeSpec.Name.Name] = name
			}
		}
	}
	return found
}
