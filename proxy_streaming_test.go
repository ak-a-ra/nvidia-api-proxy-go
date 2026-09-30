// story: e01s04
// story: e02s01
package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type shortResponseWriter struct {
	header http.Header
	status int
	body   []byte
}

func (writer *shortResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *shortResponseWriter) WriteHeader(status int) {
	writer.status = status
}

func (writer *shortResponseWriter) Write(buffer []byte) (int, error) {
	writer.body = append(writer.body, buffer...)
	return len(buffer) - 1, nil
}

func TestCopyStreamingRejectsShortWrites(t *testing.T) {
	writer := &shortResponseWriter{header: make(http.Header)}
	source := io.NopCloser(strings.NewReader("payload"))

	if err := copyStreaming(writer, source, 0); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("copyStreaming() error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestStreamingDeliversFirstChunkBeforeStreamEnds(t *testing.T) {
	releaseSecond := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: first\n\n"))
		writer.(http.Flusher).Flush()
		<-releaseSecond
		_, _ = writer.Write([]byte("data: second\n\n"))
		writer.(http.Flusher).Flush()
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
	}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	firstChunk := make(chan []byte, 1)
	go func() {
		buffer := make([]byte, len("data: first\n\n"))
		_, _ = io.ReadFull(response.Body, buffer)
		firstChunk <- buffer
	}()

	select {
	case chunk := <-firstChunk:
		if string(chunk) != "data: first\n\n" {
			t.Fatalf("first chunk = %q", chunk)
		}
	case <-time.After(500 * time.Millisecond):
		close(releaseSecond)
		t.Fatal("first chunk was buffered until the upstream stream ended")
	}
	close(releaseSecond)
}
