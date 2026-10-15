# devkit-deps: ["pypi:humanize@4.12.2"]
from std.python import Python

def main() raises:
    var humanize = Python.import_module("humanize")
    print("Formatted:", humanize.intcomma(1234567))
