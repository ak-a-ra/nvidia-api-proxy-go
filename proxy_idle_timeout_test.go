package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
		if err == nil {
			t.Fatal("stalled stream read succeeded")
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
