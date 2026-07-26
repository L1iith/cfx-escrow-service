package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifierAcceptsValidSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	verifier := NewVerifier("secret", time.Minute, 1024)
	verifier.now = func() time.Time { return now }
	body := []byte(`{"repository":"owner/repo"}`)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	request := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(string(body)))
	request.Header.Set("X-Escrow-Timestamp", timestamp)
	request.Header.Set("X-Escrow-Signature", Sign("secret", timestamp, body))
	response := httptest.NewRecorder()

	verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, response.Code)
	}
}

func TestVerifierRejectsExpiredSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	verifier := NewVerifier("secret", time.Minute, 1024)
	verifier.now = func() time.Time { return now }
	timestamp := strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10)
	body := []byte(`{}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(string(body)))
	request.Header.Set("X-Escrow-Timestamp", timestamp)
	request.Header.Set("X-Escrow-Signature", Sign("secret", timestamp, body))
	response := httptest.NewRecorder()

	verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestVerifierRejectsLargeBody(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	verifier := NewVerifier("secret", time.Minute, 1)
	verifier.now = func() time.Time { return now }
	timestamp := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(string(body)))
	request.Header.Set("X-Escrow-Timestamp", timestamp)
	request.Header.Set("X-Escrow-Signature", Sign("secret", timestamp, body))
	response := httptest.NewRecorder()

	verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected %d, got %d", http.StatusRequestEntityTooLarge, response.Code)
	}
}
