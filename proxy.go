package main

import (
	"encoding/json"
	"net/http"
	"strconv"
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

		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "Not found"})
	})
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
