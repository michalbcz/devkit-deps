# devkit-deps

Standalone CLI that prepares comment-declared script dependencies and runs the script.

The comment is this tool's convention, not a language standard. A normal interpreter ignores it. This program reads the comment, prepares the packages, and runs the script.

## Table of Contents

- [Background](#background)
- [Install](#install)
- [Usage](#usage)
- [Cache](#cache)
- [Contributing](#contributing)
- [License](#license)

## Background

One comment lists the packages a script needs. The same wrapper prepares them for Perl, PHP, Raku, Janet, Lua, Go, PowerShell, Tcl, Mojo, shell, and Red.

These languages already name dependencies in the script:

| Language | Declaration |
|---|---|
| C# | [`#:package`](https://learn.microsoft.com/en-us/dotnet/core/sdk/file-based-apps#package) in a file-based app |
| Java | [JBang `//DEPS`](https://www.jbang.dev/documentation/jbang/latest/dependencies.html) |
| F# | [`#r "nuget:"`](https://learn.microsoft.com/en-us/dotnet/fsharp/tools/fsharp-interactive/#referencing-packages-in-f-interactive) in an `.fsx` script |
| Clojure | [Babashka `:deps`](https://book.babashka.org/#_paths_and_deps) in `bb.edn` |
| Kotlin | [kscript `@file:DependsOn`](https://github.com/kscripting/kscript#declare-dependencies-with-filedependson) |
| Groovy | [Grape `@Grab`](https://docs.groovy-lang.org/latest/html/documentation/grape.html#_add_a_dependency) |

This program reimplements that behavior from Majkl DevKit. The upstream commit and license are in [NOTICE](NOTICE). The full contract is in [SPEC.md](SPEC.md).

## Install

Build from a clone of this repository. You need Go 1.22 or newer.

```sh
go build -o devkit-deps ./cmd/devkit-deps
```

The built program is `./devkit-deps` in the current directory.

### Dependencies

Each script also needs its language runtime and package manager. This tool does not install them, and it does not install operating-system packages.

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

## Usage

Put the package list in a comment near the top of the script. Most items are `name@version`.

```lua
#!/usr/bin/env lua5.4
-- devkit-deps: ["lustache@1.3.1-0"]

local lustache = require("lustache")
print(lustache:render("Hello, {{name}}!", { name = "world" }))
```

Janet uses `git::https://host/repo.git::revision`. Red uses `https://host/repo.git#` and a 40-character commit. A Mojo PyPI package starts with `pypi:`.

Run that script with the binary from [Install](#install):

```sh
./devkit-deps run samples/hello.lua
```

[samples/](samples/) has one short script per language. The tests read those files. [samples/go.mod](samples/go.mod) keeps `hello.go` out of this module. `devkit-deps run` still builds that file in its own environment.

### Runtime version

The first line can name the runtime version.

`#!/usr/bin/env` takes one program name and nothing else. `lua5.4` is the language `lua` plus the version `5.4`. A direct path uses the file name the same way: `#!/usr/bin/lua5.4`. Flags and extra words are rejected, including `#!/usr/bin/env -S lua5.4`.

| Program name | Requested version |
|---|---|
| `lua5.4`, `perl5.40`, `php8.3`, `go1.22.5` | The dotted version after the language name |
| `lua54`, `php83` | `5.4` and `8.3`. Lua and PHP only |
| `lua5` | Not read. A bare major is not part of the program name |

The number is a prefix of the version the program prints. `5.4` matches Lua 5.4.2. It does not match 5.5.1. The program must already be installed.

A single number is only the major version, the same idea as `python3`. Put that major on the command line. It wins over the first line.

```sh
./devkit-deps run --runtime-version 5 samples/hello.lua
```

`--runtime-version 5` matches Lua 5.4.2 and does not match Lua 4.

Raku, Janet, PowerShell, and Red have no version in the program name, so use that flag. Bash and zsh report a version, so the flag works for them. On Debian and Ubuntu, `sh` is dash. `sh --version` exits with `Illegal option`, and a version pin for that shell is rejected.

Go understands `go1.22`, but `go build` cannot compile a `#!` line, so pin Go on the command line:

```sh
./devkit-deps run --runtime-version 1.22 samples/hello.go
```

Tcl and Mojo take their runtime from Pixi. A host version in those scripts is not used.

### Comment marks

| Languages | Mark |
|---|---|
| Perl, Raku, Janet, PowerShell, Tcl, Mojo, shell | `#` |
| PHP, Go | `//` |
| Lua | `--` |
| Red | `;` |

### CLI

```text
devkit-deps run [flags] <script> [-- <script args...>]
devkit-deps prepare [flags] <script>
devkit-deps plan [flags] <script>
devkit-deps doctor [flags]
```

`run` prepares when needed, then runs the script. `prepare` stops after the environment is ready. `plan` prints the plan and does not install or run anything. `doctor` reports whether each runtime is usable. `doctor` with no `--lang` exits 0 even when some tools are missing. `doctor --lang` exits 3 when that language's runtime or manager is missing or incompatible.

| Flag | Meaning |
|---|---|
| `--lang` | Language, when the file does not say |
| `--runtime` | Absolute path to the interpreter |
| `--runtime-version` | Requested version. Wins over a versioned program name such as `lua5.4`. A single number is the major version: `5` matches `5.4.2` |
| `--shell` | `bash`, `zsh`, or `sh`. Shell scripts only |
| `--manager` | Package manager. Rejected for Go and PowerShell, where one binary is both runtime and manager |
| `--cache-dir` | Cache root |
| `--rebuild` | Recreate the cached environment. Cannot be combined with `--no-prepare` |
| `--no-prepare` | Require an environment that is already ready |
| `--setup-timeout` | Setup deadline. Default 600 seconds |
| `--timeout` | Script deadline. Omitted means no deadline. `0` is rejected |
| `--json` | Machine-readable output for `plan` and `prepare` |

The tool looks for a matching binary already on the machine. It does not install one, and it does not search for a newer install. `--runtime` and `--runtime-version` are rejected for Tcl and Mojo, because those runtimes come from Pixi. `--runtime` and `--manager` must be absolute paths.

| Code | Meaning |
|---|---|
| 0 | The wrapper succeeded, or this is the script's own code after it starts |
| 2 | Usage or declaration error |
| 3 | A runtime, manager, or host tool is missing or incompatible |
| 4 | Setup, build, cache, or launch failed before the script starts |
| 124 | The wrapper's script deadline fired |
| 130 | SIGINT |
| 143 | SIGTERM |

## Cache

The cache root is `--cache-dir`, otherwise `DEVKIT_DEPS_CACHE_DIR`, otherwise `~/Library/Caches/devkit-deps` on macOS, `$XDG_CACHE_HOME/devkit-deps` or `~/.cache/devkit-deps` on Linux, and `%LOCALAPPDATA%/devkit-deps/cache` on Windows.

A manager lockfile kept beside an environment is not a universal lock, and `--no-prepare` is not an offline guarantee.

## Contributing

Questions and bug reports belong in [issues](https://github.com/michalbcz/devkit-deps/issues). Pull requests are welcome.

## License

Apache-2.0. See [LICENSE.txt](LICENSE.txt).
