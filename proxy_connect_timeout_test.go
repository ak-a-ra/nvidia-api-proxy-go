package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConnectTimeoutReturnsBadGateway(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		upstream.Close()
	})

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: 100 * time.Millisecond,
		IdleTimeout:    0,
	}))
	t.Cleanup(proxy.Close)

	client := &http.Client{Timeout: 2 * time.Second}
	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.StatusCode)
	}
	if elapsed := time.Since(started); elapsed < 80*time.Millisecond {
		t.Fatalf("elapsed = %s, want at least 80ms", elapsed)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"error":"Bad gateway"}` {
		t.Fatalf("body = %q", body)
	}
}
