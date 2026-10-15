package engine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devkit-deps/internal/discover"
	"devkit-deps/internal/engine"
)

func TestPlanDoesNotRunInstaller(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "a.lua")
	if err := os.WriteFile(script, []byte("-- devkit-deps: [\"lustache@1.3.1-0\"]\nprint(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "ran")
	lua := filepath.Join(dir, "lua")
	if err := os.WriteFile(lua, []byte("#!/bin/sh\nif [ \"$1\" = \"-v\" ]; then echo 'Lua 5.4.7'; exit 0; fi\necho lua >> \""+logPath+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rocks := filepath.Join(dir, "luarocks")
	if err := os.WriteFile(rocks, []byte("#!/bin/sh\necho rocks >> \""+logPath+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	look := discover.LookerFunc(func(name string) (string, time.Time, bool) {
		switch name {
		case "lua":
			return lua, time.Unix(1, 0), true
		case "luarocks":
			return rocks, time.Unix(1, 0), true
		default:
			return "", time.Time{}, false
		}
	})
	var stdout, stderr bytes.Buffer
	code := engine.Execute(context.Background(), engine.Request{
		Command: "plan", Script: script, JSON: true,
		CacheDir: filepath.Join(dir, "cache"), Look: look,
		Stdout: &stdout, Stderr: &stderr, SetupTimeout: time.Second,
	})
	if code != 0 {
		t.Fatalf("code %d %s", code, stderr.String())
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("plan executed a tool")
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["schemaVersion"] != float64(1) {
		t.Fatalf("%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "cache")); !os.IsNotExist(err) {
		t.Fatal("plan created a cache directory")
	}
}

func TestMissingRuntimeDoesNotStartOrCache(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "a.pl")
	if err := os.WriteFile(script, []byte("print \"ran\\n\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "cache")
	look := discover.LookerFunc(func(string) (string, time.Time, bool) {
		return "", time.Time{}, false
	})
	var stdout, stderr bytes.Buffer
	code := engine.Execute(context.Background(), engine.Request{
		Command: "plan", Script: script, JSON: true, CacheDir: cacheDir, Look: look,
		Stdout: &stdout, Stderr: &stderr,
	})
	if code != 3 || !bytes.Contains(stderr.Bytes(), []byte("perl")) {
		t.Fatalf("plan code %d %s %s", code, stderr.String(), stdout.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("%v %s", err, stdout.String())
	}
	runtime := doc["runtime"].(map[string]any)
	if runtime["status"] != "missing" || doc["environmentDirectory"] != nil || len(doc["setup"].([]any)) != 0 {
		t.Fatalf("%s", stdout.String())
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatal("cache was created")
	}

	stdout.Reset()
	stderr.Reset()
	code = engine.Execute(context.Background(), engine.Request{
		Command: "prepare", Script: script, JSON: true, CacheDir: cacheDir, Look: look,
		Stdout: &stdout, Stderr: &stderr,
	})
	if code != 3 {
		t.Fatalf("prepare %d %s", code, stderr.String())
	}
	var diagDoc map[string]any
	dec := json.NewDecoder(&stdout)
	if err := dec.Decode(&diagDoc); err != nil || dec.More() {
		t.Fatalf("%v more %v %s", err, dec.More(), stdout.String())
	}
	if diagDoc["code"] != "tool.missing" {
		t.Fatalf("%s", stdout.String())
	}
	stdout.Reset()
	code = engine.Execute(context.Background(), engine.Request{
		Command: "run", Script: script, CacheDir: cacheDir, Look: look,
		Stdout: &stdout, Stderr: &stderr,
	})
	if code != 3 || stdout.Len() != 0 {
		t.Fatalf("run %d %q", code, stdout.String())
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatal("run created a cache directory")
	}
}
