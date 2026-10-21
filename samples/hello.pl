#!/usr/bin/env perl5.40
# perl5.40 asks for Perl 5.40, a prefix of the version perl prints.
# devkit-deps: ["Try::Tiny@0.32"]
# devkit-deps run installs the module with cpanm into a local lib and sets PERL5LIB.
# use finds it there. perl itself skips these comments.

use strict;
use warnings;
use Try::Tiny;

try {
    die "boom\n";
} catch {
    print "caught\n";
};
