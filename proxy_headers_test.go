package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyHeadersStripConnectionNominatedFields(t *testing.T) {
	receivedHop := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHop <- request.Header.Get("X-Hop")
		writer.Header().Set("Connection", "X-Hop")
		writer.Header().Set("X-Hop", "response")
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: upstream.URL}))
	t.Cleanup(proxy.Close)
	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	request.Header.Set("Connection", "X-Hop")
	request.Header.Set("X-Hop", "request")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if hop := <-receivedHop; hop != "" {
		t.Fatalf("upstream X-Hop = %q", hop)
	}
	if hop := response.Header.Get("X-Hop"); hop != "" {
		t.Fatalf("response X-Hop = %q", hop)
	}
}

func TestProxyCopiesHeadersAndPreservesResponseCookies(t *testing.T) {
	type receivedHeaders struct {
		contentType     string
		contentEncoding string
		cookie          string
		connection      string
	}
	received := make(chan receivedHeaders, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- receivedHeaders{
			contentType:     request.Header.Get("Content-Type"),
			contentEncoding: request.Header.Get("Content-Encoding"),
			cookie:          request.Header.Get("Set-Cookie"),
			connection:      request.Header.Get("Connection"),
		}
		writer.Header().Add("Set-Cookie", "a=1; Path=/")
		writer.Header().Add("Set-Cookie", "b=2; Path=/")
		writer.Header().Set("Connection", "keep-alive")
		writer.Header().Set("X-Upstream", "yes")
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("body"))
	}))
	t.Cleanup(upstream.Close)

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
	}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodPost, proxy.URL+"/v1/chat/completions", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Add("Set-Cookie", "sid=abc; Path=/")
	request.Header.Add("Set-Cookie", "theme=dark; Path=/")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	got := <-received
	if got.contentType != "application/json" {
		t.Fatalf("upstream Content-Type = %q", got.contentType)
	}
	if got.contentEncoding != "gzip" {
		t.Fatalf("upstream Content-Encoding = %q", got.contentEncoding)
	}
	if got.cookie != "sid=abc; Path=/, theme=dark; Path=/" {
		t.Fatalf("upstream Set-Cookie = %q", got.cookie)
	}
	if got.connection != "" {
		t.Fatalf("upstream Connection = %q", got.connection)
	}
	if got := response.Header.Values("Set-Cookie"); len(got) != 2 || got[0] != "a=1; Path=/" || got[1] != "b=2; Path=/" {
		t.Fatalf("response Set-Cookie = %#v", got)
	}
	if response.Header.Get("Connection") != "" {
		t.Fatalf("response Connection = %q", response.Header.Get("Connection"))
	}
	if response.Header.Get("X-Upstream") != "yes" {
		t.Fatalf("response X-Upstream = %q", response.Header.Get("X-Upstream"))
	}
}
