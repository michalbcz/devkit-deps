package plan

import "devkit-deps/internal/discover"

const perRunBinary = "<per-run-binary>"

func gitBinary(doc *Document, host discover.Host) string {
	if host.Manager.Path != "" {
		return host.Manager.Path
	}
	if !containsTool(doc.MissingTools, "git") {
		doc.MissingTools = append(doc.MissingTools, "git")
	}
	return "git"
}

func appendGitCheckout(doc *Document, gitBin, envDir, target, url, rev string) {
	env := managed(envDir, map[string]string{"GIT_TERMINAL_PROMPT": "0"})
	doc.Setup = append(doc.Setup,
		step([]string{gitBin, "init", "--quiet", target}, envDir, env, nil),
		step([]string{gitBin, "-C", target, "fetch", "--depth=1", "--no-tags", "--", url, rev}, envDir, env, nil),
		step([]string{gitBin, "-C", target, "checkout", "--quiet", "--detach", "FETCH_HEAD"}, envDir, env, nil),
	)
}

func containsTool(tools []string, name string) bool {
	for _, tool := range tools {
		if tool == name {
			return true
		}
	}
	return false
}

func removeTool(tools []string, name string) []string {
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool != name {
			out = append(out, tool)
		}
	}
	return out
}

func lookupCC(policy Policy) (string, bool) {
	look := policy.Look
	if look == nil {
		look = discover.PathLooker()
	}
	if path, _, ok := look.Look("cc"); ok && path != "" {
		return path, false
	}
	return "", true
}
