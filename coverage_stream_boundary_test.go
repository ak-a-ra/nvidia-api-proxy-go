// story: e02s02
package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	coverageGzipIdleTimeout  = 10 * time.Millisecond
	coverageGzipTestDeadline = 500 * time.Millisecond
)

type coverageErrorWriter struct {
	header http.Header
	err    error
}

func (writer *coverageErrorWriter) Header() http.Header {
	return writer.header
}

func (writer *coverageErrorWriter) WriteHeader(int) {}

func (writer *coverageErrorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

type coverageErrorBody struct {
	sent bool
	err  error
}

func (body *coverageErrorBody) Read(buffer []byte) (int, error) {
	if body.sent {
		return 0, body.err
	}
	body.sent = true
	return copy(buffer, "payload"), body.err
}

func (body *coverageErrorBody) Close() error {
	return nil
}

func TestCoverageResponseRejectsInvalidGzip(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Encoding": []string{"gzip"}},
			Body:       io.NopCloser(strings.NewReader("not gzip")),
		}, nil
	})}
	handler := newHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://upstream"}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.Code)
	}
	if got := response.Body.String(); got != `{"error":"Bad gateway"}` {
		t.Fatalf("body = %q, want %q", got, `{"error":"Bad gateway"}`)
	}
}

func TestCoverageResponseGzipInitializationTimeout(t *testing.T) {
	body := &stalledReadCloser{reader: bytes.NewReader(nil), closed: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Encoding": []string{"gzip"}},
			Body:       body,
		}, nil
	})}
	handler := newHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: "http://upstream",
		IdleTimeout:    coverageGzipIdleTimeout,
	}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(coverageGzipTestDeadline):
		t.Fatal("gzip initialization did not honor the idle timeout")
	}
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.Code)
	}
	if got := response.Body.String(); got != `{"error":"Bad gateway"}` {
		t.Fatalf("body = %q, want %q", got, `{"error":"Bad gateway"}`)
	}
	select {
	case <-body.closed:
	default:
		t.Fatal("gzip initialization timeout did not close the response body")
	}
}

func TestCoverageResponseDropsMultiValueContentLength(t *testing.T) {
	payload := []byte("body")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Length": []string{"4", "5"}},
			Body:       io.NopCloser(bytes.NewReader(payload)),
		}, nil
	})}
	handler := newHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://upstream"}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer pt-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if values := response.Header().Values("Content-Length"); len(values) != 0 {
		t.Fatalf("Content-Length = %#v, want omitted", values)
	}
	if got := response.Body.String(); got != string(payload) {
		t.Fatalf("body = %q, want %q", got, payload)
	}
}

func TestCoverageStreamReturnsWriterError(t *testing.T) {
	wantErr := errors.New("downstream write failed")
	writer := &coverageErrorWriter{header: make(http.Header), err: wantErr}
	source := io.NopCloser(strings.NewReader("payload"))

	if err := copyStreaming(writer, source, 0); !errors.Is(err, wantErr) {
		t.Fatalf("copyStreaming() error = %v, want %v", err, wantErr)
	}
}

func TestCoverageStreamReturnsSourceError(t *testing.T) {
	wantErr := errors.New("upstream read failed")
	source := &coverageErrorBody{err: wantErr}
	writer := httptest.NewRecorder()

	if err := copyStreaming(writer, source, 0); !errors.Is(err, wantErr) {
		t.Fatalf("copyStreaming() error = %v, want %v", err, wantErr)
	}
	if got := writer.Body.String(); got != "payload" {
		t.Fatalf("body = %q, want payload", got)
	}
}
