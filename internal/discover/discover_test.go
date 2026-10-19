package discover

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/lang"
)

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestMissingRuntimeIsReportedWithoutASubstitute(t *testing.T) {
	host, d := Discover(lang.Lua, "", "", "", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, LookerFunc(func(string) (string, time.Time, bool) {
		return "", time.Time{}, false
	}))
	if d != nil || host.Runtime.Status != "missing" || host.Runtime.Path != "" || !contains(host.MissingTools, "lua") {
		t.Fatalf("%+v %+v", host, d)
	}
}

func TestRuntimeVersionSelectsMatchingBinary(t *testing.T) {
	root := t.TempDir()
	lua55 := filepath.Join(root, "lua")
	lua54 := filepath.Join(root, "lua5.4")
	os.WriteFile(lua55, []byte("#!/bin/sh\necho 'Lua 5.5.1'\n"), 0o755)
	os.WriteFile(lua54, []byte("#!/bin/sh\necho 'Lua 5.4.7'\n"), 0o755)
	look := LookerFunc(func(name string) (string, time.Time, bool) {
		switch name {
		case "lua":
			return lua55, time.Time{}, true
		case "lua5.4", "lua54":
			return lua54, time.Time{}, true
		default:
			return "", time.Time{}, false
		}
	})
	host, d := Discover(lang.Lua, "", "", "5.4", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, look)
	if d != nil || host.Runtime.Path != lua54 || host.Runtime.RequestedVersion != "5.4" || host.Origin != "version" {
		t.Fatalf("%+v %+v", host, d)
	}
}

func TestRuntimeVersionRejectsNonMatchingInstall(t *testing.T) {
	root := t.TempDir()
	lua55 := filepath.Join(root, "lua")
	os.WriteFile(lua55, []byte("#!/bin/sh\necho 'Lua 5.5.1'\n"), 0o755)
	look := LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "lua" {
			return lua55, time.Time{}, true
		}
		return "", time.Time{}, false
	})
	_, d := Discover(lang.Lua, "", "", "5.4", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, look)
	if d == nil || d.Code != "runtime.version" || d.Exit != 3 {
		t.Fatalf("%+v", d)
	}
}

func TestLuaABIRejected(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "lua")
	os.WriteFile(bin, []byte("#!/bin/sh\necho LuaJIT 2.1\n"), 0o755)
	_, d := Discover(lang.Lua, "", bin, "", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, nil)
	if d == nil || d.Code != "runtime.abi" || d.Exit != 3 {
		t.Fatalf("%+v", d)
	}
}

func TestShellDoesNotSubstituteBash(t *testing.T) {
	host, d := Discover(lang.Shell, "bash", "", "", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "zsh" {
			return "/bin/zsh", time.Time{}, true
		}
		return "", time.Time{}, false
	}))
	if d != nil || host.Runtime.Status != "missing" {
		t.Fatalf("zsh must not satisfy bash: %+v %+v", host, d)
	}
}

func TestAbsoluteRuntimeSkipsPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "lua")
	os.WriteFile(bin, []byte("#!/bin/sh\necho 'Lua 5.5.1'\n"), 0o755)
	host, d := Discover(lang.Lua, "", bin, "", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "lua" || name == "lua5.4" || name == "lua54" {
			t.Fatal("runtime PATH lookup should not run")
		}
		return "", time.Time{}, false
	}))
	if d != nil || host.Runtime.Path != bin || host.Origin != "flag" {
		t.Fatalf("%+v %+v", host, d)
	}
}

func TestRelativeRuntimeRejected(t *testing.T) {
	_, d := Discover(lang.Lua, "", "lua", "", "", cache.Platform{GOOS: "linux", GOARCH: "amd64"}, nil)
	if d == nil || d.Code != "cli.path" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestComposerPharSibling(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.bat"), []byte("@echo off\r\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "composer.phar"), []byte("fixture"), 0o644)
	got, err := ResolveSibling(filepath.Join(dir, "composer.bat"), []string{"composer.phar"})
	if err != nil || got != filepath.Join(dir, "composer.phar") {
		t.Fatalf("%s %v", got, err)
	}
}

func TestVersionMatchesPrefix(t *testing.T) {
	if !VersionMatches("Lua 5.4.7", "5.4") || VersionMatches("Lua 5.5.1", "5.4") || !VersionMatches("anything", "") {
		t.Fatal("prefix match")
	}
	if !VersionMatches("v20.11.1", "20") || VersionMatches("v18.20.4", "20") {
		t.Fatal("major match")
	}
}

func TestNodeMajorSelectsVersionedBinary(t *testing.T) {
	root := t.TempDir()
	node20 := filepath.Join(root, "node20")
	node := filepath.Join(root, "node")
	if err := os.WriteFile(node20, []byte("#!/bin/sh\necho v20.11.1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, []byte("#!/bin/sh\necho v18.20.4\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var looked []string
	look := LookerFunc(func(name string) (string, time.Time, bool) {
		looked = append(looked, name)
		switch name {
		case "node20":
			return node20, time.Unix(1, 0), true
		case "node":
			return node, time.Unix(1, 0), true
		case "npm":
			return "/tools/npm", time.Unix(1, 0), true
		default:
			return "", time.Time{}, false
		}
	})
	host, d := Discover(lang.Node, "", "", "20", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, look)
	if d != nil || host.Runtime.Path != node20 || host.Origin != "version" || host.Manager.Name != "npm" || host.Manager.Path != "/tools/npm" {
		t.Fatalf("%+v %+v looked %v", host, d, looked)
	}
	if contains(looked, "node") {
		t.Fatalf("plain node should wait until node20 misses: %v", looked)
	}
}

func TestNodeDottedVersionFallsBackToMajorBinary(t *testing.T) {
	root := t.TempDir()
	node20 := filepath.Join(root, "node20")
	if err := os.WriteFile(node20, []byte("#!/bin/sh\necho v20.11.1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var looked []string
	look := LookerFunc(func(name string) (string, time.Time, bool) {
		looked = append(looked, name)
		switch name {
		case "node20":
			return node20, time.Unix(1, 0), true
		case "npm":
			return "/tools/npm", time.Unix(1, 0), true
		default:
			return "", time.Time{}, false
		}
	})
	host, d := Discover(lang.Node, "", "", "20.11.1", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, look)
	if d != nil || host.Runtime.Path != node20 || host.Origin != "version" {
		t.Fatalf("%+v %+v looked %v", host, d, looked)
	}
	if len(looked) < 2 || looked[0] != "node20.11.1" || looked[1] != "node20" {
		t.Fatalf("looked %v", looked)
	}
}

func TestRscriptVersionOnStderr(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "Rscript")
	body := "#!/bin/sh\necho 'R scripting front-end version 4.4.1 (2024-06-14)' >&2\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	host, d := Discover(lang.R, "", "", "4.4", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, LookerFunc(func(name string) (string, time.Time, bool) {
		if name != "Rscript" {
			t.Fatalf("looked up %s", name)
		}
		return bin, time.Unix(1, 0), true
	}))
	if d != nil || host.Runtime.Path != bin || host.Manager.Path != bin || host.Manager.Name != "Rscript" || host.Origin != "path" {
		t.Fatalf("%+v %+v", host, d)
	}
}

func TestMissingRNamesRscript(t *testing.T) {
	host, d := Discover(lang.R, "", "", "", "", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, LookerFunc(func(string) (string, time.Time, bool) {
		return "", time.Time{}, false
	}))
	if d != nil || host.Runtime.Status != "missing" || !contains(host.MissingTools, "Rscript") || contains(host.MissingTools, "r") {
		t.Fatalf("%+v %+v", host, d)
	}
}

func TestFactorProbeRejectsCoreutils(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "factor")
	script := "#!/bin/sh\necho \"factor: invalid option -- e\" >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	platform := cache.Platform{GOOS: "darwin", GOARCH: "arm64"}
	look := LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "factor" {
			return bin, time.Unix(1, 0), true
		}
		return "", time.Time{}, false
	})
	host, d := Discover(lang.Factor, "", "", "", "", platform, look)
	if d != nil || host.Runtime.Status != "missing" || host.Runtime.Path != "" || !contains(host.MissingTools, "factor") {
		t.Fatalf("%+v %+v", host, d)
	}

	vm := filepath.Join(dir, "factor-vm")
	if err := os.WriteFile(vm, []byte("#!/bin/sh\necho 0.99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	look = LookerFunc(func(name string) (string, time.Time, bool) {
		if name == "factor" {
			return vm, time.Unix(1, 0), true
		}
		return "", time.Time{}, false
	})
	host, d = Discover(lang.Factor, "", "", "", "", platform, look)
	if d != nil || host.Runtime.Status != "ok" || host.Runtime.Path != vm || host.Runtime.Version != "0.99" {
		t.Fatalf("%+v %+v", host, d)
	}
}

func TestForthAndNASMNameTheirHostTools(t *testing.T) {
	platform := cache.Platform{GOOS: "darwin", GOARCH: "arm64"}
	miss := LookerFunc(func(string) (string, time.Time, bool) { return "", time.Time{}, false })
	host, d := Discover(lang.Forth, "", "", "", "", platform, miss)
	if d != nil || !contains(host.MissingTools, "gforth") || !contains(host.MissingTools, "git") || contains(host.MissingTools, "forth") {
		t.Fatalf("forth %+v %+v", host, d)
	}
	host, d = Discover(lang.NASM, "", "", "", "", platform, miss)
	if d != nil || !contains(host.MissingTools, "nasm") || !contains(host.MissingTools, "git") || !contains(host.MissingTools, "cc") {
		t.Fatalf("nasm %+v %+v", host, d)
	}
}

func TestRManagerFlagRejected(t *testing.T) {
	_, d := Discover(lang.R, "", "", "", "/usr/bin/Rscript", cache.Platform{GOOS: "darwin", GOARCH: "arm64"}, nil)
	if d == nil || d.Code != "cli.manager" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}
