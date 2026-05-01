// askit — terminal client for OpenAI-compatible chat completion APIs.
//
// See the repository README and specs/001-askit-cli-v1/ for the full contract.
package main

import (
	"os"

	"github.com/sgaunet/askit/internal/cli"
)

func main() {
	os.Exit(int(cli.Execute(os.Args[1:])))
}
