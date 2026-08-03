package httporacle

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestHTTPReceiptCapturesAllowlistedRedactedFacts(t *testing.T) {
	t.Setenv("HTTP_ORACLE_TOKEN", "secret-token")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "secret-token" {
			http.Error(response, "missing token", http.StatusUnauthorized)
			return
		}
		response.Header().Set("X-Echo", "value=secret-token")
		response.Header().Set("Set-Cookie", "private-cookie")
		response.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(response, "secret-token:"+strings.Repeat("x", 40))
	}))
	defer server.Close()
	store := artifact.NewStore(t.TempDir())
	result, err := Execute(context.Background(), store, Spec{
		Method: http.MethodGet, URL: server.URL, Timeout: time.Second, MaxBodyBytes: 24,
		SecretHeaderHandles:    map[string]string{"Authorization": "HTTP_ORACLE_TOKEN"},
		CaptureResponseHeaders: []string{"X-Echo"}, SideEffects: []string{"network:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := store.Read(result.Output.Body)
	config, _ := store.Read(result.Receipt.Configuration)
	if strings.Contains(string(body), "secret-token") || strings.Contains(string(config), "secret-token") {
		t.Fatal("resolved HTTP secret persisted")
	}
	if result.Output.StatusCode != http.StatusCreated || result.Output.Headers["X-Echo"][0] != "value=[REDACTED]" || result.Output.Headers["Set-Cookie"] != nil {
		t.Fatalf("output = %#v", result.Output)
	}
	if string(body) != "[REDACTED]:xxxxxxxxxxxxx" || !result.Output.Truncated {
		t.Fatalf("body=%q truncated=%v", body, result.Output.Truncated)
	}
}

func TestHTTPRejectsAmbiguousBodyAndUnboundedResponse(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	if _, err := Execute(context.Background(), store, Spec{Method: "GET", URL: "relative", Timeout: time.Second, MaxBodyBytes: 1, SideEffects: []string{"none"}}); err == nil {
		t.Fatal("relative URL was accepted")
	}
	if _, err := Execute(context.Background(), store, Spec{Method: "GET", URL: "http://example.invalid", Timeout: time.Second, MaxBodyBytes: 0, SideEffects: []string{"none"}}); err == nil {
		t.Fatal("unbounded response was accepted")
	}
	if _, err := Execute(context.Background(), store, Spec{
		Method: "GET", URL: "http://example.invalid", Timeout: time.Second, MaxBodyBytes: 1,
		PublicHeaders: map[string]string{"Authorization": "public"}, SecretHeaderHandles: map[string]string{"authorization": "HANDLE"}, SideEffects: []string{"none"},
	}); err == nil {
		t.Fatal("overlapping public and secret headers were accepted")
	}
}

func TestHTTPRedactsOverlappingSecretsWithoutSuffixLeak(t *testing.T) {
	t.Setenv("HTTP_SHORT_SECRET", "token")
	t.Setenv("HTTP_LONG_SECRET", "token-suffix")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(response, "token-suffix")
	}))
	defer server.Close()
	store := artifact.NewStore(t.TempDir())
	result, err := Execute(context.Background(), store, Spec{
		Method: http.MethodGet, URL: server.URL, Timeout: time.Second, MaxBodyBytes: 64,
		SecretHeaderHandles: map[string]string{"X-Short": "HTTP_SHORT_SECRET", "X-Long": "HTTP_LONG_SECRET"}, SideEffects: []string{"network:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := store.Read(result.Output.Body)
	if string(body) != "[REDACTED]" || strings.Contains(string(body), "suffix") {
		t.Fatalf("overlapping secret body = %q", body)
	}
}
