package plan

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/validate"
)

type Document struct {
	SchemaVersion        int        `json:"schemaVersion"`
	Language             string     `json:"language"`
	Dialect              string     `json:"dialect,omitempty"`
	Declaration          string     `json:"declaration"`
	Dependencies         []string   `json:"dependencies"`
	EnvironmentDirectory string     `json:"environmentDirectory,omitempty"`
	Readiness            string     `json:"readiness"`
	Runtime              RuntimeDoc `json:"runtime"`
	MissingTools         []string   `json:"missingTools"`
	Files                []FileDoc  `json:"files"`
	Setup                []StepDoc  `json:"setup"`
	Execution            ExecDoc    `json:"execution"`
	RequiredFiles        []string   `json:"-"`
}

type RuntimeDoc struct {
	Path             string `json:"path"`
	Origin           string `json:"origin"`
	Version          string `json:"version"`
	RequestedVersion string `json:"requestedVersion,omitempty"`
	Status           string `json:"status"`
}

type FileDoc struct {
	Path string `json:"path"`
	Body string `json:"body"`
}

type StepDoc struct {
	Argv               []string          `json:"argv"`
	Cwd                string            `json:"cwd"`
	Environment        map[string]string `json:"environment"`
	CaptureEnvironment []string          `json:"captureEnvironment"`
}

type ExecDoc struct {
	PreRun              []StepDoc         `json:"preRun"`
	Argv                []string          `json:"argv"`
	Cwd                 string            `json:"cwd"`
	Environment         map[string]string `json:"environment"`
	DeferredEnvironment []string          `json:"deferredEnvironment"`
}

type Policy struct {
	CacheRoot  string
	Script     string
	ScriptArgs []string
	CallerCwd  string
	Look       discover.Looker
}

type planner func(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic)

var planners = map[lang.ID]planner{}

func Build(decl parse.Declaration, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	doc := skeleton(decl, pkgs, host, policy)
	if host.Runtime.Status == "missing" || host.Runtime.Status == "incompatible" || (host.Runtime.Path == "" && host.Origin != "pixi" && host.Language != lang.Tcl && host.Language != lang.Mojo) {
		doc.EnvironmentDirectory = ""
		doc.Files = []FileDoc{}
		doc.Setup = []StepDoc{}
		doc.Execution.Argv = []string{}
		doc.Execution.PreRun = []StepDoc{}
		return normalize(doc), missingRuntime(host)
	}
	if len(pkgs) == 0 && host.Language != lang.Tcl && host.Language != lang.Mojo {
		doc, d := noDeps(doc, host, policy)
		if d != nil {
			return normalize(doc), d
		}
		if host.Language == lang.NASM && containsTool(doc.MissingTools, "cc") {
			doc.Setup = []StepDoc{}
			doc.Execution.PreRun = []StepDoc{}
			return normalize(doc), &diag.Diagnostic{
				Code: "tool.missing", Message: "missing " + strings.Join(doc.MissingTools, ", "),
				Language: string(host.Language), Phase: diag.PhaseDiscover,
				MissingTools: append([]string{}, doc.MissingTools...), Exit: 3,
			}
		}
		return normalize(doc), nil
	}
	fn := planners[host.Language]
	if fn == nil {
		return normalize(doc), &diag.Diagnostic{
			Code: "plan.adapter", Message: "no planner for " + string(host.Language),
			Language: string(host.Language), Phase: diag.PhasePlan, Exit: 4,
		}
	}
	planned, d := fn(doc, pkgs, host, policy)
	if d != nil {
		return normalize(planned), d
	}
	if len(planned.MissingTools) > 0 {
		planned.Setup = []StepDoc{}
		planned.Execution.PreRun = []StepDoc{}
		return normalize(planned), &diag.Diagnostic{
			Code: "tool.missing", Message: "missing " + strings.Join(planned.MissingTools, ", "),
			Language: string(host.Language), Phase: diag.PhaseDiscover,
			MissingTools: append([]string{}, planned.MissingTools...), Exit: 3,
		}
	}
	return normalize(planned), nil
}

func Encode(doc Document) ([]byte, error) {
	body, err := json.MarshalIndent(normalize(doc), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func skeleton(decl parse.Declaration, pkgs []validate.Package, host discover.Host, policy Policy) Document {
	declaration, readiness := "specs", "missing"
	if len(pkgs) == 0 {
		if decl.Kind == parse.KindAbsent {
			declaration, readiness = "absent", "absent"
		} else {
			declaration, readiness = "empty", "empty"
		}
	}
	deps := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		deps = append(deps, pkg.Spec)
	}
	status := host.Runtime.Status
	if status == "" {
		status = "ok"
	}
	origin := host.Origin
	if origin == "" {
		origin = "path"
	}
	args := append([]string{}, policy.ScriptArgs...)
	return Document{
		SchemaVersion: 1,
		Language:      string(host.Language),
		Dialect:       string(host.Dialect),
		Declaration:   declaration,
		Dependencies:  deps,
		Readiness:     readiness,
		Runtime: RuntimeDoc{
			Path:             host.Runtime.Path,
			Origin:           origin,
			Version:          host.Runtime.Version,
			RequestedVersion: host.Runtime.RequestedVersion,
			Status:           status,
		},
		MissingTools: append([]string{}, host.MissingTools...),
		Files:        []FileDoc{},
		Setup:        []StepDoc{},
		Execution: ExecDoc{
			PreRun:              []StepDoc{},
			Argv:                append([]string{host.Runtime.Path, policy.Script}, args...),
			Cwd:                 policy.CallerCwd,
			Environment:         map[string]string{},
			DeferredEnvironment: []string{},
		},
	}
}

func noDeps(doc Document, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	switch host.Language {
	case lang.GoLang:
		env := map[string]string{
			"GOWORK": "off", "GOTOOLCHAIN": "local", "GO111MODULE": "on", "GOFLAGS": "",
		}
		doc.Execution.PreRun = []StepDoc{{
			Argv:               []string{host.Runtime.Path, "build", "-o", "<per-run-binary>", policy.Script},
			Cwd:                filepath.Dir(policy.Script),
			Environment:        env,
			CaptureEnvironment: []string{},
		}}
		doc.Execution.Argv = append([]string{"<per-run-binary>"}, policy.ScriptArgs...)
		doc.Execution.Environment = map[string]string{}
	case lang.Red:
		doc.Files = []FileDoc{{Path: "<per-run-shim>", Body: redShim(policy.Script)}}
		doc.Execution.Argv = append([]string{host.Runtime.Path, "<per-run-shim>"}, policy.ScriptArgs...)
		doc.Execution.Environment = map[string]string{}
	case lang.R:
		doc.Execution.Argv = append([]string{host.Runtime.Path, "--vanilla", policy.Script}, policy.ScriptArgs...)
		doc.Execution.Environment = map[string]string{}
	case lang.Odin:
		doc = odinBuild(doc, host, policy, "", map[string]string{})
	case lang.Forth:
		doc.Execution.Argv = append([]string{host.Runtime.Path, "--no-rc", policy.Script}, policy.ScriptArgs...)
		doc.Execution.Environment = map[string]string{"GFORTH_ENV": "off"}
	case lang.Factor:
		doc.Execution.Argv = append([]string{host.Runtime.Path, "-no-user-init", policy.Script}, policy.ScriptArgs...)
		doc.Execution.Environment = map[string]string{}
	case lang.NASM:
		return assembleNASM(doc, nil, host, policy)
	default:
		doc.Execution.Environment = map[string]string{}
	}
	return doc, nil
}

func redShim(script string) string {
	return "Red [Title: \"devkit-deps runner\"]\n" +
		"if error? set/any 'result try/all/keep [do %\"" + redEscape(script) + "\"] [\n" +
		"    print :result\n" +
		"    quit/return 1\n" +
		"]\n" +
		"quit/return 0\n"
}

func redEscape(path string) string {
	var b strings.Builder
	for _, r := range path {
		if r == '^' || r == '"' {
			b.WriteRune('^')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func missingRuntime(host discover.Host) *diag.Diagnostic {
	name := host.Runtime.Name
	if name == "" {
		name = string(host.Language)
	}
	code := "tool.missing"
	if host.Runtime.Status == "incompatible" {
		if host.Language == lang.Lua {
			code = "runtime.abi"
		} else {
			code = "runtime.version"
		}
	}
	tools := append([]string{}, host.MissingTools...)
	if len(tools) == 0 {
		tools = []string{name}
	}
	return &diag.Diagnostic{
		Code: code, Message: "missing " + strings.Join(tools, ", "),
		Language: string(host.Language), Phase: diag.PhaseDiscover,
		MissingTools: tools, Exit: 3,
	}
}

func place(host discover.Host, policy Policy, specs []string, profile, pixiTarget string, helpers []string) string {
	mod := int64(0)
	if !host.Manager.ModTime.IsZero() {
		mod = host.Manager.ModTime.Unix()
	}
	if helpers == nil {
		helpers = []string{}
	}
	id := cache.Identity{
		Language:       string(host.Language),
		Dialect:        string(host.Dialect),
		GOOS:           host.Platform.GOOS,
		GOARCH:         host.Platform.GOARCH,
		PixiTarget:     pixiTarget,
		Profile:        profile,
		RuntimePath:    host.Runtime.Path,
		RuntimeVersion: host.Runtime.Version,
		ManagerPath:    host.Manager.Path,
		ManagerModUnix: mod,
		Helpers:        helpers,
		Specs:          specs,
	}
	return cache.EnvDir(policy.CacheRoot, string(host.Language), cache.Hash(id))
}

func specsOf(pkgs []validate.Package) []string {
	out := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		out[i] = pkg.Spec
	}
	return out
}

func step(argv []string, cwd string, env map[string]string, capture []string) StepDoc {
	if env == nil {
		env = map[string]string{}
	}
	if capture == nil {
		capture = []string{}
	}
	return StepDoc{Argv: append([]string{}, argv...), Cwd: cwd, Environment: env, CaptureEnvironment: capture}
}

func pathPrepend(runtime string) string {
	return filepath.Dir(runtime) + string(filepath.ListSeparator)
}

func mergeEnv(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func managed(envDir string, extra map[string]string) map[string]string {
	out := map[string]string{"DEVKIT_DEPS_DIR": envDir}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func toolPath(host discover.Host) string {
	if host.Manager.Path != "" {
		return host.Manager.Path
	}
	if host.Manager.Name != "" {
		return host.Manager.Name
	}
	return host.Runtime.Path
}

func normalize(doc Document) Document {
	if doc.SchemaVersion == 0 {
		doc.SchemaVersion = 1
	}
	if doc.Dependencies == nil {
		doc.Dependencies = []string{}
	}
	if doc.MissingTools == nil {
		doc.MissingTools = []string{}
	}
	if doc.Files == nil {
		doc.Files = []FileDoc{}
	}
	if doc.Setup == nil {
		doc.Setup = []StepDoc{}
	}
	if doc.RequiredFiles == nil {
		doc.RequiredFiles = []string{}
	}
	doc.Execution.PreRun = normalizeSteps(doc.Execution.PreRun)
	if doc.Execution.Argv == nil {
		doc.Execution.Argv = []string{}
	}
	if doc.Execution.Environment == nil {
		doc.Execution.Environment = map[string]string{}
	}
	if doc.Execution.DeferredEnvironment == nil {
		doc.Execution.DeferredEnvironment = []string{}
	}
	doc.Setup = normalizeSteps(doc.Setup)
	if doc.Runtime.Status == "" {
		doc.Runtime.Status = "ok"
	}
	if doc.Runtime.Origin == "" {
		doc.Runtime.Origin = "path"
	}
	return doc
}

func normalizeSteps(steps []StepDoc) []StepDoc {
	if steps == nil {
		return []StepDoc{}
	}
	for i := range steps {
		if steps[i].Argv == nil {
			steps[i].Argv = []string{}
		}
		if steps[i].Environment == nil {
			steps[i].Environment = map[string]string{}
		}
		if steps[i].CaptureEnvironment == nil {
			steps[i].CaptureEnvironment = []string{}
		}
	}
	return steps
}
