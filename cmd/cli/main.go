package main

import (
	"log/slog"
	"os"

	"github.com/mchmarny/kusage/pkg/cli"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Simplify log output for CLI, remove "time" and "source" fields
			if a.Key == "time" || a.Key == "source" {
				return slog.Attr{}
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	if err := cli.Run(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
