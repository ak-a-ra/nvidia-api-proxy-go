package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPostBodyQueryAndAuthReplacement(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody string
	var gotAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		gotMethod = request.Method
		gotPath = request.URL.RequestURI()
		gotBody = string(body)
		gotAuthorization = request.Header.Get("Authorization")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)
	request, err := http.NewRequest(http.MethodPost, proxy.URL+"/v1/chat/completions?stream=true", strings.NewReader(`{"model":"nemotron"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if gotMethod != http.MethodPost || gotPath != "/v1/chat/completions?stream=true" {
		t.Fatalf("upstream method/path = %q %q", gotMethod, gotPath)
	}
	if gotBody != `{"model":"nemotron"}` {
		t.Fatalf("upstream body = %q", gotBody)
	}
	if gotAuthorization != "Bearer sk-test" {
		t.Fatalf("upstream authorization = %q", gotAuthorization)
	}
}

func TestLoadConfigIgnoresBaseURLPath(t *testing.T) {
	t.Setenv("NVIDIA_BASE_URL", "https://integrate.api.nvidia.com/v1")
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.UpstreamOrigin != "https://integrate.api.nvidia.com" {
		t.Fatalf("UpstreamOrigin = %q", config.UpstreamOrigin)
	}
}

func TestUnauthenticatedRequestIsRejectedBeforeUnconfigured(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalled = true
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{ProxyToken: "pt-test", UpstreamOrigin: upstream.URL, Unconfigured: true}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
	if upstreamCalled {
		t.Fatal("unauthenticated request reached upstream")
	}
}

func TestUpstreamStatusPassesThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusTeapot)
		_, _ = writer.Write([]byte("upstream"))
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/missing", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", response.StatusCode)
	}
}

func TestMidStreamFailureKeepsProxyAvailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("partial"))
		writer.(http.Flusher).Flush()
		connection, _, err := writer.(http.Hijacker).Hijack()
		if err == nil {
			_ = connection.Close()
		}
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(proxy.URL + "/v1/models")
	if response != nil {
		_, _ = io.ReadAll(response.Body)
		_ = response.Body.Close()
	}
	_ = err
	health, healthErr := client.Get(proxy.URL + "/health")
	if healthErr != nil {
		t.Fatal(healthErr)
	}
	defer health.Body.Close()
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", health.StatusCode)
	}
}

func TestUnreachableUpstreamReturnsSafeBadGateway(t *testing.T) {
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://127.0.0.1:1"}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if string(body) != `{"error":"Bad gateway"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestIdleTimeoutDisabledAllowsSlowStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("slow"))
		writer.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond)
		_, _ = writer.Write([]byte(" done"))
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL, IdleTimeout: 0}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "slow done" {
		t.Fatalf("body = %q", body)
	}
}

func TestNoContentUpstreamPassesThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.StatusCode)
	}
	if body, _ := io.ReadAll(response.Body); len(body) != 0 {
		t.Fatalf("body = %q", body)
	}
}

func TestContentLengthPassesThrough(t *testing.T) {
	body := []byte(`{"data":[1,2,3]}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = writer.Write(body)
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("Content-Length") != "16" {
		t.Fatalf("Content-Length = %q", response.Header.Get("Content-Length"))
	}
}

func TestWrongAuthTokenIsRejected(t *testing.T) {
	proxy := httptest.NewServer(NewHandler(Config{ProxyToken: "pt-secret", UpstreamOrigin: "http://127.0.0.1:1"}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer wrong-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

func TestLongStreamSurvivesConnectTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		for _, chunk := range []string{"part1-", "part2-", "part3"} {
			_, _ = writer.Write([]byte(chunk))
			writer.(http.Flusher).Flush()
			time.Sleep(120 * time.Millisecond)
		}
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL, ConnectTimeout: 50 * time.Millisecond, IdleTimeout: 0}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "part1-part2-part3" {
		t.Fatalf("body = %q", body)
	}
}

func TestHeadPreservesContentLength(t *testing.T) {
	body := []byte(`{"data":[1,2,3]}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Content-Length", "16")
		if request.Method != http.MethodHead {
			_, _ = writer.Write(body)
		}
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)
	request, _ := http.NewRequest(http.MethodHead, proxy.URL+"/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("Content-Length") != "16" {
		t.Fatalf("Content-Length = %q", response.Header.Get("Content-Length"))
	}
	if body, _ := io.ReadAll(response.Body); len(body) != 0 {
		t.Fatalf("HEAD body = %q", body)
	}
}
