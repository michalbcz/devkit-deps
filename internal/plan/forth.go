package plan

import (
	"path/filepath"
	"strconv"
	"strings"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Forth] = planForth }

func planForth(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	gitBin := gitBinary(&doc, host)
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	dirs := make([]string, 0, len(pkgs))
	for i, pkg := range pkgs {
		target := filepath.Join(envDir, "sources", strconv.Itoa(i))
		dirs = append(dirs, target)
		appendGitCheckout(&doc, gitBin, envDir, target, pkg.Name, pkg.Version)
	}
	body, d := forthPrelude(dirs)
	if d != nil {
		d.Language = string(host.Language)
		return doc, d
	}
	doc.Files = []FileDoc{{Path: "prelude.fs", Body: body}}
	doc.RequiredFiles = []string{"sources/0", "prelude.fs"}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "--no-rc", filepath.Join(envDir, "prelude.fs"), policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, map[string]string{"GFORTH_ENV": "off"})
	return doc, nil
}

func forthPrelude(dirs []string) (string, *diag.Diagnostic) {
	var b strings.Builder
	for _, dir := range dirs {
		if strings.ContainsAny(dir, "\"\r\n") {
			return "", &diag.Diagnostic{
				Code: "plan.path", Message: "a Forth search path cannot contain a quote or a newline",
				Phase: diag.PhasePlan, Exit: 4,
			}
		}
		b.WriteString("s\" ")
		b.WriteString(dir)
		b.WriteString("\" fpath also-path\n")
	}
	return b.String(), nil
}
