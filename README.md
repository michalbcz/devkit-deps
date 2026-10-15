# devkit-deps

`devkit-deps` is this wrapper's comment convention, not a language standard. An ordinary interpreter ignores the comment. Only this wrapper prepares the dependencies and runs the script.

Supported language ids: `perl`, `php`, `raku`, `janet`, `lua`, `go`, `powershell`, `tcl`, `mojo`, `shell`, `red`.

The language runtime and its package manager must already be installed. This tool does not install them.

A manager lockfile is kept beside the environment. That is not a universal lock, and it is not a guarantee that a later run can work offline.
