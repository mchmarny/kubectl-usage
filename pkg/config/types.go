// Package config provides configuration management for kusage.
// This package encapsulates all configuration-related types and validation logic,
// following the single responsibility principle for distributed systems design.
package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Mode represents the analysis mode for resource usage calculation.
type Mode string

const (
	// ModePods aggregates resource usage at the pod level
	ModePods Mode = "pods"
	// ModeContainers analyzes resource usage at the container level
	ModeContainers Mode = "containers"
)

// ResourceKind represents the type of Kubernetes resource to analyze.
type ResourceKind string

const (
	// ResourceMemory analyzes memory usage and limits
	ResourceMemory ResourceKind = "memory"
	// ResourceCPU analyzes CPU usage and limits
	ResourceCPU ResourceKind = "cpu"
)

// SortKey represents the sorting strategy for results.
type SortKey string

const (
	// SortByPercentage sorts by usage/limit percentage (descending)
	SortByPercentage SortKey = "pct"
	// SortByUsage sorts by raw usage values (descending)
	SortByUsage SortKey = "usage"
	// SortByLimit sorts by raw limit values (descending)
	SortByLimit SortKey = "limit"
)

// Options contains all configuration parameters for the kusage tool.
// This structure encapsulates all runtime configuration, making it easy to
// pass configuration through the application layers and enabling better testability.
type Options struct {
	// Namespace specifies the target Kubernetes namespace
	Namespace string
	// AllNamespaces indicates whether to analyze across all namespaces
	AllNamespaces bool
	// LabelSelector is a Kubernetes label selector for filtering resources
	LabelSelector string
	// ExcludeNamespaces is a compiled regex for excluding namespaces
	ExcludeNamespaces *regexp.Regexp
	// ExcludeLabels is a compiled regex for excluding labels
	ExcludeLabels *regexp.Regexp
	// Mode determines the analysis granularity (pods vs containers)
	Mode Mode
	// Resource specifies which resource type to analyze
	Resource ResourceKind
	// Sort determines the sorting strategy for results
	Sort SortKey
	// TopN limits the number of results returned
	TopN int
	// NoHeaders suppresses table headers in output
	NoHeaders bool
	// Timeout configures the context timeout for Kubernetes API calls
	Timeout time.Duration

	// Performance and scale options for large clusters
	// PageSize controls the number of items fetched per API call
	PageSize int64
	// MaxConcurrency limits concurrent operations
	MaxConcurrency int
	// UseStreaming enables streaming processing for memory efficiency
	UseStreaming bool
	// EnableMetrics enables detailed performance metrics collection
	EnableMetrics bool
	// MaxMemoryMB sets the maximum memory usage limit in megabytes
	MaxMemoryMB int64
	// UseFilters enables advanced filtering to reduce data volume
	UseFilters bool
}

// Validate performs comprehensive validation of the configuration options.
// This method implements defensive programming practices essential for reliable
// distributed systems by validating inputs early and providing clear error messages.
func (o *Options) Validate() error {
	// Validate timeout
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", o.Timeout)
	}

	// Validate TopN
	if o.TopN < 0 {
		return fmt.Errorf("top must be non-negative, got %d", o.TopN)
	}

	// Validate label selector format (basic validation)
	if o.LabelSelector != "" {
		// Basic validation - more comprehensive validation happens in the collector
		if strings.Contains(o.LabelSelector, ",,") {
			return fmt.Errorf("invalid label selector format: %s", o.LabelSelector)
		}
	}

	// Validate performance options
	if o.PageSize <= 0 {
		o.PageSize = 500 // Default page size for large clusters
	}

	if o.MaxConcurrency <= 0 {
		o.MaxConcurrency = 10 // Default concurrency limit
	}

	if o.MaxMemoryMB <= 0 {
		o.MaxMemoryMB = 2048 // Default 2GB memory limit
	}

	return nil
}

// ApplyDefaults sets default values for performance options
func (o *Options) ApplyDefaults() {
	if o.PageSize == 0 {
		o.PageSize = 500
	}
	if o.MaxConcurrency == 0 {
		o.MaxConcurrency = 10
	}
	if o.MaxMemoryMB == 0 {
		o.MaxMemoryMB = 2048
	}
	// Enable advanced features for large-scale operations by default
	o.UseStreaming = true
	o.UseFilters = true
}

// String returns a human-readable representation of the configuration.
// This is useful for debugging and logging in distributed environments.
func (o *Options) String() string {
	return fmt.Sprintf(`Options{
		AllNamespaces:%t, 
		Namespace:%q, 
		LabelSelector:%q, 
		NamespaceExclusion:%q, 
		LabelExclusion:%q, 
		Mode:%q, 
		Resource:%q, 
		Sort:%q, 
		TopN:%d, 
		NoHeaders:%t, 
		Timeout:%v
	}`,
		o.AllNamespaces,
		o.Namespace,
		o.LabelSelector,
		o.ExcludeNamespaces,
		o.ExcludeLabels,
		o.Mode,
		o.Resource,
		o.Sort,
		o.TopN,
		o.NoHeaders,
		o.Timeout,
	)
}
