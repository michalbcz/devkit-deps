package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"strings"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/install"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/parse"
	"devkit-deps/internal/plan"
	"devkit-deps/internal/proc"
	"devkit-deps/internal/validate"
)

type Request struct {
	Command           string
	Script            string
	ScriptArgs        []string
	Lang              string
	Runtime           string
	RuntimeVersion    string
	RuntimeVersionSet bool
	Shell             string
	Manager           string
	CacheDir          string
	Rebuild           bool
	NoPrepare         bool
	SetupTimeout      time.Duration
	ScriptTimeout     time.Duration
	JSON              bool
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Look              discover.Looker
	Runner            install.Runner
}

func Execute(ctx context.Context, req Request) int {
	if req.Stdout == nil {
		req.Stdout = os.Stdout
	}
	if req.Stderr == nil {
		req.Stderr = os.Stderr
	}
	if req.SetupTimeout == 0 {
		req.SetupTimeout = 600 * time.Second
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	box := &proc.SignalBox{}
	ctx = proc.WithSignalBox(ctx, box)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		select {
		case got := <-sigs:
			if sig, ok := got.(syscall.Signal); ok {
				box.Store(sig)
			}
			cancel()
		case <-ctx.Done():
		}
	}()

	if req.Command == "doctor" {
		return doctor(req)
	}
	if req.Command == "install" {
		return installTools(ctx, req)
	}
	if req.Script == "" {
		return report(req, &diag.Diagnostic{Code: "cli.script", Message: "a script path is required", Phase: diag.PhaseCLI, Exit: 2})
	}
	data, err := os.ReadFile(req.Script)
	if err != nil {
		return report(req, &diag.Diagnostic{Code: "cli.script", Message: err.Error(), Phase: diag.PhaseCLI, Exit: 2})
	}
	abs, err := filepath.Abs(req.Script)
	if err != nil {
		return report(req, &diag.Diagnostic{Code: "cli.script", Message: err.Error(), Phase: diag.PhaseCLI, Exit: 2})
	}
	cwd, err := os.Getwd()
	if err != nil {
		return report(req, &diag.Diagnostic{Code: "cli.cwd", Message: err.Error(), Phase: diag.PhaseCLI, Exit: 2})
	}
	id, dialect, shebangVersion, d := lang.Infer(abs, string(data), lang.ID(req.Lang), lang.Dialect(req.Shell))
	if d != nil {
		return report(req, d)
	}
	decl, d := parse.Parse(string(data), id)
	if d != nil {
		return report(req, d)
	}
	var pkgs []validate.Package
	if len(decl.Specs) > 0 {
		pkgs, d = validate.Validate(id, decl.Specs)
		if d != nil {
			return report(req, d)
		}
	}
	version := shebangVersion
	if req.RuntimeVersionSet {
		version = req.RuntimeVersion
	}
	platform := cache.Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	host, d := discover.Discover(id, dialect, req.Runtime, version, req.Manager, platform, req.Look)
	if d != nil {
		return report(req, d)
	}
	root, err := cacheRoot(req.CacheDir)
	if err != nil {
		return report(req, &diag.Diagnostic{Code: "cache.root", Message: err.Error(), Phase: diag.PhasePlan, Exit: 4})
	}
	doc, d := plan.Build(decl, pkgs, host, plan.Policy{
		CacheRoot: root, Script: abs, ScriptArgs: req.ScriptArgs, CallerCwd: cwd, Look: req.Look,
	})
	if req.Command == "plan" {
		if d == nil || d.Exit == 3 {
			body, err := plan.Encode(doc)
			if err != nil {
				return report(req, &diag.Diagnostic{Code: "plan.encode", Message: err.Error(), Phase: diag.PhasePlan, Exit: 4})
			}
			_, _ = req.Stdout.Write(body)
		}
		if d != nil {
			fmt.Fprintln(req.Stderr, d.Message)
			if req.JSON && d.Exit != 3 {
				writeDiagnostic(req.Stdout, d)
			}
			return d.Exit
		}
		return 0
	}
	if d != nil {
		return report(req, d)
	}
	prepared, code, d := RunPrepared(ctx, doc, Options{
		Rebuild: req.Rebuild, NoPrepare: req.NoPrepare,
		SetupTimeout: req.SetupTimeout, ScriptTimeout: req.ScriptTimeout,
		Stdin: req.Stdin, Stdout: req.Stdout, Stderr: req.Stderr,
		SkipLaunch: req.Command == "prepare",
	})
	if d != nil {
		return report(req, d)
	}
	if req.Command == "prepare" && req.JSON {
		body, err := plan.Encode(prepared)
		if err != nil {
			return report(req, &diag.Diagnostic{Code: "plan.encode", Message: err.Error(), Phase: diag.PhasePrepare, Exit: 4})
		}
		_, _ = req.Stdout.Write(body)
	}
	return code
}

func languageIDs() []lang.ID {
	return []lang.ID{lang.Perl, lang.PHP, lang.Raku, lang.Janet, lang.Lua, lang.GoLang, lang.PowerShell, lang.Tcl, lang.Mojo, lang.Shell, lang.Red, lang.Node, lang.R, lang.Odin, lang.Forth, lang.Factor, lang.NASM}
}

func doctor(req Request) int {
	ids := languageIDs()
	if req.Lang != "" {
		ids = []lang.ID{lang.ID(req.Lang)}
	}
	platform := cache.Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	brew := install.BrewOnPath(req.Look)
	for _, id := range ids {
		dialect := lang.Dialect("")
		if id == lang.Shell {
			dialect = lang.Bash
			if req.Shell != "" {
				dialect = lang.Dialect(req.Shell)
			}
		}
		runtimeFlag, version, manager := "", "", ""
		if req.Lang != "" {
			runtimeFlag, manager = req.Runtime, req.Manager
			if req.RuntimeVersionSet {
				version = req.RuntimeVersion
			}
		}
		if id == lang.Tcl || id == lang.Mojo || id == lang.Shell {
			if _, d := cache.PixiTarget(platform); d != nil {
				fmt.Fprintf(req.Stdout, "%s\tunsupported\t%s\n", id, d.Message)
				fmt.Fprintln(req.Stderr, d.Message)
				if req.Lang != "" {
					return 3
				}
				continue
			}
		}
		host, d := discover.Discover(id, dialect, runtimeFlag, version, manager, platform, req.Look)
		if d != nil {
			fmt.Fprintf(req.Stdout, "%s\t%s\t%s\n", id, d.Code, d.Message)
			fmt.Fprintln(req.Stderr, d.Message)
			if req.Lang != "" {
				return d.Exit
			}
			continue
		}
		if len(host.MissingTools) > 0 || host.Runtime.Status == "missing" || host.Runtime.Status == "incompatible" {
			name := host.Runtime.Name
			tools := append([]string{}, host.MissingTools...)
			if len(tools) == 0 {
				if name == "" {
					name = string(id)
				}
				tools = []string{name}
			}
			name = tools[0]
			hint := install.Direct(name, install.Options{Brew: brew, GOOS: runtime.GOOS}).FirstLine()
			fmt.Fprintf(req.Stdout, "%s\tmissing\t%s\t%s\n", id, name, hint)
			if req.Lang != "" {
				fmt.Fprintln(req.Stderr, install.Missing(tools, install.Options{Brew: brew, GOOS: runtime.GOOS}))
				return 3
			}
			continue
		}
		fmt.Fprintf(req.Stdout, "%s\tok\t%s\t%s\n", id, host.Runtime.Path, host.Runtime.Version)
	}
	return 0
}

func installTools(ctx context.Context, req Request) int {
	ids := languageIDs()
	if req.Lang != "" {
		ids = []lang.ID{lang.ID(req.Lang)}
	}
	platform := cache.Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	brew := install.BrewOnPath(req.Look)
	runner := req.Runner
	if runner == nil {
		runner = install.ExecRunner{}
	}
	opt := install.Options{Brew: brew, Auto: true, GOOS: runtime.GOOS}
	failed := false
	seen := map[string]bool{}
	for _, id := range ids {
		dialect := lang.Dialect("")
		if id == lang.Shell {
			dialect = lang.Bash
		}
		if id == lang.Tcl || id == lang.Mojo || id == lang.Shell {
			if _, d := cache.PixiTarget(platform); d != nil {
				fmt.Fprintf(req.Stdout, "%s\tunsupported\t%s\n", id, d.Message)
				fmt.Fprintln(req.Stderr, d.Message)
				failed = true
				continue
			}
		}
		host, d := discover.Discover(id, dialect, "", "", "", platform, req.Look)
		if d != nil {
			fmt.Fprintf(req.Stdout, "%s\t%s\t%s\n", id, d.Code, d.Message)
			fmt.Fprintln(req.Stderr, d.Message)
			failed = true
			continue
		}
		tools := append([]string{}, host.MissingTools...)
		rscript := ""
		if id == lang.R {
			if host.Runtime.Status == "ok" && host.Runtime.Path != "" {
				rscript = host.Runtime.Path
				if err := runner.Run(ctx, install.PakProbeArgv(rscript), io.Discard, io.Discard); err != nil {
					tools = append(tools, "pak")
				}
			} else {
				tools = append(tools, "pak")
			}
		}
		if len(tools) == 0 {
			fmt.Fprintf(req.Stdout, "%s\tok\t%s\t%s\n", id, host.Runtime.Path, host.Runtime.Version)
			continue
		}
		local := opt
		local.Rscript = rscript
		for _, tool := range tools {
			step := install.Direct(tool, local)
			fmt.Fprintf(req.Stdout, "%s\tmissing\t%s\n%s\n", id, tool, step.Text)
			if len(step.Argv) == 0 {
				failed = true
				continue
			}
			key := strings.Join(step.Argv, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			fmt.Fprintf(req.Stdout, "$ %s\n", strings.Join(step.Argv, " "))
			if err := runner.Run(ctx, step.Argv, req.Stdout, req.Stderr); err != nil {
				fmt.Fprintf(req.Stderr, "%s: %v\n", tool, err)
				failed = true
			}
		}
	}
	if failed {
		return 3
	}
	return 0
}

func cacheRoot(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := cache.EnvRoot(); env != "" {
		return env, nil
	}
	return cache.PlatformRoot()
}

func report(req Request, d *diag.Diagnostic) int {
	if d == nil {
		return 0
	}
	fmt.Fprintln(req.Stderr, d.Message)
	if req.JSON && req.Command != "run" && req.Command != "doctor" {
		writeDiagnostic(req.Stdout, d)
	}
	if d.Exit == 0 {
		return 4
	}
	return d.Exit
}

func writeDiagnostic(w io.Writer, d *diag.Diagnostic) {
	tools := d.MissingTools
	if tools == nil {
		tools = []string{}
	}
	payload := struct {
		Code         string   `json:"code"`
		Message      string   `json:"message"`
		Language     string   `json:"language"`
		Phase        string   `json:"phase"`
		MissingTools []string `json:"missingTools"`
		Line         int      `json:"line"`
	}{d.Code, d.Message, d.Language, string(d.Phase), tools, d.Line}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
