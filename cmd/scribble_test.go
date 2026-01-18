package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain_Invoke runs in a subprocess to execute main().
func TestMain_Invoke(t *testing.T) {
	if os.Getenv("SCRIBBLE_TEST_MAIN") != "1" {
		return
	}
	main()
}

func TestMain_MissingConfig(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMain_Invoke", "--", "-config", "does-not-exist.yml")
	cmd.Env = append(os.Environ(), "SCRIBBLE_TEST_MAIN=1")

	output, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got err=%v output=%s", err, string(output))
	}

	if !strings.Contains(string(output), "failed to load configuration") {
		t.Fatalf("expected config load failure, got: %s", string(output))
	}
}
