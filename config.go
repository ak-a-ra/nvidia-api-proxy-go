package main

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIKey         string
	ProxyToken     string
	BaseURL        string
	UpstreamOrigin string
	Port           int
	ConnectTimeout time.Duration
	IdleTimeout    time.Duration
	Unconfigured   bool
}

func LoadConfig() (Config, error) {
	rawBase := os.Getenv("NVIDIA_BASE_URL")
	if strings.TrimSpace(rawBase) == "" {
		return Config{}, fmt.Errorf("NVIDIA_BASE_URL is required")
	}

	base, err := url.Parse(strings.TrimSpace(rawBase))
	if err != nil || base.Scheme == "" {
		return Config{}, fmt.Errorf("NVIDIA_BASE_URL is not a valid URL: %s", rawBase)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return Config{}, fmt.Errorf("NVIDIA_BASE_URL must use http or https, got: %s", rawBase)
	}
	if base.Host == "" {
		return Config{}, fmt.Errorf("NVIDIA_BASE_URL is not a valid URL: %s", rawBase)
	}

	port := 10000
	if rawPort := os.Getenv("PORT"); strings.TrimSpace(rawPort) != "" {
		parsed, parseErr := strconv.Atoi(rawPort)
		if parseErr != nil || parsed < 0 || parsed > 65535 {
			return Config{}, fmt.Errorf("PORT must be a valid port: %s", rawPort)
		}
		port = parsed
	}

	apiKey := os.Getenv("NVIDIA_API_KEY")
	proxyToken := os.Getenv("PROXY_AUTH_TOKEN")
	return Config{
		APIKey:         apiKey,
		ProxyToken:     proxyToken,
		BaseURL:        rawBase,
		UpstreamOrigin: base.Scheme + "://" + base.Host,
		Port:           port,
		ConnectTimeout: readTimeout("UPSTREAM_CONNECT_TIMEOUT_SECONDS", 30*time.Second),
		IdleTimeout:    readTimeout("UPSTREAM_IDLE_TIMEOUT_SECONDS", 120*time.Second),
		Unconfigured:   strings.TrimSpace(apiKey) == "" || strings.TrimSpace(proxyToken) == "",
	}, nil
}

func readTimeout(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds * float64(time.Second))
}
