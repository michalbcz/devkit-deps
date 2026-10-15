package plan_test

import (
	"reflect"
	"strings"
	"testing"

	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
)

func hostLua() discover.Host {
	return discover.Host{
		Language: lang.Lua,
		Origin:   "path",
		Runtime: discover.Tool{
			Name: "lua", Path: "/selected/lua", Version: "5.5.1", Status: "ok",
		},
	}
}

func TestEmptyDeclarationDoesNotNameAnEnvironment(t *testing.T) {
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, nil, hostLua(), plan.Policy{
		Script: "/work/a.lua", CallerCwd: "/work",
	})
	if d != nil || doc.Declaration != "empty" || doc.EnvironmentDirectory != "" || len(doc.Setup) != 0 {
		t.Fatalf("%+v %+v", doc, d)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/lua", "/work/a.lua"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if len(doc.Execution.Environment) != 0 {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
}

func TestAbsentStaysDistinctFromEmpty(t *testing.T) {
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, hostLua(), plan.Policy{Script: "/work/a.lua", CallerCwd: "/work"})
	if d != nil || doc.Declaration != "absent" {
		t.Fatalf("%+v %+v", doc, d)
	}
}

func TestGoWithoutPackagesBuildsInScriptDir(t *testing.T) {
	host := discover.Host{
		Language: lang.GoLang,
		Origin:   "path",
		Runtime:  discover.Tool{Name: "go", Path: "/selected/go", Version: "go version go1.22.5", Status: "ok"},
	}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/a.go", ScriptArgs: []string{"x"}, CallerCwd: "/caller",
	})
	if d != nil || doc.EnvironmentDirectory != "" || len(doc.Setup) != 0 {
		t.Fatalf("%+v %+v", doc, d)
	}
	if len(doc.Execution.PreRun) != 1 {
		t.Fatalf("%#v", doc.Execution.PreRun)
	}
	pre := doc.Execution.PreRun[0]
	want := []string{"/selected/go", "build", "-o", "<per-run-binary>", "/work/a.go"}
	if !reflect.DeepEqual(pre.Argv, want) || pre.Cwd != "/work" {
		t.Fatalf("%#v cwd %s", pre.Argv, pre.Cwd)
	}
	if pre.Environment["GOWORK"] != "off" || pre.Environment["GOTOOLCHAIN"] != "local" || pre.Environment["GO111MODULE"] != "on" || pre.Environment["GOFLAGS"] != "" {
		t.Fatalf("%#v", pre.Environment)
	}
	if _, ok := pre.Environment["DEVKIT_DEPS_DIR"]; ok {
		t.Fatal("no-deps plan must not set DEVKIT_DEPS_DIR")
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"<per-run-binary>", "x"}) || doc.Execution.Cwd != "/caller" {
		t.Fatalf("%#v %s", doc.Execution.Argv, doc.Execution.Cwd)
	}
}

func TestRedWithoutPackagesRecordsShim(t *testing.T) {
	host := discover.Host{
		Language: lang.Red,
		Origin:   "path",
		Runtime:  discover.Tool{Name: "red", Path: "/selected/red", Version: "red", Status: "ok"},
	}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		Script: "/work/a b\"c.red", CallerCwd: "/caller", ScriptArgs: []string{"z"},
	})
	if d != nil {
		t.Fatal(d)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/red", "<per-run-shim>", "z"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if len(doc.Files) != 1 || doc.Files[0].Path != "<per-run-shim>" {
		t.Fatalf("%#v", doc.Files)
	}
	if !strings.Contains(doc.Files[0].Body, `do %"/work/a b^"c.red"`) || !strings.Contains(doc.Files[0].Body, "quit/return 1") {
		t.Fatalf("%s", doc.Files[0].Body)
	}
}

func TestEncodeUsesEmptyCollections(t *testing.T) {
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, hostLua(), plan.Policy{Script: "/work/a.lua", CallerCwd: "/work"})
	if d != nil {
		t.Fatal(d)
	}
	body, err := plan.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.HasSuffix(text, "\n") || strings.Contains(text, "null") {
		t.Fatalf("%s", text)
	}
	if !strings.Contains(text, `"schemaVersion": 1`) || !strings.Contains(text, `"declaration": "absent"`) {
		t.Fatalf("%s", text)
	}
}
