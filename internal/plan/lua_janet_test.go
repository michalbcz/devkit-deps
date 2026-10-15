package plan_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
)

func TestJanetTreeKeepsSpaces(t *testing.T) {
	host := fixedHost(lang.Janet, "/selected/janet", "jpm")
	spec := "git::https://github.com/janet-lang/spork.git::v1.3.0"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Janet, spec), host, plan.Policy{
		CacheRoot: "/cache/my deps", Script: "/work/a.janet", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/my deps", []string{spec}, "", "", nil)
	tree := "--tree=" + filepath.Join(e, "janet")
	want := []string{"/selected/janet", "/tools/jpm", tree, "install", spec}
	if len(doc.Setup) != 1 || !reflect.DeepEqual(doc.Setup[0].Argv, want) {
		t.Fatalf("%#v", doc.Setup[0].Argv)
	}
	if doc.Setup[0].Environment["JANET_TREE"] != filepath.Join(e, "janet") {
		t.Fatalf("%#v", doc.Setup[0].Environment)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/janet", "/work/a.janet"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if doc.Execution.Environment["JANET_PATH"] != filepath.Join(e, "janet", "lib") || doc.Execution.Environment["DEVKIT_DEPS_DIR"] != e {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
}

func TestLuaPathsAreCaptured(t *testing.T) {
	host := fixedHost(lang.Lua, "/opt/lua/bin/lua", "luarocks")
	host.Runtime.Version = "Lua 5.4.7"
	spec := "lustache@1.3.1-0"
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Lua, spec), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.lua", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", []string{spec}, "", "", nil)
	common := []string{"/tools/luarocks", "--lua-version=5.4", "--lua-dir=/opt/lua", "--tree=" + filepath.Join(e, "lua")}
	if len(doc.Setup) != 3 {
		t.Fatalf("%#v", doc.Setup)
	}
	install := append(append([]string{}, common...), "install", "lustache", "1.3.1-0", "--deps-mode=one")
	lrPath := append(append([]string{}, common...), "path", "--lr-path")
	lrCPath := append(append([]string{}, common...), "path", "--lr-cpath")
	if !reflect.DeepEqual(doc.Setup[0].Argv, install) || !reflect.DeepEqual(doc.Setup[1].Argv, lrPath) || !reflect.DeepEqual(doc.Setup[2].Argv, lrCPath) {
		t.Fatalf("%#v\n%#v\n%#v", doc.Setup[0].Argv, doc.Setup[1].Argv, doc.Setup[2].Argv)
	}
	if !reflect.DeepEqual(doc.Setup[1].CaptureEnvironment, []string{"LUA_PATH", "LUA_PATH_5_4"}) || !reflect.DeepEqual(doc.Setup[2].CaptureEnvironment, []string{"LUA_CPATH", "LUA_CPATH_5_4"}) {
		t.Fatalf("%#v %#v", doc.Setup[1].CaptureEnvironment, doc.Setup[2].CaptureEnvironment)
	}
	if doc.Execution.Environment["DEVKIT_DEPS_DIR"] != e || doc.Execution.Environment["LUA_PATH"] != "" {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
	if !reflect.DeepEqual(doc.Execution.DeferredEnvironment, []string{"LUA_PATH", "LUA_CPATH", "LUA_PATH_5_4", "LUA_CPATH_5_4"}) {
		t.Fatalf("%#v", doc.Execution.DeferredEnvironment)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/opt/lua/bin/lua", "/work/a.lua"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
}
