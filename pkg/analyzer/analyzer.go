// Package analyzer provides data analysis and sorting functionality for resource usage metrics.
// This package implements the strategy pattern for different sorting algorithms and
// encapsulates the business logic for processing collected metrics data.
package analyzer

import (
	"sort"

	"github.com/mchmarny/kusage/pkg/config"
	"github.com/mchmarny/kusage/pkg/metrics"
)

// Analyzer provides methods for analyzing and sorting resource usage data.
// This type implements the strategy pattern, allowing different sorting
// strategies to be applied to the collected metrics data.
type Analyzer struct {
	// Future extension point for configurable analysis strategies
}

// New creates a new Analyzer instance.
func New() *Analyzer {
	return &Analyzer{}
}

// Sort sorts the provided rows according to the specified sorting strategy.
// This method implements stable sorting with secondary sort criteria to ensure
// consistent, deterministic results across multiple runs.
//
// The sorting follows a two-level hierarchy:
// 1. Primary sort by the specified sort key (descending for numerical values)
// 2. Secondary sort by namespace/name (ascending for alphabetical consistency)
//
// This design ensures that results are both meaningful (highest usage first)
// and deterministic (consistent ordering for equal values).
func (a *Analyzer) Sort(rows []metrics.Row, opts config.Options) {
	sort.Slice(rows, func(i, j int) bool {
		return a.compareRows(rows[i], rows[j], opts)
	})
}

// Filter applies post-collection filtering to the results.
// This method implements the filter pattern and can be used to apply
// additional filtering logic after data collection and correlation.
func (a *Analyzer) Filter(rows []metrics.Row, opts config.Options) []metrics.Row {
	if opts.TopN <= 0 || opts.TopN >= len(rows) {
		return rows
	}
	return rows[:opts.TopN]
}

// compareRows implements the comparison logic for sorting rows.
// This method encapsulates the complex multi-criteria sorting logic
// and provides stable, deterministic ordering.
func (a *Analyzer) compareRows(left, right metrics.Row, opts config.Options) bool {
	// Primary sort by the specified sort key
	switch opts.Sort {
	case config.SortByUsage:
		return a.compareByUsage(left, right, opts.Resource)
	case config.SortByLimit:
		return a.compareByLimit(left, right, opts.Resource)
	default: // config.SortByPercentage
		return a.compareByPercentage(left, right)
	}
}

// compareByUsage compares rows by resource usage values.
func (a *Analyzer) compareByUsage(left, right metrics.Row, resource config.ResourceKind) bool {
	switch resource {
	case config.ResourceMemory:
		if left.UsageMi == right.UsageMi {
			return a.compareByIdentity(left, right)
		}
		return left.UsageMi > right.UsageMi // Descending order
	case config.ResourceCPU:
		if left.UsageMc == right.UsageMc {
			return a.compareByIdentity(left, right)
		}
		return left.UsageMc > right.UsageMc // Descending order
	default:
		return a.compareByIdentity(left, right)
	}
}

// compareByLimit compares rows by resource limit values.
func (a *Analyzer) compareByLimit(left, right metrics.Row, resource config.ResourceKind) bool {
	switch resource {
	case config.ResourceMemory:
		if left.LimitMi == right.LimitMi {
			return a.compareByIdentity(left, right)
		}
		return left.LimitMi > right.LimitMi // Descending order
	case config.ResourceCPU:
		if left.LimitMc == right.LimitMc {
			return a.compareByIdentity(left, right)
		}
		return left.LimitMc > right.LimitMc // Descending order
	default:
		return a.compareByIdentity(left, right)
	}
}

// compareByPercentage compares rows by usage percentage.
func (a *Analyzer) compareByPercentage(left, right metrics.Row) bool {
	if left.Percentage == right.Percentage {
		return a.compareByIdentity(left, right)
	}
	return left.Percentage > right.Percentage // Descending order
}

// compareByIdentity provides a stable secondary sort criterion.
// This ensures deterministic ordering when primary sort values are equal.
func (a *Analyzer) compareByIdentity(left, right metrics.Row) bool {
	if left.Namespace == right.Namespace {
		return left.Name < right.Name // Ascending alphabetical order
	}
	return left.Namespace < right.Namespace // Ascending alphabetical order
}
