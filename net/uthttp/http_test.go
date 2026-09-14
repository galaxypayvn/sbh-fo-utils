package uthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ctxKey struct{}

type contextCapturingTransport struct {
	base http.RoundTripper
	got  context.Context
}

func (t *contextCapturingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.got = r.Context()
	return t.base.RoundTrip(r)
}

func TestSendHTTPRequest_AttachesContextToOutboundRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	t.Cleanup(server.Close)

	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-marker")
	transport := &contextCapturingTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: transport}

	_, err := SendHTTPRequest[map[string]any](ctx, client, HTTPRequest{
		Method: http.MethodGet,
		URL:    server.URL,
		Silent: true,
	}, DefaultOptions())
	if err != nil {
		t.Fatalf("SendHTTPRequest: %v", err)
	}
	if transport.got == nil {
		t.Fatal("expected outbound request context to be set")
	}
	if transport.got.Value(ctxKey{}) != "trace-marker" {
		t.Fatalf("outbound request context was not propagated from caller ctx")
	}
}

func TestSendHTTPRequest_IgnoresCallerCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	t.Cleanup(server.Close)

	parent := context.WithValue(context.Background(), ctxKey{}, "trace-marker")
	ctx, cancel := context.WithCancel(parent)
	cancel()

	transport := &contextCapturingTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: transport}

	_, err := SendHTTPRequest[map[string]any](ctx, client, HTTPRequest{
		Method: http.MethodGet,
		URL:    server.URL,
		Silent: true,
	}, DefaultOptions())
	if err != nil {
		t.Fatalf("SendHTTPRequest with canceled caller ctx: %v", err)
	}
	if transport.got.Value(ctxKey{}) != "trace-marker" {
		t.Fatal("expected caller context values to still be on the outbound request")
	}
	if err := transport.got.Err(); err != nil {
		t.Fatalf("outbound request context should not be canceled, got %v", err)
	}
}
