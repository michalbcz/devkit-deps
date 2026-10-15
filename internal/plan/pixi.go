package plan

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() {
	planners[lang.Tcl] = planPixi
	planners[lang.Mojo] = planPixi
	planners[lang.Shell] = planPixi
}

type pixiPair struct{ name, version string }

func planPixi(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	target, derr := cache.PixiTarget(host.Platform)
	if derr != nil {
		return doc, derr
	}
	profile := cache.ProfileShell
	switch host.Language {
	case lang.Tcl:
		profile = cache.ProfileTcl
	case lang.Mojo:
		profile = cache.ProfileMojo
	}
	envDir := place(host, policy, specsOf(pkgs), profile, target, nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	doc.RequiredFiles = []string{"pixi.lock"}

	var conda []pixiPair
	var pypi []pixiPair
	if host.Language == lang.Tcl {
		conda = upsert(conda, "tk", ">=8.6,<8.7")
	}
	if host.Language == lang.Mojo {
		conda = upsert(conda, "mojo", "==1.1.0")
		conda = upsert(conda, "python", ">=3.11,<3.12")
	}
	var tcllibVersion string
	for _, pkg := range pkgs {
		if host.Language == lang.Tcl && pkg.Name == "tcllib" {
			tcllibVersion = pkg.Version
			continue
		}
		version := pkg.Version
		if version != "" && version[0] >= '0' && version[0] <= '9' {
			version = "==" + version
		}
		if strings.HasPrefix(pkg.Identity, "pypi:") {
			pypi = upsert(pypi, pkg.Name, version)
		} else {
			conda = upsert(conda, pkg.Name, version)
		}
	}
	channels := []string{"conda-forge"}
	if host.Language == lang.Mojo {
		channels = []string{"https://conda.modular.com/max/", "conda-forge"}
	}
	doc.Files = []FileDoc{{Path: "pixi.toml", Body: pixiToml(target, channels, conda, pypi)}}

	pixi := host.Manager.Path
	if pixi == "" {
		pixi = "pixi"
	}
	manifest := filepath.Join(envDir, "pixi.toml")
	doc.Setup = []StepDoc{step([]string{pixi, "install", "--manifest-path", manifest}, envDir, nil, nil)}
	execEnv := managed(envDir, nil)
	if tcllibVersion != "" {
		gitBin, missing := lookupGit(policy)
		if missing {
			doc.MissingTools = append(doc.MissingTools, "git")
			gitBin = "git"
		}
		tag := "tcllib-" + strings.ReplaceAll(tcllibVersion, ".", "-")
		targetDir := filepath.Join(envDir, "tcllib")
		doc.Setup = append(doc.Setup, step(
			[]string{gitBin, "clone", "--depth=1", "--branch", tag, "--", "https://github.com/tcltk/tcllib.git", targetDir},
			envDir, map[string]string{"GIT_TERMINAL_PROMPT": "0"}, nil,
		))
		execEnv["TCLLIBPATH"] = tclListElement(filepath.Join(targetDir, "modules"))
	}
	var interp []string
	switch host.Language {
	case lang.Mojo:
		interp = []string{"mojo", "run"}
	case lang.Tcl:
		interp = []string{"tclsh8.6"}
	default:
		interp = []string{host.Runtime.Path}
	}
	argv := []string{pixi, "run", "--manifest-path", manifest, "--frozen", "--"}
	argv = append(argv, interp...)
	argv = append(argv, policy.Script)
	argv = append(argv, policy.ScriptArgs...)
	doc.Execution.Argv = argv
	doc.Execution.Environment = execEnv
	return doc, nil
}

func lookupGit(policy Policy) (string, bool) {
	if policy.Look != nil {
		if path, _, ok := policy.Look.Look("git"); ok {
			return path, false
		}
		return "", true
	}
	if path, err := exec.LookPath("git"); err == nil {
		return path, false
	}
	return "", true
}

func upsert(list []pixiPair, name, version string) []pixiPair {
	for i := range list {
		if list[i].name == name {
			list[i].version = version
			return list
		}
	}
	return append(list, pixiPair{name: name, version: version})
}

func pixiToml(target string, channels []string, conda, pypi []pixiPair) string {
	encoded := jsonRaw(channels)
	var b strings.Builder
	b.WriteString("[workspace]\nname = \"devkit-script\"\n")
	b.WriteString("channels = " + string(encoded) + "\n")
	b.WriteString("platforms = [\"" + target + "\"]\n\n[dependencies]\n")
	for _, item := range conda {
		b.WriteString(jsonQuote(item.name) + " = " + jsonQuote(item.version) + "\n")
	}
	if len(pypi) > 0 {
		b.WriteString("\n[pypi-dependencies]\n")
		for _, item := range pypi {
			b.WriteString(jsonQuote(item.name) + " = " + jsonQuote(item.version) + "\n")
		}
	}
	return b.String()
}

func jsonQuote(s string) string { return jsonRaw(s) }

func jsonRaw(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(buf.String(), "\n")
}

func tclListElement(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) || strings.ContainsRune(`\{}[]$;"`, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
