package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// fastPolicy usa intervalos curtos para o teste não demorar.
func fastPolicy() Policy {
	return Policy{
		Attempts: 3,
		Timeout:  200 * time.Millisecond,
		Delays:   []time.Duration{10 * time.Millisecond, 20 * time.Millisecond},
	}
}

func testEvent() contracts.WebhookInboundEvent {
	return contracts.WebhookInboundEvent{EventID: "evt-1", EventType: contracts.EventMessageReceived}
}

func TestPolicyValidate(t *testing.T) {
	if err := DefaultPolicy.Validate(); err != nil {
		t.Fatalf("DefaultPolicy inválida: %v", err)
	}
	if DefaultPolicy.Attempts != 6 {
		t.Errorf("Attempts = %d; esperado 6 (1 inicial + 5 reentregas)", DefaultPolicy.Attempts)
	}
	wantDelays := []time.Duration{5 * time.Second, 15 * time.Second, 25 * time.Second, 45 * time.Second, 60 * time.Second}
	if len(DefaultPolicy.Delays) != len(wantDelays) {
		t.Fatalf("Delays = %v; esperado %v", DefaultPolicy.Delays, wantDelays)
	}
	for i, want := range wantDelays {
		if DefaultPolicy.Delays[i] != want {
			t.Errorf("Delays[%d] = %v; esperado %v", i, DefaultPolicy.Delays[i], want)
		}
	}

	bad := []Policy{
		{Attempts: 0, Timeout: time.Second},
		{Attempts: 2, Timeout: 0, Delays: []time.Duration{time.Second}},
		{Attempts: 3, Timeout: time.Second, Delays: []time.Duration{time.Second}},
	}
	for i, p := range bad {
		if err := p.Validate(); err == nil {
			t.Errorf("política inválida %d passou na validação", i)
		}
	}
}

func TestDeliverSucceedsFirstAttempt(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("X-GabeWhats-Signature") == "" {
			t.Error("assinatura ausente")
		}
		if r.Header.Get("X-GabeWhats-Delivery") != "evt-1" {
			t.Errorf("header de entrega = %q", r.Header.Get("X-GabeWhats-Delivery"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d, err := New(fastPolicy(), server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}

	result := d.Deliver(context.Background(), contracts.WebhookConfig{URL: server.URL}, testEvent())
	if !result.Succeeded() {
		t.Fatalf("entrega deveria ter sucesso: %+v", result)
	}
	if result.Attempts != 1 {
		t.Errorf("Attempts = %d; esperado 1", result.Attempts)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("chamadas = %d; esperado 1", got)
	}
}

func TestDeliverRetriesUntilSuccessAndRespectsDelays(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d, err := New(fastPolicy(), server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}

	var slept []time.Duration
	d.sleep = func(_ context.Context, delay time.Duration) error {
		slept = append(slept, delay)
		return nil
	}

	result := d.Deliver(context.Background(), contracts.WebhookConfig{URL: server.URL}, testEvent())
	if !result.Succeeded() {
		t.Fatalf("entrega deveria ter sucesso na 3ª tentativa: %+v", result)
	}
	if result.Attempts != 3 {
		t.Errorf("Attempts = %d; esperado 3", result.Attempts)
	}
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	if len(slept) != len(want) {
		t.Fatalf("esperas = %v; esperado %v", slept, want)
	}
	for i := range want {
		if slept[i] != want[i] {
			t.Errorf("espera[%d] = %v; esperado %v", i, slept[i], want[i])
		}
	}
}

func TestDeliverExhaustsAttempts(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	d, err := New(fastPolicy(), server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d.sleep = func(context.Context, time.Duration) error { return nil }

	result := d.Deliver(context.Background(), contracts.WebhookConfig{URL: server.URL}, testEvent())
	if result.Succeeded() {
		t.Fatal("entrega não deveria ter sucesso")
	}
	if result.Attempts != 3 {
		t.Errorf("Attempts = %d; esperado 3", result.Attempts)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("chamadas = %d; esperado 3", got)
	}
}

func TestDeliverTimesOutSlowReceiver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	policy := fastPolicy()
	policy.Attempts = 2
	policy.Delays = []time.Duration{0}

	d, err := New(policy, server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d.sleep = func(context.Context, time.Duration) error { return nil }

	result := d.Deliver(context.Background(), contracts.WebhookConfig{URL: server.URL}, testEvent())
	if result.Succeeded() {
		t.Fatal("receptor lento deveria estourar o timeout")
	}
	if result.Attempts != 2 {
		t.Errorf("Attempts = %d; esperado 2", result.Attempts)
	}
}

func TestDeliverSignsBody(t *testing.T) {
	const secret = "segredo-hmac"
	var gotSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-GabeWhats-Signature")
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d, _ := New(fastPolicy(), server.Client(), nil)
	event := testEvent()
	result := d.Deliver(context.Background(), contracts.WebhookConfig{URL: server.URL, Secret: secret}, event)
	if !result.Succeeded() {
		t.Fatalf("entrega falhou: %+v", result)
	}

	body, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSig != want {
		t.Errorf("assinatura = %q; esperado %q", gotSig, want)
	}
}

func TestDeliverWithoutURLFails(t *testing.T) {
	d, _ := New(fastPolicy(), nil, nil)
	result := d.Deliver(context.Background(), contracts.WebhookConfig{}, testEvent())
	if result.Succeeded() {
		t.Fatal("entrega sem URL deveria falhar")
	}
}
