package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/engine"
	"devkit-deps/internal/lang"
)

func Main(args []string) int {
	req, d := Parse(args)
	if d != nil {
		fmt.Fprintln(os.Stderr, d.Message)
		return d.Exit
	}
	req.Stdin = os.Stdin
	req.Stdout = os.Stdout
	req.Stderr = os.Stderr
	return engine.Execute(context.Background(), req)
}

func Parse(args []string) (engine.Request, *diag.Diagnostic) {
	req := engine.Request{SetupTimeout: 600 * time.Second}
	if len(args) == 0 {
		return req, usage("a command is required")
	}
	req.Command = args[0]
	switch req.Command {
	case "run", "prepare", "plan", "doctor", "install":
	default:
		return req, usage("unknown command " + req.Command)
	}
	var positionals []string
	setupTimeoutSet := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if req.Command != "run" {
				return req, usage("-- is only valid for run")
			}
			req.ScriptArgs = append([]string{}, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "--") {
			positionals = append(positionals, arg)
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		needsValue := name != "--rebuild" && name != "--no-prepare" && name != "--json"
		if needsValue && !hasValue {
			i++
			if i >= len(args) {
				return req, usage(name + " needs a value")
			}
			value = args[i]
		}
		switch name {
		case "--lang":
			req.Lang = value
		case "--runtime":
			req.Runtime = value
		case "--runtime-version":
			req.RuntimeVersion = value
			req.RuntimeVersionSet = true
		case "--shell":
			req.Shell = value
		case "--manager":
			req.Manager = value
		case "--cache-dir":
			req.CacheDir = value
		case "--setup-timeout":
			seconds, d := positiveSeconds("--setup-timeout", value)
			if d != nil {
				return req, d
			}
			setupTimeoutSet = true
			req.SetupTimeout = time.Duration(seconds) * time.Second
		case "--timeout":
			seconds, d := positiveSeconds("--timeout", value)
			if d != nil {
				return req, d
			}
			req.ScriptTimeout = time.Duration(seconds) * time.Second
		case "--rebuild":
			req.Rebuild = true
		case "--no-prepare":
			req.NoPrepare = true
		case "--json":
			req.JSON = true
		default:
			return req, usage("unknown flag " + name)
		}
	}
	if req.JSON && (req.Command == "run" || req.Command == "doctor") {
		return req, usage("--json is only valid for plan and prepare")
	}
	if req.Rebuild && req.NoPrepare {
		return req, usage("--rebuild cannot be combined with --no-prepare")
	}
	if req.Lang != "" && !knownLang(req.Lang) {
		return req, usage("unknown language " + req.Lang)
	}
	if req.Shell != "" && req.Shell != "bash" && req.Shell != "zsh" && req.Shell != "sh" {
		return req, usage("unknown shell " + req.Shell)
	}
	if req.Command == "install" {
		if len(positionals) != 0 {
			return req, usage("install does not take a script")
		}
		if req.Runtime != "" || req.RuntimeVersionSet || req.Shell != "" || req.Manager != "" || req.CacheDir != "" || req.Rebuild || req.NoPrepare || req.JSON || req.ScriptTimeout != 0 || setupTimeoutSet {
			return req, usage("install accepts only --lang")
		}
		return req, nil
	}
	if req.Command == "doctor" {
		if len(positionals) != 0 {
			return req, usage("doctor does not take a script")
		}
		return req, nil
	}
	if len(positionals) != 1 {
		return req, usage("a script path is required")
	}
	req.Script = positionals[0]
	return req, nil
}

func knownLang(id string) bool {
	switch lang.ID(id) {
	case lang.Perl, lang.PHP, lang.Raku, lang.Janet, lang.Lua, lang.GoLang, lang.PowerShell, lang.Tcl, lang.Mojo, lang.Shell, lang.Red, lang.Node, lang.R, lang.Odin, lang.Forth, lang.Factor, lang.NASM:
		return true
	default:
		return false
	}
}

func positiveSeconds(flag, value string) (int, *diag.Diagnostic) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, usage(flag + " accepts a positive number of seconds")
	}
	return n, nil
}

func usage(message string) *diag.Diagnostic {
	return &diag.Diagnostic{Code: "cli.usage", Message: message, Phase: diag.PhaseCLI, Exit: 2}
}
