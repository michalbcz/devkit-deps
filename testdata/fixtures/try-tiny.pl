# devkit-deps: ["Try::Tiny@0.32"]
# Run installs the pinned package locally with cpanm before executing this script.
use strict;
use warnings;
use Try::Tiny;

try {
    die "Invalid number\n";
} catch {
    chomp(my $message = $_);
    print "Recovered: $message\n";
};
print "The script continues\n";
