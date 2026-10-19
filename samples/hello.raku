#!/usr/bin/env raku
# devkit-deps: ["Text::Levenshtein@0.2.4"]
# Pin Raku with: devkit-deps run --runtime-version 2024.10 hello.raku

use Text::Levenshtein:ver<0.2.4>;
say distance("kitten", "sitting");
