package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var strippedHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"host":                true,
	"content-length":      true,
}

var strippedResponseHeaders = map[string]bool{
	"content-encoding": true,
}

func NewHandler(config Config) http.Handler {
	client := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: config.ConnectTimeout}}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/health" {
			if config.Unconfigured {
				writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "unconfigured"})
				return
			}
			writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		if !isV1Path(request.URL.Path) {
			writeJSON(writer, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		if !authorized(request, config.ProxyToken) {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		if config.Unconfigured {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "Proxy is not configured"})
			return
		}
		proxyRequest(writer, request, config, client)
	})
}

func isV1Path(path string) bool {
	return strings.HasPrefix(path, "/v1/") && path != "/v1/"
}

func authorized(request *http.Request, token string) bool {
	if token == "" {
		return false
	}
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	expected := sha256.Sum256([]byte(token))
	actual := sha256.Sum256([]byte(strings.TrimPrefix(header, "Bearer ")))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}

func copyRequestHeaders(destination, source http.Header) {
	for name, values := range source {
		if strippedHeaders[strings.ToLower(name)] {
			continue
		}
		destination.Set(name, strings.Join(values, ", "))
	}
}

func copyResponseHeaders(destination, source http.Header) {
	for name, values := range source {
		lowerName := strings.ToLower(name)
		if strippedHeaders[lowerName] || strippedResponseHeaders[lowerName] {
			continue
		}
		destination[name] = append([]string(nil), values...)
	}
}

func proxyRequest(writer http.ResponseWriter, request *http.Request, config Config, client *http.Client) {
	target := config.UpstreamOrigin + request.URL.RequestURI()
	var body io.Reader
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		body = request.Body
	}
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, target, body)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": "Bad gateway"})
		return
	}
	copyRequestHeaders(upstreamRequest.Header, request.Header)
	upstreamRequest.Header.Set("Authorization", "Bearer "+config.APIKey)
	upstreamRequest.Header.Set("Accept-Encoding", "identity")
	upstreamResponse, err := client.Do(upstreamRequest)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": "Bad gateway"})
		return
	}
	defer upstreamResponse.Body.Close()
	copyResponseHeaders(writer.Header(), upstreamResponse.Header)
	writer.WriteHeader(upstreamResponse.StatusCode)
	copyStreaming(writer, upstreamResponse.Body, config.IdleTimeout)
}

func copyStreaming(writer http.ResponseWriter, source io.ReadCloser, idleTimeout time.Duration) {
	flusher, canFlush := writer.(http.Flusher)
	var idleTimer *time.Timer
	if idleTimeout > 0 {
		idleTimer = time.AfterFunc(idleTimeout, func() {
			_ = source.Close()
		})
		defer idleTimer.Stop()
	}
	buffer := make([]byte, 32*1024)
	for {
		count, err := source.Read(buffer)
		if count > 0 {
			_, _ = writer.Write(buffer[:count])
			if canFlush {
				flusher.Flush()
			}
			if idleTimer != nil {
				idleTimer.Reset(idleTimeout)
			}
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	writer.WriteHeader(status)
	_, _ = writer.Write(payload)
}
