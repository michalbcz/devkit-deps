#! /usr/bin/env gforth
\ devkit-deps: ["https://github.com/forthy42/gforth.git#c9536461d750ea7b7ac8b738bf8763b839ecf1b9"]
\ The space after #! is required so Gforth treats this line as a comment.
\ Read arguments with next-arg. bye leaves Gforth instead of staying interactive.

include random.fs
." Hello, world!" cr
bye
