// Package collector - pagination support for large-scale clusters
package collector

import (
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	// DefaultPageSize is optimized for large clusters to balance memory usage and API efficiency
	DefaultPageSize = 500

	// MaxConcurrentPages limits concurrent pagination requests to prevent API server overload
	MaxConcurrentPages = 10
)

// PaginatedCollector implements chunked data collection for large-scale clusters
type PaginatedCollector struct {
	coreClient    *kubernetes.Clientset
	metricsClient *metricsv.Clientset
	pageSize      int64
}

// NewPaginatedCollector creates a collector optimized for large clusters
func NewPaginatedCollector(coreClient *kubernetes.Clientset, metricsClient *metricsv.Clientset) *PaginatedCollector {
	return &PaginatedCollector{
		coreClient:    coreClient,
		metricsClient: metricsClient,
		pageSize:      DefaultPageSize,
	}
}

// WithPageSize sets a custom page size for testing or tuning
func (c *PaginatedCollector) WithPageSize(size int64) *PaginatedCollector {
	c.pageSize = size
	return c
}
