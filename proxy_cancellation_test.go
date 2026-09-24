package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDownstreamDisconnectCancelsPendingUpstream(t *testing.T) {
	upstreamStarted := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(upstreamStarted)
		<-request.Context().Done()
		close(upstreamCanceled)
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: 0,
		IdleTimeout:    0,
	}))
	t.Cleanup(proxy.Close)

	clientContext, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(clientContext, http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	requestDone := make(chan struct{})
	go func() {
		_, _ = http.DefaultClient.Do(request)
		close(requestDone)
	}()

	select {
	case <-upstreamStarted:
	case <-time.After(time.Second):
		t.Fatal("upstream did not receive request")
	}
	cancel()
	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("upstream request was not canceled")
	}
	<-requestDone
}

func TestDownstreamDisconnectCancelsActiveStream(t *testing.T) {
	upstreamStarted := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(upstreamStarted)
		_, _ = writer.Write([]byte("part1"))
		writer.(http.Flusher).Flush()
		<-request.Context().Done()
		close(upstreamCanceled)
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: 0,
		IdleTimeout:    0,
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
	select {
	case <-upstreamStarted:
	case <-time.After(time.Second):
		t.Fatal("upstream did not start stream")
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("active upstream stream was not canceled")
	}
}
