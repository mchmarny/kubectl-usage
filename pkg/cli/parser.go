// Package cli provides command-line interface functionality for kusage.
// This package implements the command pattern and encapsulates all CLI-specific logic,
// including argument parsing, validation, and help text generation.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mchmarny/kusage/pkg/config"
)

const (
	// Name of the CLI program.
	Name = "kusage"
)

var (
	// Set at build time via -ldflags "-X main.version=..."
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Parser handles command-line argument parsing and validation.
// This type implements the command pattern and encapsulates all
// CLI argument processing logic.
type Parser struct {
	programName string
}

// NewParser creates a new CLI parser instance.
func NewParser() *Parser {
	return &Parser{
		programName: Name,
	}
}

// Parse processes command-line arguments and returns a validated configuration.
// This method implements comprehensive argument parsing with proper error handling
// and validation, following CLI best practices for user experience.
func (p *Parser) Parse(args []string) (*config.Options, error) {
	if len(args) < 2 {
		p.PrintUsage()
		return nil, errors.New("missing subcommand: pods|containers")
	}

	// Parse subcommand
	subcommand := args[1]
	mode, err := p.parseMode(subcommand)
	if err != nil {
		if subcommand == "-h" || subcommand == "--help" || subcommand == "help" {
			p.PrintUsage()
			return nil, nil // Not an error, just help request
		}
		p.PrintUsage()
		return nil, err
	}

	// Create flag set for the subcommand
	fs := flag.NewFlagSet(p.programName, flag.ExitOnError)

	// Define flags with appropriate defaults and help text
	var (
		allNamespaces = fs.Bool("A", false, "If present, list across all namespaces")
		namespace     = fs.String("n", "default", "Namespace to use (ignored with -A)")
		labelSelector = fs.String("l", "", "Label selector")
		excludeNS     = fs.String("nx", "", "Regex of namespaces to exclude (e.g. ^(osmo-prod|gpu-operator)$)")
		resource      = fs.String("resource", "memory", "Resource to score: memory|cpu (default: memory)")
		sortBy        = fs.String("sort", "pct", "Sort key: pct|usage|limit (default: pct)")
		topN          = fs.Int("top", 20, "Show top N rows")
		noHeaders     = fs.Bool("no-headers", false, "If true, suppress headers in the output")

		// Performance flags for large-scale operations
		pageSize       = fs.Int64("page-size", 500, "Number of items to fetch per API call")
		maxConcurrency = fs.Int("max-concurrency", 10, "Maximum number of concurrent operations")
		useStreaming   = fs.Bool("streaming", true, "Enable streaming processing for memory efficiency")
		enableMetrics  = fs.Bool("metrics", false, "Enable detailed performance metrics collection")
		maxMemoryMB    = fs.Int64("max-memory", 2048, "Maximum memory usage in MB")
		useFilters     = fs.Bool("filters", true, "Enable advanced filtering to reduce data volume")
	)

	// Parse flags from the remaining arguments
	if err := fs.Parse(args[2:]); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Build and validate configuration
	opts := &config.Options{
		Namespace:     *namespace,
		AllNamespaces: *allNamespaces,
		LabelSelector: *labelSelector,
		Mode:          mode,
		Resource:      p.parseResource(*resource),
		Sort:          p.parseSort(*sortBy),
		TopN:          *topN,
		NoHeaders:     *noHeaders,
		Timeout:       30 * time.Second, // Default timeout for Kubernetes operations

		// Performance options for large-scale operations
		PageSize:       *pageSize,
		MaxConcurrency: *maxConcurrency,
		UseStreaming:   *useStreaming,
		EnableMetrics:  *enableMetrics,
		MaxMemoryMB:    *maxMemoryMB,
		UseFilters:     *useFilters,
	}

	// Parse and validate namespace exclusion regex
	if *excludeNS != "" {
		excludeRegex, err := regexp.Compile(*excludeNS)
		if err != nil {
			return nil, fmt.Errorf("invalid --nx regex: %w", err)
		}
		opts.ExcludeNamespaces = excludeRegex
	}

	// Validate the complete configuration
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return opts, nil
}

// parseMode converts a string subcommand to a Mode value.
func (p *Parser) parseMode(subcommand string) (config.Mode, error) {
	switch subcommand {
	case string(config.ModePods):
		return config.ModePods, nil
	case string(config.ModeContainers):
		return config.ModeContainers, nil
	default:
		return "", fmt.Errorf("unknown subcommand %q (expected pods|containers)", subcommand)
	}
}

// parseResource converts a string resource type to a ResourceKind value.
func (p *Parser) parseResource(resource string) config.ResourceKind {
	switch strings.ToLower(resource) {
	case "cpu":
		return config.ResourceCPU
	default:
		return config.ResourceMemory
	}
}

// parseSort converts a string sort key to a SortKey value.
func (p *Parser) parseSort(sortKey string) config.SortKey {
	switch strings.ToLower(sortKey) {
	case "usage":
		return config.SortByUsage
	case "limit":
		return config.SortByLimit
	default:
		return config.SortByPercentage
	}
}

// PrintUsage outputs comprehensive usage information.
// This method provides detailed help text following Unix CLI conventions
// and includes examples for common use cases.
func (p *Parser) PrintUsage() {
	fmt.Fprintf(os.Stderr, `kusage — rank pods/containers by resource limit usage

Usage:
  kusage pods [flags]
  kusage containers [flags]

Basic Flags:
  -A                         All namespaces
  -n string                  Namespace (ignored with -A) (default "default")
  -l string                  Label selector
  --nx string                Regex of namespaces to exclude (e.g. ^(osmo-prod|gpu-operator)$)
  --resource string          Resource to score: memory|cpu (default memory)
  --sort string              Sort key: pct|usage|limit (default pct)
  --top int                  Show top N rows (default 20)
  --no-headers               Suppress headers

Performance Flags (for large clusters):
  --page-size int            Items to fetch per API call (default 500)
  --max-concurrency int      Maximum concurrent operations (default 10)
  --streaming                Enable streaming processing (default true)
  --metrics                  Enable performance metrics collection (default false)
  --max-memory int           Maximum memory usage in MB (default 2048)
  --filters                  Enable advanced filtering (default true)

Requirements:
  This tool requires the following permissions:
  - pods (get, list) in target namespaces
  - pods/metrics (get, list) via metrics.k8s.io API group
  - metrics-server must be installed and running in the cluster

Examples:
  kusage pods -A --sort pct --top 20 --nx '^(kube-system|monitoring)$'
  kusage containers -A --resource pct --sort memory --top 50

`)
}
