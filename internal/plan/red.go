package plan

import (
	"path/filepath"
	"strconv"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Red] = planRed }

func planRed(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	gitBin := host.Manager.Path
	if gitBin == "" {
		gitBin = "git"
		doc.MissingTools = append(doc.MissingTools, "git")
	}
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	env := managed(envDir, map[string]string{"GIT_TERMINAL_PROMPT": "0"})
	for i, pkg := range pkgs {
		target := filepath.Join(envDir, "sources", strconv.Itoa(i))
		doc.Setup = append(doc.Setup,
			step([]string{gitBin, "init", "--quiet", target}, envDir, env, nil),
			step([]string{gitBin, "-C", target, "fetch", "--depth=1", "--no-tags", "--", pkg.Name, pkg.Version}, envDir, env, nil),
			step([]string{gitBin, "-C", target, "checkout", "--quiet", "--detach", "FETCH_HEAD"}, envDir, env, nil),
		)
	}
	doc.RequiredFiles = []string{"sources/0"}
	doc.Files = []FileDoc{{Path: "<per-run-shim>", Body: redShim(policy.Script)}}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "<per-run-shim>"}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, nil)
	return doc, nil
}
