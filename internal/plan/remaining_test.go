package plan_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
)

func ccLooker() discover.Looker {
	return discover.LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "cc" {
			return "/usr/bin/cc", time.Unix(1, 0), true
		}
		return "", time.Time{}, false
	})
}

func gitLooker() discover.Looker {
	return discover.LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "git" {
			return "/tools/git", time.Unix(1_700_000_000, 0), true
		}
		return "", time.Time{}, false
	})
}

func TestGoQuotePlan(t *testing.T) {
	host := fixedHost(lang.GoLang, "/selected/go", "go")
	host.Manager.Path = host.Runtime.Path
	spec := "rsc.io/quote/v4@v4.0.1"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.GoLang, spec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/quote.go", ScriptArgs: []string{"a"}, CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", []string{spec}, "", "", nil)
	if doc.EnvironmentDirectory != e {
		t.Fatalf("%s", doc.EnvironmentDirectory)
	}
	if len(doc.Files) != 1 || doc.Files[0].Path != "go.mod" || doc.Files[0].Body != "module majkl.dev/console\n\ngo 1.18\n" {
		t.Fatalf("%#v", doc.Files)
	}
	want := []string{"/selected/go", "get", spec}
	if len(doc.Setup) != 1 || !reflect.DeepEqual(doc.Setup[0].Argv, want) || doc.Setup[0].Cwd != e {
		t.Fatalf("%#v", doc.Setup[0])
	}
	if doc.Setup[0].Environment["GOTOOLCHAIN"] != "local" || doc.Setup[0].Environment["GOWORK"] != "off" || doc.Setup[0].Environment["GO111MODULE"] != "on" || doc.Setup[0].Environment["GOFLAGS"] != "" || doc.Setup[0].Environment["DEVKIT_DEPS_DIR"] != e {
		t.Fatalf("%#v", doc.Setup[0].Environment)
	}
	if !reflect.DeepEqual(doc.RequiredFiles, []string{"go.mod", "go.sum"}) {
		t.Fatalf("%#v", doc.RequiredFiles)
	}
	pre := doc.Execution.PreRun[0]
	if !reflect.DeepEqual(pre.Argv, []string{"/selected/go", "build", "-o", "<per-run-binary>", "/work/quote.go"}) || pre.Cwd != e {
		t.Fatalf("%#v %s", pre.Argv, pre.Cwd)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"<per-run-binary>", "a"}) || doc.Execution.Cwd != "/caller" {
		t.Fatalf("%#v %s", doc.Execution.Argv, doc.Execution.Cwd)
	}
}

func TestPowerShellPlanQuotesPaths(t *testing.T) {
	host := fixedHost(lang.PowerShell, "/selected/pwsh", "pwsh")
	spec := "powershell-yaml@0.4.12"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.PowerShell, spec), host, plan.Policy{
		CacheRoot: "/cache/o'hara", Script: "/work/a.ps1", ScriptArgs: []string{"a b"}, CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/o'hara", []string{spec}, "", "", nil)
	var prepare, run string
	for _, f := range doc.Files {
		switch f.Path {
		case "prepare.ps1":
			prepare = f.Body
		case "run.ps1":
			run = f.Body
		}
	}
	if !strings.Contains(prepare, "Save-PSResource -Name 'powershell-yaml' -Version '0.4.12'") || !strings.Contains(prepare, "o''hara") {
		t.Fatalf("%s", prepare)
	}
	if !strings.Contains(run, "exit $LASTEXITCODE") || !strings.Contains(run, "/work/a.ps1") || !strings.Contains(run, "o''hara") {
		t.Fatalf("%s", run)
	}
	if strings.Contains(run, "param(") || !strings.Contains(run, "@args") {
		t.Fatalf("wrapper must forward raw arguments\n%s", run)
	}
	want := []string{host.Runtime.Path, "-NoProfile", "-NonInteractive", "-File", filepath.Join(e, "run.ps1"), "a b"}
	if !reflect.DeepEqual(doc.Execution.Argv, want) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if len(doc.Setup) != 1 || doc.Setup[0].Argv[len(doc.Setup[0].Argv)-1] != filepath.Join(e, "prepare.ps1") {
		t.Fatalf("%#v", doc.Setup)
	}
}

func pixiHost(id lang.ID, runtime string) discover.Host {
	host := fixedHost(id, runtime, "pixi")
	host.Origin = "pixi"
	host.Manager.Path = "/tools/pixi"
	host.Manager.Name = "pixi"
	if id == lang.Tcl || id == lang.Mojo {
		host.Runtime = discover.Tool{Name: string(id), Status: "ok"}
	}
	return host
}

func TestTcllibPlan(t *testing.T) {
	host := pixiHost(lang.Tcl, "")
	spec := "tcllib@2.0"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Tcl, spec), host, plan.Policy{
		CacheRoot: "/cache/my deps$", Script: "/work/a.tcl", CallerCwd: "/caller", Look: gitLooker(),
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/my deps$", []string{spec}, cache.ProfileTcl, "osx-arm64", nil)
	if doc.EnvironmentDirectory != e {
		t.Fatalf("%s", doc.EnvironmentDirectory)
	}
	manifest := filepath.Join(e, "pixi.toml")
	var body string
	for _, f := range doc.Files {
		if f.Path == "pixi.toml" {
			body = f.Body
		}
	}
	if strings.Contains(body, "tcllib") || !strings.Contains(body, `"tk" = ">=8.6,<8.7"`) || !strings.Contains(body, `platforms = ["osx-arm64"]`) {
		t.Fatalf("%s", body)
	}
	if len(doc.Setup) != 2 {
		t.Fatalf("%#v", doc.Setup)
	}
	if !reflect.DeepEqual(doc.Setup[0].Argv, []string{"/tools/pixi", "install", "--manifest-path", manifest}) {
		t.Fatalf("%#v", doc.Setup[0].Argv)
	}
	clone := []string{"/tools/git", "clone", "--depth=1", "--branch", "tcllib-2-0", "--", "https://github.com/tcltk/tcllib.git", filepath.Join(e, "tcllib")}
	if !reflect.DeepEqual(doc.Setup[1].Argv, clone) || doc.Setup[1].Environment["GIT_TERMINAL_PROMPT"] != "0" {
		t.Fatalf("%#v", doc.Setup[1])
	}
	got := doc.Execution.Environment["TCLLIBPATH"]
	if !strings.Contains(got, `\ `) || !strings.Contains(got, `\$`) {
		t.Fatalf("%q", got)
	}
	prefix := []string{"/tools/pixi", "run", "--manifest-path", manifest, "--frozen", "--"}
	if doc.Execution.Argv[len(prefix)] != "tclsh8.6" {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
}

func TestMojoHumanizePlan(t *testing.T) {
	host := pixiHost(lang.Mojo, "")
	spec := "pypi:humanize@4.12.2"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Mojo, spec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.mojo", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", []string{spec}, cache.ProfileMojo, "osx-arm64", nil)
	var body string
	for _, f := range doc.Files {
		if f.Path == "pixi.toml" {
			body = f.Body
		}
	}
	if !strings.Contains(body, `["https://conda.modular.com/max/","conda-forge"]`) || !strings.Contains(body, `"mojo" = "==1.1.0"`) || !strings.Contains(body, `"python" = ">=3.11,<3.12"`) || !strings.Contains(body, "[pypi-dependencies]") || !strings.Contains(body, `"humanize" = "==4.12.2"`) {
		t.Fatalf("%s", body)
	}
	manifest := filepath.Join(e, "pixi.toml")
	want := []string{"/tools/pixi", "run", "--manifest-path", manifest, "--frozen", "--", "mojo", "run", "/work/a.mojo"}
	if doc.EnvironmentDirectory != e || !reflect.DeepEqual(doc.Execution.Argv, want) {
		t.Fatalf("%s %#v", doc.EnvironmentDirectory, doc.Execution.Argv)
	}
}

func TestShellRipgrepStaysZsh(t *testing.T) {
	host := pixiHost(lang.Shell, "/bin/zsh")
	host.Dialect = lang.Zsh
	host.Runtime.Version = "zsh 5.9"
	spec := "ripgrep@15.1.0"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Shell, spec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.zsh", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", []string{spec}, cache.ProfileShell, "osx-arm64", nil)
	manifest := filepath.Join(e, "pixi.toml")
	prefix := []string{"/tools/pixi", "run", "--manifest-path", manifest, "--frozen", "--"}
	if doc.EnvironmentDirectory != e || len(doc.Execution.Argv) < len(prefix)+1 || doc.Execution.Argv[len(prefix)] != "/bin/zsh" {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	var body string
	for _, f := range doc.Files {
		if f.Path == "pixi.toml" {
			body = f.Body
		}
	}
	if !strings.Contains(body, `"ripgrep" = "==15.1.0"`) || strings.Contains(body, "mojo") {
		t.Fatalf("%s", body)
	}
}

func TestRedCommitPlan(t *testing.T) {
	host := fixedHost(lang.Red, "/selected/red", "git")
	spec := "https://github.com/rebolek/red-tools.git#565774c8fb6f1817e04393fa5f034fe6d363c17f"
	script := "/work/a b\"c.red"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Red, spec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: script, CallerCwd: "/caller", ScriptArgs: []string{"z"},
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", []string{spec}, "", "", nil)
	target := filepath.Join(e, "sources", "0")
	steps := [][]string{
		{"/tools/git", "init", "--quiet", target},
		{"/tools/git", "-C", target, "fetch", "--depth=1", "--no-tags", "--", "https://github.com/rebolek/red-tools.git", "565774c8fb6f1817e04393fa5f034fe6d363c17f"},
		{"/tools/git", "-C", target, "checkout", "--quiet", "--detach", "FETCH_HEAD"},
	}
	if len(doc.Setup) != len(steps) {
		t.Fatalf("%#v", doc.Setup)
	}
	for i, want := range steps {
		if !reflect.DeepEqual(doc.Setup[i].Argv, want) || doc.Setup[i].Environment["GIT_TERMINAL_PROMPT"] != "0" || doc.Setup[i].Environment["DEVKIT_DEPS_DIR"] != e {
			t.Fatalf("%#v", doc.Setup[i])
		}
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/red", "<per-run-shim>", "z"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if len(doc.Files) != 1 || !strings.Contains(doc.Files[0].Body, `do %"/work/a b^"c.red"`) || !strings.Contains(doc.Files[0].Body, "quit/return 1") {
		t.Fatalf("%s", doc.Files[0].Body)
	}
}

func TestPixiRejectsUnknownCPU(t *testing.T) {
	host := pixiHost(lang.Shell, "/bin/bash")
	host.Platform = cache.Platform{GOOS: "linux", GOARCH: "386"}
	_, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Shell, "ripgrep@15.1.0"), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.sh", CallerCwd: "/caller",
	})
	if d == nil || d.Code != "platform.unsupported" || d.Exit != 3 {
		t.Fatalf("%+v", d)
	}
}

func TestInstallRootsAreRequired(t *testing.T) {
	cases := []struct {
		id   lang.ID
		spec string
		root string
	}{
		{lang.Perl, "Try::Tiny@0.32", "perl/lib/perl5"},
		{lang.Raku, "Text::Levenshtein@0.2.4", "raku"},
		{lang.Janet, "git::https://github.com/janet-lang/spork.git::v1.3.0", "janet"},
		{lang.Lua, "lustache@1.3.1-0", "lua"},
		{lang.PowerShell, "powershell-yaml@0.4.12", "modules"},
		{lang.Red, "https://github.com/rebolek/red-tools.git#565774c8fb6f1817e04393fa5f034fe6d363c17f", "sources/0"},
		{lang.Node, "left-pad@1.3.0", "node_modules"},
		{lang.R, "praise@1.0.0", "library"},
		{lang.Odin, "http=https://github.com/laytan/odin-http.git#fac113fbd828aad3d71479a534b5de4358b6a07b", "collections/http"},
		{lang.Forth, "https://github.com/forthy42/gforth.git#c9536461d750ea7b7ac8b738bf8763b839ecf1b9", "sources/0"},
		{lang.Factor, "https://github.com/mrjbq7/re-factor.git#eb488704620c49a023d77cf78d2cbcb72b9f1a73", "sources/0"},
		{lang.NASM, "https://github.com/netwide-assembler/nasm.git#fbdc88565ccccfa0e8c69b21b1953d1eded02e89", "sources/0"},
	}
	for _, tc := range cases {
		host := fixedHost(tc.id, "/selected/"+string(tc.id), managerFor(tc.id))
		if tc.id == lang.Lua {
			host.Runtime.Version = "Lua 5.4.7"
		}
		policy := plan.Policy{CacheRoot: "/cache/devkit-deps", Script: "/work/a", CallerCwd: "/caller"}
		if tc.id == lang.NASM {
			policy.Look = ccLooker()
		}
		doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, tc.id, tc.spec), host, policy)
		if d != nil {
			t.Fatalf("%s: %+v", tc.id, d)
		}
		want := []string{tc.root}
		if tc.id == lang.Forth {
			want = []string{"sources/0", "prelude.fs"}
		}
		if !reflect.DeepEqual(doc.RequiredFiles, want) {
			t.Fatalf("%s required %#v", tc.id, doc.RequiredFiles)
		}
	}
}

func managerFor(id lang.ID) string {
	switch id {
	case lang.Perl:
		return "cpanm"
	case lang.Raku:
		return "zef"
	case lang.Janet:
		return "jpm"
	case lang.Lua:
		return "luarocks"
	case lang.PowerShell:
		return "pwsh"
	case lang.Red:
		return "git"
	case lang.Node:
		return "npm"
	case lang.R:
		return "Rscript"
	case lang.Odin, lang.Forth, lang.Factor, lang.NASM:
		return "git"
	default:
		return string(id)
	}
}
