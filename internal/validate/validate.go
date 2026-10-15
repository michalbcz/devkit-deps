package validate

import (
	"regexp"
	"strings"
	"unicode"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/lang"
)

type Package struct {
	Spec     string
	Name     string
	Version  string
	Identity string
}

var (
	versionRE  = regexp.MustCompile(`^[A-Za-z0-9.*+^~<>=,!:_-]+$`)
	perlName   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_:]*$`)
	phpName    = regexp.MustCompile(`^[a-z0-9_.-]+/[a-z0-9_.-]+$`)
	simpleName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	goName     = regexp.MustCompile(`^[A-Za-z0-9_./~-]+$`)
	janetSpec  = regexp.MustCompile(`^git::https://\S+::[A-Za-z0-9_.-]+$`)
	redSpec    = regexp.MustCompile(`^https://\S+#[a-fA-F0-9]{40}$`)
	tcllibVer  = regexp.MustCompile(`^[0-9]+(\.[0-9]+)+$`)
)

func Validate(id lang.ID, specs []string) ([]Package, *diag.Diagnostic) {
	out := make([]Package, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		pkg, d := one(id, spec)
		if d != nil {
			return nil, d
		}
		if _, ok := seen[pkg.Identity]; ok {
			return nil, fail(id, "package.conflict", "conflicting specifications for "+pkg.Identity)
		}
		seen[pkg.Identity] = struct{}{}
		out = append(out, pkg)
	}
	return out, nil
}

func one(id lang.ID, spec string) (Package, *diag.Diagnostic) {
	switch id {
	case lang.Janet:
		if !janetSpec.MatchString(spec) {
			return Package{}, fail(id, "package.spec", "use jpm git::https://repository.git::revision")
		}
		rest := strings.TrimPrefix(spec, "git::")
		repo := rest[:strings.LastIndex(rest, "::")]
		rev := rest[strings.LastIndex(rest, "::")+2:]
		return Package{Spec: spec, Name: repo, Version: rev, Identity: repo}, nil
	case lang.Red:
		if !redSpec.MatchString(spec) {
			return Package{}, fail(id, "package.spec", "Red source dependencies use https://repository.git#full-40-character-commit")
		}
		url := spec[:strings.LastIndex(spec, "#")]
		rev := spec[strings.LastIndex(spec, "#")+1:]
		return Package{Spec: spec, Name: url, Version: rev, Identity: url}, nil
	case lang.Tcl, lang.Mojo, lang.Shell:
		return pixi(id, spec)
	default:
		name, version, d := split(id, spec)
		if d != nil {
			return Package{}, d
		}
		if d := checkName(id, name); d != nil {
			return Package{}, d
		}
		return Package{Spec: spec, Name: name, Version: version, Identity: name}, nil
	}
}

func pixi(id lang.ID, spec string) (Package, *diag.Diagnostic) {
	kind := "conda"
	body := spec
	for _, prefix := range []string{"conda:", "pypi:"} {
		if strings.HasPrefix(body, prefix) {
			if kind != "conda" || body != spec {
				return Package{}, fail(id, "package.prefix", "at most one conda: or pypi: prefix is accepted")
			}
			kind = strings.TrimSuffix(prefix, ":")
			body = strings.TrimPrefix(spec, prefix)
			if strings.HasPrefix(body, "conda:") || strings.HasPrefix(body, "pypi:") {
				return Package{}, fail(id, "package.prefix", "at most one conda: or pypi: prefix is accepted")
			}
			break
		}
	}
	if kind == "pypi" && id != lang.Mojo {
		return Package{}, fail(id, "package.prefix", "pypi: is supported only for mojo")
	}
	name, version, d := split(id, body)
	if d != nil {
		return Package{}, d
	}
	if !simpleName.MatchString(name) {
		return Package{}, fail(id, "package.spec", "use a Pixi package-name@version")
	}
	if id == lang.Tcl && name == "tcllib" && !tcllibVer.MatchString(version) {
		return Package{}, fail(id, "package.spec", "Tcllib source releases require an exact version such as tcllib@2.0")
	}
	return Package{Spec: spec, Name: name, Version: version, Identity: kind + ":" + name}, nil
}

func split(id lang.ID, spec string) (string, string, *diag.Diagnostic) {
	at := strings.LastIndex(spec, "@")
	if at <= 0 || at == len(spec)-1 {
		return "", "", fail(id, "package.spec", "use package@version")
	}
	name, version := spec[:at], spec[at+1:]
	if !validName(name) || strings.HasPrefix(version, "-") || !versionRE.MatchString(version) {
		return "", "", fail(id, "package.spec", "invalid package specification "+spec)
	}
	return name, version, nil
}

func checkName(id lang.ID, name string) *diag.Diagnostic {
	switch id {
	case lang.Perl, lang.Raku:
		if !perlName.MatchString(name) {
			return fail(id, "package.spec", "use a module name@version")
		}
	case lang.PHP:
		if !phpName.MatchString(name) {
			return fail(id, "package.spec", "use Composer vendor/package@version")
		}
	case lang.Lua, lang.PowerShell:
		if !simpleName.MatchString(name) {
			return fail(id, "package.spec", "invalid package name")
		}
	case lang.GoLang:
		if !strings.Contains(name, "/") || !goName.MatchString(name) {
			return fail(id, "package.spec", "use a Go module/path@version")
		}
	default:
		return fail(id, "package.spec", "unsupported language")
	}
	return nil
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	first := []rune(name)[0]
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func fail(id lang.ID, code, message string) *diag.Diagnostic {
	return &diag.Diagnostic{Code: code, Message: message, Language: string(id), Phase: diag.PhaseValidate, Exit: 2}
}
