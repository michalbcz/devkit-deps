package install

import (
	"context"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Looker matches discover.Looker. A nil looker uses the process PATH.
type Looker interface {
	Look(name string) (string, time.Time, bool)
}

type Options struct {
	Brew    bool
	Auto    bool
	Rscript string
	GOOS    string
}

type Step struct {
	Tool string
	Text string
	Argv []string
}

func (s Step) FirstLine() string {
	line, _, _ := strings.Cut(s.Text, "\n")
	return line
}

type Runner interface {
	Run(ctx context.Context, argv []string, stdout, stderr io.Writer) error
}

type RunnerFunc func(ctx context.Context, argv []string, stdout, stderr io.Writer) error

func (f RunnerFunc) Run(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	return f(ctx, argv, stdout, stderr)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return exec.ErrNotFound
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func BrewOnPath(look Looker) bool {
	if look == nil {
		_, err := exec.LookPath("brew")
		return err == nil
	}
	_, _, ok := look.Look("brew")
	return ok
}

func PakProbeArgv(rscript string) []string {
	return []string{rscript, "--vanilla", "-e", `quit(status=if (requireNamespace("pak", quietly=TRUE)) 0 else 1)`}
}

func Missing(tools []string, opt Options) string {
	opt.Auto = false
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if len(tools) == 0 {
		return ""
	}
	parts := []string{"missing " + strings.Join(tools, ", ")}
	seen := map[string]struct{}{}
	for _, tool := range tools {
		text := Direct(tool, opt).Text
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func Direct(tool string, opt Options) Step {
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if strings.ContainsAny(tool, `/\`) {
		return Step{Tool: tool, Text: tool + " does not exist"}
	}
	switch tool {
	case "cc":
		if opt.GOOS == "darwin" {
			step := Step{Tool: tool, Text: "xcode-select --install"}
			if opt.Auto {
				step.Argv = []string{"xcode-select", "--install"}
			}
			return step
		}
		return Step{Tool: tool, Text: "sudo apt-get install build-essential"}
	case "sh":
		return Step{Tool: tool, Text: "A POSIX sh is part of the system. Install one and put sh on PATH."}
	case "pak":
		return pakStep(opt)
	case "factor":
		return factorStep(opt)
	}
	rec, ok := catalog[tool]
	if !ok {
		return Step{Tool: tool, Text: "Install " + tool + " and put it on PATH."}
	}
	return fromRecipe(tool, rec, opt)
}

func pakStep(opt Options) Step {
	expr := `install.packages("pak", repos = sprintf("https://r-lib.github.io/p/pak/stable/%s/%s/%s", .Platform$pkgType, R.Version()$os, R.Version()$arch))`
	bin := "Rscript"
	if opt.Rscript != "" {
		bin = opt.Rscript
	}
	step := Step{Tool: "pak", Text: bin + " --vanilla -e '" + expr + "'"}
	if opt.Auto && opt.Rscript != "" {
		step.Argv = []string{opt.Rscript, "--vanilla", "-e", expr}
	}
	return step
}

func factorStep(opt Options) Step {
	const note = "The Factor cask is an Intel build in /Applications/factor. Add that directory to PATH. On Apple silicon, install Rosetta: softwareupdate --install-rosetta --agree-to-license"
	if opt.Brew || opt.GOOS == "darwin" {
		text := "brew install --cask factor"
		if opt.GOOS == "darwin" {
			text += "\n" + note
		}
		if !opt.Brew {
			text += "\nHomebrew: https://brew.sh"
		}
		step := Step{Tool: "factor", Text: text}
		if opt.Auto && opt.Brew {
			step.Argv = []string{"brew", "install", "--cask", "factor"}
		}
		return step
	}
	return Step{Tool: "factor", Text: "https://factorcode.org/"}
}

type recipe struct {
	brew     []string
	apt      string
	url      string
	note     string
	brewNote string
}

func fromRecipe(tool string, rec recipe, opt Options) Step {
	if opt.Brew && len(rec.brew) > 0 {
		text := strings.Join(rec.brew, " ")
		if rec.brewNote != "" {
			text += "\n" + rec.brewNote
		}
		step := Step{Tool: tool, Text: text}
		if opt.Auto {
			step.Argv = append([]string{}, rec.brew...)
		}
		return step
	}
	if opt.GOOS == "linux" && rec.apt != "" {
		return Step{Tool: tool, Text: "sudo apt-get install " + rec.apt}
	}
	if opt.GOOS == "darwin" && len(rec.brew) > 0 {
		text := strings.Join(rec.brew, " ") + "\nHomebrew: https://brew.sh"
		if rec.brewNote != "" {
			text += "\n" + rec.brewNote
		}
		return Step{Tool: tool, Text: text}
	}
	if rec.url != "" {
		text := rec.url
		if rec.note != "" {
			text += "\n" + rec.note
		}
		return Step{Tool: tool, Text: text}
	}
	if len(rec.brew) > 0 {
		return Step{Tool: tool, Text: strings.Join(rec.brew, " ") + "\nHomebrew: https://brew.sh"}
	}
	return Step{Tool: tool, Text: "Install " + tool + " and put it on PATH."}
}

func brew(formula string) []string {
	return []string{"brew", "install", formula}
}

var catalog = map[string]recipe{
	"perl":     {brew: brew("perl"), apt: "perl"},
	"cpanm":    {brew: brew("cpanminus"), apt: "cpanminus"},
	"php":      {brew: brew("php"), apt: "php-cli"},
	"composer": {brew: brew("composer"), apt: "composer"},
	"raku":     {brew: brew("rakudo"), apt: "rakudo", brewNote: "Rakudo does not always include zef."},
	"zef":      {url: "https://github.com/ugexe/zef", note: "Rakudo does not always include zef."},
	"janet":    {brew: brew("janet")},
	"jpm":      {url: "https://github.com/janet-lang/jpm"},
	"lua":      {brew: brew("lua"), apt: "lua5.4"},
	"luarocks": {brew: brew("luarocks"), apt: "luarocks"},
	"go":       {brew: brew("go"), apt: "golang-go"},
	"pwsh":     {brew: brew("powershell"), url: "https://github.com/PowerShell/PowerShell#get-powershell"},
	"pixi":     {brew: brew("pixi"), url: "https://pixi.sh"},
	"red":      {url: "https://www.red-lang.org/p/download.html"},
	"node":     {brew: brew("node"), apt: "nodejs"},
	"nodejs":   {brew: brew("node"), apt: "nodejs"},
	"npm":      {brew: brew("node"), apt: "npm", brewNote: "Homebrew's node formula provides npm."},
	"Rscript":  {brew: brew("r"), apt: "r-base"},
	"git":      {brew: brew("git"), apt: "git"},
	"odin":     {brew: brew("odin"), url: "https://odin-lang.org/docs/install/"},
	"gforth":   {brew: brew("gforth"), apt: "gforth"},
	"nasm":     {brew: brew("nasm"), apt: "nasm"},
	"bash":     {brew: brew("bash"), apt: "bash"},
	"zsh":      {brew: brew("zsh"), apt: "zsh"},
}
