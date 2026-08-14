package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

// exitErrorWith builds a real *exec.ExitError by running the test binary's
// own helper — `go test` compiles a test binary, and `os/exec` on it with an
// unknown flag exits with code 1 deterministically. To obtain other codes we
// spawn the binary with -test.run for a helper that exits with a given code.
func exitErrorWithCode(t *testing.T, code int) *exec.ExitError {
	t.Helper()

	if code == 0 {
		t.Fatal("code 0 never produces an ExitError")
	}

	// Re-run this very test binary as a child with GO_HELPER_EXIT set: the
	// TestMain-free trick below uses -test.run to select a helper test that
	// exits with the requested code.
	//nolint:gosec // intentional: re-executes the test binary itself (os.Args[0]) with fixed flags — the env-var code is a controlled test input
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperExitProcess", "-test.v")
	cmd.Env = append(os.Environ(), "GO_HELPER_EXIT="+strconv.Itoa(code))
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError for code %d, got %v", code, err)
	}
	return exitErr
}

// TestHelperExitProcess is not a real test: it exists so exitErrorWithCode can
// spawn this binary and produce a genuine non-zero exit code. Skipped unless
// GO_HELPER_EXIT is set (the child-mode marker).
func TestHelperExitProcess(t *testing.T) {
	codeStr := os.Getenv("GO_HELPER_EXIT")
	if codeStr == "" {
		t.Skip("helper process only — runs when GO_HELPER_EXIT is set")
	}

	code, err := strconv.Atoi(codeStr)
	if err != nil {
		code = 3
	}
	os.Exit(code)
}

func TestDelegatedExitCode(t *testing.T) {
	tests := []struct {
		name   string
		errGen func(t *testing.T) error
		stderr string
		want   int
	}{
		{
			name:   "success",
			errGen: func(*testing.T) error { return nil },
			want:   0,
		},
		{
			name:   "child violations (exit 1)",
			errGen: func(t *testing.T) error { return exitErrorWithCode(t, 1) },
			stderr: "some warning output\nexit status 1\n",
			want:   1,
		},
		{
			name:   "child config error (exit 2)",
			errGen: func(t *testing.T) error { return exitErrorWithCode(t, 1) },
			stderr: "ERR: spec is broken\nexit status 2\n",
			want:   2,
		},
		{
			name:   "compile failure maps to config error",
			errGen: func(t *testing.T) error { return exitErrorWithCode(t, 1) },
			stderr: "# arch-lint-local\n./main.go:12:3: undefined: Component\n",
			want:   2,
		},
		{
			name:   "exit status line with trailing whitespace",
			errGen: func(t *testing.T) error { return exitErrorWithCode(t, 1) },
			stderr: "exit status 2  \n",
			want:   2,
		},
		{
			name: "not an ExitError (go binary missing)",
			errGen: func(*testing.T) error {
				return errors.New("exec: \"go\": executable file not found in $PATH")
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := delegatedExitCode(tt.errGen(t), tt.stderr)
			if got != tt.want {
				t.Errorf("delegatedExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}
