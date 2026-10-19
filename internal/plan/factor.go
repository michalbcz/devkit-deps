package plan

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Factor] = planFactor }

func planFactor(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	gitBin := gitBinary(&doc, host)
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	sep := string(os.PathListSeparator)
	roots := make([]string, 0, len(pkgs))
	for i, pkg := range pkgs {
		target := filepath.Join(envDir, "sources", strconv.Itoa(i))
		if strings.Contains(target, sep) || strings.ContainsAny(target, "\r\n") {
			return doc, &diag.Diagnostic{
				Code: "plan.path", Message: "a Factor vocabulary root cannot contain the path-list separator or a newline",
				Language: string(host.Language), Phase: diag.PhasePlan, Exit: 4,
			}
		}
		roots = append(roots, target)
		appendGitCheckout(&doc, gitBin, envDir, target, pkg.Name, pkg.Version)
	}
	doc.RequiredFiles = []string{"sources/0"}
	argv := []string{host.Runtime.Path, "-no-user-init", "-roots=" + strings.Join(roots, sep), policy.Script}
	doc.Execution.Argv = append(argv, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, nil)
	return doc, nil
}
