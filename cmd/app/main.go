package main

import (
	"log/slog"
	"os"

	platformCLI "go-example/internal/platform/cli"
)

func main() {
	rootCommand := platformCLI.NewRootCommand()
	if err := rootCommand.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
