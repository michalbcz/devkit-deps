package proc_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devkit-deps/internal/plan"
	"devkit-deps/internal/proc"
)

func TestLaunchPreservesBinaryStdoutAndStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", "-c", "printf 'a\\000b'"},
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, bytes.NewReader([]byte{'a', 0, 'b'}), &stdout, &stderr, 0)
	if d != nil || code != 0 {
		t.Fatalf("code %d %+v", code, d)
	}
	if !bytes.Equal(stdout.Bytes(), []byte{'a', 0, 'b'}) {
		t.Fatalf("%#v", stdout.Bytes())
	}

	stdout.Reset()
	doc.Execution.Argv = []string{"/bin/sh", "-c", "cat"}
	code, d = proc.Launch(context.Background(), doc, bytes.NewReader([]byte{'a', 0, 'b'}), &stdout, &stderr, 0)
	if d != nil || code != 0 || !bytes.Equal(stdout.Bytes(), []byte{'a', 0, 'b'}) {
		t.Fatalf("code %d %+v %#v", code, d, stdout.Bytes())
	}
}

func TestLaunchForwardsArguments(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "args.sh")
	body := "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\0' \"$arg\"; done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", script, "a b", "", "ü", "--flag"},
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, nil, &stdout, ioDiscard(), 0)
	if d != nil || code != 0 {
		t.Fatalf("%d %+v", code, d)
	}
	want := append([]byte("a b"), 0)
	want = append(want, 0)
	want = append(want, []byte("ü")...)
	want = append(want, 0)
	want = append(want, []byte("--flag")...)
	want = append(want, 0)
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("%#v", stdout.Bytes())
	}
}

func TestLaunchExitCodesAndCallerCwd(t *testing.T) {
	for _, code := range []int{0, 7, 42} {
		doc := plan.Document{Execution: plan.ExecDoc{
			Argv:        []string{"/bin/sh", "-c", "exit " + itoa(code)},
			Environment: map[string]string{},
		}}
		got, d := proc.Launch(context.Background(), doc, nil, ioDiscard(), ioDiscard(), 0)
		if d != nil || got != code {
			t.Fatalf("exit %d -> %d %+v", code, got, d)
		}
	}
	dir := t.TempDir()
	var stdout bytes.Buffer
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", "-c", "pwd"},
		Cwd:         dir,
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, nil, &stdout, ioDiscard(), 0)
	if d != nil || code != 0 {
		t.Fatal(d)
	}
	got := strings.TrimSpace(stdout.String())
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want && got != dir {
		t.Fatalf("pwd %s want %s", got, want)
	}
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
