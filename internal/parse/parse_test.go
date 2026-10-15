package parse

import (
	"reflect"
	"testing"

	"devkit-deps/internal/lang"
)

func TestP01OrderedSpecsAndWhitespace(t *testing.T) {
	src := "\n  #   devkit-deps:   [\"Try::Tiny@0.32\", \"Other@1\"]  \nprint 1\n"
	got, d := Parse(src, lang.Perl)
	if d != nil || got.Kind != KindDependencies || !reflect.DeepEqual(got.Specs, []string{"Try::Tiny@0.32", "Other@1"}) {
		t.Fatalf("%+v %+v", got, d)
	}
}

func TestP02ShebangThenMetadata(t *testing.T) {
	src := "#!/usr/bin/env bash\n# devkit-deps: [\"ripgrep@15.1.0\"]\nprintf hi\n"
	got, d := Parse(src, lang.Shell)
	if d != nil || got.Specs[0] != "ripgrep@15.1.0" || got.Line != 2 {
		t.Fatalf("%+v %+v", got, d)
	}
}

func TestP03PHPTagAndRedHeader(t *testing.T) {
	php := "<?php\n// devkit-deps: [\"symfony/yaml@7.4.21\"]\necho 1;\n"
	got, d := Parse(php, lang.PHP)
	if d != nil || got.Line != 2 {
		t.Fatalf("php %+v %+v", got, d)
	}
	red := "; devkit-deps: [\"https://host/repository.git#0123456789abcdef0123456789abcdef01234567\"]\nRed [Title: \"x\"]\n"
	got, d = Parse(red, lang.Red)
	if d != nil || got.Line != 1 {
		t.Fatalf("red %+v %+v", got, d)
	}
}

func TestP04MarkerAfterCodeAndLuaLongComment(t *testing.T) {
	goSrc := "package main\nvar example = `\n// devkit-deps: [\"example@1.0\"]\n`\n"
	got, d := Parse(goSrc, lang.GoLang)
	if d != nil || got.Kind != KindAbsent {
		t.Fatalf("go %+v %+v", got, d)
	}
	lua := "--[[\n-- devkit-deps: [\"dkjson@2.11-1\"]\n--]]\n"
	got, d = Parse(lua, lang.Lua)
	if d != nil || got.Kind != KindAbsent {
		t.Fatalf("lua %+v %+v", got, d)
	}
}

func TestP05DuplicateDeclarations(t *testing.T) {
	_, d := Parse("# devkit-deps: []\n# devkit-deps: []\n", lang.Perl)
	if d == nil || d.Code != "declaration.duplicate" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestP06BadJSON(t *testing.T) {
	for _, body := range []string{"not JSON", "{}", "[1]", "[\"ok\"] trailing", "[\"a\", null]"} {
		_, d := Parse("# devkit-deps: "+body+"\n", lang.Perl)
		if d == nil || d.Code != "declaration.json" {
			t.Fatalf("%s -> %+v", body, d)
		}
	}
}

func TestP07OptionLikeSpec(t *testing.T) {
	_, d := Parse("# devkit-deps: [\"--global\"]\n", lang.Perl)
	if d == nil || d.Code != "package.option" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestP08AbsentVersusEmpty(t *testing.T) {
	absent, d := Parse("print 1\n", lang.Perl)
	if d != nil || absent.Kind != KindAbsent {
		t.Fatalf("absent %+v %+v", absent, d)
	}
	empty, d := Parse("# devkit-deps: []\nprint 1\n", lang.Perl)
	if d != nil || empty.Kind != KindDependencies || len(empty.Specs) != 0 {
		t.Fatalf("empty %+v %+v", empty, d)
	}
}

func TestBOMRejected(t *testing.T) {
	src := "\uFEFF# devkit-deps: [\"Try::Tiny@0.32\"]\n"
	_, d := Parse(src, lang.Perl)
	if d == nil || d.Code != "declaration.bom" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}
