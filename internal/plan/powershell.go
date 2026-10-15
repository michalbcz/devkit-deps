package plan

import (
	"path/filepath"
	"strings"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.PowerShell] = planPowerShell }

func planPowerShell(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	modules := filepath.Join(envDir, "modules")
	var prepare strings.Builder
	prepare.WriteString("$ErrorActionPreference = 'Stop'\n")
	prepare.WriteString("if (-not (Get-Command Save-PSResource -ErrorAction SilentlyContinue)) { throw 'PSResourceGet is required. Install it using the official manager instructions.' }\n")
	prepare.WriteString("New-Item -ItemType Directory -Path " + psQuote(modules) + " -Force | Out-Null\n")
	for _, pkg := range pkgs {
		prepare.WriteString("Save-PSResource -Name " + psQuote(pkg.Name) + " -Version " + psQuote(pkg.Version) + " -Repository PSGallery -Path " + psQuote(modules) + " -TrustRepository -Confirm:$false -ErrorAction Stop\n")
	}
	var run strings.Builder
	run.WriteString("param([Parameter(ValueFromRemainingArguments = $true)][string[]]$ScriptArgs)\n")
	run.WriteString("$ErrorActionPreference = 'Stop'\n")
	run.WriteString("$env:PSModulePath = " + psQuote(modules) + " + [IO.Path]::PathSeparator + (Join-Path $PSHOME 'Modules')\n")
	run.WriteString("& " + psQuote(host.Runtime.Path) + " -NoProfile -NonInteractive -File " + psQuote(policy.Script) + " @ScriptArgs\n")
	run.WriteString("exit $LASTEXITCODE\n")
	doc.Files = []FileDoc{
		{Path: "prepare.ps1", Body: prepare.String()},
		{Path: "run.ps1", Body: run.String()},
	}
	doc.Setup = []StepDoc{step(
		[]string{host.Runtime.Path, "-NoProfile", "-NonInteractive", "-File", filepath.Join(envDir, "prepare.ps1")},
		envDir, nil, nil,
	)}
	doc.Execution.Argv = append([]string{host.Runtime.Path, "-NoProfile", "-NonInteractive", "-File", filepath.Join(envDir, "run.ps1")}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, nil)
	return doc, nil
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
