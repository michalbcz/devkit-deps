# devkit-deps: ["pypi:humanize@4.12.2"]
# Pixi supplies Mojo. This script does not pick a host mojo version.

from std.python import Python

def main() raises:
    var humanize = Python.import_module("humanize")
    print(humanize.intcomma(1234567))
