package main

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestSIGTERMExitsPromptly(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "proxy")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(binary)
	command.Env = append(os.Environ(),
		"NVIDIA_BASE_URL=http://127.0.0.1:1",
		"NVIDIA_API_KEY=sk-test",
		"PROXY_AUTH_TOKEN=pt-test",
		"PORT="+strconv.Itoa(port),
	)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
		}
	})

	client := &http.Client{Timeout: 200 * time.Millisecond}
	ready := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, requestErr := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
		if requestErr == nil {
			_ = response.Body.Close()
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("proxy did not become ready: %s", stderr.String())
	}

	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		finished <- command.Wait()
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("SIGTERM exit error = %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("proxy did not exit after SIGTERM")
	}
}
