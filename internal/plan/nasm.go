package plan

import (
	"path/filepath"
	"strconv"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.NASM] = planNASM }

func planNASM(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	return assembleNASM(doc, pkgs, host, policy)
}

func assembleNASM(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	format, linkExtra, d := nasmTarget(host.Platform)
	if d != nil {
		d.Language = string(host.Language)
		doc.Execution.Argv = []string{}
		doc.Execution.PreRun = []StepDoc{}
		return doc, d
	}
	cc, missing := lookupCC(policy)
	if missing {
		if !containsTool(doc.MissingTools, "cc") {
			doc.MissingTools = append(doc.MissingTools, "cc")
		}
		cc = "cc"
	}
	env := map[string]string{}
	var includes []string
	if len(pkgs) > 0 {
		gitBin := gitBinary(&doc, host)
		envDir := place(host, policy, specsOf(pkgs), "", "", nil)
		doc.EnvironmentDirectory = envDir
		doc.Readiness = "missing"
		for i, pkg := range pkgs {
			target := filepath.Join(envDir, "sources", strconv.Itoa(i))
			includes = append(includes, target)
			appendGitCheckout(&doc, gitBin, envDir, target, pkg.Name, pkg.Version)
		}
		doc.RequiredFiles = []string{"sources/0"}
		env = managed(envDir, nil)
	} else {
		doc.MissingTools = removeTool(doc.MissingTools, "git")
	}
	nasmArgs := []string{host.Runtime.Path, "-f", format}
	for _, dir := range includes {
		nasmArgs = append(nasmArgs, "-i"+dir)
	}
	nasmArgs = append(nasmArgs, "-o", perRunBinary+".o", policy.Script)
	ccArgs := append([]string{cc}, linkExtra...)
	ccArgs = append(ccArgs, "-o", perRunBinary, perRunBinary+".o")
	cwd := filepath.Dir(policy.Script)
	doc.Execution.PreRun = []StepDoc{
		step(nasmArgs, cwd, env, nil),
		step(ccArgs, cwd, env, nil),
	}
	doc.Execution.Argv = append([]string{perRunBinary}, policy.ScriptArgs...)
	doc.Execution.Cwd = policy.CallerCwd
	doc.Execution.Environment = env
	return doc, nil
}

func nasmTarget(platform cache.Platform) (string, []string, *diag.Diagnostic) {
	switch platform.GOOS {
	case "darwin":
		return "macho64", []string{"-arch", "x86_64"}, nil
	case "linux":
		if platform.GOARCH == "amd64" {
			return "elf64", nil, nil
		}
	}
	return "", nil, &diag.Diagnostic{
		Code:    "platform.unsupported",
		Message: "NASM supports macho64 on darwin and elf64 on linux/amd64",
		Phase:   diag.PhasePlan, Exit: 3,
	}
}
