package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mchmarny/kubectl-usage/pkg/analyzer"
	"github.com/mchmarny/kubectl-usage/pkg/cli"
	"github.com/mchmarny/kubectl-usage/pkg/collector"
	"github.com/mchmarny/kubectl-usage/pkg/k8s"
	"github.com/mchmarny/kubectl-usage/pkg/output"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	parser := cli.NewParser(cli.Name)
	opts, err := parser.Parse(os.Args)
	if err != nil {
		return err
	}

	if opts == nil {
		return nil
	}

	clientManager, err := k8s.NewClientManager()
	if err != nil {
		return err
	}

	// app components using dependency injection
	dataCollector := collector.New(clientManager.CoreClient(), clientManager.MetricsClient())
	dataAnalyzer := analyzer.New()
	outputFormatter := output.New()
	defer outputFormatter.Close()

	// Create context with timeout for all Kubernetes operations
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Collect data from Kubernetes APIs
	rows, err := dataCollector.Collect(ctx, *opts)
	if err != nil {
		return err
	}

	// Analyze and sort the collected data
	dataAnalyzer.Sort(rows, *opts)

	// Apply post-processing filters
	rows = dataAnalyzer.Filter(rows, *opts)

	// Format and output the results
	return outputFormatter.PrintTable(rows, *opts)
}
