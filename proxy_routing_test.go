package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRoutingPreservesPathAndQuery(t *testing.T) {
	var receivedPath string
	var receivedAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.RequestURI()
		receivedAuthorization = request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	handler := NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions?stream=true", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if receivedPath != "/v1/chat/completions?stream=true" {
		t.Fatalf("upstream path = %q", receivedPath)
	}
	if receivedAuthorization != "Bearer sk-test" {
		t.Fatalf("upstream authorization = %q", receivedAuthorization)
	}
}

func TestProxyRoutingRejectsUnauthorizedRequests(t *testing.T) {
	handler := NewHandler(Config{ProxyToken: "pt-test", UpstreamOrigin: "http://127.0.0.1:1"})

	for _, authorization := range []string{"", "Bearer wrong-token", "Basic pt-test"} {
		request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q: status = %d, want 401", authorization, response.Code)
		}
	}
}

func TestProxyRoutingRejectsUnconfiguredProxyAfterAuthentication(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Fatal("unconfigured proxy must not contact upstream")
	}))
	t.Cleanup(upstream.Close)

	handler := NewHandler(Config{
		APIKey:         " ",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		Unconfigured:   true,
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"error":"Proxy is not configured"}` {
		t.Fatalf("body = %q", body)
	}
}
