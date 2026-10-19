// devkit-deps: ["http=https://github.com/laytan/odin-http.git#fac113fbd828aad3d71479a534b5de4358b6a07b"]
// The checkout is collections/http and the build adds -collection:deps=<that parent>.
// Odin prints a date, not a dotted version, so --runtime-version does not select it.
package main

import "core:fmt"
import "deps:http"

main :: proc() {
	line := http.Requestline{}
	fmt.println(line.method)
}
