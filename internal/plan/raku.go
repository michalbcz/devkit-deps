package plan

import (
	"path/filepath"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Raku] = planRaku }

func planRaku(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	tool := host.Manager.Path
	if tool == "" {
		tool = "zef"
	}
	script, includes, err := discover.ZefLaunch(tool)
	if err != nil {
		if d, ok := err.(*diag.Diagnostic); ok {
			return doc, d
		}
		return doc, &diag.Diagnostic{Code: "tool.wrapper", Message: err.Error(), Language: "raku", Phase: diag.PhaseDiscover, Exit: 3}
	}
	helpers := append([]string{}, includes...)
	if script != "" && script != tool {
		helpers = append([]string{script}, helpers...)
	}
	envDir := place(host, policy, specsOf(pkgs), "", "", helpers)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	target := "inst#" + filepath.Join(envDir, "raku")
	argv := []string{host.Runtime.Path}
	argv = append(argv, includes...)
	argv = append(argv, script, "--install-to="+target, "--contained", "install")
	for _, pkg := range pkgs {
		argv = append(argv, pkg.Name+":ver<"+pkg.Version+">")
	}
	setupEnv := map[string]string{"PATH": pathPrepend(host.Runtime.Path)}
	doc.Setup = []StepDoc{step(argv, envDir, setupEnv, nil)}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "-I" + target, policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, map[string]string{"RAKULIB": target})
	return doc, nil
}
