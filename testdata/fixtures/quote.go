// devkit-deps: ["rsc.io/quote/v4@v4.0.1"]
package main

import (
    "fmt"
    "rsc.io/quote/v4"
)

func main() {
    fmt.Println(quote.Go())
}
