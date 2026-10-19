package validate

import "testing"

import "devkit-deps/internal/lang"

func TestPixiPrefixConflict(t *testing.T) {
	_, d := Validate(lang.Shell, []string{"conda:ripgrep@15.1.0", "ripgrep@15.2.0"})
	if d == nil || d.Code != "package.conflict" || d.Exit != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestStackedPixiPrefixRejected(t *testing.T) {
	_, d := Validate(lang.Mojo, []string{"conda:pypi:humanize@4.12.2"})
	if d == nil || d.Code != "package.prefix" {
		t.Fatalf("%+v", d)
	}
}

func TestJanetRevisionConflict(t *testing.T) {
	specs := []string{
		"git::https://github.com/janet-lang/spork.git::v1.3.0",
		"git::https://github.com/janet-lang/spork.git::v1.2.0",
	}
	_, d := Validate(lang.Janet, specs)
	if d == nil || d.Code != "package.conflict" {
		t.Fatalf("%+v", d)
	}
}

func TestAcceptedFixturePins(t *testing.T) {
	cases := []struct {
		id   lang.ID
		spec string
	}{
		{lang.Perl, "Try::Tiny@0.32"},
		{lang.PHP, "symfony/yaml@7.4.21"},
		{lang.Raku, "Text::Levenshtein@0.2.4"},
		{lang.Janet, "git::https://github.com/janet-lang/spork.git::v1.3.0"},
		{lang.Lua, "lustache@1.3.1-0"},
		{lang.GoLang, "rsc.io/quote/v4@v4.0.1"},
		{lang.PowerShell, "powershell-yaml@0.4.12"},
		{lang.Tcl, "tcllib@2.0"},
		{lang.Mojo, "pypi:humanize@4.12.2"},
		{lang.Shell, "ripgrep@15.1.0"},
		{lang.Red, "https://github.com/rebolek/red-tools.git#565774c8fb6f1817e04393fa5f034fe6d363c17f"},
		{lang.Node, "left-pad@1.3.0"},
		{lang.Node, "@sindresorhus/is@4.6.0"},
		{lang.R, "praise@1.0.0"},
		{lang.Odin, "http=https://github.com/laytan/odin-http.git#fac113fbd828aad3d71479a534b5de4358b6a07b"},
		{lang.Forth, "https://github.com/forthy42/gforth.git#c9536461d750ea7b7ac8b738bf8763b839ecf1b9"},
		{lang.Factor, "https://github.com/mrjbq7/re-factor.git#eb488704620c49a023d77cf78d2cbcb72b9f1a73"},
		{lang.NASM, "https://github.com/netwide-assembler/nasm.git#fbdc88565ccccfa0e8c69b21b1953d1eded02e89"},
	}
	for _, tc := range cases {
		pkgs, d := Validate(tc.id, []string{tc.spec})
		if d != nil || len(pkgs) != 1 || pkgs[0].Spec != tc.spec {
			t.Fatalf("%s %+v %+v", tc.id, pkgs, d)
		}
	}
}

func TestRejectedNames(t *testing.T) {
	cases := []struct {
		id   lang.ID
		spec string
		code string
	}{
		{lang.Perl, "1bad@1", "package.spec"},
		{lang.PHP, "Symfony/Yaml@1", "package.spec"},
		{lang.GoLang, "quote@v1", "package.spec"},
		{lang.Red, "https://host/repo.git#abc", "package.spec"},
		{lang.Shell, "pypi:humanize@4.12.2", "package.prefix"},
		{lang.Node, "Left-Pad@1.0.0", "package.spec"},
		{lang.Node, "@scope@1.0.0", "package.spec"},
		{lang.R, "1praise@1.0.0", "package.spec"},
		{lang.R, "praise", "package.spec"},
		{lang.Odin, "https://host/repo.git#0123456789abcdef0123456789abcdef01234567", "package.spec"},
		{lang.Odin, "1http=https://host/repo.git#0123456789abcdef0123456789abcdef01234567", "package.spec"},
		{lang.Forth, "https://host/repo.git#abc", "package.spec"},
		{lang.NASM, "left-pad@1.0.0", "package.spec"},
	}
	for _, tc := range cases {
		_, d := Validate(tc.id, []string{tc.spec})
		if d == nil || d.Code != tc.code || d.Exit != 2 {
			t.Fatalf("%s %s -> %+v", tc.id, tc.spec, d)
		}
	}
}

func TestOdinCollectionNameIsIdentity(t *testing.T) {
	spec := "http=https://github.com/laytan/odin-http.git#fac113fbd828aad3d71479a534b5de4358b6a07b"
	pkgs, d := Validate(lang.Odin, []string{spec})
	if d != nil || pkgs[0].Identity != "http" || pkgs[0].Name != "https://github.com/laytan/odin-http.git" || pkgs[0].Version != "fac113fbd828aad3d71479a534b5de4358b6a07b" {
		t.Fatalf("%+v %+v", pkgs, d)
	}
	_, d = Validate(lang.Odin, []string{spec, "http=https://github.com/other/http.git#0123456789abcdef0123456789abcdef01234567"})
	if d == nil || d.Code != "package.conflict" {
		t.Fatalf("%+v", d)
	}
}

func TestNodeScopedNameKeepsScope(t *testing.T) {
	pkgs, d := Validate(lang.Node, []string{"@sindresorhus/is@4.6.0"})
	if d != nil || pkgs[0].Name != "@sindresorhus/is" || pkgs[0].Version != "4.6.0" || pkgs[0].Identity != "@sindresorhus/is" {
		t.Fatalf("%+v %+v", pkgs, d)
	}
}

func TestPixiConstraintKept(t *testing.T) {
	pkgs, d := Validate(lang.Mojo, []string{"conda:python@>=3.11,<3.12"})
	if d != nil || pkgs[0].Name != "python" || pkgs[0].Version != ">=3.11,<3.12" || pkgs[0].Identity != "conda:python" {
		t.Fatalf("%+v %+v", pkgs, d)
	}
}
