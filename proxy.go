package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func NewHandler(config Config) http.Handler {
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
		proxyRequest(writer, request, config)
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

func proxyRequest(writer http.ResponseWriter, request *http.Request, config Config) {
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
	upstreamRequest.Header.Set("Authorization", "Bearer "+config.APIKey)
	upstreamResponse, err := http.DefaultClient.Do(upstreamRequest)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": "Bad gateway"})
		return
	}
	defer upstreamResponse.Body.Close()
	for name, values := range upstreamResponse.Header {
		writer.Header()[name] = append([]string(nil), values...)
	}
	writer.WriteHeader(upstreamResponse.StatusCode)
	copyStreaming(writer, upstreamResponse.Body)
}

func copyStreaming(writer http.ResponseWriter, source io.Reader) {
	flusher, canFlush := writer.(http.Flusher)
	buffer := make([]byte, 32*1024)
	for {
		count, err := source.Read(buffer)
		if count > 0 {
			_, _ = writer.Write(buffer[:count])
			if canFlush {
				flusher.Flush()
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
