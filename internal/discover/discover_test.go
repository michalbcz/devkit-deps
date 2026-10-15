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
}
