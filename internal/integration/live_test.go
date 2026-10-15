//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fixture struct {
	name  string
	lang  string
	tools []string
}

func TestLiveFixtures(t *testing.T) {
	bin := buildCLI(t)
	root := moduleRoot(t)
	fixtures := []fixture{
		{name: "try-tiny.pl", lang: "perl", tools: []string{"perl", "cpanm"}},
		{name: "symfony.php", lang: "php", tools: []string{"php", "composer"}},
		{name: "levenshtein.raku", lang: "raku", tools: []string{"raku", "zef"}},
		{name: "spork.janet", lang: "janet", tools: []string{"janet", "jpm", "git"}},
		{name: "lustache.lua", lang: "lua", tools: []string{"lua", "luarocks"}},
		{name: "quote.go", lang: "go", tools: []string{"go"}},
		{name: "tcllib.tcl", lang: "tcl", tools: []string{"pixi"}},
		{name: "ripgrep.sh", lang: "shell", tools: []string{"pixi", "sh"}},
		{name: "humanize.mojo", lang: "mojo", tools: []string{"pixi"}},
		{name: "yaml.ps1", lang: "powershell", tools: []string{"pwsh"}},
		{name: "red-tools.red", lang: "red", tools: []string{"red", "git"}},
	}
	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			for _, tool := range fx.tools {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("%s is not on PATH", tool)
				}
			}
			script := filepath.Join(root, "testdata", "fixtures", fx.name)
			want, err := os.ReadFile(script + ".out")
			if err != nil {
				t.Fatal(err)
			}
			cache := t.TempDir()
			cwd := t.TempDir()
			stdout, stderr, code := runCLI(t, bin, cache, script, cwd)
			if code != 0 {
				t.Fatalf("first exit %d\n%s", code, tail(stderr))
			}
			if stdout != string(want) {
				t.Fatalf("stdout %q want %q\n%s", stdout, want, tail(stderr))
			}
			stdout, stderr, code = runCLI(t, bin, cache, script, cwd)
			if code != 0 {
				t.Fatalf("second exit %d\n%s", code, tail(stderr))
			}
			if stdout != string(want) {
				t.Fatalf("cached stdout %q want %q", stdout, want)
			}
			if !strings.Contains(stderr, "Using cached environment") {
				t.Fatalf("cache was not reused\n%s", tail(stderr))
			}
			env := envDir(t, cache, fx.lang)
			removeRequired(t, env)
			_, stderr, code = runCLI(t, bin, cache, script, cwd, "--no-prepare")
			if code != 4 {
				t.Fatalf("--no-prepare exit %d\n%s", code, tail(stderr))
			}
		})
	}
}

func TestGoCallerCwd(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	bin := buildCLI(t)
	script := filepath.Join(moduleRoot(t), "testdata", "fixtures", "quote-cwd.go")
	cwd := t.TempDir()
	stdout, stderr, code := runCLI(t, bin, t.TempDir(), script, cwd)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, tail(stderr))
	}
	if stdout != cwd+"\n" {
		t.Fatalf("cwd %q want %q\n%s", stdout, cwd+"\n", tail(stderr))
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "devkit-deps")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/devkit-deps")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	return bin
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the integration test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func runCLI(t *testing.T, bin, cache, script, cwd string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"run", "--cache-dir", cache}, extra...)
	args = append(args, script)
	cmd := exec.Command(bin, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("%v\n%s", err, stderr.String())
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func envDir(t *testing.T, cache, lang string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(cache, lang+"-*"))
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
		t.Fatalf("environment directories: %v", matches)
	}
	return dirs[0]
}

func removeRequired(t *testing.T, env string) {
	t.Helper()
	marker := filepath.Join(env, ".ready.json")
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	var ready struct {
		RequiredFiles []string `json:"requiredFiles"`
	}
	if err := json.Unmarshal(data, &ready); err != nil {
		t.Fatal(err)
	}
	target := marker
	if len(ready.RequiredFiles) > 0 {
		target = filepath.Join(env, filepath.FromSlash(ready.RequiredFiles[0]))
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
}

func tail(s string) string {
	if len(s) > 4000 {
		return s[len(s)-4000:]
	}
	return s
}
