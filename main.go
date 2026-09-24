package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: NewHandler(config),
	}
	fmt.Fprintf(os.Stderr, "NVIDIA API proxy listening on port %d\n", config.Port)
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
