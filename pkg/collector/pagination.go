// Package collector - pagination support for large-scale clusters
package collector

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/mchmarny/kusage/pkg/config"
	"github.com/mchmarny/kusage/pkg/metrics"
)

const (
	// DefaultPageSize is optimized for large clusters to balance memory usage and API efficiency
	// Reference: https://kubernetes.io/docs/reference/using-api/api-concepts/#retrieving-large-results-sets-in-chunks
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

// fetchPodsWithPagination retrieves pods using pagination to handle large result sets
// Deprecated: Use StreamingCollector for better memory efficiency in large clusters.
//
//nolint:unused // Keeping as legacy API for backward compatibility
func (c *PaginatedCollector) fetchPodsWithPagination(ctx context.Context, opts config.Options) ([]corev1.Pod, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	var allPods []corev1.Pod
	continueToken := ""

	slog.Debug("fetching pods with pagination",
		"namespace", namespace,
		"pageSize", c.pageSize,
		"labelSelector", opts.LabelSelector)

	for {
		listOptions := metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
			Limit:         c.pageSize,
			Continue:      continueToken,
		}

		podList, err := c.coreClient.CoreV1().Pods(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to list pods page in namespace %q: %w", namespace, err)
		}

		allPods = append(allPods, podList.Items...)

		slog.Debug("fetched pods page",
			"count", len(podList.Items),
			"totalSoFar", len(allPods),
			"hasMore", podList.Continue != "")

		// Check if there are more pages
		if podList.Continue == "" {
			break
		}
		continueToken = podList.Continue

		// Check context cancellation between pages
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	slog.Debug("completed paginated pod fetch", "totalPods", len(allPods))
	return allPods, nil
}

// fetchPodMetricsWithPagination retrieves metrics using pagination
// Deprecated: Use StreamingCollector for better memory efficiency in large clusters.
//
//nolint:unused // Keeping as legacy API for backward compatibility
func (c *PaginatedCollector) fetchPodMetricsWithPagination(ctx context.Context, opts config.Options) ([]metrics.PodMetrics, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	var allMetrics []metrics.PodMetrics
	continueToken := ""

	slog.Debug("fetching pod metrics with pagination",
		"namespace", namespace,
		"pageSize", c.pageSize,
		"labelSelector", opts.LabelSelector)

	for {
		listOptions := metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
			Limit:         c.pageSize,
			Continue:      continueToken,
		}

		metricsList, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to list pod metrics page in namespace %q: %w", namespace, err)
		}

		// Convert to internal metrics type
		pageMetrics := make([]metrics.PodMetrics, 0, len(metricsList.Items))
		for _, item := range metricsList.Items {
			pm := metrics.PodMetrics{
				TypeMeta:   item.TypeMeta,
				ObjectMeta: item.ObjectMeta,
				Timestamp:  item.Timestamp,
				Window:     item.Window,
				Containers: make([]metrics.ContainerMetrics, 0, len(item.Containers)),
			}
			for _, container := range item.Containers {
				pm.Containers = append(pm.Containers, metrics.ContainerMetrics{
					Name:  container.Name,
					Usage: container.Usage,
				})
			}
			pageMetrics = append(pageMetrics, pm)
		}

		allMetrics = append(allMetrics, pageMetrics...)

		slog.Debug("fetched metrics page",
			"count", len(pageMetrics),
			"totalSoFar", len(allMetrics),
			"hasMore", metricsList.Continue != "")

		// Check if there are more pages
		if metricsList.Continue == "" {
			break
		}
		continueToken = metricsList.Continue

		// Check context cancellation between pages
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	slog.Debug("completed paginated metrics fetch", "totalMetrics", len(allMetrics))
	return allMetrics, nil
}

// WithPageSize sets a custom page size for testing or tuning
func (c *PaginatedCollector) WithPageSize(size int64) *PaginatedCollector {
	c.pageSize = size
	return c
}
