package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	healthOKPayload           = `{"status":"ok"}`
	healthUnconfiguredPayload = `{"status":"unconfigured"}`
	notFoundPayload           = `{"error":"Not found"}`
	unauthorizedPayload       = `{"error":"Unauthorized"}`
	unconfiguredProxyPayload  = `{"error":"Proxy is not configured"}`
	badGatewayPayload         = `{"error":"Bad gateway"}`
)

type proxyHandler struct {
	config      Config
	client      *http.Client
	tokenDigest [sha256.Size]byte
}

func NewHandler(config Config) http.Handler {
	client := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: config.ConnectTimeout}}
	return newHandler(config, client)
}

func newHandler(config Config, client *http.Client) http.Handler {
	return &proxyHandler{
		config:      config,
		client:      client,
		tokenDigest: sha256.Sum256([]byte(config.ProxyToken)),
	}
}

func (handler *proxyHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path, target := normalizedRequestTarget(request)
	if path == "/health" {
		if handler.config.Unconfigured {
			writeJSON(writer, http.StatusServiceUnavailable, healthUnconfiguredPayload)
			return
		}
		writeJSON(writer, http.StatusOK, healthOKPayload)
		return
	}
	if !isV1Path(path) {
		writeJSON(writer, http.StatusNotFound, notFoundPayload)
		return
	}
	if !handler.authorized(request) {
		writeJSON(writer, http.StatusUnauthorized, unauthorizedPayload)
		return
	}
	if handler.config.Unconfigured {
		writeJSON(writer, http.StatusServiceUnavailable, unconfiguredProxyPayload)
		return
	}
	handler.proxyRequest(writer, request, target)
}

func normalizedRequestTarget(request *http.Request) (string, string) {
	path := normalizeEscapedPath(request.URL.EscapedPath())
	target := path
	if request.URL.RawQuery != "" {
		target += "?" + request.URL.RawQuery
	}
	return path, target
}

func normalizeEscapedPath(escapedPath string) string {
	if escapedPath == "" {
		return "/"
	}
	parts := strings.Split(escapedPath, "/")
	normalized := make([]string, 0, len(parts))
	for index, part := range parts {
		decodedDots := strings.ReplaceAll(strings.ToLower(part), "%2e", ".")
		switch decodedDots {
		case ".":
			if index == len(parts)-1 && !atRoot(normalized) {
				normalized = append(normalized, "")
			}
		case "..":
			if len(normalized) > 0 {
				normalized = normalized[:len(normalized)-1]
			}
			if index == len(parts)-1 && !atRoot(normalized) {
				normalized = append(normalized, "")
			}
		default:
			normalized = append(normalized, part)
		}
	}
	result := strings.Join(normalized, "/")
	if result == "" {
		return "/"
	}
	return result
}

func atRoot(parts []string) bool {
	return len(parts) == 1 && parts[0] == ""
}

func isV1Path(path string) bool {
	return strings.HasPrefix(path, "/v1/") && path != "/v1/"
}

func (handler *proxyHandler) authorized(request *http.Request) bool {
	if handler.config.ProxyToken == "" {
		return false
	}
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	actual := sha256.Sum256([]byte(strings.TrimPrefix(header, "Bearer ")))
	return subtle.ConstantTimeCompare(handler.tokenDigest[:], actual[:]) == 1
}

func (handler *proxyHandler) proxyRequest(writer http.ResponseWriter, request *http.Request, target string) {
	upstreamResponse, err := handler.doUpstreamRequest(request, target)
	if err != nil {
		writeBadGateway(writer)
		return
	}
	if !handler.forwardResponse(writer, request, upstreamResponse) {
		writeBadGateway(writer)
	}
}

func (handler *proxyHandler) doUpstreamRequest(request *http.Request, target string) (*http.Response, error) {
	ctx, cancel, stopTimeout := preResponseContext(request.Context(), handler.config.ConnectTimeout)
	upstreamRequest, err := handler.newUpstreamRequest(ctx, request, target)
	if err != nil {
		stopTimeout()
		cancel()
		return nil, err
	}
	upstreamResponse, err := handler.client.Do(upstreamRequest)
	stopTimeout()
	if err != nil {
		cancel()
		if upstreamResponse != nil && upstreamResponse.Body != nil {
			_ = upstreamResponse.Body.Close()
		}
		return nil, err
	}
	return upstreamResponse, nil
}

func (handler *proxyHandler) newUpstreamRequest(
	ctx context.Context,
	request *http.Request,
	target string,
) (*http.Request, error) {
	var body io.Reader
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		body = request.Body
	}
	upstreamRequest, err := http.NewRequestWithContext(
		ctx,
		request.Method,
		handler.config.UpstreamOrigin+target,
		body,
	)
	if err != nil {
		return nil, err
	}
	copyRequestHeaders(upstreamRequest.Header, request.Header)
	upstreamRequest.Header.Set("Authorization", "Bearer "+handler.config.APIKey)
	upstreamRequest.Header.Set("Accept-Encoding", "identity")
	return upstreamRequest, nil
}

func (handler *proxyHandler) forwardResponse(
	writer http.ResponseWriter,
	request *http.Request,
	response *http.Response,
) bool {
	defer response.Body.Close()
	preparedBody, err := prepareResponseBody(
		response,
		request.Method,
		handler.config.IdleTimeout,
	)
	if err != nil {
		return false
	}
	copyResponseHeaders(
		writer.Header(),
		response.Header,
		response.StatusCode,
		request.Method,
		preparedBody.decoded,
		preparedBody.dropEncoding,
	)
	writer.WriteHeader(response.StatusCode)
	if err := copyStreaming(writer, preparedBody.body, handler.config.IdleTimeout); err != nil {
		panic(http.ErrAbortHandler)
	}
	return true
}

func preResponseContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc, func()) {
	if timeout <= 0 {
		return parent, func() {}, func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	timer := time.AfterFunc(timeout, cancel)
	return ctx, cancel, func() { timer.Stop() }
}

func writeBadGateway(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusBadGateway, badGatewayPayload)
}

func writeJSON(writer http.ResponseWriter, status int, payload string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	writer.WriteHeader(status)
	_, _ = io.WriteString(writer, payload)
}
