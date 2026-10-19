package plan_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
)

const (
	odinSpec   = "http=https://github.com/laytan/odin-http.git#fac113fbd828aad3d71479a534b5de4358b6a07b"
	forthSpec  = "https://github.com/forthy42/gforth.git#c9536461d750ea7b7ac8b738bf8763b839ecf1b9"
	factorSpec = "https://github.com/mrjbq7/re-factor.git#eb488704620c49a023d77cf78d2cbcb72b9f1a73"
	nasmSpec   = "https://github.com/netwide-assembler/nasm.git#fbdc88565ccccfa0e8c69b21b1953d1eded02e89"
)

func TestOdinCollectionBuild(t *testing.T) {
	host := fixedHost(lang.Odin, "/selected/odin", "git")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Odin, odinSpec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/hello.odin", CallerCwd: "/caller", ScriptArgs: []string{"a"},
	})
	if d != nil {
		t.Fatal(d)
	}
	root := filepath.Join(doc.EnvironmentDirectory, "collections")
	target := filepath.Join(root, "http")
	if doc.Setup[1].Argv[len(doc.Setup[1].Argv)-2] != "https://github.com/laytan/odin-http.git" || doc.Setup[1].Argv[len(doc.Setup[1].Argv)-1] != "fac113fbd828aad3d71479a534b5de4358b6a07b" {
		t.Fatalf("%#v", doc.Setup[1].Argv)
	}
	if doc.Setup[0].Argv[len(doc.Setup[0].Argv)-1] != target {
		t.Fatalf("%#v", doc.Setup[0].Argv)
	}
	want := []string{"/selected/odin", "build", "/work/hello.odin", "-file", "-collection:deps=" + root, "-out:<per-run-binary>"}
	if !reflect.DeepEqual(doc.Execution.PreRun[0].Argv, want) || !reflect.DeepEqual(doc.Execution.Argv, []string{"<per-run-binary>", "a"}) {
		t.Fatalf("pre %#v\nrun %#v", doc.Execution.PreRun[0].Argv, doc.Execution.Argv)
	}
}

func TestOdinWithoutPackagesBuildsFile(t *testing.T) {
	host := fixedHost(lang.Odin, "/selected/odin", "git")
	host.MissingTools = []string{"git"}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/hello.odin", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	if strings.Contains(strings.Join(doc.Execution.PreRun[0].Argv, " "), "-collection:") || doc.Execution.Environment["DEVKIT_DEPS_DIR"] != "" {
		t.Fatalf("%#v %#v", doc.Execution.PreRun, doc.Execution.Environment)
	}
}

func TestForthPreludeDoesNotReplacePath(t *testing.T) {
	host := fixedHost(lang.Forth, "/selected/gforth", "git")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Forth, forthSpec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/hello.fs", CallerCwd: "/caller", ScriptArgs: []string{"hi"},
	})
	if d != nil {
		t.Fatal(d)
	}
	dir := filepath.Join(doc.EnvironmentDirectory, "sources", "0")
	if len(doc.Files) != 1 || doc.Files[0].Path != "prelude.fs" || doc.Files[0].Body != "s\" "+dir+"\" fpath also-path\n" {
		t.Fatalf("%#v", doc.Files)
	}
	want := []string{"/selected/gforth", "--no-rc", filepath.Join(doc.EnvironmentDirectory, "prelude.fs"), "/work/hello.fs", "hi"}
	if !reflect.DeepEqual(doc.Execution.Argv, want) || doc.Execution.Environment["GFORTH_ENV"] != "off" || strings.Contains(strings.Join(doc.Execution.Argv, " "), "--path") {
		t.Fatalf("%#v %#v", doc.Execution.Argv, doc.Execution.Environment)
	}
}

func TestForthWithoutPackagesStaysNonInteractive(t *testing.T) {
	host := fixedHost(lang.Forth, "/selected/gforth", "git")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/hello.fs", CallerCwd: "/caller", ScriptArgs: []string{"hi"},
	})
	if d != nil {
		t.Fatal(d)
	}
	want := []string{"/selected/gforth", "--no-rc", "/work/hello.fs", "hi"}
	if !reflect.DeepEqual(doc.Execution.Argv, want) || doc.Execution.Environment["GFORTH_ENV"] != "off" || doc.Execution.Environment["DEVKIT_DEPS_DIR"] != "" {
		t.Fatalf("%#v %#v", doc.Execution.Argv, doc.Execution.Environment)
	}
}

func TestForthRejectsQuoteInSearchPath(t *testing.T) {
	host := fixedHost(lang.Forth, "/selected/gforth", "git")
	_, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Forth, forthSpec), host, plan.Policy{
		CacheRoot: "/cache/a\"b", Script: "/work/hello.fs", CallerCwd: "/caller",
	})
	if d == nil || d.Code != "plan.path" || d.Exit != 4 {
		t.Fatalf("%+v", d)
	}
}

func TestFactorRootsUsePathList(t *testing.T) {
	host := fixedHost(lang.Factor, "/selected/factor", "git")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Factor, factorSpec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/hello.factor", CallerCwd: "/caller", ScriptArgs: []string{"x"},
	})
	if d != nil {
		t.Fatal(d)
	}
	root := filepath.Join(doc.EnvironmentDirectory, "sources", "0")
	want := []string{"/selected/factor", "-no-user-init", "-roots=" + root, "/work/hello.factor", "x"}
	if !reflect.DeepEqual(doc.Execution.Argv, want) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
}

func TestFactorRejectsSeparatorInRoot(t *testing.T) {
	host := fixedHost(lang.Factor, "/selected/factor", "git")
	root := "/cache/a" + string(os.PathListSeparator) + "b"
	_, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Factor, factorSpec), host, plan.Policy{
		CacheRoot: root, Script: "/work/hello.factor", CallerCwd: "/caller",
	})
	if d == nil || d.Code != "plan.path" || d.Exit != 4 {
		t.Fatalf("%+v", d)
	}
}

func TestNASMLinksMachoOnDarwin(t *testing.T) {
	host := fixedHost(lang.NASM, "/selected/nasm", "git")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.NASM, nasmSpec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/hello.asm", CallerCwd: "/caller", ScriptArgs: []string{"a"}, Look: ccLooker(),
	})
	if d != nil {
		t.Fatal(d)
	}
	inc := "-i" + filepath.Join(doc.EnvironmentDirectory, "sources", "0")
	nasm := []string{"/selected/nasm", "-f", "macho64", inc, "-o", "<per-run-binary>.o", "/work/hello.asm"}
	cc := []string{"/usr/bin/cc", "-arch", "x86_64", "-o", "<per-run-binary>", "<per-run-binary>.o"}
	if !reflect.DeepEqual(doc.Execution.PreRun[0].Argv, nasm) || !reflect.DeepEqual(doc.Execution.PreRun[1].Argv, cc) {
		t.Fatalf("%#v\n%#v", doc.Execution.PreRun[0].Argv, doc.Execution.PreRun[1].Argv)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"<per-run-binary>", "a"}) || doc.Execution.Cwd != "/caller" {
		t.Fatalf("%#v %s", doc.Execution.Argv, doc.Execution.Cwd)
	}
}

func TestNASMLinksElfOnLinuxAMD64(t *testing.T) {
	host := fixedHost(lang.NASM, "/selected/nasm", "git")
	host.Platform = cache.Platform{GOOS: "linux", GOARCH: "amd64"}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/hello.asm", CallerCwd: "/caller", Look: ccLooker(),
	})
	if d != nil {
		t.Fatal(d)
	}
	if doc.Execution.PreRun[0].Argv[2] != "elf64" || strings.Join(doc.Execution.PreRun[1].Argv, " ") != "/usr/bin/cc -o <per-run-binary> <per-run-binary>.o" {
		t.Fatalf("%#v %#v", doc.Execution.PreRun[0].Argv, doc.Execution.PreRun[1].Argv)
	}
}

func TestNASMRejectsOtherPlatforms(t *testing.T) {
	host := fixedHost(lang.NASM, "/selected/nasm", "git")
	host.Platform = cache.Platform{GOOS: "linux", GOARCH: "arm64"}
	_, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/hello.asm", CallerCwd: "/caller", Look: ccLooker(),
	})
	if d == nil || d.Code != "platform.unsupported" || d.Exit != 3 {
		t.Fatalf("%+v", d)
	}
}

func TestNASMWithoutCC(t *testing.T) {
	host := fixedHost(lang.NASM, "/selected/nasm", "git")
	_, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/hello.asm", CallerCwd: "/caller",
	})
	if d == nil || d.Code != "tool.missing" || d.Exit != 3 || !reflect.DeepEqual(d.MissingTools, []string{"cc"}) {
		t.Fatalf("%+v", d)
	}
}
