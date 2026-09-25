package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestBodylessGzipResponseKeepsStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header: http.Header{
				"Content-Encoding": []string{"gzip"},
				"Content-Length":   []string{"7"},
			},
			Body: http.NoBody,
		}, nil
	})}
	handler := newHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://upstream"}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if encoding := response.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("Content-Encoding = %q", encoding)
	}
	if length := response.Header().Get("Content-Length"); length != "" {
		t.Fatalf("Content-Length = %q", length)
	}
}

func TestResetContentDropsStaleContentLength(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusResetContent,
			Header:     http.Header{"Content-Length": []string{"7"}},
			Body:       io.NopCloser(strings.NewReader("ignored")),
		}, nil
	})}
	handler := newHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://upstream"}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusResetContent {
		t.Fatalf("status = %d, want 205", response.Code)
	}
	if length := response.Header().Get("Content-Length"); length != "" {
		t.Fatalf("Content-Length = %q", length)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", response.Body.String())
	}
}

func TestUnsupportedUpstreamEncodingPassesThrough(t *testing.T) {
	payload := []byte("encoded-bytes")
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Encoding", "br")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(payload)
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
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

	if encoding := response.Header.Get("Content-Encoding"); encoding != "br" {
		t.Fatalf("Content-Encoding = %q, want br", encoding)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, payload) {
		t.Fatalf("body = %q, want %q", body, payload)
	}
}

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

	type receivedRequest struct {
		body     []byte
		encoding string
	}
	received := make(chan receivedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		received <- receivedRequest{
			body:     body,
			encoding: request.Header.Get("Content-Encoding"),
		}
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

	got := <-received
	if got.encoding != "gzip" {
		t.Fatalf("upstream Content-Encoding = %q", got.encoding)
	}
	if !bytes.Equal(got.body, compressed.Bytes()) {
		t.Fatalf("upstream body length = %d, want %d", len(got.body), compressed.Len())
	}
}

func TestInvalidUpstreamContentLengthIsDropped(t *testing.T) {
	payload := []byte("body")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Length": []string{"not-a-number"}},
			Body:       io.NopCloser(bytes.NewReader(payload)),
		}, nil
	})}
	handler := newHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://upstream"}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if values := response.Header().Values("Content-Length"); len(values) != 0 {
		t.Fatalf("Content-Length = %#v, want omitted", values)
	}
	if got := response.Body.String(); got != string(payload) {
		t.Fatalf("body = %q, want %q", got, payload)
	}
}
