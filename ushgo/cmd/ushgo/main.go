// Command ushgo is the Go port of ush, the local `//` shell for zsh. The port proceeds module by
// module (docs/TODO.md); until it passes ush's conformance tests, ushgo answers no requests.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time: go build -ldflags "-X main.version=0.1.0".
var version = "0.0.0-dev"

const usage = `usage: ushgo --version | --help

ushgo is the Go port of ush and does not answer requests yet; use ush.
The state of the port is in ushgo/docs/TODO.md.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process: it writes to the given streams and returns the exit status,
// 2 for a usage error as in ush.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 {
		switch args[0] {
		case "--version":
			fmt.Fprintf(stdout, "ushgo %s\n", version)
			return 0
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0
		}
	}
	fmt.Fprint(stderr, usage)
	return 2
}
