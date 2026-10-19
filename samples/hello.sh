#!/bin/sh
# devkit-deps: ["ripgrep@15.1.0"]
# The first line picks sh, bash, or zsh.
# Pin bash or zsh with --runtime-version. A pin for sh is rejected when sh prints no version.

printf '%s\n' 'keep' 'drop me' | rg --no-config --color never '^keep$'
