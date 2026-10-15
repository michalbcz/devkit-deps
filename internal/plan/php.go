package plan

import (
	"fmt"
	"path/filepath"
	"strings"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.PHP] = planPHP }

func planPHP(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	script := host.Manager.Path
	if script == "" {
		script = "composer"
	} else if resolved, err := discover.ResolveSibling(script, []string{"composer.phar"}); err != nil {
		return doc, err.(*diag.Diagnostic)
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
	doc.Files = []FileDoc{{Path: "php/composer.json", Body: composerJSON(pkgs)}}
	doc.RequiredFiles = []string{"php/vendor/autoload.php"}
	setupEnv := map[string]string{
		"PATH":                pathPrepend(host.Runtime.Path),
		"COMPOSER":            "composer.json",
		"COMPOSER_HOME":       filepath.Join(envDir, "composer-home"),
		"COMPOSER_VENDOR_DIR": filepath.Join(envDir, "php", "vendor"),
	}
	argv := []string{
		host.Runtime.Path, script, "install", "--no-interaction", "--no-ansi", "--no-progress",
		"--no-plugins", "--no-scripts", "--prefer-dist",
	}
	doc.Setup = []StepDoc{step(argv, filepath.Join(envDir, "php"), setupEnv, nil)}
	prepend := "auto_prepend_file=" + filepath.Join(envDir, "php", "vendor", "autoload.php")
	doc.Execution.Argv = append([]string{host.Runtime.Path, "-d", prepend, policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, nil)
	return doc, nil
}

func composerJSON(pkgs []validate.Package) string {
	var b strings.Builder
	b.WriteString("{\n  \"name\": \"majkl-devkit/script-dependencies\",\n  \"require\": {")
	if len(pkgs) == 1 {
		fmt.Fprintf(&b, "%q: %q", pkgs[0].Name, pkgs[0].Version)
		b.WriteString("}\n}\n")
		return b.String()
	}
	for i, pkg := range pkgs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "\n    %q: %q", pkg.Name, pkg.Version)
	}
	b.WriteString("\n  }\n}\n")
	return b.String()
}
