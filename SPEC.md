# devkit-deps standalone CLI handoff specification

Status: current mechanism specification and proposed standalone CLI contract  
Date: 2026-10-15  
Baseline: Majkl DevKit commit `2bfc2df7020f5ce041cfbe7cb1d6b65440b22979`  
Provisional executable name: `devkit-deps`

## 1. Purpose and contract boundaries

Extract the dependency mechanism into a standalone command-line wrapper. It reads dependency comments from a local script, prepares packages with the ecosystem's actual tool, connects the installed packages to the appropriate interpreter, and executes the script.

**The comment format is a Majkl DevKit invention.** It is not a language standard or a native package-manager directive. An ordinary interpreter ignores the comment; only a compatible wrapper prepares the dependencies.

This specification distinguishes:

- **Current implementation:** behavior present at the baseline commit, including limitations.
- **CLI v1 proposal:** a terminal-oriented contract to implement in the standalone project. The proposed commands do not exist in Majkl DevKit today.
- **Deferred work:** extensions that are unnecessary for an initial extraction.

Preserve valid existing declarations, but do not copy desktop-specific stdin, output, cwd and timeout behavior into a CLI without adapting it. This handoff specifies the extraction; it does not implement the wrapper or remove the desktop implementation.

## 2. Initial scope

Retain the eleven current adapters:

| Language ID | Resolver | Runtime |
|---|---|---|
| `perl` | cpanminus | Selected host Perl |
| `php` | Composer | Selected host PHP |
| `raku` | zef | Selected host Raku/Rakudo |
| `janet` | jpm | Selected host Janet |
| `lua` | LuaRocks | Selected host Lua 5.x ABI |
| `go` | Go modules | Selected host toolchain |
| `powershell` | PSResourceGet | Selected host PowerShell |
| `tcl` | Pixi plus Git for Tcllib | Pixi-owned Tcl 8.6 |
| `mojo` | Pixi, Conda and optional PyPI packages | Pixi-owned Mojo and CPython |
| `shell` | Pixi executable packages | Selected host Bash, Zsh or sh |
| `red` | Git source vendoring | Selected host Red CLI |

The current Tcl/Mojo/shell Pixi adapters reject native Windows. WSL can use the Linux profiles. Other adapters contain platform-aware logic, but Windows argv generation is not evidence of actual Windows acceptance.

### Exclusions

Python/uv PEP 723, Java/JBang, Kotlin main-kts, Groovy/Grape, Ruby/Bundler, Deno/Bun imports, .NET NuGet and Julia/Pkg are separate mechanisms. Node's older app-specific `// deps:` syntax is separate too. Do not conflate them with `devkit-deps` or bring the embedded Kotlin/Groovy hosts into the CLI. Native-resolver dispatch can be a later explicit feature.

Defer remote-script URLs, stdin-as-source, arbitrary project execution, automatic global runtime installation, custom setup hooks, universal lockfiles and cache garbage collection. Local single-file scripts are sufficient.

## 3. Source extraction map

The current core is Kotlin/JVM with no Compose imports. It still depends on application-owned types and helpers.

| File | What to extract |
|---|---|
| `src/main/kotlin/services/ScriptDependencyEnvironments.kt` | Parser, package validation, native plans/manifests, module paths, cache, preparation, execution leases and setup process runner |
| `src/main/kotlin/services/BinaryLocator.kt` | PATH lookup, absolute paths, common installation locations |
| `src/main/kotlin/services/ScriptingService.kt` | Small runtime-detection/base-argv helpers, process-tree cleanup and Red error shim; not the entire service |
| `src/main/kotlin/services/SystemInstaller.kt` | Optional zef bootstrap reference and human-readable command formatting |
| `src/main/kotlin/services/Managed*DependencySamples.kt` | Functional fixtures for all eleven adapters |
| `src/main/kotlin/services/DynamicFeatureSamples.kt` | Additional managed JSON fixtures |
| `src/test/kotlin/services/ScriptDependencyEnvironmentsTest.kt` | Parser, manifests, launcher resolution and identity tests |
| `src/test/kotlin/services/ManagedScriptExecutionTest.kt` | Fixture installation/import, reuse, cancellation, pipe draining, shell selection and leases |
| `src/test/kotlin/services/RakuManagedDependenciesTest.kt` | Real/generated Raku launchers, repository lookup and bootstrap |
| `src/test/kotlin/services/FeatureScriptSamplesTest.kt` | Expected functional output and opt-in network tests |

Remove references to `ScriptingService.ScriptLang`, `ShellDialect`, `RunController`, `RuntimeStatus`, `destroyProcessTree` and `SystemInstaller.formatCommand` by moving small equivalents into the standalone core.

A direct extraction needs JDK process/filesystem APIs, JSON serialization and coroutine primitives. The application currently uses Jackson Databind 2.17.1 and kotlinx-coroutines-core 1.8.0; these are baseline versions, not a required choice for the new project. Compose, Swing coroutines, compiler-embeddable, Groovy and Testcontainers are unnecessary runtime dependencies.

A port to another implementation language is acceptable if it passes conformance tests. The protocol is language-independent. Carry the applicable `LICENSE.txt` terms and attribution with extracted source.

The [pinned source tree](https://github.com/michalbcz/majkl-devkit/tree/2bfc2df7020f5ce041cfbe7cb1d6b65440b22979) makes this handoff usable outside the original checkout.

## 4. Current wire format

### Declaration

One physical comment line contains the exact case-sensitive key `devkit-deps:` and a JSON array of strings:

~~~text
<language-comment-prefix> devkit-deps: ["package@version", "another-package@version"]
~~~

| Languages | Prefix | Example |
|---|---|---|
| Perl, Raku, Janet, Tcl, Mojo, shell, PowerShell | `#` | `# devkit-deps: ["Try::Tiny@0.32"]` |
| PHP, Go | `//` | `// devkit-deps: ["rsc.io/quote/v4@v4.0.1"]` |
| Lua | `--` | `-- devkit-deps: ["lustache@1.3.1-0"]` |
| Red | `;` | `; devkit-deps: ["https://host/repository.git#<40-hex-commit>"]` |

Whitespace before the prefix, between prefix and key, and around JSON is accepted. Multiline arrays, environment interpolation, command substitution and executable expressions are not supported.

There is no format-version field in a comment. `managed-environment-v2` is the current cache strategy identifier, not a source-syntax version.

### Header scanning

Only the leading header region is scanned. It consists of:

1. Blank lines.
2. A first-line shebang.
3. PHP's standalone `<?php` line.
4. A single-line Red header matching `Red [ ... ]`, case-insensitively.
5. Lines beginning with the language's comment prefix.

The first other line ends the scan. A Lua long-bracket comment opener matching `--[=[` and other equal-sign counts ends it as well. This is intentionally not a complete language lexer.

Consequences:

- PHP metadata belongs after a standalone opening tag.
- Metadata can precede a multiline Red header, but the current scan does not recognize metadata after that header.
- Declarations after imports, `package main` or executable source are ignored.
- Marker-looking text inside source literals after the first code line must not be interpreted or removed.
- Block comments and documentation syntaxes are not alternate metadata containers.
- UTF-8 BOM handling is not explicit. Portable fixtures use UTF-8 without BOM. The CLI must deliberately support or clearly reject a BOM rather than silently missing requirements.

~~~php
<?php
// devkit-deps: ["symfony/yaml@7.4.21"]
use Symfony\Component\Yaml\Yaml;
echo Yaml::parse("name: console")["name"], PHP_EOL;
~~~

~~~sh
#!/usr/bin/env bash
# devkit-deps: ["ripgrep@15.1.0"]
printf '%s\n' 'INFO ready' 'ERROR upload failed' | rg --no-config '^ERROR'
~~~

### Validation

- No recognized declaration returns “absent”.
- At most one declaration is permitted, even if its array is empty.
- JSON must be an array containing only strings; trailing JSON tokens are rejected.
- Strings are trimmed; empty strings, leading `-` and ISO control characters are rejected.
- Exact duplicate strings after trimming are rejected.
- `[]` is valid. The desktop prepares no environment for an empty or absent declaration.

Conflicting specifications for the same package are not consistently rejected today. PHP/Pixi map construction can overwrite an earlier entry, while other tools receive both. CLI v1 must reject such conflicts before setup.

The desktop editor serializes replacements, preserves a shebang and inserts PHP metadata inside its opening tag. A CLI source-editing command is not needed for v1.

## 5. Current package specification grammar

Ordinary adapters split `name@version` at the **last** `@`. Both parts must be nonempty. A version cannot start with `-` and accepts only:

~~~text
A-Z a-z 0-9 . * + ^ ~ < > = , ! : _ -
~~~

The common name check requires a letter/digit first character and rejects whitespace/control characters. Adapter-specific rules follow:

| Adapter | Form and additional validation |
|---|---|
| Perl | `Module::Name@version`; name `[A-Za-z][A-Za-z0-9_:]*` |
| PHP | `vendor/package@version`; lowercase `[a-z0-9_.-]+/[a-z0-9_.-]+` |
| Raku | `Module::Name@version`; same module-name rule; translate to `Module::Name:ver<version>` |
| Janet | `git::https://repository.git::revision`; revision `[A-Za-z0-9_.-]+` |
| Lua | `rock-name@version-revision`; name `[A-Za-z0-9_.-]+` |
| Go | `module/path@version`; name contains `/` and matches `[A-Za-z0-9_./~-]+` |
| PowerShell | `module-name@version`; name `[A-Za-z0-9_.-]+` |
| Tcl/Pixi | Conda `name@version`; special `tcllib@exact.numeric.release` |
| Mojo/Pixi | Conda form and `pypi:python-package@version` |
| Shell/Pixi | Conda executable-package form |
| Red | HTTPS repository URL plus `#` and a full 40-hexadecimal commit |

Pixi accepts optional `conda:` prefixes. `pypi:` is supported only for Mojo. Its package names are restricted to `[A-Za-z0-9_.-]+`. Numeric-leading versions become `==VERSION` in manifests; constraints such as `>=3.11,<3.12` are retained.

These are lexical rules, not a universal version grammar. A wildcard/range works only if the manager accepts it; the shared allowlist does not admit PowerShell's broader bracketed NuGet ranges. Prefer exact pins in examples.

Tcllib release `2.0` maps to Git tag `tcllib-2-0`. Janet/Tcllib tags are not immutable commit pins. Red requires a full commit. CLI v1 should allow at most one Pixi prefix and reject conflicting normalized package identities; these are intentional validation improvements.

## 6. Current adapter recipes

Notation: `E` is the absolute dependency environment, `R` the selected host runtime/toolchain, `S` the script path, and `M` the manager. Commands are argv arrays, never shell strings. Every managed script receives `DEVKIT_DEPS_DIR=E`.

Child processes inherit the caller environment and then apply adapter overrides. Installed language defaults/global paths can remain visible: placement isolation is not full hermeticity.

### Perl

Run the manager script with the selected Perl:

~~~text
R M --local-lib-contained E/perl --no-interactive --no-prompt --no-sudo
    --no-lwp --mirror https://cpan.metacpan.org --mirror-only Module@version ...
~~~

Setup sets `PERL_CPANM_HOME=E/cpanm-work`, clears `PERL_MM_OPT`/`PERL_MB_OPT` and prepends the selected Perl directory to PATH. HTTPS/curl-compatible transport avoids a reproduced macOS LWP failure.

Execute `R -IE/perl/lib/perl5 S` with `PERL5LIB=E/perl/lib/perl5`. Native modules may need matching headers and a compiler.

### PHP

Generate `E/php/composer.json` with a `require` object. In `E/php` run:

~~~text
R <composer-script-or-phar> install --no-interaction --no-ansi --no-progress
    --no-plugins --no-scripts --prefer-dist
~~~

Set `COMPOSER=composer.json`, `COMPOSER_HOME=E/composer-home`, `COMPOSER_VENDOR_DIR=E/php/vendor`. Retain lock/vendor data and leave platform/security checks enabled.

Execute `R -d auto_prepend_file=E/php/vendor/autoload.php S`. Windows launcher resolution must find the underlying `composer.phar` instead of parsing a batch script as PHP.

### Raku

Resolve manager symlinks. Recent Rakudo generates a shell/native `zef` launcher with sibling `zef.raku`; prefer that dispatcher. Add `-Iinst#<manager-repository>` if the repository contains `dist`, or `-I<checkout>/lib` for a source checkout with `Zef::CLI` source.

~~~text
R <manager-includes> <zef.raku-or-source-script>
    --install-to=inst#E/raku --contained install Module::Name:ver<VERSION> ...
~~~

Execute `R -Iinst#E/raku S` and set `RAKULIB=inst#E/raku`. Keep package tests enabled. A real regression involved both an incorrectly interpreted launcher and JSON::Fast 0.19 failing tests on Rakudo 2026.09; the practical JSON sample now pins 0.21.0.

The desktop separately bootstraps official zef v1.1.4 into `~/.devutils/script-tools/zef`. Standalone `run` should report a missing tool; a later explicit tool-install command can reuse bootstrap. Windows lookup must consider `zef.exe`, `.bat`, `.cmd` and the sibling Raku dispatcher.

### Janet

Use the selected Janet to execute the actual manager source:

~~~text
R <jpm-script> --tree=E/janet install git::https://repository.git::revision
~~~

Set `JANET_TREE=E/janet` for setup and `JANET_PATH=E/janet/lib` for execution; run `R S`. Resolve the source underlying Windows wrappers. Spork's native modules require a C compiler and matching Janet development files.

### Lua

Detect the selected Lua's `5.x` ABI and derive its prefix from the real interpreter path. For each rock, run:

~~~text
M --lua-version=5.x --lua-dir=<selected-prefix> --tree=E/lua
    install rock-name VERSION-REVISION --deps-mode=one
~~~

Then use the same flags with `path --lr-path` and `path --lr-cpath`. The final nonblank output line supplies each search path. Set both `LUA_PATH`/`LUA_CPATH` and `LUA_PATH_5_x`/`LUA_CPATH_5_x`; retain captured values in readiness data.

Execute `R S`. LuaJIT setup is not implemented. dkjson 2.11-1 and lustache 1.3.1-0 passed Lua 5.5.1 checks; an older dkjson rockspec excluded that ABI.

### Go

Generate `E/go.mod` with module `majkl.dev/console` and `go 1.18`. Set `GOWORK=off`, `GOTOOLCHAIN=local`, `GO111MODULE=on` and empty `GOFLAGS`. Prepare using `R get module/path@version ...` in `E`, retaining `go.mod` and `go.sum`.

The desktop executes `R run S` with cwd `E`. This avoids a caller's project and implicit compiler downloads. `go.sum` is checksum data, not a complete conventional lockfile. CLI v1 needs different final execution to preserve cwd and user exit status; see section 9.

### PowerShell

Generate `prepare.ps1`, stop on errors, check `Save-PSResource`, create `E/modules` and run per module:

~~~powershell
Save-PSResource -Name 'module-name' -Version 'VERSION' -Repository PSGallery -Path '<E>/modules' -TrustRepository -Confirm:$false -ErrorAction Stop
~~~

Invoke with the selected runtime's `-NoProfile -NonInteractive -File`. Trust is scoped to this save operation; no persistent repository setting changes. Interactive license acceptance is not supplied.

A run wrapper sets `PSModulePath` after PowerShell startup to `E/modules` plus `$PSHOME/Modules`, then invokes the source. Standalone argument forwarding and explicit script exit propagation must be tested; copying this wrapper unchanged is insufficient.

### Shared Pixi profiles

Generate `E/pixi.toml`, then:

~~~text
pixi install --manifest-path E/pixi.toml
pixi run --manifest-path E/pixi.toml --frozen -- <interpreter> S
~~~

Retain `pixi.lock` and `.pixi`. Current host targets are `osx-arm64`, `osx-64`, `linux-aarch64` and `linux-64`. CLI v1 must reject an unknown CPU instead of silently mapping it to x64.

| Profile | Base dependencies | Execution |
|---|---|---|
| Tcl | `tk >=8.6,<8.7` | Pixi `tclsh8.6` |
| Mojo | `mojo ==1.1.0`, `python >=3.11,<3.12` | Pixi `mojo run` |
| Shell | Declared executable packages | Absolute selected host Bash/Zsh/sh under activated PATH |

All use `conda-forge`; Mojo additionally uses `https://conda.modular.com/max/` and optional `[pypi-dependencies]` for its Python bridge. These are current tested runtime pins, not permanent protocol requirements. Profile changes must affect cache identity.

For `tcllib@2.0`, clone the official `tcllib-2-0` Git tag into `E/tcllib` after Pixi setup. Set `TCLLIBPATH` to its `modules` path encoded as one Tcl list element. This replaces a tested empty Conda Tcllib artifact. Escape whitespace and Tcl metacharacters for Tcl list parsing.

`--frozen` reuses Pixi's lock; it does not guarantee that missing artifacts never need downloading. Native Mojo module search beyond this environment/Python bridge has no additional standardized wrapper contract here.

### Red

For each HTTPS URL/commit, create `E/sources/N` in declaration order:

~~~text
git init --quiet E/sources/N
git -C E/sources/N fetch --depth=1 --no-tags -- <url> <full-commit>
git -C E/sources/N checkout --quiet --detach FETCH_HEAD
~~~

Set `GIT_TERMINAL_PROMPT=0`. Source explicitly loads files below `DEVKIT_DEPS_DIR/sources/N`. Do not enable recursive submodules implicitly.

The desktop uses a Red shim that catches errors from `do %script.red` and calls `quit/return 1`; otherwise printed script errors can leave a successful process exit. Standalone execution needs an absolute-source-path shim and verified argument forwarding.

## 7. Current cache, locks and subprocess behavior

Desktop root: `~/.devutils/script-environments/`. Each directory is `<language>-<sha256>`. The digest hashes Jackson JSON serialization of an ordered list:

1. `managed-environment-v2`.
2. Language ID and shell dialect ID.
3. Host `os.name` and `os.arch`.
4. Selected runtime path/version.
5. Manager path and file modification time when available.
6. Dependency strings in declaration order.

Script contents/location are not included, enabling reuse across scripts. This key does not fully fingerprint secondary tools, sibling manager scripts, compilers, resolver configuration or remote state. It is not a hermetic content lock.

Preparation sequence:

1. Validate required tools.
2. Acquire the use lease and hold it through interpreter exit.
3. Acquire the preparation mutex and sibling `.lock` file lock.
4. Reuse a successful `.ready.json` unless rebuild is requested.
5. Otherwise remove the old/incomplete tree without following symlinks; write manifests.
6. Execute setup steps sequentially; abort on failure, timeout or cancellation.
7. Write captured Lua paths into `.ready.json` only after success.
8. Release the preparation lock, execute, clean up the child, then release the use lease.

The use lease combines a mutex and sibling `.usage.lock` file lock. Equal-environment runs serialize. Another cooperating process cannot rebuild a live environment; waiting is cancellable. Locks must remain outside the tree being deleted.

Readiness currently stores captured environment overrides, not a package inventory or integrity proof. Corrupt markers or externally deleted libraries are not robustly repaired automatically.

Setup subprocesses close stdin, merge/stream output line by line, retain only 200 tail lines, and enforce a ten-minute deadline per step. The watchdog tracks observed descendants and remains active until output drains, even if a parent exits before its child closes an inherited pipe. Never replace this with “wait, then read stdout”. Process polling is best-effort cleanup, not OS containment.

The desktop separately closes script stdin, merges script output and applies a one-minute script limit. Those are desktop constraints, not appropriate CLI defaults.

## 8. CLI v1 proposal: interface

The name is provisional; no executable is supplied by this document.

~~~sh
devkit-deps run release.lua
devkit-deps run --lang raku --runtime /opt/rakudo/bin/raku script.raku -- first "two words"
devkit-deps run --lang shell --shell zsh tools.zsh
devkit-deps plan --json script.go
devkit-deps prepare script.php
devkit-deps run --no-prepare script.php
devkit-deps run --rebuild script.php
devkit-deps doctor --lang lua
~~~

| Command | Required behavior |
|---|---|
| `run` | Parse, validate, plan, prepare/reuse, execute |
| `prepare` | Prepare dependencies without executing source |
| `plan` | Show manifests, setup/execution argv, cwd/env overrides, prerequisites and runtime provenance; no installs or script evaluation |
| `doctor` | Inspect local prerequisite paths/versions and platform support; no installs |

Local tool-version probes are allowed for `plan`/`doctor`. Loading or evaluating source is not.

| Option | Contract |
|---|---|
| `--lang ID` | Explicit selection overrides inference |
| `--runtime ABSOLUTE_PATH` | Select host interpreter; reject for Tcl/Mojo profiles supplying their own |
| `--shell bash\|zsh\|sh` | Shell-only dialect selection |
| `--manager ABSOLUTE_PATH` | Override primary tool; Go/PowerShell use their selected runtime as manager |
| `--cache-dir DIR` | Override cache root |
| `--rebuild` | Recreate this environment after acquiring its use lease |
| `--no-prepare` | Require ready environment; incompatible with `--rebuild` |
| `--setup-timeout SECONDS` | Per-step deadline, default 600 |
| `--timeout SECONDS` | Script deadline, default unlimited |
| `--json` | Machine output for `plan`/`prepare`, not a wrapper around script stdout |
| `--` | End wrapper options; remaining arguments belong to script |

`--no-prepare` does not guarantee offline execution: source and runtimes can access the network. Defer `--offline` until there is genuine native-manager-specific enforcement.

Infer from `.pl`, `.php`, `.raku`/`.rakumod`, `.janet`, `.lua`, `.go`, `.ps1`, `.tcl`, `.mojo`, `.sh`/`.bash`/`.zsh` and `.red`. Extensions are hints; explicit language wins. A module extension is not a guarantee that its file is directly runnable.

For extensionless files, require a recognized simple shebang or explicit language. Recognize interpreter basenames and `/usr/bin/env INTERPRETER`; do not execute shebang text or arbitrary `env -S` contents to infer a language. For shell: explicit dialect, then recognized shebang/extension, then Bash as compatibility default.

A supported script with no declaration or `[]` runs without setup. Unknown languages fail explicitly; later native-resolver support must not silently reinterpret their comments.

## 9. CLI v1 proposal: execution semantics

### Source, arguments and working directory

Resolve the original file to an absolute path and execute it unchanged. Preserve the caller's cwd. Do not silently copy source to a temporary directory: source-relative imports and filename variables can change.

Forward script arguments as exact argv elements, including spaces, empty strings, Unicode and leading dashes. Never concatenate them into an executable shell string.

Go needs two stages:

1. Build the absolute single-file source with the managed module workspace as build cwd.
2. Run its per-execution temporary binary from caller cwd with script arguments.
3. Hold the use lease through build/execution; delete only that run's temporary binary afterward.

This avoids `go run` helper-process exit semantics and managed cwd leaking into the program. Arbitrary multi-file project execution is outside scope.

PowerShell's wrapper must forward arguments and preserve explicit exits. Red's shim must run the absolute original source, retain error handling and forward arguments according to tested Red semantics. These are acceptance requirements, not established desktop capabilities.

### Streams, cancellation and errors

Inherit or transparently relay script stdin/stdout/stderr separately, preserving bytes. Support binary output, pipelines, terminal input and Unicode. Wrapper progress, installer output and diagnostics go to stderr. Source stdout stays untouched.

Setup remains noninteractive with stdin closed. Drain output during execution with bounded retained logs. Final script execution must not inherit the desktop's line-oriented merged-output abstraction.

Forward SIGINT/SIGTERM where supported, stop active process trees, wait for cleanup and release leases. Include interruption while waiting for locks. Native Windows requires an appropriate tested process-tree strategy.

| Outcome | Proposed exit code |
|---|---|
| Successful utility command or script | `0` |
| User program started and exited | Its actual exit code |
| Invalid CLI/declaration, unsupported language, conflicting specs | `2` |
| Missing/incompatible interpreter, manager or host profile | `3` |
| Setup/build/cache/launch failure before the user program starts | `4` |
| Wrapper deadline fired | `124` |
| SIGINT / SIGTERM on POSIX | `130` / `143` |

A script's codes can overlap wrapper codes; stderr distinguishes the phase. Go build failure is a wrapper failure; preserve the built program's exit after it starts. A script that exits 124 itself is not evidence that the wrapper timed out.

### CLI-owned cache

Precedence: `--cache-dir`, `DEVKIT_DEPS_CACHE_DIR`, then platform default:

- Linux: `$XDG_CACHE_HOME/devkit-deps` or `~/.cache/devkit-deps`.
- macOS: `~/Library/Caches/devkit-deps`.
- Windows: `%LOCALAPPDATA%/devkit-deps/cache`.

`DEVKIT_DEPS_DIR` remains the child environment location, not the cache-root option.

Version the CLI's cache separately. Do not automatically trust desktop readiness markers or share an incompatible lock layout. Include adapter/profile version, host target, runtime identity, relevant manager/helper identities and dependency order in cache selection. Changing built-in Mojo/Python pins must select a new environment.

Publish readiness atomically with a versioned marker and validate essential files before reuse. Corruption/incompleteness triggers an exclusive repair, or a clear failure with `--no-prepare`. Directory existence alone is not readiness.

## 10. Intentional improvements and compatibility limits

| Current limitation | CLI requirement |
|---|---|
| Different specs for one package can overwrite map entries | Reject normalized identity conflicts before setup |
| Repeated prefix stripping can admit stacked Pixi prefixes | Accept exactly zero/one supported prefix |
| Narrow header scan | Preserve portable forms; document and test extensions |
| BOM/multiline headers can hide metadata | Deliberate support or explicit diagnosis |
| Runtime pins live in adapter code | Version profile inputs in cache identity |
| Marker is captured env only | Version schema; validate and repair incomplete state |
| App-owned zef can be outside PATH | Support explicit manager path; retain needed fallback during desktop migration |
| Batch/native wrappers are not interpreter source | Resolve underlying source/PHAR or fail precisely |
| Native lock/search-path guarantees differ | Preserve manager metadata; no false universal lock/hermeticity claim |

Treat declarations as data. Escape JSON/TOML/PowerShell/Tcl appropriately, separately from human-readable shell quoting. Delete only owned cache paths without following symlinks out of the tree. Failed/cancelled setup must not publish readiness; rebuild must not remove a cooperating live script's dependencies.

Packages, build hooks and scripts execute with user permissions. This mechanism is not a host security sandbox. Normal `run` must not auto-elevate, install OS packages, change global repository trust/configuration or disable package/platform checks. Explicit tool bootstrap can be a separate later command.

## 11. Proposed core API and plan schema

Keep planning independent from CLI/UI presentation:

~~~text
parse(source, language) -> Absent | Dependencies(orderedSpecs)
validate(dependencies, adapter) -> NormalizedDependencies | Diagnostic
discover(language, overrides, platform) -> RuntimeAndTools
plan(dependencies, runtimeAndTools, cachePolicy) -> EnvironmentPlan
acquireUseLease(plan, cancellation) -> Lease
prepare(plan, lease, cancellation, outputSink) -> PreparedEnvironment
launch(prepared, sourcePath, scriptArgs, cwd, stdio, cancellation) -> ExitResult
~~~

One plan must drive both inspection and execution. It includes generated files, exact argv, cwd, env overrides, runtime origin, prerequisites and deferred captured values.

Proposed `plan --json` shape, with an abbreviated Lua example:

~~~json
{
  "schemaVersion": 1,
  "language": "lua",
  "dependencies": ["lustache@1.3.1-0"],
  "environmentDirectory": "/cache/devkit-deps/lua-<key>",
  "readiness": "missing",
  "runtime": {"path": "/selected/lua", "origin": "host", "version": "5.5.1"},
  "missingTools": [],
  "files": [],
  "setup": [
    {
      "argv": ["/selected/luarocks", "--lua-version=5.5", "--lua-dir=/selected", "--tree=/cache/devkit-deps/lua-<key>/lua", "install", "lustache", "1.3.1-0", "--deps-mode=one"],
      "cwd": "/cache/devkit-deps/lua-<key>",
      "environment": {},
      "captureEnvironment": []
    }
  ],
  "execution": {
    "preRun": [],
    "argv": ["/selected/lua", "/absolute/release.lua"],
    "cwd": "/caller",
    "environment": {"DEVKIT_DEPS_DIR": "/cache/devkit-deps/lua-<key>"},
    "deferredEnvironment": ["LUA_PATH", "LUA_CPATH", "LUA_PATH_5_5", "LUA_CPATH_5_5"]
  }
}
~~~

A real Lua plan also includes both path-query steps. Unprepared output identifies deferred values; prepared output includes actual captured paths. Go's `preRun` contains its build step.

Output only managed env overrides, not the caller's credentials/environment. `prepare --json` emits one stdout result after setup, with progress on stderr. Version the machine schema independently from comment syntax and cache layout.

Diagnostics should provide `code`, `message`, `language`, `phase` and `missingTools`, plus source `line` when available.

## 12. Portable example fixtures

These declarations are accepted by the current mechanism. The proposed invocation is `devkit-deps run FILE`.

### Lua template

~~~lua
-- devkit-deps: ["lustache@1.3.1-0"]
local lustache = require("lustache")
print(lustache:render("Release {{version}}: {{feature}}", {
    version = "2.0", feature = "Offline tools"
}))
~~~

Expected: `Release 2.0: Offline tools`.

### Raku spelling distance

~~~raku
# devkit-deps: ["Text::Levenshtein@0.2.4"]
use Text::Levenshtein:ver<0.2.4>;
say distance('kitten', 'sitting');
~~~

Expected: `3`. The desktop's full sample compares several suggestions.

### Go module

~~~go
// devkit-deps: ["rsc.io/quote/v4@v4.0.1"]
package main

import (
    "fmt"
    "rsc.io/quote/v4"
)

func main() { fmt.Println(quote.Go()) }
~~~

Expected: `Don't communicate by sharing memory, share memory by communicating.`

For other adapters, transfer the complete current managed sample with its expected output. Treat pins as snapshot fixtures, updated alongside compatibility/profile tests.

## 13. Acceptance tests

### Offline parser/plan corpus

| ID | Case | Assertion |
|---|---|---|
| P01 | Valid prefix, whitespace, JSON string array | Ordered specs extracted |
| P02 | Shebang then metadata | Shebang retained; metadata found |
| P03 | PHP tag / single-line Red header | Correct header placement |
| P04 | Go literal marker after code / Lua long comment | Never interpreted or removed as metadata |
| P05 | Duplicate declarations, including two empty arrays | Error |
| P06 | Object/non-string/blank/control character/trailing JSON | Error |
| P07 | Option-like package or version | Rejected before setup |
| P08 | No metadata versus empty array | Distinguishable parse results; no preparation |
| P09 | Exact duplicate / conflicting normalized package | Rejected |
| P10 | Unknown language / unsupported Windows Pixi profile | Clear pre-install error |
| P11 | Spaces/metacharacters in paths | Literal argv and correct manifest escaping |
| P12 | Changed pin/runtime/profile versus same deps in another script | Correct cache separation/reuse |

### Local fixture manager and process tests

| ID | Case | Assertion |
|---|---|---|
| E01 | Fixture installs an importable module | Script imports through planned paths |
| E02 | Repeat the same run | Cache reused; fixture installer not invoked |
| E03 | Setup fails, is cancelled or times out | No ready marker |
| E04 | Thousands of installer lines | No deadlock; bounded retained logs |
| E05 | Parent exits while child retains stdout | Deadline remains active; child cleaned up |
| E06 | Cross-process live run plus rebuild | Rebuild waits; active files remain intact |
| E07 | Cancel while waiting for lease | Prompt exit without altering active environment |
| E08 | Symlink points outside incomplete cache | Cleanup does not traverse/delete target |
| E09 | Cache path contains spaces | Install, import and reuse succeed |
| E10 | Corrupt marker/missing installed files | Exclusive repair or explicit no-prepare failure |
| E11 | Selected shell is Zsh | Actual child remains Zsh |
| E12 | Composer/Rakudo wrapper fixtures | Correct PHAR/source/repository selected |

### CLI behavior not established by desktop tests

1. Exact argument forwarding: spaces, empty strings, Unicode, leading dashes.
2. Binary stdout, separate stderr, stdin pipes and terminal input.
3. Caller cwd and relative file access, especially Go and Red.
4. Script exit values 0, 7 and 42; Go binary, PowerShell wrapper and Red caught-error path.
5. Signals, deadlines, child cleanup and released leases.
6. Plan never installs/evaluates; prepare never runs source.
7. JSON machine output parses and does not mix with progress.
8. Missing prerequisites produce diagnostics without global installation.
9. Empty/no-header source invokes no manager.
10. Unsupported native-resolver languages do not silently reinterpret metadata.
11. Profile/manager changes invalidate appropriate cache selections.
12. Source bytes remain unchanged.

Keep real downloads opt-in. Exercise Try::Tiny, Symfony YAML, Text::Levenshtein, Spork Base64, lustache, Go quote, powershell-yaml, Tcllib SHA-256, Mojo/Humanize, ripgrep and Red source formatting. Separate cold installs, warm reuse and platform evidence. Skips are not passed integrations; native Windows needs actual runtime tests.

## 14. Migration sequence and release completion

1. Freeze the parser corpus and eleven-adapter fixtures at the baseline commit.
2. Extract UI-free language/runtime, cancellation and process helpers, or equivalents in a port.
3. Move pure parsing/plan generation; compare argv/manifests/module lookup with baseline.
4. Port preparation, locks, readiness and cross-process use leases.
5. Implement terminal execution semantics, with explicit Go/PowerShell/Red work.
6. Run offline fixtures, then opt-in real-manager tests on claimed platforms.
7. Ship a CLI that works without the desktop application.
8. Have DevKit consume the same core library or a versioned CLI machine protocol; keep one adapter implementation authoritative.

Do not remove desktop functionality first, or copy the whole application dependency graph just to obtain an enum or helper.

A standalone release is complete when valid script comments work without user-authored sidecar manifests; installed packages are actually discoverable; source/args/cwd/streams/exits obey the CLI contract; cache/rebuild/cancellation/recovery are proven; missing tools fail clearly; and shipped platforms are tested.

Its README must call `devkit-deps` the wrapper's convention, identify native prerequisites and preserve source/license attribution. Native lockfiles are retained without claiming a universal transitive lock or guaranteed offline execution.

## 15. Pinned sources and supporting research

Current source is authoritative for existing behavior; proposed sections are the extraction target.

- [Dependency core](https://github.com/michalbcz/majkl-devkit/blob/2bfc2df7020f5ce041cfbe7cb1d6b65440b22979/src/main/kotlin/services/ScriptDependencyEnvironments.kt)
- [Desktop execution service](https://github.com/michalbcz/majkl-devkit/blob/2bfc2df7020f5ce041cfbe7cb1d6b65440b22979/src/main/kotlin/services/ScriptingService.kt)
- [Binary lookup](https://github.com/michalbcz/majkl-devkit/blob/2bfc2df7020f5ce041cfbe7cb1d6b65440b22979/src/main/kotlin/services/BinaryLocator.kt)
- [Architecture decision](https://github.com/michalbcz/majkl-devkit/blob/2bfc2df7020f5ce041cfbe7cb1d6b65440b22979/docs/adrs/006-managed-script-dependency-environments.md)
- [Desktop usage](SCRIPT_DEPENDENCY_ENVIRONMENTS.md)
- [Dynamic-manager research](research/MANAGED_DEPENDENCIES_DYNAMIC.md)
- [Go/Pixi/PowerShell research](research/MANAGED_DEPENDENCIES_MISC.md)
- [Native mechanisms outside this protocol](SCRIPT_DEPENDENCIES.md)

When copying this handoff into a new repository, retain pinned links and vendor the referenced fixtures, or preserve their pinned source links. Later desktop changes must not silently redefine the standalone contract.

