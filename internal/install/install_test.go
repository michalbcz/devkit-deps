package install_test

import (
	"strings"
	"testing"

	"devkit-deps/internal/install"
)

func TestDirectOdinUsesBrew(t *testing.T) {
	step := install.Direct("odin", install.Options{Brew: true, Auto: true, GOOS: "darwin"})
	if strings.Join(step.Argv, " ") != "brew install odin" {
		t.Fatalf("%#v", step.Argv)
	}
}

func TestDirectOdinWithoutBrewPrintsThePage(t *testing.T) {
	step := install.Direct("odin", install.Options{GOOS: "linux"})
	if step.Argv != nil || !strings.Contains(step.Text, "https://odin-lang.org/docs/install/") {
		t.Fatalf("%#v", step)
	}
}

func TestDirectLinuxGforthIsAptUntilBrewExists(t *testing.T) {
	apt := install.Direct("gforth", install.Options{GOOS: "linux"})
	if apt.Argv != nil || apt.Text != "sudo apt-get install gforth" {
		t.Fatalf("%#v", apt)
	}
	brew := install.Direct("gforth", install.Options{Brew: true, Auto: true, GOOS: "linux"})
	if strings.Join(brew.Argv, " ") != "brew install gforth" {
		t.Fatalf("%#v", brew.Argv)
	}
}

func TestDirectFactorIsACask(t *testing.T) {
	step := install.Direct("factor", install.Options{Brew: true, Auto: true, GOOS: "darwin"})
	if strings.Join(step.Argv, " ") != "brew install --cask factor" {
		t.Fatalf("%#v", step.Argv)
	}
	if !strings.Contains(step.Text, "/Applications/factor") || !strings.Contains(step.Text, "Rosetta") {
		t.Fatalf("%s", step.Text)
	}
}

func TestDirectRedZefAndJpmHaveNoFormula(t *testing.T) {
	cases := map[string]string{
		"red": "https://www.red-lang.org/p/download.html",
		"zef": "https://github.com/ugexe/zef",
		"jpm": "https://github.com/janet-lang/jpm",
	}
	for tool, url := range cases {
		step := install.Direct(tool, install.Options{Brew: true, Auto: true, GOOS: "darwin"})
		if step.Argv != nil || !strings.Contains(step.Text, url) {
			t.Fatalf("%s %#v", tool, step)
		}
	}
}

func TestDirectCC(t *testing.T) {
	darwin := install.Direct("cc", install.Options{Brew: true, Auto: true, GOOS: "darwin"})
	if strings.Join(darwin.Argv, " ") != "xcode-select --install" {
		t.Fatalf("%#v", darwin)
	}
	linux := install.Direct("cc", install.Options{Brew: true, Auto: true, GOOS: "linux"})
	if linux.Argv != nil || linux.Text != "sudo apt-get install build-essential" {
		t.Fatalf("%#v", linux)
	}
}

func TestDirectNpmInstallsNode(t *testing.T) {
	step := install.Direct("npm", install.Options{Brew: true, Auto: true, GOOS: "darwin"})
	if strings.Join(step.Argv, " ") != "brew install node" {
		t.Fatalf("%#v", step.Argv)
	}
}

func TestDirectPakRunsOnlyWhenRscriptExists(t *testing.T) {
	off := install.Direct("pak", install.Options{Auto: true, GOOS: "darwin"})
	if off.Argv != nil || !strings.Contains(off.Text, "install.packages") {
		t.Fatalf("%#v", off)
	}
	on := install.Direct("pak", install.Options{Auto: true, Rscript: "/usr/bin/Rscript", GOOS: "darwin"})
	if len(on.Argv) != 4 || on.Argv[0] != "/usr/bin/Rscript" || on.Argv[2] != "-e" {
		t.Fatalf("%#v", on.Argv)
	}
	if !strings.Contains(on.Argv[3], "r-lib.github.io/p/pak/stable") {
		t.Fatalf("%s", on.Argv[3])
	}
}

func TestHintDoesNotCarryACommand(t *testing.T) {
	step := install.Direct("odin", install.Options{Brew: true, GOOS: "darwin"})
	if step.Argv != nil || step.Text != "brew install odin" {
		t.Fatalf("%#v", step)
	}
}

func TestMissingJoinsDirections(t *testing.T) {
	got := install.Missing([]string{"odin", "git"}, install.Options{Brew: true, GOOS: "darwin"})
	if got != "missing odin, git\nbrew install odin\nbrew install git" {
		t.Fatalf("%q", got)
	}
}
