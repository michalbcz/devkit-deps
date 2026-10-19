package lang

import (
	"path"
	"regexp"
	"strings"

	"devkit-deps/internal/diag"
)

type ID string

const (
	Perl       ID = "perl"
	PHP        ID = "php"
	Raku       ID = "raku"
	Janet      ID = "janet"
	Lua        ID = "lua"
	GoLang     ID = "go"
	PowerShell ID = "powershell"
	Tcl        ID = "tcl"
	Mojo       ID = "mojo"
	Shell      ID = "shell"
	Red        ID = "red"
	Node       ID = "node"
	R          ID = "r"
)

type Dialect string

const (
	Bash Dialect = "bash"
	Zsh  Dialect = "zsh"
	Sh   Dialect = "sh"
)

type Source struct {
	Path     string
	Explicit ID
	Shell    Dialect
}

var (
	dottedVersion = regexp.MustCompile(`^\d+(?:\.\d+)+$`)
	compactTwo    = regexp.MustCompile(`^\d{2}$`)
	allDigits     = regexp.MustCompile(`^\d+$`)
)

var extensions = map[string]ID{
	".pl": Perl, ".php": PHP, ".raku": Raku, ".rakumod": Raku,
	".janet": Janet, ".lua": Lua, ".go": GoLang, ".ps1": PowerShell,
	".tcl": Tcl, ".mojo": Mojo, ".sh": Shell, ".bash": Shell, ".zsh": Shell,
	".red": Red,
	".js":  Node, ".mjs": Node, ".cjs": Node, ".r": R,
}

var plainInterpreters = map[string]ID{
	"perl": Perl, "php": PHP, "raku": Raku, "rakudo": Raku,
	"janet": Janet, "lua": Lua, "go": GoLang,
	"pwsh": PowerShell, "powershell": PowerShell,
	"tclsh": Tcl, "mojo": Mojo,
	"bash": Shell, "zsh": Shell, "sh": Shell,
	"red":  Red,
	"node": Node, "nodejs": Node,
	"Rscript": R,
}

func CommentPrefix(id ID) (string, error) {
	switch id {
	case PHP, GoLang, Node:
		return "//", nil
	case Lua:
		return "--", nil
	case Red:
		return ";", nil
	case Perl, Raku, Janet, PowerShell, Tcl, Mojo, Shell, R:
		return "#", nil
	default:
		return "", &diag.Diagnostic{Code: "language.unknown", Message: "unknown language " + string(id), Phase: diag.PhaseCLI, Exit: 2}
	}
}

func Infer(filePath, header string, explicit ID, shellFlag Dialect) (ID, Dialect, string, *diag.Diagnostic) {
	base, shebangID, shebangVersion, shebangErr := parseShebang(header)
	if shebangErr != nil && explicit == "" {
		return "", "", "", shebangErr
	}
	ext := strings.ToLower(path.Ext(filePath))
	extID := extensions[ext]

	var id ID
	switch {
	case explicit != "":
		id = explicit
	case extID != "":
		id = extID
	case shebangID != "":
		id = shebangID
	default:
		return "", "", "", &diag.Diagnostic{
			Code: "language.unknown", Message: "unknown language; pass --lang or a recognized shebang",
			Phase: diag.PhaseCLI, Exit: 2,
		}
	}

	version := ""
	if shebangVersion != "" && (explicit == "" || explicit == shebangID) {
		version = shebangVersion
	}
	_ = base

	if shellFlag != "" && id != Shell {
		return "", "", "", &diag.Diagnostic{
			Code: "cli.shell", Message: "--shell is valid only for shell",
			Language: string(id), Phase: diag.PhaseCLI, Exit: 2,
		}
	}

	var dialect Dialect
	if id == Shell {
		switch {
		case shellFlag != "":
			dialect = shellFlag
		case shebangID == Shell && (base == "bash" || base == "zsh" || base == "sh"):
			dialect = Dialect(base)
		case ext == ".bash":
			dialect = Bash
		case ext == ".zsh":
			dialect = Zsh
		case ext == ".sh":
			dialect = Sh
		default:
			dialect = Bash
		}
	}
	return id, dialect, version, nil
}

func parseShebang(header string) (base string, id ID, version string, d *diag.Diagnostic) {
	line := header
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	if !strings.HasPrefix(line, "#!") {
		return "", "", "", nil
	}
	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	if len(fields) == 0 {
		return "", "", "", &diag.Diagnostic{Code: "language.shebang", Message: "unrecognized shebang", Phase: diag.PhaseCLI, Exit: 2}
	}
	interp := fields[0]
	if path.Base(interp) == "env" {
		if len(fields) != 2 || fields[1] == "-S" || strings.HasPrefix(fields[1], "-") {
			return "", "", "", &diag.Diagnostic{Code: "language.shebang", Message: "env shebangs with options are not used for language inference", Phase: diag.PhaseCLI, Exit: 2}
		}
		interp = fields[1]
	}
	base = path.Base(interp)
	if id, ok := plainInterpreters[base]; ok {
		return base, id, "", nil
	}
	if id, version, ok := splitVersioned(base); ok {
		return base, id, version, nil
	}
	return base, "", "", &diag.Diagnostic{Code: "language.shebang", Message: "unrecognized interpreter " + base, Phase: diag.PhaseCLI, Exit: 2}
}

func splitVersioned(base string) (ID, string, bool) {
	for _, item := range []struct {
		name string
		id   ID
	}{
		{"perl", Perl},
		{"php", PHP},
		{"lua", Lua},
		{"go", GoLang},
		{"node", Node},
	} {
		if !strings.HasPrefix(base, item.name) || len(base) == len(item.name) {
			continue
		}
		rest := base[len(item.name):]
		if dottedVersion.MatchString(rest) {
			return item.id, rest, true
		}
		if (item.name == "lua" || item.name == "php") && compactTwo.MatchString(rest) {
			return item.id, rest[:1] + "." + rest[1:], true
		}
		// node20 is Node 20. lua5 and php8 stay unrecognized.
		if item.name == "node" && allDigits.MatchString(rest) {
			return item.id, rest, true
		}
	}
	return "", "", false
}
