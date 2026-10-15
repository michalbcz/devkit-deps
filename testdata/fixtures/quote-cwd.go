// devkit-deps: ["rsc.io/quote/v4@v4.0.1"]
package main

import (
	"fmt"
	"os"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(cwd)
}
