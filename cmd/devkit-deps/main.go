package main

import (
	"os"

	"devkit-deps/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
