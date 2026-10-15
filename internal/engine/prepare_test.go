package engine_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/engine"
	"devkit-deps/internal/lease"
	"devkit-deps/internal/plan"
	"devkit-deps/internal/proc"
)

func hold(t *testing.T, env string) *lease.Lease {
	t.Helper()
	held, err := lease.AcquireUse(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { held.Release() })
	return held
}

func installer(t *testing.T, counter string, exit string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "install.sh")
	body := "#!/bin/sh\necho preparing\necho x >> \"$1\"\nexit " + exit + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func docFor(env, script, counter string) plan.Document {
	return plan.Document{
		Language:             "lua",
		EnvironmentDirectory: env,
		Readiness:            "missing",
		Files:                []plan.FileDoc{{Path: "note.txt", Body: "hello"}},
		Setup: []plan.StepDoc{{
			Argv: []string{"/bin/sh", script, counter}, Cwd: env,
			Environment: map[string]string{}, CaptureEnvironment: []string{},
		}},
		Execution: plan.ExecDoc{
			Argv: []string{"/bin/sh", "-c", "printf 'a\\000b'"}, Cwd: env,
			Environment: map[string]string{}, DeferredEnvironment: []string{},
		},
	}
}

func TestPrepareReusesAndSkipsInstaller(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my deps")
	env := filepath.Join(root, "lua-key")
	counter := filepath.Join(root, "count.txt")
	script := installer(t, counter, "0")
	doc := docFor(env, script, counter)
	held := hold(t, env)
	var stderr bytes.Buffer
	got, d := engine.Prepare(context.Background(), doc, held, false, false, 30*time.Second, &stderr)
	if d != nil || got.Readiness != "ready" {
		t.Fatalf("%+v %+v", got.Readiness, d)
	}
	stderr.Reset()
	got, d = engine.Prepare(context.Background(), doc, held, false, false, 30*time.Second, &stderr)
	if d != nil {
		t.Fatal(d)
	}
	body, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), "x") != 1 {
		t.Fatalf("installer ran %q", body)
	}
	if !strings.Contains(stderr.String(), "Using cached environment: "+env) {
		t.Fatalf("%s", stderr.String())
	}
	var stdout bytes.Buffer
	code, d := proc.Launch(context.Background(), got, nil, &stdout, &stderr, 0)
	if d != nil || code != 0 || !bytes.Equal(stdout.Bytes(), []byte{'a', 0, 'b'}) {
		t.Fatalf("code %d %+v %#v stderr %s", code, d, stdout.Bytes(), stderr.String())
	}
	if strings.Contains(stdout.String(), "preparing") || bytes.Contains(stdout.Bytes(), []byte("Using cached")) {
		t.Fatal("progress leaked into stdout")
	}
}

func TestPrepareFailurePublishesNoMarker(t *testing.T) {
	env := filepath.Join(t.TempDir(), "lua-key")
	counter := filepath.Join(t.TempDir(), "count.txt")
	script := installer(t, counter, "1")
	doc := docFor(env, script, counter)
	_, d := engine.Prepare(context.Background(), doc, hold(t, env), false, false, 30*time.Second, &bytes.Buffer{})
	if d == nil || d.Exit != 4 {
		t.Fatalf("%+v", d)
	}
	if _, err := os.Stat(filepath.Join(env, ".ready.json")); !os.IsNotExist(err) {
		t.Fatal("readiness marker was published")
	}
}

func TestNoPrepareRejectsIncompleteReadyFile(t *testing.T) {
	env := filepath.Join(t.TempDir(), "lua-key")
	if err := cache.WriteReady(env, cache.Ready{Schema: 1, RequiredFiles: []string{"missing.txt"}}); err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(t.TempDir(), "count.txt")
	script := installer(t, counter, "0")
	doc := docFor(env, script, counter)
	held := hold(t, env)
	_, d := engine.Prepare(context.Background(), doc, held, false, true, 30*time.Second, &bytes.Buffer{})
	if d == nil || d.Code != "cache.not_ready" || d.Exit != 4 {
		t.Fatalf("%+v", d)
	}
	if _, err := os.Stat(counter); !os.IsNotExist(err) {
		t.Fatal("no-prepare ran the installer")
	}
	if _, d := engine.Prepare(context.Background(), doc, held, false, false, 30*time.Second, &bytes.Buffer{}); d != nil {
		t.Fatal(d)
	}
	if _, err := os.Stat(counter); err != nil {
		t.Fatal(err)
	}
}
