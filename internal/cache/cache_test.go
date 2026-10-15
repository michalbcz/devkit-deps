package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashSeparatesPinRuntimeAndProfile(t *testing.T) {
	base := Identity{Language: "go", GOOS: "darwin", GOARCH: "arm64", RuntimePath: "/go", Specs: []string{"rsc.io/quote/v4@v4.0.1"}}
	other := base
	other.Specs = []string{"rsc.io/quote/v4@v4.0.0"}
	if Hash(base) == Hash(other) {
		t.Fatal("pin change must change the hash")
	}
	sameScriptDifferentNothing := base
	if Hash(base) != Hash(sameScriptDifferentNothing) {
		t.Fatal("identical identity must reuse")
	}
	mojo := Identity{Language: "mojo", Profile: ProfileMojo, Specs: []string{"pypi:humanize@4.12.2"}}
	bumped := mojo
	bumped.Profile = "mojo==1.2.0;python>=3.11,<3.12"
	if Hash(mojo) == Hash(bumped) {
		t.Fatal("profile pin change must change the hash")
	}
	lua54 := Identity{Language: "lua", RuntimePath: "/opt/lua54/bin/lua", RuntimeVersion: "Lua 5.4.7", Specs: []string{"lustache@1.3.1-0"}}
	lua55 := lua54
	lua55.RuntimePath = "/opt/lua55/bin/lua"
	lua55.RuntimeVersion = "Lua 5.5.1"
	if Hash(lua54) == Hash(lua55) {
		t.Fatal("runtime path and version must select a different environment")
	}
}

func TestRemoveTreeDoesNotFollowSymlink(t *testing.T) {
	outside := t.TempDir()
	victim := filepath.Join(outside, "keep.txt")
	os.WriteFile(victim, []byte("safe"), 0o644)
	root := filepath.Join(t.TempDir(), "env")
	os.MkdirAll(root, 0o755)
	os.Symlink(victim, filepath.Join(root, "link"))
	if err := RemoveTree(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("symlink target was removed")
	}
}

func TestUnknownCPURejected(t *testing.T) {
	_, d := PixiTarget(Platform{GOOS: "linux", GOARCH: "386"})
	if d == nil || d.Code != "platform.unsupported" || d.Exit != 3 {
		t.Fatalf("%+v", d)
	}
}

func TestReadyAcceptsRequiredDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "perl", "lib", "perl5"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteReady(dir, Ready{Schema: 1, Captured: map[string]string{}, RequiredFiles: []string{"perl/lib/perl5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReady(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "perl")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReady(dir); err == nil {
		t.Fatal("deleted install directory must be incomplete")
	}
}

func TestReadyRejectsMissingRequiredFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteReady(dir, Ready{Schema: 1, Captured: map[string]string{}, RequiredFiles: []string{"php/vendor/autoload.php"}}); err != nil {
		t.Fatal(err)
	}
	_, err := ReadReady(dir)
	if err == nil {
		t.Fatal("missing required file must be incomplete")
	}
}
