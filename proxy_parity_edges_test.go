package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestUpstreamGzipResponseIsDecoded(t *testing.T) {
	jsonBody := []byte(`{"ok":true}`)
	var compressed bytes.Buffer
	encoder := gzip.NewWriter(&compressed)
	if _, err := encoder.Write(jsonBody); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Encoding", "gzip")
		writer.Header().Set("Content-Length", strconv.Itoa(compressed.Len()))
		_, _ = writer.Write(compressed.Bytes())
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
	}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.Header.Get("Content-Encoding") != "" {
		t.Fatalf("Content-Encoding = %q", response.Header.Get("Content-Encoding"))
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, jsonBody) {
		t.Fatalf("body = %q, want %q", body, jsonBody)
	}
}

func TestGzipRequestBodyReachesUpstreamUnchanged(t *testing.T) {
	payload := []byte(`{"model":"nemotron"}`)
	var compressed bytes.Buffer
	encoder := gzip.NewWriter(&compressed)
	if _, err := encoder.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	var receivedBody []byte
	var receivedEncoding string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedEncoding = request.Header.Get("Content-Encoding")
		receivedBody, _ = io.ReadAll(request.Body)
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
	}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodPost, proxy.URL+"/v1/chat/completions", bytes.NewReader(compressed.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if receivedEncoding != "gzip" {
		t.Fatalf("upstream Content-Encoding = %q", receivedEncoding)
	}
	if !bytes.Equal(receivedBody, compressed.Bytes()) {
		t.Fatalf("upstream body length = %d, want %d", len(receivedBody), compressed.Len())
	}
}
