package plan

import (
	"path/filepath"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Odin] = planOdin }

func planOdin(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	gitBin := gitBinary(&doc, host)
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	root := filepath.Join(envDir, "collections")
	for _, pkg := range pkgs {
		target := filepath.Join(root, pkg.Identity)
		appendGitCheckout(&doc, gitBin, envDir, target, pkg.Name, pkg.Version)
	}
	doc.RequiredFiles = []string{filepath.Join("collections", pkgs[0].Identity)}
	return odinBuild(doc, host, policy, root, managed(envDir, nil)), nil
}

func odinBuild(doc Document, host discover.Host, policy Policy, collection string, env map[string]string) Document {
	argv := []string{host.Runtime.Path, "build", policy.Script, "-file"}
	if collection != "" {
		argv = append(argv, "-collection:deps="+collection)
	}
	argv = append(argv, "-out:"+perRunBinary)
	if env == nil {
		env = map[string]string{}
	}
	doc.Execution.PreRun = []StepDoc{step(argv, filepath.Dir(policy.Script), env, nil)}
	doc.Execution.Argv = append([]string{perRunBinary}, policy.ScriptArgs...)
	doc.Execution.Cwd = policy.CallerCwd
	doc.Execution.Environment = env
	return doc
}
