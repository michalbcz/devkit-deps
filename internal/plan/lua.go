package plan

import (
	"path/filepath"
	"regexp"
	"strings"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/discover"
	"devkit-deps/internal/lang"
	"devkit-deps/internal/validate"
)

func init() { planners[lang.Lua] = planLua }

var luaABI = regexp.MustCompile(`5\.\d+`)

func planLua(doc Document, pkgs []validate.Package, host discover.Host, policy Policy) (Document, *diag.Diagnostic) {
	abi := luaABI.FindString(host.Runtime.Version)
	if abi == "" {
		return doc, &diag.Diagnostic{
			Code: "runtime.abi", Message: "Lua runtime version must contain a 5.x ABI",
			Language: "lua", Phase: diag.PhaseDiscover, Exit: 3,
		}
	}
	envDir := place(host, policy, specsOf(pkgs), "", "", nil)
	doc.EnvironmentDirectory = envDir
	doc.Readiness = "missing"
	manager := host.Manager.Path
	if manager == "" {
		manager = "luarocks"
	}
	common := []string{manager, "--lua-version=" + abi, "--lua-dir=" + luaPrefix(host.Runtime.Path), "--tree=" + filepath.Join(envDir, "lua")}
	setupEnv := map[string]string{"PATH": pathPrepend(host.Runtime.Path)}
	doc.Setup = make([]StepDoc, 0, len(pkgs)+2)
	for _, pkg := range pkgs {
		argv := append(append([]string{}, common...), "install", pkg.Name, pkg.Version, "--deps-mode=one")
		doc.Setup = append(doc.Setup, step(argv, envDir, setupEnv, nil))
	}
	suffix := strings.ReplaceAll(abi, ".", "_")
	doc.Setup = append(doc.Setup,
		step(append(append([]string{}, common...), "path", "--lr-path"), envDir, nil, []string{"LUA_PATH", "LUA_PATH_" + suffix}),
		step(append(append([]string{}, common...), "path", "--lr-cpath"), envDir, nil, []string{"LUA_CPATH", "LUA_CPATH_" + suffix}),
	)
	doc.Execution.Argv = append([]string{host.Runtime.Path, policy.Script}, policy.ScriptArgs...)
	doc.Execution.Environment = managed(envDir, nil)
	doc.Execution.DeferredEnvironment = []string{"LUA_PATH", "LUA_CPATH", "LUA_PATH_" + suffix, "LUA_CPATH_" + suffix}
	return doc, nil
}

func luaPrefix(bin string) string {
	path := bin
	if real, err := filepath.EvalSymlinks(bin); err == nil {
		path = real
	}
	return filepath.Dir(filepath.Dir(path))
}
