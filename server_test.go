// story: e01s02
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthConfigured(t *testing.T) {
	handler := NewHandler(Config{Unconfigured: false})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := response.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("body = %q", got)
	}
}

func TestHealthUnconfigured(t *testing.T) {
	handler := NewHandler(Config{Unconfigured: true})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if got := response.Body.String(); got != `{"status":"unconfigured"}` {
		t.Fatalf("body = %q", got)
	}
}

func TestLocalNotFoundResponses(t *testing.T) {
	handler := NewHandler(Config{Unconfigured: false})
	for _, path := range []string{"/v1", "/v1/", "/foo"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", response.Code)
			}
			if got := response.Header().Get("Content-Length"); got != "21" {
				t.Fatalf("Content-Length = %q, want 21", got)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(body); got != `{"error":"Not found"}` {
				t.Fatalf("body = %q", got)
			}
		})
	}
}
