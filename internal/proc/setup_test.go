package proc_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"devkit-deps/internal/plan"
	"devkit-deps/internal/proc"
)

func TestSetupKeepsTailAndReturns(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "lines.sh")
	body := "i=0\nwhile [ \"$i\" -lt 5000 ]; do\n  echo \"line $i\"\n  i=$((i+1))\ndone\n"
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res := proc.RunSetup(context.Background(), []plan.StepDoc{{
		Argv: []string{"/bin/sh", script}, Cwd: dir,
		Environment: map[string]string{}, CaptureEnvironment: []string{},
	}}, 30*time.Second, io.Discard)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(res.Tail) != 200 {
		t.Fatalf("tail %d", len(res.Tail))
	}
	if res.Tail[len(res.Tail)-1] != "line 4999" {
		t.Fatalf("%q", res.Tail[len(res.Tail)-1])
	}
}

func TestSetupDeadlineKillsDescendantHoldingStdout(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	script := filepath.Join(dir, "hold.sh")
	body := "sleep 30 &\necho $! > \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res := proc.RunSetup(context.Background(), []plan.StepDoc{{
		Argv: []string{"/bin/sh", script, pidFile}, Cwd: dir,
		Environment: map[string]string{}, CaptureEnvironment: []string{},
	}}, time.Second, io.Discard)
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	if res.Err == nil || res.Err.Code != "setup.timeout" || res.Err.Exit != 124 {
		t.Fatalf("%+v", res.Err)
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatal("descendant still holds the pipe")
	}
}
