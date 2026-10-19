package discover

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/diag"
	"devkit-deps/internal/lang"
)

type Tool struct {
	Name             string
	Path             string
	ModTime          time.Time
	Version          string
	RequestedVersion string
	Status           string
}

type Host struct {
	Language     lang.ID
	Dialect      lang.Dialect
	Runtime      Tool
	Manager      Tool
	Helpers      []string
	Platform     cache.Platform
	Origin       string
	MissingTools []string
}

type Looker interface {
	Look(name string) (path string, mod time.Time, ok bool)
}

type LookerFunc func(name string) (string, time.Time, bool)

func (f LookerFunc) Look(name string) (string, time.Time, bool) { return f(name) }

var (
	dottedNumber = regexp.MustCompile(`\d+(?:\.\d+)+`)
	luaABI       = regexp.MustCompile(`5\.\d+`)
	factorVM     = regexp.MustCompile(`^\d+\.\d+`)
)

func VersionMatches(probed, requested string) bool {
	if requested == "" {
		return true
	}
	found := dottedNumber.FindString(probed)
	if found == "" {
		return false
	}
	want := strings.Split(requested, ".")
	got := strings.Split(found, ".")
	if len(want) > len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

func Discover(id lang.ID, dialect lang.Dialect, runtimeFlag, runtimeVersion, managerFlag string, platform cache.Platform, look Looker) (Host, *diag.Diagnostic) {
	if look == nil {
		look = defaultLooker{}
	}
	host := Host{Language: id, Dialect: dialect, Platform: platform, Origin: "path", MissingTools: []string{}}
	if id == lang.Tcl || id == lang.Mojo {
		if runtimeFlag != "" {
			return host, usage(id, "cli.runtime", "--runtime is not used for "+string(id))
		}
		if runtimeVersion != "" {
			return host, usage(id, "cli.runtime_version", "--runtime-version is not used for "+string(id))
		}
		host.Origin = "pixi"
		host.Runtime = Tool{Name: string(id), Status: "ok"}
		if path, mod, ok := look.Look("pixi"); ok {
			host.Manager = Tool{Name: "pixi", Path: path, ModTime: mod, Status: "ok"}
		} else {
			host.Manager = Tool{Name: "pixi", Status: "missing"}
			host.MissingTools = append(host.MissingTools, "pixi")
		}
		return host, nil
	}
	if runtimeIsManager(id) && managerFlag != "" {
		return host, usage(id, "cli.manager", "--manager is not used when the runtime is the manager")
	}
	if runtimeFlag != "" && !filepath.IsAbs(runtimeFlag) {
		return host, usage(id, "cli.path", "--runtime must be an absolute path")
	}
	if managerFlag != "" && !filepath.IsAbs(managerFlag) {
		return host, usage(id, "cli.path", "--manager must be an absolute path")
	}

	runtimeName := conventionalRuntime(id, dialect)
	var tool Tool
	var origin string
	var d *diag.Diagnostic
	if runtimeFlag != "" {
		tool, d = inspectPath(id, runtimeFlag, runtimeVersion, runtimeName)
		origin = "flag"
	} else {
		tool, origin, d = searchRuntime(id, dialect, runtimeVersion, runtimeName, look)
	}
	if d != nil {
		return host, d
	}
	tool.RequestedVersion = runtimeVersion
	host.Runtime = tool
	host.Origin = origin
	if tool.Status == "missing" {
		host.MissingTools = append(host.MissingTools, runtimeName)
		host.Origin = "missing"
	}

	if runtimeIsManager(id) {
		host.Manager = tool
		host.Manager.Name = runtimeName
		return host, nil
	}
	managerName := managerBinary(id)
	if managerFlag != "" {
		mgr, err := inspectPath(id, managerFlag, "", managerName)
		if err != nil {
			return host, err
		}
		mgr.Name = managerName
		host.Manager = mgr
		if mgr.Status == "missing" {
			host.MissingTools = append(host.MissingTools, managerFlag)
		}
	} else if path, mod, ok := look.Look(managerName); ok {
		host.Manager = Tool{Name: managerName, Path: path, ModTime: mod, Status: "ok"}
	} else {
		host.Manager = Tool{Name: managerName, Status: "missing"}
		host.MissingTools = append(host.MissingTools, managerName)
	}
	if id == lang.NASM {
		if _, _, ok := look.Look("cc"); !ok {
			host.MissingTools = append(host.MissingTools, "cc")
		}
	}
	if id == lang.Raku && host.Manager.Path != "" {
		script, includes, zerr := ZefLaunch(host.Manager.Path)
		if zerr == nil {
			if script != "" {
				host.Helpers = append(host.Helpers, script)
			}
			host.Helpers = append(host.Helpers, includes...)
		}
	}
	return host, nil
}

func conventionalRuntime(id lang.ID, dialect lang.Dialect) string {
	switch id {
	case lang.Shell:
		if dialect == "" {
			return "bash"
		}
		return string(dialect)
	case lang.PowerShell:
		return "pwsh"
	case lang.R:
		return "Rscript"
	case lang.Forth:
		return "gforth"
	default:
		return string(id)
	}
}

func runtimeIsManager(id lang.ID) bool {
	return id == lang.GoLang || id == lang.PowerShell || id == lang.R
}

func managerBinary(id lang.ID) string {
	switch id {
	case lang.Perl:
		return "cpanm"
	case lang.PHP:
		return "composer"
	case lang.Raku:
		return "zef"
	case lang.Janet:
		return "jpm"
	case lang.Lua:
		return "luarocks"
	case lang.Node:
		return "npm"
	case lang.Shell:
		return "pixi"
	case lang.Red, lang.Odin, lang.Forth, lang.Factor, lang.NASM:
		return "git"
	default:
		return string(id)
	}
}

func versionedNames(id lang.ID, requested string) []string {
	if requested == "" {
		return nil
	}
	switch id {
	case lang.Lua, lang.PHP:
		names := []string{string(id) + requested}
		parts := strings.Split(requested, ".")
		if len(parts) == 2 {
			names = append(names, string(id)+parts[0]+parts[1])
		}
		return names
	case lang.Perl:
		if strings.HasPrefix(requested, "5.") {
			return []string{"perl" + requested}
		}
	case lang.GoLang:
		names := []string{"go" + requested}
		parts := strings.Split(requested, ".")
		if len(parts) > 2 {
			names = append(names, "go"+parts[0]+"."+parts[1])
		}
		return names
	case lang.Node:
		names := []string{"node" + requested}
		if dot := strings.IndexByte(requested, '.'); dot > 0 {
			names = append(names, "node"+requested[:dot])
		}
		return names
	}
	return nil
}

func searchRuntime(id lang.ID, dialect lang.Dialect, requested, conventional string, look Looker) (Tool, string, *diag.Diagnostic) {
	type candidate struct {
		name    string
		version bool
	}
	var names []candidate
	for _, name := range versionedNames(id, requested) {
		names = append(names, candidate{name, true})
	}
	names = append(names, candidate{conventional, false})
	var found []string
	sawAny := false
	for _, cand := range names {
		path, mod, ok := look.Look(cand.name)
		if !ok {
			continue
		}
		sawAny = true
		tool, d := finishProbe(id, path, mod, requested, cand.name)
		if d != nil && d.Code == "runtime.version" {
			found = append(found, d.Message)
			continue
		}
		if d != nil {
			return Tool{}, "", d
		}
		if tool.Status != "ok" {
			continue
		}
		origin := "path"
		if cand.version {
			origin = "version"
		}
		return tool, origin, nil
	}
	if requested != "" && sawAny {
		return Tool{}, "", &diag.Diagnostic{
			Code: "runtime.version", Exit: 3, Phase: diag.PhaseDiscover, Language: string(id),
			Message: fmt.Sprintf("requested %s %s; found %s", conventional, requested, strings.Join(found, "; ")),
		}
	}
	return Tool{Name: conventional, RequestedVersion: requested, Status: "missing"}, "missing", nil
}

func inspectPath(id lang.ID, path, requested, name string) (Tool, *diag.Diagnostic) {
	info, err := os.Stat(path)
	if err != nil {
		return Tool{Name: name, RequestedVersion: requested, Status: "missing"}, nil
	}
	if info.IsDir() {
		return Tool{}, &diag.Diagnostic{Code: "tool.not_executable", Message: path + " is not a file", Language: string(id), Phase: diag.PhaseDiscover, Exit: 3}
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return Tool{}, &diag.Diagnostic{Code: "tool.not_executable", Message: path + " is not executable", Language: string(id), Phase: diag.PhaseDiscover, Exit: 3}
	}
	return finishProbe(id, path, info.ModTime(), requested, name)
}

func finishProbe(id lang.ID, path string, mod time.Time, requested, name string) (Tool, *diag.Diagnostic) {
	line, ok := probe(path, id)
	tool := Tool{Name: name, Path: path, ModTime: mod, Version: line, RequestedVersion: requested, Status: "ok"}
	// GNU coreutils ships a different program named factor. Its -e probe exits 1.
	// A Factor VM exits 0 and prints vm-version, such as 0.99.
	if id == lang.Factor && (!ok || !factorVM.MatchString(line)) {
		return Tool{Name: name, RequestedVersion: requested, Status: "missing"}, nil
	}
	if requested != "" && !VersionMatches(line, requested) {
		return Tool{}, &diag.Diagnostic{
			Code: "runtime.version", Exit: 3, Phase: diag.PhaseDiscover, Language: string(id),
			Message: fmt.Sprintf("%s is %s", path, line),
		}
	}
	if id == lang.Lua && !luaABI.MatchString(line) {
		tool.Status = "incompatible"
		return Tool{}, &diag.Diagnostic{
			Code: "runtime.abi", Message: "Lua runtime must report a 5.x ABI",
			Language: string(id), Phase: diag.PhaseDiscover, Exit: 3,
		}
	}
	if line == "" && requested != "" {
		return Tool{}, &diag.Diagnostic{Code: "runtime.version", Message: "could not read a version from " + path, Language: string(id), Phase: diag.PhaseDiscover, Exit: 3}
	}
	return tool, nil
}

func probe(bin string, id lang.ID) (string, bool) {
	args := []string{"-v"}
	switch id {
	case lang.GoLang, lang.Odin:
		args = []string{"version"}
	case lang.Shell, lang.Node, lang.R, lang.Forth:
		args = []string{"--version"}
	case lang.Factor:
		args = []string{"-e=USING: system ; vm-version print"}
	case lang.PowerShell:
		args = []string{"-NoProfile", "-NonInteractive", "-Command", "$PSVersionTable.PSVersion.ToString()"}
	case lang.Red:
		args = []string{"-v"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = nil
	var out []byte
	var err error
	if id == lang.R {
		// Rscript --version exits 0 and writes the version to stderr.
		out, err = cmd.CombinedOutput()
	} else {
		out, err = cmd.Output()
		if err != nil && len(out) == 0 {
			if combined, cerr := exec.CommandContext(ctx, bin, args...).CombinedOutput(); cerr == nil || len(combined) > 0 {
				out = combined
			}
		}
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line, err == nil
}

func ResolveSibling(toolPath string, names []string) (string, error) {
	lower := strings.ToLower(toolPath)
	if !strings.HasSuffix(lower, ".bat") && !strings.HasSuffix(lower, ".cmd") && !strings.HasSuffix(lower, ".exe") {
		return toolPath, nil
	}
	dir := filepath.Dir(toolPath)
	for _, name := range names {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", &diag.Diagnostic{
		Code: "tool.wrapper", Message: "launcher needs its interpreter script alongside it",
		Phase: diag.PhaseDiscover, Exit: 3,
	}
}

func ZefLaunch(toolPath string) (string, []string, error) {
	resolved := toolPath
	if real, err := filepath.EvalSymlinks(toolPath); err == nil {
		resolved = real
	}
	sibling := filepath.Join(filepath.Dir(resolved), "zef.raku")
	script := resolved
	if info, err := os.Stat(sibling); err == nil && info.Mode().IsRegular() {
		script = sibling
	} else if alt, err := ResolveSibling(resolved, []string{"zef.raku", "zef"}); err == nil {
		script = alt
	}
	repo := filepath.Dir(filepath.Dir(resolved))
	var includes []string
	if info, err := os.Stat(filepath.Join(repo, "dist")); err == nil && info.IsDir() {
		includes = append(includes, "-Iinst#"+repo)
	} else if _, err := os.Stat(filepath.Join(repo, "lib", "Zef", "CLI.rakumod")); err == nil {
		includes = append(includes, "-I"+filepath.Join(repo, "lib"))
	} else if _, err := os.Stat(filepath.Join(repo, "lib", "Zef", "CLI.pm6")); err == nil {
		includes = append(includes, "-I"+filepath.Join(repo, "lib"))
	}
	return script, includes, nil
}

func usage(id lang.ID, code, message string) *diag.Diagnostic {
	return &diag.Diagnostic{Code: code, Message: message, Language: string(id), Phase: diag.PhaseCLI, Exit: 2}
}

// PathLooker searches PATH and the per-user fallbacks. Discover uses it when no looker is supplied.
func PathLooker() Looker { return defaultLooker{} }

type defaultLooker struct{}

func (defaultLooker) Look(name string) (string, time.Time, bool) {
	if path, ok := lookPath(name); ok {
		return statPath(path)
	}
	for _, candidate := range fallbacks(name) {
		if path, mod, ok := statPath(candidate); ok {
			return path, mod, true
		}
	}
	return "", time.Time{}, false
}

func lookPath(name string) (string, bool) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, name)
		if path, _, ok := statPath(candidate); ok {
			return path, true
		}
	}
	return "", false
}

func statPath(path string) (string, time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", time.Time{}, false
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return "", time.Time{}, false
	}
	return path, info.ModTime(), true
}

func fallbacks(name string) []string {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".local", "bin", name),
		filepath.Join(home, ".asdf", "shims", name),
	}
	switch name {
	case "raku":
		paths = append(paths, filepath.Join(home, ".rakubrew", "bin", name))
	case "zef":
		paths = append(paths,
			filepath.Join(home, ".devutils", "script-tools", "zef", "bin", "zef"),
			filepath.Join(home, ".raku", "bin", "zef"),
			filepath.Join(home, ".rakubrew", "bin", "zef"),
		)
		if raku, ok := lookPath("raku"); ok {
			paths = append(paths, filepath.Join(filepath.Dir(filepath.Dir(raku)), "share", "perl6", "site", "bin", "zef"))
		}
		if runtime.GOOS == "windows" {
			var more []string
			for _, item := range paths {
				more = append(more, item+".exe", item+".bat", item+".cmd")
			}
			paths = append(paths, more...)
		}
	case "pixi":
		paths = append(paths, filepath.Join(home, ".pixi", "bin", "pixi"))
	}
	return paths
}
