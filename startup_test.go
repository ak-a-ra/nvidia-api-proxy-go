// story: e01s01
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidBaseURLExitsWithCodeOne(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "proxy")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "missing", base: "", want: "NVIDIA_BASE_URL is required"},
		{name: "malformed", base: "not-a-url", want: "NVIDIA_BASE_URL is not a valid URL"},
		{name: "unsupported scheme", base: "file:///etc/hosts", want: "NVIDIA_BASE_URL must use http or https"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(binary)
			command.Env = append(os.Environ(),
				"NVIDIA_BASE_URL="+test.base,
				"NVIDIA_API_KEY=sk-test",
				"PROXY_AUTH_TOKEN=pt-test",
			)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatal("process exited with nil error")
			}
			exitError, ok := err.(*exec.ExitError)
			if !ok || exitError.ExitCode() != 1 {
				t.Fatalf("exit error = %v", err)
			}
			if !strings.Contains(string(output), test.want) {
				t.Fatalf("output = %q, want substring %q", output, test.want)
			}
		})
	}
}
