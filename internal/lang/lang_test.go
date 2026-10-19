package lang

import "testing"

func TestInferExplicitWinsOverExtension(t *testing.T) {
	id, dialect, version, d := Infer("x.lua", "", Raku, "")
	if d != nil || id != Raku || dialect != "" || version != "" {
		t.Fatalf("id=%s dialect=%s version=%s err=%v", id, dialect, version, d)
	}
}

func TestInferEnvDashSRejected(t *testing.T) {
	_, _, _, d := Infer("tool", "#!/usr/bin/env -S perl -x\n", "", "")
	if d == nil || d.Code != "language.shebang" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestVersionedShebangRequestsRuntime(t *testing.T) {
	id, _, version, d := Infer("tool", "#!/usr/bin/env lua5.4\n", "", "")
	if d != nil || id != Lua || version != "5.4" {
		t.Fatalf("id=%s version=%s err=%v", id, version, d)
	}
	id, _, version, d = Infer("tool", "#!/usr/bin/lua54\n", "", "")
	if d != nil || id != Lua || version != "5.4" {
		t.Fatalf("compact lua54: id=%s version=%s err=%v", id, version, d)
	}
	_, _, version, d = Infer("a.lua", "#!/usr/bin/env php8.3\n", Lua, "")
	if d != nil || version != "" {
		t.Fatalf("other-language shebang must not force a lua version: %s %v", version, d)
	}
	id, _, version, d = Infer("hello.js", "#!/usr/bin/env node20\n", "", "")
	if d != nil || id != Node || version != "20" {
		t.Fatalf("node20: id=%s version=%s err=%v", id, version, d)
	}
	id, _, version, d = Infer("hello.js", "#!/usr/bin/env node20.11.1\n", "", "")
	if d != nil || id != Node || version != "20.11.1" {
		t.Fatalf("node20.11.1: id=%s version=%s err=%v", id, version, d)
	}
	_, _, _, d = Infer("tool", "#!/usr/bin/env lua5\n", "", "")
	if d == nil || d.Code != "language.shebang" {
		t.Fatalf("lua5 must stay unread: %+v", d)
	}
	id, _, version, d = Infer("hello.R", "#!/usr/bin/env Rscript\n", "", "")
	if d != nil || id != R || version != "" {
		t.Fatalf("Rscript: id=%s version=%s err=%v", id, version, d)
	}
	id, _, version, d = Infer("hello.js", "#!/usr/bin/env nodejs\n", "", "")
	if d != nil || id != Node || version != "" {
		t.Fatalf("nodejs: id=%s version=%s err=%v", id, version, d)
	}
}

func TestShellDefaultsToBash(t *testing.T) {
	id, dialect, _, d := Infer("noext", "#!/bin/sh\n", "", "")
	if d != nil || id != Shell || dialect != Dialect("sh") {
		t.Fatalf("id=%s dialect=%s err=%v", id, dialect, d)
	}
	id, dialect, _, d = Infer("script.sh", "", "", "")
	if d != nil || dialect != "sh" {
		t.Fatalf("sh extension: %s %v", dialect, d)
	}
	id, dialect, _, d = Infer("script", "", Shell, "")
	if d != nil || dialect != "bash" {
		t.Fatalf("default bash: %s %v", dialect, d)
	}
}

func TestCommentPrefixes(t *testing.T) {
	cases := map[ID]string{Perl: "#", PHP: "//", Lua: "--", Red: ";", GoLang: "//", Node: "//", R: "#"}
	for id, prefix := range cases {
		got, err := CommentPrefix(id)
		if err != nil || got != prefix {
			t.Fatalf("%s: %q %v", id, got, err)
		}
	}
}

func TestExtensionlessWithoutShebangRejected(t *testing.T) {
	_, _, _, d := Infer("tool", "print 1\n", "", "")
	if d == nil || d.Code != "language.unknown" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestShellFlagRejectedOnOtherLanguage(t *testing.T) {
	_, _, _, d := Infer("a.pl", "", Perl, "zsh")
	if d == nil || d.Code != "cli.shell" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}
