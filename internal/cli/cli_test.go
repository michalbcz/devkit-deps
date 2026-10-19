package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"devkit-deps/internal/cli"
	"devkit-deps/internal/lease"
)

func TestMain(m *testing.M) {
	if os.Getenv("DEVKIT_DEPS_CLI") == "1" {
		os.Exit(cli.Main(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runCLI(t *testing.T, dir string, extra []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	if dir != "" {
		cmd.Dir = dir
	}
	env := os.Environ()
	skip := map[string]struct{}{}
	for _, item := range extra {
		k, _, _ := strings.Cut(item, "=")
		skip[k] = struct{}{}
	}
	filtered := make([]string, 0, len(env)+len(extra)+1)
	for _, item := range env {
		k, _, _ := strings.Cut(item, "=")
		if _, ok := skip[k]; ok {
			continue
		}
		filtered = append(filtered, item)
	}
	filtered = append(filtered, extra...)
	filtered = append(filtered, "DEVKIT_DEPS_CLI=1")
	cmd.Env = filtered
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{"run", "--json", "a.pl"},
		{"run", "--timeout", "0", "a.pl"},
		{"run", "--rebuild", "--no-prepare", "a.pl"},
		{"run", "--not-a-flag"},
		{"run"},
		{"nope"},
		{"install", "a.lua"},
		{"install", "--json"},
		{"install", "--rebuild"},
	}
	for _, args := range cases {
		_, stderr, code := runCLI(t, "", nil, args...)
		if code != 2 || stderr == "" {
			t.Fatalf("%v -> %d %s", args, code, stderr)
		}
	}
}

func TestDoctorNamesMissingRuntime(t *testing.T) {
	_, stderr, code := runCLI(t, "", nil, "doctor", "--lang", "lua", "--runtime", "/no/such/lua")
	if code != 3 || !strings.Contains(stderr, "lua") {
		t.Fatalf("%d %s", code, stderr)
	}
	stdout, _, code := runCLI(t, "", nil, "doctor")
	if code != 0 || !strings.Contains(stdout, "lua") || !strings.Contains(stdout, "perl") {
		t.Fatalf("%d %s", code, stdout)
	}
}

func TestShellRunReturnsProgramExit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "a.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "cache")
	stdout, stderr, code := runCLI(t, dir, nil, "run", "--lang", "shell", "--shell", "sh", "--runtime", "/bin/sh", "--cache-dir", cacheDir, script)
	if code != 7 {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatal("cache directory was created")
	}
}

func TestRuntimeAndManagerFlags(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "a.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCLI(t, dir, nil, "run", "--lang", "go", "--manager", "/bin/sh", goFile)
	if code != 2 || !strings.Contains(stderr, "manager") {
		t.Fatalf("%d %s", code, stderr)
	}
	tcl := filepath.Join(dir, "a.tcl")
	if err := os.WriteFile(tcl, []byte("puts hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCLI(t, dir, nil, "run", "--lang", "tcl", "--runtime", "/bin/sh", tcl)
	if code != 2 || !strings.Contains(stderr, "runtime") {
		t.Fatalf("%d %s", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.pl"), []byte("print 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCLI(t, dir, nil, "run", "--lang", "perl", "--runtime", "perl", filepath.Join(dir, "a.pl"))
	if code != 2 || !strings.Contains(stderr, "absolute") {
		t.Fatalf("%d %s", code, stderr)
	}
}

func TestSignalReleasesUseLease(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	pixi := filepath.Join(bin, "pixi")
	body := "#!/bin/sh\nif [ \"$1\" = install ]; then exit 0; fi\nif [ \"$1\" = run ]; then\n  while [ $# -gt 0 ] && [ \"$1\" != -- ]; do shift; done\n  shift\n  exec \"$@\"\nfi\nexit 1\n"
	if err := os.WriteFile(pixi, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(dir, "ready")
	script := filepath.Join(dir, "nap.sh")
	scriptBody := "#!/bin/sh\nprintf ready > \"$1\"\ntrap 'exit 130' INT\nsleep 30\n"
	if err := os.WriteFile(script, []byte("# devkit-deps: [\"ripgrep@15.1.0\"]\n"+scriptBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "cache")
	cmd := exec.Command(os.Args[0], "run", "--lang", "shell", "--shell", "sh", "--runtime", "/bin/sh", "--cache-dir", cacheDir, script, "--", ready)
	cmd.Env = append(filterEnv("PATH"), "PATH="+bin+":/bin:/usr/bin", "DEVKIT_DEPS_CLI=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil && cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("script did not start\n%s\n%s", stdout.String(), stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	waitErr := cmd.Wait()
	code := 0
	if waitErr != nil {
		var exit *exec.ExitError
		if !errors.As(waitErr, &exit) {
			t.Fatal(waitErr)
		}
		code = exit.ExitCode()
	}
	if code != 130 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout.String(), stderr.String())
	}
	matches, err := filepath.Glob(filepath.Join(cacheDir, "shell-*"))
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr == nil && info.IsDir() {
			dirs = append(dirs, match)
		}
	}
	if len(dirs) != 1 {
		t.Fatalf("env %v", matches)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	held, err := lease.AcquireUse(ctx, dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	held.Release()
}

func filterEnv(drop string) []string {
	var out []string
	for _, item := range os.Environ() {
		k, _, _ := strings.Cut(item, "=")
		if k == drop {
			continue
		}
		out = append(out, item)
	}
	return out
}
