# devkit-deps

`devkit-deps` is this wrapper's comment convention, not a language standard. An ordinary interpreter ignores the comment. Only this wrapper prepares the dependencies and runs the script.

## A script

The comment lists packages. Most items are `name@version`. Janet uses `git::https://host/repo.git::revision`. Red uses `https://host/repo.git#` and a 40-character commit. A Mojo PyPI package starts with `pypi:`. A versioned program name on the first line asks for that runtime. `lua5.4` means Lua 5.4.

```lua
#!/usr/bin/env lua5.4
-- devkit-deps: ["lustache@1.3.1-0"]

local lustache = require("lustache")
print(lustache:render("Hello, {{name}}!", { name = "world" }))
```

```sh
go build -o devkit-deps ./cmd/devkit-deps
./devkit-deps run samples/hello.lua
```

`5.4` matches Lua 5.4.2. It does not match 5.5. Lua must already be installed.

The same first-line form works for Perl (`perl5.40`) and PHP (`php8.3`). `--runtime-version 5.4` asks for the same thing and wins over the first line. Raku, Janet, PowerShell, and Red have no version in the program name, so use that flag. Bash and zsh report a version, so the flag works for them. Debian and Ubuntu `sh` is dash. `sh --version` exits with `Illegal option`, and a version pin for that shell is rejected. Go understands `go1.22`, but `go build` cannot compile a `#!` line, so pin Go on the command line:

```sh
./devkit-deps run --runtime-version 1.22 samples/hello.go
```

Tcl and Mojo take their runtime from Pixi. A host version in those scripts is not used.

Comment marks: `#` for Perl, Raku, Janet, PowerShell, Tcl, Mojo, and shell. `//` for PHP and Go. `--` for Lua. `;` for Red.

`samples/` has one short script per language. The tests read those files. `samples/go.mod` keeps `hello.go` out of this module. `devkit-deps run` still builds that file in its own environment.

## Commands

```text
devkit-deps run [flags] <script> [-- <script args...>]
devkit-deps prepare [flags] <script>
devkit-deps plan [flags] <script>
devkit-deps doctor [flags]
```

`run` prepares when needed, then runs the script. `prepare` stops after the environment is ready. `plan` prints the plan and does not install or run anything. `doctor` reports whether each runtime is usable. `doctor` with no `--lang` exits 0 even when some tools are missing. `doctor --lang` exits 3 when that language's runtime or manager is missing or incompatible.

Flags: `--lang`, `--runtime`, `--runtime-version`, `--shell`, `--manager`, `--cache-dir`, `--rebuild`, `--no-prepare`, `--setup-timeout`, `--timeout`, `--json`.

`--runtime` is an absolute path to the interpreter. `--runtime-version` asks for that version, and it wins over a versioned shebang name such as `lua5.4`. The tool looks for a matching binary already on the machine. It does not install one, and it does not search for a newer install. `--runtime` and `--runtime-version` are rejected for `tcl` and `mojo`, because those runtimes come from Pixi. `--manager` is rejected for `go` and `powershell`, because that binary is both the runtime and the manager. `--shell` is `bash`, `zsh`, or `sh`, and only for shell scripts.

`--json` is valid for `plan` and `prepare`. `--setup-timeout` defaults to 600 seconds. An omitted `--timeout` means the script has no deadline. `--timeout 0` is rejected. `--rebuild` cannot be combined with `--no-prepare`.

Exit codes: `0` when the wrapper succeeds, or the script's own code after it starts. `2` for usage and declaration errors. `3` when a runtime, manager, or host tool is missing or incompatible. `4` when setup, build, cache, or launch fails before the script starts. `124` when the wrapper's script deadline fires. `130` for SIGINT. `143` for SIGTERM.

## Prerequisites

The language runtime and its package manager must already be installed. This tool does not install them, and it does not install operating-system packages.

| Language | Tools |
|---|---|
| perl | `perl`, `cpanm` |
| php | `php`, `composer` |
| raku | `raku`, `zef` |
| janet | `janet`, `jpm`, `git` |
| lua | `lua`, `luarocks` |
| go | `go` |
| powershell | `pwsh` |
| tcl, mojo, shell | `pixi` |
| red | `red`, plus `git` when the script declares dependencies |

Tcl and Mojo use the Pixi-provided interpreters, not a host `tclsh` or `mojo`. Pixi environments are prepared for macOS and Linux on arm64 and amd64.

## Cache

The cache root is `--cache-dir`, otherwise `DEVKIT_DEPS_CACHE_DIR`, otherwise `~/Library/Caches/devkit-deps` on macOS, `$XDG_CACHE_HOME/devkit-deps` or `~/.cache/devkit-deps` on Linux, and `%LOCALAPPDATA%/devkit-deps/cache` on Windows.

A manager lockfile kept beside an environment is not a universal lock, and `--no-prepare` is not an offline guarantee.

## Attribution

This program reimplements the managed script-dependency behavior of Majkl DevKit. See `NOTICE` for the upstream commit and license.
