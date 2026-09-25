package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const shutdownTimeout = 10 * time.Second

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
	if err := runServer(server); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(server *http.Server) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	fmt.Fprintf(os.Stderr, "NVIDIA API proxy listening on port %d\n", port)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	shutdownComplete := make(chan struct{})
	go func() {
		<-signals
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = server.Shutdown(ctx)
		close(shutdownComplete)
	}()

	err = server.Serve(listener)
	if err == http.ErrServerClosed {
		<-shutdownComplete
		return nil
	}
	return err
}
