package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyCopiesHeadersAndPreservesResponseCookies(t *testing.T) {
	var receivedContentType string
	var receivedContentEncoding string
	var receivedRequestCookie string
	var receivedConnection string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedContentType = request.Header.Get("Content-Type")
		receivedContentEncoding = request.Header.Get("Content-Encoding")
		receivedRequestCookie = request.Header.Get("Set-Cookie")
		receivedConnection = request.Header.Get("Connection")
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

	if receivedContentType != "application/json" {
		t.Fatalf("upstream Content-Type = %q", receivedContentType)
	}
	if receivedContentEncoding != "gzip" {
		t.Fatalf("upstream Content-Encoding = %q", receivedContentEncoding)
	}
	if receivedRequestCookie != "sid=abc; Path=/, theme=dark; Path=/" {
		t.Fatalf("upstream Set-Cookie = %q", receivedRequestCookie)
	}
	if receivedConnection != "" {
		t.Fatalf("upstream Connection = %q", receivedConnection)
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
