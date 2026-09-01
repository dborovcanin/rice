// Command rice generates consistent configuration for a SwayFX desktop.
package main

import (
	"fmt"
	"os"

	"github.com/dborovcanin/rice/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "rice:", err)
		os.Exit(1)
	}
}
