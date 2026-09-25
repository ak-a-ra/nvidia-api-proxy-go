package main

import (
	"net/http"
	"strconv"
	"strings"
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

func copyRequestHeaders(destination, source http.Header) {
	connectionHeaders := connectionNominatedHeaders(source)
	for name, values := range source {
		if shouldStripHeader(name, connectionHeaders) {
			continue
		}
		destination.Set(name, strings.Join(values, ", "))
	}
}

func copyResponseHeaders(destination, source http.Header, status int, method string, decoded, dropEncoding bool) {
	connectionHeaders := connectionNominatedHeaders(source)
	preserveLength := !decoded &&
		!(dropEncoding && source.Get("Content-Encoding") != "") &&
		preserveContentLength(status, method)
	for name, values := range source {
		lowerName := strings.ToLower(name)
		if (strippedHeaders[lowerName] && lowerName != "content-length") || connectionHeaders[lowerName] {
			continue
		}
		if (decoded || dropEncoding) && lowerName == "content-encoding" {
			continue
		}
		if lowerName == "content-length" && (!preserveLength || !validContentLength(values)) {
			continue
		}
		destination[name] = append([]string(nil), values...)
	}
}

func connectionNominatedHeaders(source http.Header) map[string]bool {
	values := source.Values("Connection")
	if len(values) == 0 {
		return nil
	}
	headers := make(map[string]bool, len(values))
	for _, value := range values {
		for _, name := range strings.Split(value, ",") {
			if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
				headers[name] = true
			}
		}
	}
	return headers
}

func shouldStripHeader(name string, connectionHeaders map[string]bool) bool {
	lowerName := strings.ToLower(name)
	return strippedHeaders[lowerName] || connectionHeaders[lowerName]
}

func validContentLength(values []string) bool {
	if len(values) != 1 {
		return false
	}
	length, err := strconv.ParseInt(strings.TrimSpace(values[0]), 10, 64)
	return err == nil && length >= 0
}

func preserveContentLength(status int, method string) bool {
	if method == http.MethodHead {
		return true
	}
	return status >= http.StatusOK && status != http.StatusNoContent && status != http.StatusResetContent &&
		status != http.StatusNotModified
}
