package plan

import (
	"path/filepath"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Perl] = planPerl }

func planPerl(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	script := host.Manager.Path
	if script == "" {
		script = "cpanm"
	} else if resolved, err := discover.ResolveSibling(script, []string{"cpanm", "cpanm.pl"}); err != nil {
		return doc, err.(*diag.Diagnostic)
	} else {
		script = resolved
	}
	var helpers []string
	if script != host.Manager.Path && host.Manager.Path != "" {
		helpers = []string{script}
	}
	envDir := place(host, policy, specsOf(pkgs), "", "", helpers)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	lib := filepath.Join(envDir, "perl", "lib", "perl5")
	setupEnv := map[string]string{
		"PATH":            pathPrepend(host.Runtime.Path),
		"PERL_CPANM_HOME": filepath.Join(envDir, "cpanm-work"),
		"PERL_MM_OPT":     "",
		"PERL_MB_OPT":     "",
	}
	argv := []string{
		host.Runtime.Path, script, "--local-lib-contained", filepath.Join(envDir, "perl"),
		"--no-interactive", "--no-prompt", "--no-sudo", "--no-lwp",
		"--mirror", "https://cpan.metacpan.org", "--mirror-only",
	}
	for _, pkg := range pkgs {
		argv = append(argv, pkg.Spec)
	}
	doc.Setup = []StepDoc{step(argv, envDir, setupEnv, nil)}
	doc.RequiredFiles = []string{"perl/lib/perl5"}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "-I" + lib, policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, map[string]string{"PERL5LIB": lib})
	return doc, nil
}
