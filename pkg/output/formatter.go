// Package output provides formatted output functionality for resource usage analysis results.
// This package implements the presenter pattern and encapsulates all output formatting logic,
// supporting multiple output formats while maintaining separation of concerns.
package output

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mchmarny/kubectl-usage/pkg/config"
	"github.com/mchmarny/kubectl-usage/pkg/metrics"
)

// Formatter handles the formatting and presentation of analysis results.
// This type implements the strategy pattern for different output formats
// and encapsulates all presentation logic.
type Formatter struct {
	writer *tabwriter.Writer
}

// New creates a new Formatter instance configured for tabular output.
// The tabwriter is configured with production-ready defaults for CLI tools.
func New() *Formatter {
	// Configure tabwriter for clean, aligned output
	// Parameters: output, minwidth, tabwidth, padding, padchar, flags
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	return &Formatter{
		writer: writer,
	}
}

// PrintTable outputs the analysis results in tabular format.
// This method implements the table presenter pattern and handles
// both header generation and data formatting based on the analysis mode.
//
// The output format is optimized for human readability while maintaining
// machine-parseable structure when headers are suppressed.
func (f *Formatter) PrintTable(rows []metrics.Row, opts config.Options) error {
	// Print headers unless suppressed
	if !opts.NoHeaders {
		if err := f.printHeaders(opts); err != nil {
			return fmt.Errorf("failed to print headers: %w", err)
		}
	}

	// Print data rows
	for _, row := range rows {
		if err := f.printRow(row, opts); err != nil {
			return fmt.Errorf("failed to print row: %w", err)
		}
	}

	// Flush the tabwriter to ensure all output is written
	return f.writer.Flush()
}

// printHeaders outputs the table headers based on the analysis configuration.
func (f *Formatter) printHeaders(opts config.Options) error {
	// Format the resource name column header
	var resourceName string
	if opts.Mode == config.ModeContainers {
		resourceName = "CONTAINER (POD)"
	} else {
		resourceName = "POD"
	}

	// Format the resource-specific columns
	var usageHeader, limitHeader string
	switch opts.Resource {
	case config.ResourceMemory:
		usageHeader = "USED(Mi)"
		limitHeader = "LIMIT(Mi)"
	case config.ResourceCPU:
		usageHeader = "USED(mCPU)"
		limitHeader = "LIMIT(mCPU)"
	default:
		usageHeader = "USED"
		limitHeader = "LIMIT"
	}

	_, err := fmt.Fprintf(f.writer, "NAMESPACE\t%s\t%s\t%s\t%%USED\n",
		resourceName, usageHeader, limitHeader)
	return err
}

// printRow outputs a single data row in the appropriate format.
func (f *Formatter) printRow(row metrics.Row, opts config.Options) error {
	// Format the resource name for display
	displayName := f.formatResourceName(row.Name, opts.Mode)

	// Format the resource values based on type
	switch opts.Resource {
	case config.ResourceMemory:
		_, err := fmt.Fprintf(f.writer, "%s\t%s\t%.1f\t%.1f\t%.1f%%\n",
			row.Namespace, displayName, row.UsageMi, row.LimitMi, row.Percentage)
		return err
	case config.ResourceCPU:
		_, err := fmt.Fprintf(f.writer, "%s\t%s\t%d\t%d\t%.1f%%\n",
			row.Namespace, displayName, row.UsageMc, row.LimitMc, row.Percentage)
		return err
	default:
		return fmt.Errorf("unknown resource type: %v", opts.Resource)
	}
}

// formatResourceName formats the resource name for display based on the analysis mode.
// For container mode, it converts "pod:container" format to "container (pod)" for better readability.
func (f *Formatter) formatResourceName(name string, mode config.Mode) string {
	if mode == config.ModeContainers {
		// Container rows are in "pod:container" format; convert to "container (pod)" for display
		parts := strings.SplitN(name, ":", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("%s (%s)", parts[1], parts[0])
		}
	}
	return name
}

// Close flushes any remaining output and cleans up resources.
func (f *Formatter) Close() error {
	return f.writer.Flush()
}
