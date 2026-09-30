// story: e02s02
package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverageConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		base string
		port string
		want string
	}{
		{
			name: "missing host",
			base: "http://",
			want: "NVIDIA_BASE_URL is not a valid URL: http://",
		},
		{
			name: "non-numeric port",
			base: "http://127.0.0.1:8080",
			port: "abc",
			want: "PORT must be a valid port: abc",
		},
		{
			name: "negative port",
			base: "http://127.0.0.1:8080",
			port: "-1",
			want: "PORT must be a valid port: -1",
		},
		{
			name: "port above range",
			base: "http://127.0.0.1:8080",
			port: "65536",
			want: "PORT must be a valid port: 65536",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NVIDIA_BASE_URL", test.base)
			t.Setenv("NVIDIA_API_KEY", "sk-test")
			t.Setenv("PROXY_AUTH_TOKEN", "pt-test")
			t.Setenv("PORT", test.port)
			t.Setenv("UPSTREAM_CONNECT_TIMEOUT_SECONDS", "")
			t.Setenv("UPSTREAM_IDLE_TIMEOUT_SECONDS", "")

			_, err := LoadConfig()
			if err == nil || err.Error() != test.want {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCoverageRoutingNormalizesEmptyAndTrailingPaths(t *testing.T) {
	tests := []struct {
		name    string
		request string
		clear   bool
	}{
		{name: "empty", request: "/placeholder", clear: true},
		{name: "root dot", request: "/."},
		{name: "v1 dot", request: "/v1/."},
		{name: "v1 parent", request: "/v1/a/.."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("normalized non-v1 path reached the upstream")
				return nil, nil
			})}
			handler := newHandler(Config{APIKey: "sk-test", ProxyToken: "pt-test", UpstreamOrigin: "http://upstream"}, client)
			request := httptest.NewRequest(http.MethodGet, test.request, nil)
			if test.clear {
				request.URL.Path = ""
				request.URL.RawPath = ""
			}
			request.Header.Set("Authorization", "Bearer pt-test")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", response.Code)
			}
			if got := response.Body.String(); got != `{"error":"Not found"}` {
				t.Fatalf("body = %q, want %q", got, `{"error":"Not found"}`)
			}
		})
	}
}

func TestCoverageAuthorizationRejectsEmptyConfiguredToken(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("request with an empty configured token reached the upstream")
		return nil, nil
	})}
	handler := newHandler(Config{
		ProxyToken:     "",
		UpstreamOrigin: "http://upstream",
		Unconfigured:   true,
	}, client)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer anything")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := response.Body.String(); got != `{"error":"Unauthorized"}` {
		t.Fatalf("body = %q, want %q", got, `{"error":"Unauthorized"}`)
	}
}

func TestCoverageRoutingRejectsMalformedUpstreamOrigin(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("malformed upstream URL reached the transport")
		return nil, nil
	})}
	handler := newHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: "://invalid",
	}, client)
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

func TestCoverageRoutingReturnsSafeBadGatewayOnRedirectError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://redirect.invalid/next"}},
				Body:       http.NoBody,
				Request:    request,
			}, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("redirect blocked")
		},
	}
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
