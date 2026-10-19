package plan

import (
	"path/filepath"
	"strings"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.R] = planR }

func planR(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	lib := filepath.Join(envDir, "library")
	refs := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		refs[i] = rQuote(pkg.Name + "@" + pkg.Version)
	}
	expr := "lib <- " + rQuote(lib) +
		"; dir.create(lib, recursive = TRUE, showWarnings = FALSE); pak::pkg_install(c(" +
		strings.Join(refs, ", ") + "), lib = lib, ask = FALSE, upgrade = FALSE)"
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	doc.RequiredFiles = []string{"library"}
	doc.Setup = []StepDoc{step(
		[]string{host.Runtime.Path, "--vanilla", "-e", expr},
		envDir, nil, nil,
	)}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "--vanilla", policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, map[string]string{"R_LIBS": lib})
	return doc, nil
}

func rQuote(value string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range value {
		if r == '\\' || r == '\'' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}
