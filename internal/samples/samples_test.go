package samples_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/engine"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/validate"
)

type sample struct {
	file           string
	lang           lang.ID
	runtimeVersion string
	specs          []string
	probe          string
}

func TestSamplesDeclareDependenciesAndRuntime(t *testing.T) {
	root := samplesDir(t)
	catalog := []sample{
		{file: "hello.pl", lang: lang.Perl, runtimeVersion: "5.40", specs: []string{"Try::Tiny@0.32"}, probe: "5.40.1"},
		{file: "hello.php", lang: lang.PHP, runtimeVersion: "8.3", specs: []string{"symfony/yaml@7.4.21"}, probe: "8.3.14"},
		{file: "hello.raku", lang: lang.Raku, specs: []string{"Text::Levenshtein@0.2.4"}, probe: "2024.10"},
		{file: "hello.janet", lang: lang.Janet, specs: []string{"git::https://github.com/janet-lang/spork.git::v1.3.0"}, probe: "1.35.0"},
		{file: "hello.lua", lang: lang.Lua, runtimeVersion: "5.4", specs: []string{"lustache@1.3.1-0"}, probe: "Lua 5.4.2"},
		{file: "hello.go", lang: lang.GoLang, specs: []string{"rsc.io/quote/v4@v4.0.1"}, probe: "go version go1.22.5"},
		{file: "hello.ps1", lang: lang.PowerShell, specs: []string{"powershell-yaml@0.4.12"}, probe: "7.4.6"},
		{file: "hello.tcl", lang: lang.Tcl, specs: []string{"tcllib@2.0"}, probe: "8.6.14"},
		{file: "hello.mojo", lang: lang.Mojo, specs: []string{"pypi:humanize@4.12.2"}, probe: "1.1.0"},
		{file: "hello.sh", lang: lang.Shell, specs: []string{"ripgrep@15.1.0"}, probe: "5.2.15"},
		{file: "hello.red", lang: lang.Red, specs: []string{"https://github.com/rebolek/red-tools.git#565774c8fb6f1817e04393fa5f034fe6d363c17f"}, probe: "0.6.36"},
		{file: "hello.js", lang: lang.Node, runtimeVersion: "20", specs: []string{"left-pad@1.3.0"}, probe: "v20.11.1"},
		{file: "hello.R", lang: lang.R, specs: []string{"praise@1.0.0"}, probe: "R scripting front-end version 4.4.1 (2024-06-14)"},
		{file: "hello.odin", lang: lang.Odin, specs: []string{"http=https://github.com/laytan/odin-http.git#fac113fbd828aad3d71479a534b5de4358b6a07b"}, probe: "odin version dev-2025-01"},
		{file: "hello.fs", lang: lang.Forth, specs: []string{"https://github.com/forthy42/gforth.git#c9536461d750ea7b7ac8b738bf8763b839ecf1b9"}, probe: "gforth 0.7.3"},
		{file: "hello.factor", lang: lang.Factor, specs: []string{"https://github.com/mrjbq7/re-factor.git#eb488704620c49a023d77cf78d2cbcb72b9f1a73"}, probe: "0.99"},
		{file: "hello.asm", lang: lang.NASM, specs: []string{"https://github.com/netwide-assembler/nasm.git#fbdc88565ccccfa0e8c69b21b1953d1eded02e89"}, probe: "NASM version 2.16.01"},
	}
	seen := map[string]struct{}{}
	for _, item := range catalog {
		seen[item.file] = struct{}{}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		// go.mod only excludes hello.go from the parent module.
		if entry.Name() == "go.mod" {
			continue
		}
		if _, ok := seen[entry.Name()]; !ok {
			t.Fatalf("sample %s is not covered by the test", entry.Name())
		}
		delete(seen, entry.Name())
	}
	if len(seen) != 0 {
		t.Fatalf("missing sample files: %v", seen)
	}

	for _, item := range catalog {
		item := item
		t.Run(item.file, func(t *testing.T) {
			path := filepath.Join(root, item.file)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			id, _, version, d := lang.Infer(path, string(body), "", "")
			if d != nil || id != item.lang || version != item.runtimeVersion {
				t.Fatalf("infer id=%s version=%q err=%v", id, version, d)
			}
			decl, d := parse.Parse(string(body), id)
			if d != nil {
				t.Fatal(d)
			}
			if decl.Kind != parse.KindDependencies {
				t.Fatalf("kind %s", decl.Kind)
			}
			pkgs, d := validate.Validate(id, decl.Specs)
			if d != nil {
				t.Fatal(d)
			}
			got := make([]string, len(pkgs))
			for i, pkg := range pkgs {
				got[i] = pkg.Spec
			}
			if stringsJoin(got) != stringsJoin(item.specs) {
				t.Fatalf("specs %v", got)
			}

			if item.lang == lang.Tcl || item.lang == lang.Mojo || item.lang == lang.Shell {
				if _, d := cache.PixiTarget(cache.Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}); d != nil {
					t.Skip(d.Message)
				}
			}
			doc := planSample(t, path, item.probe)
			if doc["language"] != string(item.lang) {
				t.Fatalf("language %v", doc["language"])
			}
			if stringsJoin(stringList(doc["dependencies"])) != stringsJoin(item.specs) {
				t.Fatalf("plan deps %v", doc["dependencies"])
			}
			runtimeDoc, _ := doc["runtime"].(map[string]any)
			requested, _ := runtimeDoc["requestedVersion"].(string)
			if requested != item.runtimeVersion {
				t.Fatalf("requestedVersion %q", requested)
			}
			if runtimeDoc["status"] != "ok" {
				t.Fatalf("runtime %#v", runtimeDoc)
			}
			if len(stringList(doc["setup"])) == 0 {
				t.Fatal("sample produced no setup")
			}
		})
	}
}

func planSample(t *testing.T, script, probe string) map[string]any {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tool")
	body := "#!/bin/sh\nif [ \"$1\" = version ]; then echo " + shellQuote(probe) + "; exit 0; fi\necho " + shellQuote(probe) + "\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	look := discover.LookerFunc(func(string) (string, time.Time, bool) {
		return bin, time.Unix(1, 0), true
	})
	var stdout, stderr bytes.Buffer
	code := engine.Execute(context.Background(), engine.Request{
		Command: "plan", Script: script, JSON: true,
		CacheDir: t.TempDir(), Look: look,
		Stdout: &stdout, Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("plan exit %d\n%s\n%s", code, stderr.String(), stdout.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func samplesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "samples")
}

func stringList(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		out = append(out, text)
	}
	return out
}

func stringsJoin(items []string) string {
	return "|" + join(items) + "|"
}

func join(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "\n"
		}
		out += item
	}
	return out
}

func shellQuote(value string) string {
	return "'" + value + "'"
}
