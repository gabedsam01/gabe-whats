package whatsapp

import (
	"errors"
	"testing"
)

func TestRetryableSendError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"limite de taxa", errors.New("server returned error 429"), true},
		{"indisponivel", errors.New("server returned error 503"), true},
		{"conexao caiu", errors.New("connection reset by peer"), true},
		{"timeout", errors.New("i/o timeout"), true},
		{"entrada invalida", errors.New("destinatário vazio"), false},
		{"erro de negocio", errors.New("server returned error 473"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryableSendError(tc.err); got != tc.want {
				t.Errorf("retryableSendError(%v) = %v; esperado %v", tc.err, got, tc.want)
			}
		})
	}
}
