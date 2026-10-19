# devkit-deps

A comment names the packages. `devkit-deps run` installs them and runs the script.

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

`lua5.4` means Lua 5.4. Each package is `name@version`. A normal interpreter ignores the comment.

`samples/` has one short script per language, including Perl, PHP, Go, PowerShell, Node, and R. The build needs Go 1.22 or newer. The language runtime, and its package manager, must already be installed.

## Runtime version

`#!/usr/bin/env` takes one program name and nothing else. `lua5.4` is the language `lua` plus the version `5.4`. A direct path uses the file name the same way: `#!/usr/bin/lua5.4`. Flags and extra words are rejected, including `#!/usr/bin/env -S lua5.4`.

| Program name | Requested version |
|---|---|
| `lua5.4`, `perl5.40`, `php8.3`, `go1.22.5`, `node20.11.1` | The dotted version after the language name |
| `lua54`, `php83` | `5.4` and `8.3`. Lua and PHP only |
| `node20` | Node 20. Node is the only language where a bare major is part of the program name |
| `lua5` | Not read. A bare major is not part of the program name |

The number is a prefix of the version the program prints. `5.4` matches Lua 5.4.2. It does not match 5.5.1.

A single number is only the major version, the same idea as `python3`. Put it on the command line. It wins over the first line. `node20` is the one program name that already means a major version: Node 20. `--runtime-version 20` matches Node 20.11.1.

```sh
./devkit-deps run --runtime-version 5 samples/hello.lua
```

`--runtime-version 5` matches Lua 5.4.2 and does not match Lua 4.

Raku, Janet, PowerShell, Red, R, Forth, Factor, and NASM have no version in the program name, so use that flag. Odin prints a date such as `dev-2025-01`, which is not a dotted version, so `--runtime-version` does not select it. Bash and zsh report a version, so the flag works for them. On Debian and Ubuntu, `sh` is dash. `sh --version` exits with `Illegal option`, and a version pin for that shell is rejected.

Go understands `go1.22`, but `go build` cannot compile a `#!` line, so pin Go on the command line:

```sh
./devkit-deps run --runtime-version 1.22 samples/hello.go
```

Tcl and Mojo take their runtime from Pixi. A host version in those scripts is not used.

## Packages

Most items are `name@version`. Janet uses `git::https://host/repo.git::revision`. Red, Forth, Factor, and NASM use `https://host/repo.git#` and a 40-character commit. Odin uses `name=https://host/repo.git#` and that same commit. The name is the package directory. A Mojo PyPI package starts with `pypi:`.

| Languages | Comment mark |
|---|---|
| Perl, Raku, Janet, PowerShell, Tcl, Mojo, shell, R | `#` |
| PHP, Go, Node, Odin | `//` |
| Lua | `--` |
| Forth | `\` |
| Factor | `!` |
| Red, NASM | `;` |

[samples/go.mod](samples/go.mod) keeps `hello.go` out of this module. `devkit-deps run` still builds that file in its own environment. The tests read the files in `samples/`.

## Tools

`run` does not install language runtimes or operating-system packages. `./devkit-deps install` does that when Homebrew can, and otherwise prints the command.

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
| node | `node`, `npm` |
| r | `Rscript`, and the `pak` package already installed for that R |
| odin | `odin`, plus `git` when the script declares dependencies |
| forth | `gforth`, plus `git` when the script declares dependencies |
| factor | `factor`, plus `git` when the script declares dependencies |
| nasm | `nasm` and `cc`, plus `git` when the script declares dependencies |

Node installs with `npm` and exposes packages through `NODE_PATH`, so scripts use `require()`. R installs with `pak::pkg_install`. `./devkit-deps install --lang r` installs `pak` when `Rscript` is already on PATH. `run` does not. Tcl and Mojo use the Pixi-provided interpreters, not a host `tclsh` or `mojo`. Pixi environments are prepared for macOS and Linux on arm64 and amd64.

Odin checks each dependency out under `collections/<name>` and builds with `odin build -file -collection:deps=<that directory>`. Import it as `deps:<name>`. Forth writes a prelude that runs `s" <dir>" fpath also-path`. It does not replace `GFORTHPATH`. Arguments after the script are Gforth OS arguments, read with `next-arg`. End the script with `bye`. The Gforth sample starts with `#! /usr/bin/env gforth`, including the space, because that is the form Gforth ignores. Factor passes each checkout to `-roots` and starts with `-no-user-init`. Put the `!` declaration above `USING:`. NASM is x86-64 only. `.asm` means NASM, not the GNU assembler. macOS assembles `macho64` and links with `cc -arch x86_64`, so Apple silicon needs Rosetta. `linux/amd64` assembles `elf64`. Other systems are rejected.

## Commands

```text
devkit-deps run [flags] <script> [-- <script args...>]
devkit-deps prepare [flags] <script>
devkit-deps plan [flags] <script>
devkit-deps doctor [flags]
devkit-deps install [--lang <id>]
```

`run` prepares when needed, then runs the script. `prepare` stops after the environment is ready. `plan` prints the plan and does not install or run anything. `doctor` reports whether each runtime is usable. `doctor` with no `--lang` exits 0 even when some tools are missing. `doctor --lang` exits 3 when that language's runtime or manager is missing or incompatible. Both print how to install a missing tool.

`./devkit-deps install` installs missing tools with Homebrew when `brew` is on PATH and a formula, or the Factor cask, exists. Otherwise it prints the command or the download page. `./devkit-deps install --lang forth` limits that to one language. A failed `run` prints the same direction and does not start the installer. Factor's cask is an Intel build in `/Applications/factor`, so add that directory to `PATH`. Apple silicon also needs Rosetta.

| Flag | Meaning |
|---|---|
| `--lang` | Language, when the file does not say |
| `--runtime` | Absolute path to the interpreter |
| `--runtime-version` | Requested version. Wins over a versioned program name such as `lua5.4`. A single number is the major version: `5` matches `5.4.2` |
| `--shell` | `bash`, `zsh`, or `sh`. Shell scripts only |
| `--manager` | Package manager. Rejected for Go, PowerShell, and R, where one binary is both runtime and manager |
| `--cache-dir` | Cache root |
| `--rebuild` | Recreate the cached environment. Cannot be combined with `--no-prepare` |
| `--no-prepare` | Require an environment that is already ready |
| `--setup-timeout` | Setup deadline. Default 600 seconds |
| `--timeout` | Script deadline. Omitted means no deadline. `0` is rejected |
| `--json` | Machine-readable output for `plan` and `prepare` |

The tool looks for a matching binary already on the machine. `run`, `plan`, `prepare`, and `doctor` do not install one, and they do not search for a newer install. `--runtime` and `--runtime-version` are rejected for Tcl and Mojo, because those runtimes come from Pixi. `--runtime` and `--manager` must be absolute paths.

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

## Other languages

These already have a package tool. This wrapper does not implement them.

| Language | Declaration |
|---|---|
| C# | [`#:package`](https://learn.microsoft.com/en-us/dotnet/core/sdk/file-based-apps#package) in a file-based app |
| Java | [JBang `//DEPS`](https://www.jbang.dev/documentation/jbang/latest/dependencies.html) |
| F# | [`#r "nuget:"`](https://learn.microsoft.com/en-us/dotnet/fsharp/tools/fsharp-interactive/#referencing-packages-in-f-interactive) in an `.fsx` script |
| Clojure | [Babashka `:deps`](https://book.babashka.org/#_paths_and_deps) in `bb.edn` |
| Kotlin | [kscript `@file:DependsOn`](https://github.com/kscripting/kscript#declare-dependencies-with-filedependson) |
| Groovy | [Grape `@Grab`](https://docs.groovy-lang.org/latest/html/documentation/grape.html#_add_a_dependency) |
| Elixir | [`Mix.install/2`](https://hexdocs.pm/mix/Mix.html#install/2) |
| Scala | [Scala CLI `//> using dep`](https://scala-cli.virtuslab.org/docs/guides/introduction/using-directives) |
| Rust | [Cargo single-file packages](https://doc.rust-lang.org/cargo/reference/unstable.html#script) |
| Julia | [`Pkg.activate(; temp=true)`](https://pkgdocs.julialang.org/dev/api/) and `Pkg.add` |
| D | [DUB single-file `dub.sdl`](https://dub.pm/dub-guide/single) |
| Swift | [`Package.swift`](https://docs.swift.org/latest/documentation/packagemanagerdocs/addingdependencies) |
| Objective-C | Swift Package Manager, or [CocoaPods](https://guides.cocoapods.org/using/using-cocoapods.html) |
| C, C++ | [Conan](https://docs.conan.io/), [vcpkg](https://vcpkg.io/), [build2](https://build2.org/), [clib](https://github.com/clibs/clib). There is no single script syntax |
| Nim | [Nimble](https://nim-lang.github.io/nimble/), [Atlas](https://nim-lang.github.io/Nim/atlas.html) |
| Zig | [`build.zig.zon`](https://ziglang.org/learn/build-system/) |
| Erlang | [rebar3 dependencies](https://rebar3.org/docs/configuration/dependencies/) |
| Gleam | [`gleam.toml`](https://gleam.run/writing-gleam/gleam-toml/) |
| Free Pascal | [`fppkg`](https://wiki.freepascal.org/fppkg) |
| Lazarus | [Online Package Manager](https://wiki.lazarus.freepascal.org/Online_Package_Manager) |

The upstream commit and license are in [NOTICE](NOTICE). The full contract is in [SPEC.md](SPEC.md).

## Contributing

Questions and bug reports belong in [issues](https://github.com/michalbcz/devkit-deps/issues). Pull requests are welcome.

## License

Apache-2.0. See [LICENSE.txt](LICENSE.txt).
