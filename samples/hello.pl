#!/usr/bin/env perl5.40
# perl5.40 asks for Perl 5.40.
# devkit-deps: ["Try::Tiny@0.32"]

use strict;
use warnings;
use Try::Tiny;

try {
    die "boom\n";
} catch {
    print "caught\n";
};
