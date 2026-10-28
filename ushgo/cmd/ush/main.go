// Command ush is the Go implementation of ush, the local `//` shell for zsh: the same command as
// the Python ush, built from ushgo/. The port proceeds module by module (docs/TODO.md); until it
// passes ush's conformance tests, this implementation answers no requests.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time: go build -ldflags "-X main.version=0.1.0".
var version = "0.0.0-dev"

// implementation tells the two implementations of the same command apart in `ush --version`.
const implementation = "go"

const usage = `usage: ush --version | --help

This is the Go implementation of ush. It does not answer requests yet;
use the Python implementation (ush/install.sh) for now.
The state of the port is in ushgo/docs/TODO.md.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process: it writes to the given streams and returns the exit status,
// 2 for a usage error as in the Python ush.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 {
		switch args[0] {
		case "--version":
			fmt.Fprintf(stdout, "ush %s (%s)\n", version, implementation)
			return 0
		case "-h", "--help", "help":
			fmt.Fprint(stdout, usage)
			return 0
		}
	}
	fmt.Fprint(stderr, usage)
	return 2
}
