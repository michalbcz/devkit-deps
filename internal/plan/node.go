package plan

import (
	"encoding/json"
	"path/filepath"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Node] = planNode }

func planNode(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	doc.Files = []FileDoc{{Path: "package.json", Body: nodeManifest(pkgs)}}
	doc.RequiredFiles = []string{"node_modules"}
	npm := host.Manager.Path
	if npm == "" {
		npm = "npm"
	}
	doc.Setup = []StepDoc{step(
		[]string{npm, "install", "--prefix", envDir, "--no-fund", "--no-audit"},
		envDir,
		map[string]string{"PATH": pathPrepend(host.Runtime.Path)},
		nil,
	)}
	doc.Execution.Argv = append([]string{host.Runtime.Path, policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, map[string]string{
		"NODE_PATH": filepath.Join(envDir, "node_modules"),
	})
	return doc, nil
}

func nodeManifest(pkgs []validate.Package) string {
	deps := make(map[string]string, len(pkgs))
	for _, pkg := range pkgs {
		deps[pkg.Name] = pkg.Version
	}
	body, err := json.MarshalIndent(struct {
		Name         string            `json:"name"`
		Private      bool              `json:"private"`
		Dependencies map[string]string `json:"dependencies"`
	}{
		Name:         "devkit-deps-script",
		Private:      true,
		Dependencies: deps,
	}, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(body) + "\n"
}
