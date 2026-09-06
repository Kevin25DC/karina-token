package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPostsExpectedShape(t *testing.T) {
	var got Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := Payload{
		Content: "Karina · Claude: Semanal al 87%",
		Text:    "Karina · Claude: Semanal al 87%",
		Karina: Details{
			Provider:        "anthropic",
			ProviderDisplay: "Claude",
			Window:          "Semanal (7 días)",
			Percent:         87,
			FiredAt:         "2026-09-11T00:00:00Z",
		},
	}

	if err := Send(context.Background(), srv.Client(), srv.URL, payload); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got.Content != payload.Content || got.Text != payload.Text {
		t.Fatalf("got %+v, want %+v", got, payload)
	}
	if got.Karina.Percent != 87 {
		t.Fatalf("got percent %v, want 87", got.Karina.Percent)
	}
}

func TestSendBlankURLIsNoop(t *testing.T) {
	if err := Send(context.Background(), nil, "", Payload{}); err != nil {
		t.Fatalf("Send() with blank url should be a no-op, got %v", err)
	}
	if err := Send(context.Background(), nil, "   ", Payload{}); err != nil {
		t.Fatalf("Send() with blank url should be a no-op, got %v", err)
	}
}

func TestSendNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := Send(context.Background(), srv.Client(), srv.URL, Payload{}); err == nil {
		t.Fatal("Send() error = nil, want error on non-2xx status")
	}
}
