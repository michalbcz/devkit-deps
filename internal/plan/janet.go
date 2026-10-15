package plan

import (
	"path/filepath"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Janet] = planJanet }

func planJanet(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	script := host.Manager.Path
	if script == "" {
		script = "jpm"
	} else if resolved, err := discover.ResolveSibling(script, []string{"jpm", "jpm.janet"}); err != nil {
		d, _ := err.(*diag.Diagnostic)
		return doc, d
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
	tree := filepath.Join(envDir, "janet")
	setupEnv := map[string]string{
		"PATH":       pathPrepend(host.Runtime.Path),
		"JANET_TREE": tree,
	}
	doc.Setup = make([]StepDoc, 0, len(pkgs))
	for _, pkg := range pkgs {
		argv := []string{host.Runtime.Path, script, "--tree=" + tree, "install", pkg.Spec}
		doc.Setup = append(doc.Setup, step(argv, envDir, setupEnv, nil))
	}
	doc.Execution.Argv = append([]string{host.Runtime.Path, policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, map[string]string{"JANET_PATH": filepath.Join(tree, "lib")})
	return doc, nil
}
