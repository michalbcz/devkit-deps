#!/bin/sh
# devkit-deps: ["ripgrep@15.1.0"]
# The first line picks sh, bash, or zsh. Pin its version with --runtime-version.

printf '%s\n' 'keep' 'drop me' | rg --no-config --color never '^keep$'
