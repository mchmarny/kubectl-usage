// Package collector - streaming and memory-efficient processing
package collector

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/mchmarny/kusage/pkg/config"
	"github.com/mchmarny/kusage/pkg/metrics"
)

const (
	// BufferSize controls the channel buffer size for streaming processing
	// Sized to balance memory usage with throughput
	BufferSize = 1000
)

var (
	// MaxConcurrency limits concurrent processing to prevent resource exhaustion
	MaxConcurrency = int64(runtime.NumCPU() * 2)
)

// StreamingResult represents a processed result that can be streamed
type StreamingResult struct {
	Row   *metrics.Row
	Error error
}

// StreamingCollector implements memory-efficient streaming collection
type StreamingCollector struct {
	*Collector // Embed original collector for compute methods
	*PaginatedCollector
	maxConcurrency int64
}

// NewStreamingCollector creates a collector optimized for memory efficiency
func NewStreamingCollector(coreClient *kubernetes.Clientset, metricsClient *metricsv.Clientset) *StreamingCollector {
	return &StreamingCollector{
		Collector:          New(coreClient, metricsClient),
		PaginatedCollector: NewPaginatedCollector(coreClient, metricsClient),
		maxConcurrency:     MaxConcurrency,
	}
}

// CollectStreaming performs streaming collection with bounded memory usage
// This method processes data in chunks and streams results to avoid memory exhaustion
func (c *StreamingCollector) CollectStreaming(ctx context.Context, opts config.Options) <-chan StreamingResult {
	resultChan := make(chan StreamingResult, BufferSize)

	// Use errgroup with bounded concurrency
	g, ctx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(c.maxConcurrency)

	go func() {
		defer close(resultChan)

		c.processStreamingData(ctx, opts, resultChan, g, sem)

		// Wait for all processing to complete
		if err := g.Wait(); err != nil {
			select {
			case resultChan <- StreamingResult{Error: err}:
			case <-ctx.Done():
			}
		}
	}()

	return resultChan
}

// processStreamingData handles the core streaming logic
func (c *StreamingCollector) processStreamingData(
	ctx context.Context,
	opts config.Options,
	resultChan chan<- StreamingResult,
	g *errgroup.Group,
	sem *semaphore.Weighted,
) {
	// Create channels for streaming pod specs and metrics
	podChan := make(chan []corev1.Pod, 10)
	metricsChan := make(chan []metrics.PodMetrics, 10)

	// Start paginated fetching in background
	g.Go(func() error {
		return c.streamPods(ctx, opts, podChan)
	})

	g.Go(func() error {
		return c.streamMetrics(ctx, opts, metricsChan)
	})

	// Process pods and metrics as they arrive
	g.Go(func() error {
		return c.correlateStreamingData(ctx, opts, podChan, metricsChan, resultChan, g, sem)
	})
}

// streamPods fetches pods in pages and streams them through a channel
func (c *StreamingCollector) streamPods(ctx context.Context, opts config.Options, podChan chan<- []corev1.Pod) error {
	defer close(podChan)

	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	continueToken := ""

	for {
		listOptions := metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
			Limit:         c.pageSize,
			Continue:      continueToken,
		}

		podList, err := c.PaginatedCollector.coreClient.CoreV1().Pods(namespace).List(ctx, listOptions)
		if err != nil {
			return fmt.Errorf("failed to stream pods page: %w", err)
		}

		// Send page to processing channel
		select {
		case podChan <- podList.Items:
		case <-ctx.Done():
			return ctx.Err()
		}

		if podList.Continue == "" {
			break
		}
		continueToken = podList.Continue
	}

	return nil
}

// streamMetrics fetches metrics in pages and streams them through a channel
func (c *StreamingCollector) streamMetrics(ctx context.Context, opts config.Options, metricsChan chan<- []metrics.PodMetrics) error {
	defer close(metricsChan)

	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	continueToken := ""

	for {
		listOptions := metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
			Limit:         c.pageSize,
			Continue:      continueToken,
		}

		metricsList, err := c.PaginatedCollector.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, listOptions)
		if err != nil {
			return fmt.Errorf("failed to stream metrics page: %w", err)
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

		// Send page to processing channel
		select {
		case metricsChan <- pageMetrics:
		case <-ctx.Done():
			return ctx.Err()
		}

		if metricsList.Continue == "" {
			break
		}
		continueToken = metricsList.Continue
	}

	return nil
}

// correlateStreamingData processes streaming data and produces results
func (c *StreamingCollector) correlateStreamingData(
	ctx context.Context,
	opts config.Options,
	podChan <-chan []corev1.Pod,
	metricsChan <-chan []metrics.PodMetrics,
	resultChan chan<- StreamingResult,
	g *errgroup.Group,
	sem *semaphore.Weighted,
) error {
	// Build pod index from streaming data
	podIndex := sync.Map{} // Thread-safe map for concurrent access

	// Process pods as they arrive
	for podPage := range podChan {
		// Process this page concurrently
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			c.indexPodPage(podPage, opts, &podIndex)
			return nil
		})
	}

	// Process metrics as they arrive
	for metricsPage := range metricsChan {
		// Process this page concurrently
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			return c.processMetricsPage(ctx, metricsPage, opts, &podIndex, resultChan)
		})
	}

	return nil
}

// indexPodPage adds a page of pods to the thread-safe index
func (c *StreamingCollector) indexPodPage(pods []corev1.Pod, opts config.Options, podIndex *sync.Map) {
	for i := range pods {
		pod := &pods[i]

		// Apply filters
		if opts.ExcludeNamespaces != nil && opts.ExcludeNamespaces.MatchString(pod.Namespace) {
			continue
		}

		// Check label exclusion
		if opts.ExcludeLabels != nil {
			labelString := formatLabels(pod.Labels)
			if opts.ExcludeLabels.MatchString(labelString) {
				continue
			}
		}

		key := pod.Namespace + "/" + pod.Name
		podIndex.Store(key, metrics.NewPodSpecInfo(pod))
	}
}

// processMetricsPage processes a page of metrics and sends results
func (c *StreamingCollector) processMetricsPage(
	ctx context.Context,
	metricsPage []metrics.PodMetrics,
	opts config.Options,
	podIndex *sync.Map,
	resultChan chan<- StreamingResult,
) error {

	for _, pm := range metricsPage {
		key := pm.Namespace + "/" + pm.Name
		value, exists := podIndex.Load(key)
		if !exists {
			continue // No matching pod spec
		}

		podInfo := value.(*metrics.PodSpecInfo)

		// Process based on mode
		switch opts.Mode {
		case config.ModePods:
			if row := c.computePodRow(pm, podInfo, opts.Resource); row != nil {
				select {
				case resultChan <- StreamingResult{Row: row}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case config.ModeContainers:
			containerRows := c.computeContainerRows(pm, podInfo, opts.Resource)
			for _, row := range containerRows {
				select {
				case resultChan <- StreamingResult{Row: &row}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	return nil
}

// WithMaxConcurrency sets the maximum concurrent operations
func (c *StreamingCollector) WithMaxConcurrency(maxConcurrency int64) *StreamingCollector {
	c.maxConcurrency = maxConcurrency
	return c
}

// formatLabels converts a label map to a string for regex matching
// Format: "key1=value1,key2=value2"
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	parts := make([]string, 0, len(labels))
	for key, value := range labels {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}
