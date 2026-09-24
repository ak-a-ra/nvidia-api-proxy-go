package main

import "testing"
import "time"

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("NVIDIA_BASE_URL", "https://integrate.api.nvidia.com/v1")
	t.Setenv("NVIDIA_API_KEY", "sk-test")
	t.Setenv("PROXY_AUTH_TOKEN", "pt-test")
	t.Setenv("PORT", "")
	t.Setenv("UPSTREAM_CONNECT_TIMEOUT_SECONDS", "")
	t.Setenv("UPSTREAM_IDLE_TIMEOUT_SECONDS", "")

	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Port != 10000 {
		t.Fatalf("Port = %d, want 10000", config.Port)
	}
	if config.ConnectTimeout != 30*time.Second {
		t.Fatalf("ConnectTimeout = %s, want 30s", config.ConnectTimeout)
	}
	if config.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout = %s, want 120s", config.IdleTimeout)
	}
	if config.Unconfigured {
		t.Fatal("Unconfigured = true, want false")
	}
}

func TestLoadConfigInvalidBaseURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "missing", base: "", want: "NVIDIA_BASE_URL is required"},
		{name: "malformed", base: "not-a-url", want: "NVIDIA_BASE_URL is not a valid URL: not-a-url"},
		{name: "unsupported scheme", base: "file:///etc/hosts", want: "NVIDIA_BASE_URL must use http or https, got: file:///etc/hosts"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NVIDIA_BASE_URL", test.base)
			_, err := LoadConfig()
			if err == nil {
				t.Fatal("LoadConfig() error = nil")
			}
			if err.Error() != test.want {
				t.Fatalf("error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestLoadConfigTimeoutValues(t *testing.T) {
	t.Setenv("NVIDIA_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("NVIDIA_API_KEY", "sk-test")
	t.Setenv("PROXY_AUTH_TOKEN", "pt-test")
	t.Setenv("UPSTREAM_CONNECT_TIMEOUT_SECONDS", "0")
	t.Setenv("UPSTREAM_IDLE_TIMEOUT_SECONDS", "invalid")

	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnectTimeout != 0 {
		t.Fatalf("ConnectTimeout = %s, want disabled", config.ConnectTimeout)
	}
	if config.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout = %s, want 120s", config.IdleTimeout)
	}
}
