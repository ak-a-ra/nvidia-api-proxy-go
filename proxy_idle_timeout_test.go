package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type stalledReadCloser struct {
	reader *bytes.Reader
	closed chan struct{}
	once   sync.Once
}

func (body *stalledReadCloser) Read(buffer []byte) (int, error) {
	if body.reader.Len() > 0 {
		return body.reader.Read(buffer)
	}
	<-body.closed
	return 0, io.ErrClosedPipe
}

func (body *stalledReadCloser) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

func TestIdleTimeoutClosesTransportBodyForGzipStream(t *testing.T) {
	var compressed bytes.Buffer
	encoder := gzip.NewWriter(&compressed)
	if _, err := encoder.Write([]byte("part1")); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	upstreamBody := &stalledReadCloser{reader: bytes.NewReader(compressed.Bytes()), closed: make(chan struct{})}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Encoding": []string{"gzip"}},
			Body:       upstreamBody,
		}, nil
	})}
	proxy := httptest.NewServer(newHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: "http://upstream",
		IdleTimeout:    100 * time.Millisecond,
	}, client))
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

	first := make([]byte, len("part1"))
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 1)
	_, streamErr := response.Body.Read(buffer)
	if streamErr == nil || errors.Is(streamErr, io.EOF) {
		t.Fatalf("gzip idle stream error = %v", streamErr)
	}
	select {
	case <-upstreamBody.closed:
	default:
		t.Fatal("gzip idle timeout did not close the transport body")
	}
}

func TestIdleTimeoutCutsStalledStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("part1"))
		writer.(http.Flusher).Flush()
		<-request.Context().Done()
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: 0,
		IdleTimeout:    100 * time.Millisecond,
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

	first := make([]byte, len("part1"))
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	secondRead := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1)
		_, readErr := response.Body.Read(buffer)
		secondRead <- readErr
	}()

	select {
	case err := <-secondRead:
		if err == nil || errors.Is(err, io.EOF) {
			t.Fatalf("stalled stream error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("stalled stream remained open after the idle timeout")
	}
}

func TestIdleTimeoutResetsOnChunks(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		for _, chunk := range []string{"part1", "part2", "part3"} {
			_, _ = writer.Write([]byte(chunk))
			writer.(http.Flusher).Flush()
			time.Sleep(60 * time.Millisecond)
		}
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: 0,
		IdleTimeout:    100 * time.Millisecond,
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

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "part1part2part3" {
		t.Fatalf("body = %q", body)
	}
}
