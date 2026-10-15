package plan

import (
	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.GoLang] = planGo }

func planGo(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	doc.Files = []FileDoc{{Path: "go.mod", Body: "module majkl.dev/console\n\ngo 1.18\n"}}
	doc.RequiredFiles = []string{"go.mod", "go.sum"}
	goEnv := managed(envDir, map[string]string{
		"GOWORK": "off", "GOTOOLCHAIN": "local", "GO111MODULE": "on", "GOFLAGS": "",
	})
	argv := []string{host.Runtime.Path, "get"}
	for _, pkg := range pkgs {
		argv = append(argv, pkg.Spec)
	}
	doc.Setup = []StepDoc{step(argv, envDir, goEnv, nil)}
	doc.Execution.PreRun = []StepDoc{step(
		[]string{host.Runtime.Path, "build", "-o", "<per-run-binary>", policy.Script},
		envDir, goEnv, nil,
	)}
	doc.Execution.Argv = append([]string{"<per-run-binary>"}, policy.ScriptArgs...)
	doc.Execution.Cwd = policy.CallerCwd
	doc.Execution.Environment = goEnv
	return doc, nil
}
