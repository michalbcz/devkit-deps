package plan_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
)

func TestNodeInstallsWithNpmAndNodePath(t *testing.T) {
	host := fixedHost(lang.Node, "/selected/node", "npm")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Node, "left-pad@1.3.0", "@sindresorhus/is@4.6.0"), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/hello.js", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	if len(doc.Files) != 1 || doc.Files[0].Path != "package.json" {
		t.Fatalf("%#v", doc.Files)
	}
	var manifest struct {
		Name         string            `json:"name"`
		Private      bool              `json:"private"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(doc.Files[0].Body), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "devkit-deps-script" || !manifest.Private ||
		manifest.Dependencies["left-pad"] != "1.3.0" || manifest.Dependencies["@sindresorhus/is"] != "4.6.0" {
		t.Fatalf("%s", doc.Files[0].Body)
	}
	npm := []string{"/tools/npm", "install", "--prefix", doc.EnvironmentDirectory, "--no-fund", "--no-audit"}
	if !reflect.DeepEqual(doc.Setup[0].Argv, npm) {
		t.Fatalf("%#v", doc.Setup[0].Argv)
	}
	if doc.Execution.Environment["NODE_PATH"] != filepath.Join(doc.EnvironmentDirectory, "node_modules") {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/node", "/work/hello.js"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
}

func TestRInstallsWithPak(t *testing.T) {
	host := fixedHost(lang.R, "/selected/Rscript", "Rscript")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.R, "praise@1.0.0"), host, plan.Policy{
		CacheRoot: "/cache/o'brien", Script: "/work/hello.R", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	if !reflect.DeepEqual(doc.RequiredFiles, []string{"library"}) {
		t.Fatalf("%#v", doc.RequiredFiles)
	}
	argv := doc.Setup[0].Argv
	if len(argv) != 4 || argv[0] != "/selected/Rscript" || argv[1] != "--vanilla" || argv[2] != "-e" {
		t.Fatalf("%#v", argv)
	}
	expr := argv[3]
	for _, want := range []string{"pak::pkg_install", "'praise@1.0.0'", "ask = FALSE", "upgrade = FALSE", `o\'brien`} {
		if !strings.Contains(expr, want) {
			t.Fatalf("missing %s in %s", want, expr)
		}
	}
	lib := filepath.Join(doc.EnvironmentDirectory, "library")
	if doc.Execution.Environment["R_LIBS"] != lib {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/Rscript", "--vanilla", "/work/hello.R"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
}

func TestRWithoutDependenciesUsesVanilla(t *testing.T) {
	host := fixedHost(lang.R, "/selected/Rscript", "Rscript")
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindAbsent}, nil, host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.R", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	if len(doc.Setup) != 0 || !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/Rscript", "--vanilla", "/work/a.R"}) {
		t.Fatalf("setup %#v argv %#v", doc.Setup, doc.Execution.Argv)
	}
}
