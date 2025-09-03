// Package collector implements the core data collection logic for Kubernetes resource metrics.
// This package follows the collector pattern common in monitoring systems and provides
// concurrent, fault-tolerant data collection from multiple Kubernetes APIs.
package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/mchmarny/kubectl-usage/pkg/config"
	"github.com/mchmarny/kubectl-usage/pkg/metrics"
)

// Collector handles the collection and correlation of Kubernetes resource data.
// This type implements the collector pattern and encapsulates all the complex
// logic for gathering data from multiple Kubernetes APIs concurrently.
type Collector struct {
	coreClient    *kubernetes.Clientset
	metricsClient *metricsv.Clientset
}

// New creates a new Collector instance.
func New(coreClient *kubernetes.Clientset, metricsClient *metricsv.Clientset) *Collector {
	return &Collector{
		coreClient:    coreClient,
		metricsClient: metricsClient,
	}
}

// Collect gathers pod specifications and metrics data, then correlates them to produce
// resource usage analysis results. This method implements concurrent data collection
// using errgroup for improved performance in distributed environments.
//
// The collection process follows these steps:
// 1. Concurrently fetch pod specifications and metrics from Kubernetes APIs
// 2. Apply filters (namespace exclusions, label selectors)
// 3. Index pod specifications for efficient lookup
// 4. Correlate metrics with specifications to compute usage percentages
// 5. Return structured results for further processing
//
// This design follows the scatter-gather pattern common in distributed systems
// and implements proper error handling and context cancellation.
func (c *Collector) Collect(ctx context.Context, opts config.Options) ([]metrics.Row, error) {
	var (
		podsList    []corev1.Pod
		metricsList []metrics.PodMetrics
	)

	// Use errgroup for concurrent data collection with proper error handling
	// This pattern is essential for responsive CLI tools that need to gather
	// data from multiple API endpoints efficiently
	g, ctx := errgroup.WithContext(ctx)

	// Fetch pod specifications concurrently
	g.Go(func() error {
		pods, err := c.fetchPods(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to fetch pods: %w", err)
		}
		podsList = pods
		return nil
	})

	// Fetch pod metrics concurrently
	g.Go(func() error {
		podMetrics, err := c.fetchPodMetrics(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to fetch pod metrics: %w", err)
		}
		metricsList = podMetrics
		return nil
	})

	// Wait for both operations to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Validate that we have the necessary data
	if len(podsList) == 0 {
		return nil, errors.New("no pods found - check namespace and label selector")
	}
	if len(metricsList) == 0 {
		return nil, errors.New("no pod metrics found - ensure metrics-server is installed and running")
	}

	// Correlate data and compute results
	return c.correlateData(podsList, metricsList, opts)
}

// fetchPods retrieves pod specifications from the Kubernetes API.
func (c *Collector) fetchPods(ctx context.Context, opts config.Options) ([]corev1.Pod, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	listOptions := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	}

	slog.Debug("fetching pods",
		"namespace", namespace,
		"labelSelector", opts.LabelSelector)

	podList, err := c.coreClient.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %q: %w", namespace, err)
	}

	if podList == nil || len(podList.Items) == 0 {
		slog.Warn("no pods found",
			"namespace", namespace,
			"labelSelector", opts.LabelSelector)
		return nil, nil
	}

	slog.Debug("fetched pods", "count", len(podList.Items))
	return podList.Items, nil
}

// fetchPodMetrics retrieves pod metrics from the metrics API.
func (c *Collector) fetchPodMetrics(ctx context.Context, opts config.Options) ([]metrics.PodMetrics, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	listOptions := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	}

	slog.Debug("fetching pod metrics",
		"namespace", namespace,
		"labelSelector", opts.LabelSelector)

	metricsList, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pod metrics in namespace %q (ensure metrics-server is running): %w", namespace, err)
	}

	if metricsList == nil || len(metricsList.Items) == 0 {
		slog.Warn("no pod metrics found",
			"namespace", namespace,
			"labelSelector", opts.LabelSelector)
		return nil, nil
	}

	// Convert to internal metrics type
	result := make([]metrics.PodMetrics, 0, len(metricsList.Items))
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
		result = append(result, pm)
	}

	slog.Debug("fetched pod metrics", "count", len(result))
	return result, nil
}

// correlateData joins pod specifications with metrics data and computes usage analysis.
func (c *Collector) correlateData(pods []corev1.Pod, podMetrics []metrics.PodMetrics, opts config.Options) ([]metrics.Row, error) {
	// Parse label selector for filtering
	labelSelector, err := labels.Parse(opts.LabelSelector)
	if err != nil && opts.LabelSelector != "" {
		return nil, fmt.Errorf("invalid label selector %q: %w", opts.LabelSelector, err)
	}

	// Build an index of pod specifications for efficient lookup
	// Use map for O(1) lookups instead of O(n) iteration for better performance
	podIndex := make(map[string]*metrics.PodSpecInfo, len(pods))

	for i := range pods {
		pod := &pods[i]

		// Apply namespace exclusion filter
		if opts.ExcludeNamespaces != nil && opts.ExcludeNamespaces.MatchString(pod.Namespace) {
			continue
		}

		// Apply label selector filter
		if labelSelector != nil && !labelSelector.Matches(labels.Set(pod.Labels)) {
			continue
		}

		key := pod.Namespace + "/" + pod.Name
		podIndex[key] = metrics.NewPodSpecInfo(pod)
	}

	// Process metrics and compute usage rows
	return c.computeUsageRows(podMetrics, podIndex, opts)
}

// computeUsageRows processes metrics data and computes usage analysis results.
func (c *Collector) computeUsageRows(podMetrics []metrics.PodMetrics, podIndex map[string]*metrics.PodSpecInfo, opts config.Options) ([]metrics.Row, error) {
	var rows []metrics.Row

	for _, pm := range podMetrics {
		key := pm.Namespace + "/" + pm.Name
		podInfo, exists := podIndex[key]
		if !exists {
			continue // metrics for a pod we didn't list (filtered or race condition)
		}

		switch opts.Mode {
		case config.ModePods:
			if row := c.computePodRow(pm, podInfo, opts.Resource); row != nil {
				rows = append(rows, *row)
			}
		case config.ModeContainers:
			containerRows := c.computeContainerRows(pm, podInfo, opts.Resource)
			rows = append(rows, containerRows...)
		}
	}

	return rows, nil
}

// computePodRow computes a usage row for pod-level aggregation.
func (c *Collector) computePodRow(pm metrics.PodMetrics, podInfo *metrics.PodSpecInfo, resource config.ResourceKind) *metrics.Row {
	switch resource {
	case config.ResourceMemory:
		return c.computePodMemoryRow(pm, podInfo)
	case config.ResourceCPU:
		return c.computePodCPURow(pm, podInfo)
	default:
		return nil
	}
}

// computePodMemoryRow computes memory usage for a pod.
func (c *Collector) computePodMemoryRow(pm metrics.PodMetrics, podInfo *metrics.PodSpecInfo) *metrics.Row {
	if !podInfo.HasMemoryLimit() {
		return nil
	}

	var totalUsageMi float64
	for _, container := range pm.Containers {
		if !podInfo.ContainerHasMemoryLimit(container.Name) {
			continue
		}
		if qty, ok := container.Usage[corev1.ResourceMemory]; ok {
			totalUsageMi += float64(qty.Value()) / (1024 * 1024)
		}
	}

	percentage := (totalUsageMi / podInfo.MemoryLimitMi) * 100
	return &metrics.Row{
		Namespace:  pm.Namespace,
		Name:       pm.Name,
		UsageMi:    totalUsageMi,
		LimitMi:    podInfo.MemoryLimitMi,
		Percentage: percentage,
	}
}

// computePodCPURow computes CPU usage for a pod.
func (c *Collector) computePodCPURow(pm metrics.PodMetrics, podInfo *metrics.PodSpecInfo) *metrics.Row {
	if !podInfo.HasCPULimit() {
		return nil
	}

	var totalUsageMc int64
	for _, container := range pm.Containers {
		if !podInfo.ContainerHasCPULimit(container.Name) {
			continue
		}
		if qty, ok := container.Usage[corev1.ResourceCPU]; ok {
			totalUsageMc += qty.MilliValue()
		}
	}

	percentage := (float64(totalUsageMc) / float64(podInfo.CPULimitMc)) * 100
	return &metrics.Row{
		Namespace:  pm.Namespace,
		Name:       pm.Name,
		UsageMc:    totalUsageMc,
		LimitMc:    podInfo.CPULimitMc,
		Percentage: percentage,
	}
}

// computeContainerRows computes usage rows for container-level analysis.
func (c *Collector) computeContainerRows(pm metrics.PodMetrics, podInfo *metrics.PodSpecInfo, resource config.ResourceKind) []metrics.Row {
	var rows []metrics.Row

	for _, container := range pm.Containers {
		containerName := pm.Name + ":" + container.Name

		switch resource {
		case config.ResourceMemory:
			if row := c.computeContainerMemoryRow(pm.Namespace, containerName, container, podInfo); row != nil {
				rows = append(rows, *row)
			}
		case config.ResourceCPU:
			if row := c.computeContainerCPURow(pm.Namespace, containerName, container, podInfo); row != nil {
				rows = append(rows, *row)
			}
		}
	}

	return rows
}

// computeContainerMemoryRow computes memory usage for a container.
func (c *Collector) computeContainerMemoryRow(namespace, containerName string, container metrics.ContainerMetrics, podInfo *metrics.PodSpecInfo) *metrics.Row {
	limitMi, hasLimit := podInfo.ContainerMemoryLimits[container.Name]
	if !hasLimit || limitMi <= 0 {
		return nil
	}

	var usageMi float64
	if qty, ok := container.Usage[corev1.ResourceMemory]; ok {
		usageMi = float64(qty.Value()) / (1024 * 1024)
	}

	percentage := (usageMi / limitMi) * 100
	return &metrics.Row{
		Namespace:  namespace,
		Name:       containerName,
		UsageMi:    usageMi,
		LimitMi:    limitMi,
		Percentage: percentage,
	}
}

// computeContainerCPURow computes CPU usage for a container.
func (c *Collector) computeContainerCPURow(namespace, containerName string, container metrics.ContainerMetrics, podInfo *metrics.PodSpecInfo) *metrics.Row {
	limitMc, hasLimit := podInfo.ContainerCPULimits[container.Name]
	if !hasLimit || limitMc <= 0 {
		return nil
	}

	var usageMc int64
	if qty, ok := container.Usage[corev1.ResourceCPU]; ok {
		usageMc = qty.MilliValue()
	}

	percentage := (float64(usageMc) / float64(limitMc)) * 100
	return &metrics.Row{
		Namespace:  namespace,
		Name:       containerName,
		UsageMc:    usageMc,
		LimitMc:    limitMc,
		Percentage: percentage,
	}
}
