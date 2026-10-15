package plan_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
	"devkit-deps/internal/validate"
)

func fixedHost(id lang.ID, runtime, manager string) discover.Host {
	if runtime == "" {
		runtime = "/selected/" + string(id)
	}
	return discover.Host{
		Language: id,
		Origin:   "path",
		Platform: cache.Platform{GOOS: "darwin", GOARCH: "arm64"},
		Runtime:  discover.Tool{Name: string(id), Path: runtime, Version: "1.0.0", Status: "ok"},
		Manager: discover.Tool{
			Name: manager, Path: "/tools/" + manager, Status: "ok",
			ModTime: time.Unix(1_700_000_000, 0),
		},
	}
}

func mustPkgs(t *testing.T, id lang.ID, specs ...string) []validate.Package {
	t.Helper()
	pkgs, d := validate.Validate(id, specs)
	if d != nil {
		t.Fatal(d)
	}
	return pkgs
}

func envDir(host discover.Host, root string, specs []string, profile, pixi string, helpers []string) string {
	mod := int64(0)
	if !host.Manager.ModTime.IsZero() {
		mod = host.Manager.ModTime.Unix()
	}
	if helpers == nil {
		helpers = []string{}
	}
	id := cache.Identity{
		Language: string(host.Language), Dialect: string(host.Dialect),
		GOOS: host.Platform.GOOS, GOARCH: host.Platform.GOARCH,
		PixiTarget: pixi, Profile: profile,
		RuntimePath: host.Runtime.Path, RuntimeVersion: host.Runtime.Version,
		ManagerPath: host.Manager.Path, ManagerModUnix: mod,
		Helpers: helpers, Specs: specs,
	}
	return cache.EnvDir(root, string(host.Language), cache.Hash(id))
}

func TestPerlExactArgv(t *testing.T) {
	host := fixedHost(lang.Perl, "/selected/perl", "cpanm")
	specs := []string{"Try::Tiny@0.32"}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Perl, specs...), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.pl", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", specs, "", "", nil)
	want := []string{
		"/selected/perl", "/tools/cpanm", "--local-lib-contained", filepath.Join(e, "perl"),
		"--no-interactive", "--no-prompt", "--no-sudo", "--no-lwp",
		"--mirror", "https://cpan.metacpan.org", "--mirror-only", "Try::Tiny@0.32",
	}
	if doc.EnvironmentDirectory != e || len(doc.Setup) != 1 || !reflect.DeepEqual(doc.Setup[0].Argv, want) {
		t.Fatalf("dir %s\nargv %#v", doc.EnvironmentDirectory, doc.Setup[0].Argv)
	}
	if doc.Setup[0].Cwd != e || doc.Setup[0].Environment["PERL_CPANM_HOME"] != filepath.Join(e, "cpanm-work") || doc.Setup[0].Environment["PERL_MM_OPT"] != "" || doc.Setup[0].Environment["PERL_MB_OPT"] != "" {
		t.Fatalf("%#v", doc.Setup[0])
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/perl", "-I" + filepath.Join(e, "perl", "lib", "perl5"), "/work/a.pl"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if doc.Execution.Environment["DEVKIT_DEPS_DIR"] != e || doc.Execution.Environment["PERL5LIB"] != filepath.Join(e, "perl", "lib", "perl5") {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
}

func TestPHPExactArgv(t *testing.T) {
	host := fixedHost(lang.PHP, "/selected/php", "composer")
	specs := []string{"symfony/yaml@7.4.21"}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.PHP, specs...), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.php", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", specs, "", "", nil)
	want := []string{
		"/selected/php", "/tools/composer", "install", "--no-interaction", "--no-ansi", "--no-progress",
		"--no-plugins", "--no-scripts", "--prefer-dist",
	}
	if len(doc.Setup) != 1 || !reflect.DeepEqual(doc.Setup[0].Argv, want) || doc.Setup[0].Cwd != filepath.Join(e, "php") {
		t.Fatalf("%#v %s", doc.Setup[0].Argv, doc.Setup[0].Cwd)
	}
	if doc.Setup[0].Environment["COMPOSER"] != "composer.json" || doc.Setup[0].Environment["COMPOSER_HOME"] != filepath.Join(e, "composer-home") || doc.Setup[0].Environment["COMPOSER_VENDOR_DIR"] != filepath.Join(e, "php", "vendor") {
		t.Fatalf("%#v", doc.Setup[0].Environment)
	}
	body := ""
	for _, f := range doc.Files {
		if f.Path == "php/composer.json" {
			body = f.Body
		}
	}
	const wantBody = "{\n  \"name\": \"majkl-devkit/script-dependencies\",\n  \"require\": {\"symfony/yaml\": \"7.4.21\"}\n}\n"
	if body != wantBody {
		t.Fatalf("%q", body)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/php", "-d", "auto_prepend_file=" + filepath.Join(e, "php", "vendor", "autoload.php"), "/work/a.php"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if len(doc.RequiredFiles) != 1 || doc.RequiredFiles[0] != "php/vendor/autoload.php" {
		t.Fatalf("%#v", doc.RequiredFiles)
	}
}

func TestComposerPharSibling(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "composer.bat")
	phar := filepath.Join(dir, "composer.phar")
	os.WriteFile(bat, []byte("@echo off\n"), 0o644)
	os.WriteFile(phar, []byte("phar"), 0o644)
	host := fixedHost(lang.PHP, "/selected/php", "composer")
	host.Manager.Path = bat
	specs := []string{"symfony/yaml@7.4.21"}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.PHP, specs...), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.php", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	if len(doc.Setup) == 0 || doc.Setup[0].Argv[1] != phar {
		t.Fatalf("%#v", doc.Setup)
	}
}

func TestRakuExactArgv(t *testing.T) {
	host := fixedHost(lang.Raku, "/selected/raku", "zef")
	specs := []string{"Text::Levenshtein@0.2.4"}
	doc, d := plan.Build(parse.Declaration{Kind: parse.KindDependencies}, mustPkgs(t, lang.Raku, specs...), host, plan.Policy{
		CacheRoot: "/cache/devkit-deps", Script: "/work/a.raku", CallerCwd: "/caller",
	})
	if d != nil {
		t.Fatal(d)
	}
	e := envDir(host, "/cache/devkit-deps", specs, "", "", nil)
	want := []string{"/selected/raku", "/tools/zef", "--install-to=inst#" + filepath.Join(e, "raku"), "--contained", "install", "Text::Levenshtein:ver<0.2.4>"}
	if len(doc.Setup) != 1 || !reflect.DeepEqual(doc.Setup[0].Argv, want) {
		t.Fatalf("%#v", doc.Setup[0].Argv)
	}
	if !reflect.DeepEqual(doc.Execution.Argv, []string{"/selected/raku", "-Iinst#" + filepath.Join(e, "raku"), "/work/a.raku"}) {
		t.Fatalf("%#v", doc.Execution.Argv)
	}
	if doc.Execution.Environment["RAKULIB"] != "inst#"+filepath.Join(e, "raku") || doc.Execution.Environment["DEVKIT_DEPS_DIR"] != e {
		t.Fatalf("%#v", doc.Execution.Environment)
	}
}
