# devkit-deps: ["tcllib@2.0"]
# Pixi supplies Tcl. This script does not pick a host tclsh version.

package require sha256
puts [::sha2::sha256 -hex "abc"]
