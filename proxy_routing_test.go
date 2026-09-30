// story: e01s03
// story: e02s01
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRoutingPreservesPathAndQuery(t *testing.T) {
	type receivedRequest struct {
		path          string
		authorization string
	}
	received := make(chan receivedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- receivedRequest{
			path:          request.URL.RequestURI(),
			authorization: request.Header.Get("Authorization"),
		}
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
	got := <-received
	if got.path != "/v1/chat/completions?stream=true" {
		t.Fatalf("upstream path = %q", got.path)
	}
	if got.authorization != "Bearer sk-test" {
		t.Fatalf("upstream authorization = %q", got.authorization)
	}
}

func TestProxyRoutingNormalizesDotSegments(t *testing.T) {
	tests := []struct {
		name       string
		requestURI string
		wantStatus int
		wantPath   string
	}{
		{name: "escape", requestURI: "/v1/../admin", wantStatus: http.StatusNotFound},
		{name: "encoded escape", requestURI: "/v1/%2e%2e/admin", wantStatus: http.StatusNotFound},
		{name: "internal", requestURI: "/v1/a/../models?stream=true", wantStatus: http.StatusOK, wantPath: "/v1/models?stream=true"},
		{name: "encoded slash", requestURI: "/v1/a%2Fb", wantStatus: http.StatusOK, wantPath: "/v1/a%2Fb"},
		{name: "encoded slash before dot", requestURI: "/v1/a%2F../admin", wantStatus: http.StatusOK, wantPath: "/v1/a%2F../admin"},
		{name: "double slash", requestURI: "/v1//models", wantStatus: http.StatusOK, wantPath: "/v1//models"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			received := make(chan string, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				received <- request.URL.RequestURI()
				writer.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(upstream.Close)

			handler := NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL})
			request := httptest.NewRequest(http.MethodGet, test.requestURI, nil)
			request.Header.Set("Authorization", "Bearer pt-test")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantPath == "" {
				select {
				case path := <-received:
					t.Fatalf("unexpected upstream request %q", path)
				default:
				}
				return
			}
			if path := <-received; path != test.wantPath {
				t.Fatalf("upstream path = %q, want %q", path, test.wantPath)
			}
		})
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
