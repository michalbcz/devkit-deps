// devkit-deps: ["rsc.io/quote/v4@v4.0.1"]
// Pin Go with: devkit-deps run --runtime-version 1.22 hello.go
package main

import (
	"fmt"

	"rsc.io/quote/v4"
)

func main() {
	fmt.Println(quote.Go())
}
