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
	"devkit-deps/internal/discover"
	"devkit-deps/internal/engine"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
	"devkit-deps/internal/validate"
)

func writeExe(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGoBuildRunsFromCallerCwd(t *testing.T) {
	goBin := writeExe(t, "go", "#!/bin/sh\nprev=\nmode=\nout=\nfor arg in \"$@\"; do\n  if [ \"$arg\" = build ]; then mode=build; fi\n  if [ \"$prev\" = \"-o\" ]; then out=$arg; fi\n  prev=$arg\ndone\nif [ \"$mode\" = build ]; then\n  printf '%s\\n' '#!/bin/sh' 'pwd' 'exit 42' > \"$out\"\n  chmod +x \"$out\"\n  exit 0\nfi\nexit 0\n")
	caller := t.TempDir()
	script := filepath.Join(t.TempDir(), "quote.go")
	if err := os.WriteFile(script, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := discover.Host{Language: lang.GoLang, Origin: "path", Runtime: discover.Tool{Name: "go", Path: goBin, Version: "go1.22.0", Status: "ok"}}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{Script: script, CallerCwd: caller})
	if d != nil {
		t.Fatal(d)
	}
	var stdout bytes.Buffer
	code, d := engine.Run(context.Background(), doc, engine.Options{SetupTimeout: 30 * time.Second, Stdout: &stdout, Stderr: &bytes.Buffer{}})
	if d != nil || code != 42 {
		t.Fatalf("code %d %+v\n%s", code, d, stdout.String())
	}
	want, err := filepath.EvalSymlinks(caller)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != want {
		t.Fatalf("pwd %q want %s", stdout.String(), want)
	}
}

func TestGoBuildFailureDoesNotStartProgram(t *testing.T) {
	goBin := writeExe(t, "go", "#!/bin/sh\nexit 1\n")
	host := discover.Host{Language: lang.GoLang, Origin: "path", Runtime: discover.Tool{Name: "go", Path: goBin, Status: "ok"}}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{Script: "/work/a.go", CallerCwd: t.TempDir()})
	if d != nil {
		t.Fatal(d)
	}
	var stdout bytes.Buffer
	code, d := engine.Run(context.Background(), doc, engine.Options{SetupTimeout: 30 * time.Second, Stdout: &stdout})
	if d == nil || d.Code != "launch.build" || code != 4 || stdout.Len() != 0 {
		t.Fatalf("code %d %+v %q", code, d, stdout.String())
	}
}

func TestPowerShellWrapperForwardsArgs(t *testing.T) {
	pwsh := writeExe(t, "pwsh", "#!/bin/sh\nfile=\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-File\" ]; then file=$2; shift 2; break; fi\n  shift\ndone\ncase \"$file\" in\n  *prepare.ps1) exit 0 ;;\n  *run.ps1)\n    for arg in \"$@\"; do printf '%s\\0' \"$arg\"; done\n    exit 42 ;;\nesac\nexit 1\n")
	host := discover.Host{
		Language: lang.PowerShell, Origin: "path",
		Platform: cache.Platform{GOOS: "darwin", GOARCH: "arm64"},
		Runtime:  discover.Tool{Name: "pwsh", Path: pwsh, Version: "7.4.0", Status: "ok"},
		Manager:  discover.Tool{Name: "pwsh", Path: pwsh, Status: "ok", ModTime: time.Unix(1_700_000_000, 0)},
	}
	script := filepath.Join(t.TempDir(), "a.ps1")
	if err := os.WriteFile(script, []byte("Write-Output hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, diagErr := validate.Validate(lang.PowerShell, []string{"powershell-yaml@0.4.12"})
	if diagErr != nil {
		t.Fatal(diagErr)
	}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, pkgs, host, plan.Policy{
		CacheRoot: t.TempDir(), Script: script, ScriptArgs: []string{"a b", "c"}, CallerCwd: t.TempDir(),
	})
	if d != nil {
		t.Fatal(d)
	}
	var stdout bytes.Buffer
	code, d := engine.Run(context.Background(), doc, engine.Options{SetupTimeout: 30 * time.Second, Stdout: &stdout, Stderr: &bytes.Buffer{}})
	if d != nil || code != 42 {
		t.Fatalf("code %d %+v %q", code, d, stdout.String())
	}
	want := append([]byte("a b"), 0)
	want = append(want, 'c', 0)
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("%#v", stdout.Bytes())
	}
	body, err := os.ReadFile(filepath.Join(doc.EnvironmentDirectory, "run.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), script) || !strings.Contains(string(body), "exit $LASTEXITCODE") {
		t.Fatalf("%s", body)
	}
}

func TestRedShimIsRemovedAfterLaunch(t *testing.T) {
	red := writeExe(t, "red", "#!/bin/sh\nprintf '%s\\n' \"$1\"\ncat \"$1\"\n")
	dir := t.TempDir()
	script := filepath.Join(dir, "a b\"c.red")
	if err := os.WriteFile(script, []byte("Red []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := discover.Host{Language: lang.Red, Origin: "path", Runtime: discover.Tool{Name: "red", Path: red, Status: "ok"}}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{Script: script, CallerCwd: dir})
	if d != nil {
		t.Fatal(d)
	}
	var stdout bytes.Buffer
	code, d := engine.Run(context.Background(), doc, engine.Options{SetupTimeout: 30 * time.Second, Stdout: &stdout})
	if d != nil || code != 0 {
		t.Fatalf("%d %+v %s", code, d, stdout.String())
	}
	text := stdout.String()
	shim := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	if _, err := os.Stat(shim); !os.IsNotExist(err) {
		t.Fatal("shim was not removed")
	}
	if !strings.Contains(text, "quit/return 1") || !strings.Contains(text, `do %"`) {
		t.Fatalf("%s", text)
	}
}

func TestSourceBytesStayUnchanged(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "a.sh")
	original := []byte("echo hi\n")
	if err := os.WriteFile(script, original, 0o644); err != nil {
		t.Fatal(err)
	}
	host := discover.Host{Language: lang.Shell, Dialect: lang.Sh, Origin: "path", Runtime: discover.Tool{Name: "sh", Path: "/bin/sh", Status: "ok"}}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{Script: script, CallerCwd: dir})
	if d != nil {
		t.Fatal(d)
	}
	code, d := engine.Run(context.Background(), doc, engine.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if d != nil || code != 0 {
		t.Fatalf("%d %+v", code, d)
	}
	got, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("%q", got)
	}
}
