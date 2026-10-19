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
	gitBin := gitBinary(&doc, host)
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	for i, pkg := range pkgs {
		target := filepath.Join(envDir, "sources", strconv.Itoa(i))
		appendGitCheckout(&doc, gitBin, envDir, target, pkg.Name, pkg.Version)
	}
	doc.RequiredFiles = []string{"sources/0"}
	doc.Files = []FileDoc{{Path: "<per-run-shim>", Body: redShim(policy.Script)}}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "<per-run-shim>"}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, nil)
	return doc, nil
}
