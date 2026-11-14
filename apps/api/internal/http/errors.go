package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// contentTypeProblem é o Content-Type exigido pela RFC 9457 e pelo contrato.
const contentTypeProblem = "application/problem+json"

// problemError carrega um contracts.Problem e implementa huma.StatusError, de
// modo que o próprio Huma serialize o corpo no formato acordado.
type problemError struct {
	// contracts.Problem embutido: os campos precisam ser promovidos no JSON.
	contracts.Problem
	status int
}

func (e *problemError) Error() string  { return e.Detail }
func (e *problemError) GetStatus() int { return e.status }

// ContentType força application/problem+json em qualquer negociação, conforme
// a interface huma.ContentTypeFilter.
func (e *problemError) ContentType(string) string { return contentTypeProblem }

// newProblem monta um problema com código estável e detalhes opcionais.
func newProblem(status int, code, detail string, fields ...contracts.ProblemField) *problemError {
	return &problemError{
		status: status,
		Problem: contracts.Problem{
			Type:   "about:blank",
			Title:  http.StatusText(status),
			Status: status,
			Detail: detail,
			Code:   code,
			Errors: fields,
		},
	}
}

// fieldError descreve uma falha de validação de campo no formato do contrato.
func fieldError(field, message string) contracts.ProblemField {
	return contracts.ProblemField{Field: field, Message: message}
}

// installProblemErrors substitui a fábrica de erros do Huma para que toda falha
// gerada pelo framework (validação, método, mídia, panic) saia como Problem.
// Deve ser chamada uma única vez, na construção do servidor.
func installProblemErrors() {
	build := func(status int, msg string, errs ...error) huma.StatusError {
		fields := make([]contracts.ProblemField, 0, len(errs))
		for _, err := range errs {
			if err == nil {
				continue
			}
			var detailer huma.ErrorDetailer
			if asDetail, ok := err.(huma.ErrorDetailer); ok {
				detailer = asDetail
				detail := detailer.ErrorDetail()
				location := detail.Location
				if location == "" {
					location = "body"
				}
				fields = append(fields, contracts.ProblemField{Field: location, Message: detail.Message})
				continue
			}
			fields = append(fields, contracts.ProblemField{Field: "body", Message: err.Error()})
		}

		code := codeForStatus(status)
		if msg == "" {
			msg = http.StatusText(status)
		}
		return newProblem(status, code, msg, fields...)
	}

	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		return build(status, msg, errs...)
	}
	huma.NewErrorWithContext = func(_ huma.Context, status int, msg string, errs ...error) huma.StatusError {
		return build(status, msg, errs...)
	}
}

// codeForStatus devolve o código estável de máquina usado pela API.
func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusGone:
		return "gone"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "request_failed"
	}
}

// writeProblem escreve um Problem diretamente no ResponseWriter. Usado por
// middlewares, que ficam fora do pipeline de serialização do Huma.
func writeProblem(w http.ResponseWriter, status int, code, detail string, fields ...contracts.ProblemField) {
	body := contracts.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
		Errors: fields,
	}
	w.Header().Set("Content-Type", contentTypeProblem)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		_, _ = fmt.Fprintf(w, "%s", http.StatusText(status))
	}
}
