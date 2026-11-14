package httpapi

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// specRelPath é o caminho do documento OpenAPI relativo ao diretório deste
// pacote (apps/api/internal/http). São necessários quatro níveis até a raiz do
// módulo: http -> internal -> api -> apps -> raiz.
const specRelPath = "../../../../docs/openapi.yaml"

// openAPIOperation é a projeção mínima de uma operação da spec usada pelos
// testes de paridade, de operationId e de segurança.
type openAPIOperation struct {
	Method           string
	Path             string
	OperationID      string
	SecurityDeclared bool
	SecuritySchemes  []string
}

// publicOpenAPIPaths são os únicos endpoints que dispensam autenticação,
// conforme docs/openapi.yaml e a especificação da API.
var publicOpenAPIPaths = map[string]bool{
	"/health":       true,
	"/docs":         true,
	"/openapi.json": true,
	"/openapi.yaml": true,
}

// TestPlannedRoutesMatchOpenAPI garante paridade exata (método + path,
// ignorando ordem) entre PlannedRoutes e o bloco paths: de docs/openapi.yaml.
func TestPlannedRoutesMatchOpenAPI(t *testing.T) {
	operations := loadOpenAPIOperations(t)

	specRoutes := make(map[string]struct{}, len(operations))
	for _, operation := range operations {
		key := operation.Method + " " + operation.Path
		if _, duplicate := specRoutes[key]; duplicate {
			t.Errorf("operação duplicada na spec OpenAPI: %s", key)
		}
		specRoutes[key] = struct{}{}
	}

	routerRoutes := make(map[string]struct{}, len(PlannedRoutes))
	for _, route := range PlannedRoutes {
		key := strings.ToUpper(route.Method) + " " + route.Path
		if _, duplicate := routerRoutes[key]; duplicate {
			t.Errorf("rota duplicada em PlannedRoutes: %s", key)
		}
		routerRoutes[key] = struct{}{}
	}

	var onlyInRouter, onlyInSpec []string
	for key := range routerRoutes {
		if _, ok := specRoutes[key]; !ok {
			onlyInRouter = append(onlyInRouter, key)
		}
	}
	for key := range specRoutes {
		if _, ok := routerRoutes[key]; !ok {
			onlyInSpec = append(onlyInSpec, key)
		}
	}
	sort.Strings(onlyInRouter)
	sort.Strings(onlyInSpec)

	if len(onlyInRouter) == 0 && len(onlyInSpec) == 0 {
		return
	}

	var report strings.Builder
	fmt.Fprintf(&report, "divergência entre PlannedRoutes (%d rotas) e docs/openapi.yaml (%d operações):\n", len(routerRoutes), len(specRoutes))
	if len(onlyInRouter) > 0 {
		report.WriteString("  somente no router (ausentes na spec):\n")
		for _, key := range onlyInRouter {
			fmt.Fprintf(&report, "    - %s\n", key)
		}
	}
	if len(onlyInSpec) > 0 {
		report.WriteString("  somente na spec (ausentes no router):\n")
		for _, key := range onlyInSpec {
			fmt.Fprintf(&report, "    - %s\n", key)
		}
	}
	t.Error(report.String())
}

// TestOpenAPIOperationIDs garante que toda operação tem operationId não vazio
// e que nenhum operationId se repete.
func TestOpenAPIOperationIDs(t *testing.T) {
	operations := loadOpenAPIOperations(t)

	byID := make(map[string][]string, len(operations))
	for _, operation := range operations {
		location := operation.Method + " " + operation.Path
		if strings.TrimSpace(operation.OperationID) == "" {
			t.Errorf("%s: operação sem operationId", location)
			continue
		}
		byID[operation.OperationID] = append(byID[operation.OperationID], location)
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		locations := byID[id]
		if len(locations) < 2 {
			continue
		}
		sort.Strings(locations)
		t.Errorf("operationId %q duplicado em: %s", id, strings.Join(locations, ", "))
	}
}

// TestOpenAPISecurity garante que os quatro endpoints públicos declaram
// security: [] e que todas as demais operações exigem exatamente ApiKeyAuth.
//
// A verificação é deliberadamente conservadora: exige a declaração de security
// no nível da operação (e não apenas a herança do security global). Assim um
// endpoint novo não herda autenticação nem vira público por acidente, e a
// exceção dos endpoints públicos fica explícita no documento.
func TestOpenAPISecurity(t *testing.T) {
	operations := loadOpenAPIOperations(t)

	seenPublic := make(map[string]bool, len(publicOpenAPIPaths))
	for _, operation := range operations {
		location := operation.Method + " " + operation.Path

		if publicOpenAPIPaths[operation.Path] {
			seenPublic[operation.Path] = true
			if !operation.SecurityDeclared {
				t.Errorf("%s: endpoint público deve declarar security: [] explicitamente", location)
				continue
			}
			if len(operation.SecuritySchemes) != 0 {
				t.Errorf("%s: endpoint público deve usar security: [], encontrado %v", location, operation.SecuritySchemes)
			}
			continue
		}

		if !operation.SecurityDeclared {
			t.Errorf("%s: security não declarado na operação; esperado ApiKeyAuth", location)
			continue
		}
		if len(operation.SecuritySchemes) != 1 || operation.SecuritySchemes[0] != "ApiKeyAuth" {
			t.Errorf("%s: esperado security com apenas ApiKeyAuth, encontrado %v", location, operation.SecuritySchemes)
		}
	}

	paths := make([]string, 0, len(publicOpenAPIPaths))
	for path := range publicOpenAPIPaths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if !seenPublic[path] {
			t.Errorf("endpoint público %q não encontrado na spec OpenAPI", path)
		}
	}
}

// loadOpenAPIOperations lê e interpreta a spec, falhando com o caminho e o
// diretório de trabalho quando a leitura ou a interpretação não for possível.
func loadOpenAPIOperations(t *testing.T) []openAPIOperation {
	t.Helper()

	data, err := os.ReadFile(specRelPath)
	if err != nil {
		cwd, _ := os.Getwd()
		t.Fatalf("não foi possível ler a spec OpenAPI em %q (diretório de trabalho: %s): %v", specRelPath, cwd, err)
	}

	operations, err := parseOpenAPIOperations(string(data))
	if err != nil {
		t.Fatalf("falha ao interpretar %s com o leitor mínimo por indentação: %v", specRelPath, err)
	}
	return operations
}

// parseOpenAPIOperations extrai método, path, operationId e security do bloco
// paths: sem depender de biblioteca YAML externa (go.mod deve ficar intocado).
//
// Premissas deliberadas do leitor mínimo (violá-las gera erro explícito):
//   - a chave paths: aparece na coluna 0 e o bloco termina na próxima chave de
//     coluna 0 (neste documento, components:);
//   - cada path tem exatamente 2 espaços de indentação e começa com "/";
//   - cada método HTTP tem exatamente 4 espaços de indentação;
//   - operationId e security têm exatamente 6 espaços de indentação;
//   - itens da lista de security têm exatamente 8 espaços ("- ApiKeyAuth: []").
//
// Linhas em branco e comentários são ignorados. Blocos de descrição (| e >-)
// ficam com indentação maior e não colidem com as âncoras acima.
func parseOpenAPIOperations(document string) ([]openAPIOperation, error) {
	lines := strings.Split(strings.ReplaceAll(document, "\r\n", "\n"), "\n")

	start := -1
	for index, line := range lines {
		if indentOf(line) == 0 && strings.TrimSpace(line) == "paths:" {
			start = index
			break
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("chave de topo %q não encontrada na coluna 0", "paths:")
	}

	var operations []openAPIOperation
	currentPath := ""
	currentOperation := -1
	inSecurityList := false

parsePaths:
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " "), "\t") {
			return nil, fmt.Errorf("linha %d: indentação com tab não é suportada pelo leitor mínimo; use espaços", index+1)
		}

		switch indent := indentOf(line); indent {
		case 0:
			// Fim do bloco paths: (próxima chave de topo, ex. components:).
			break parsePaths
		case 2:
			if !strings.HasPrefix(trimmed, "/") {
				return nil, fmt.Errorf("linha %d: esperado path iniciado por '/' com 2 espaços de indentação, obtido %q", index+1, trimmed)
			}
			if !strings.HasSuffix(trimmed, ":") {
				return nil, fmt.Errorf("linha %d: path %q não termina em ':'", index+1, trimmed)
			}
			currentPath = strings.TrimSuffix(trimmed, ":")
			currentOperation = -1
			inSecurityList = false
		case 4:
			method := strings.TrimSuffix(trimmed, ":")
			if !strings.HasSuffix(trimmed, ":") || !isHTTPMethod(method) {
				return nil, fmt.Errorf("linha %d: esperado método HTTP (get, post, put, patch, delete, head, options, trace) com 4 espaços de indentação, obtido %q", index+1, trimmed)
			}
			if currentPath == "" {
				return nil, fmt.Errorf("linha %d: método %q encontrado antes de qualquer path", index+1, method)
			}
			operations = append(operations, openAPIOperation{
				Method: strings.ToUpper(method),
				Path:   currentPath,
			})
			currentOperation = len(operations) - 1
			inSecurityList = false
		case 6:
			if currentOperation < 0 {
				continue
			}
			inSecurityList = false
			switch {
			case strings.HasPrefix(trimmed, "operationId:"):
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "operationId:"))
				operations[currentOperation].OperationID = trimYAMLQuotes(value)
			case trimmed == "security: []":
				operations[currentOperation].SecurityDeclared = true
				operations[currentOperation].SecuritySchemes = nil
			case trimmed == "security:":
				operations[currentOperation].SecurityDeclared = true
				inSecurityList = true
			}
		case 8:
			if currentOperation < 0 || !inSecurityList {
				continue
			}
			if !strings.HasPrefix(trimmed, "- ") {
				return nil, fmt.Errorf("linha %d: esperado item de security iniciado por '- ', obtido %q", index+1, trimmed)
			}
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			name := item
			if separator := strings.Index(item, ":"); separator >= 0 {
				name = strings.TrimSpace(item[:separator])
			}
			name = trimYAMLQuotes(name)
			if name == "" {
				return nil, fmt.Errorf("linha %d: item de security sem nome de esquema", index+1)
			}
			operations[currentOperation].SecuritySchemes = append(operations[currentOperation].SecuritySchemes, name)
		}
	}

	if len(operations) == 0 {
		return nil, fmt.Errorf("nenhuma operação encontrada no bloco paths: (premissa: paths em 2 espaços e métodos HTTP em 4)")
	}
	return operations, nil
}

// indentOf devolve a quantidade de espaços iniciais da linha.
func indentOf(line string) int {
	spaces := 0
	for spaces < len(line) && line[spaces] == ' ' {
		spaces++
	}
	return spaces
}

// isHTTPMethod informa se o texto é um método HTTP aceito pelo leitor mínimo.
func isHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "put", "post", "delete", "patch", "head", "options", "trace":
		return true
	default:
		return false
	}
}

// trimYAMLQuotes remove aspas simples ou duplas ao redor de um valor escalar.
func trimYAMLQuotes(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
