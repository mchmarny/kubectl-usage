// Package benchmark provides performance testing for large-scale cluster operations
package benchmark

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mchmarny/kusage/pkg/metrics"
	"github.com/mchmarny/kusage/pkg/observability"
)

// BenchmarkConfig defines parameters for performance benchmarks
type BenchmarkConfig struct {
	PodCount         int
	ContainersPerPod int
	NamespaceCount   int
	PageSize         int64
	MaxConcurrency   int
}

// LargeClusterConfig provides configuration for testing with large cluster simulation
func LargeClusterConfig() BenchmarkConfig {
	return BenchmarkConfig{
		PodCount:         20000, // Simulate 20k pods (1000 nodes * 20 pods)
		ContainersPerPod: 2,     // Average 2 containers per pod
		NamespaceCount:   50,    // Distributed across 50 namespaces
		PageSize:         500,   // Large page size for efficiency
		MaxConcurrency:   20,    // High concurrency for large clusters
	}
}

// MediumClusterConfig provides configuration for medium cluster testing
func MediumClusterConfig() BenchmarkConfig {
	return BenchmarkConfig{
		PodCount:         5000,
		ContainersPerPod: 2,
		NamespaceCount:   20,
		PageSize:         200,
		MaxConcurrency:   10,
	}
}

// SmallClusterConfig provides configuration for small cluster testing
func SmallClusterConfig() BenchmarkConfig {
	return BenchmarkConfig{
		PodCount:         500,
		ContainersPerPod: 1,
		NamespaceCount:   5,
		PageSize:         50,
		MaxConcurrency:   5,
	}
}

// GenerateMockPods creates mock pod data for benchmarking
func GenerateMockPods(config BenchmarkConfig) []corev1.Pod {
	pods := make([]corev1.Pod, 0, config.PodCount)
	podsPerNamespace := config.PodCount / config.NamespaceCount

	for nsIndex := 0; nsIndex < config.NamespaceCount; nsIndex++ {
		namespace := fmt.Sprintf("namespace-%d", nsIndex)

		for podIndex := 0; podIndex < podsPerNamespace; podIndex++ {
			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("pod-%d-%d", nsIndex, podIndex),
					Namespace: namespace,
					Labels: map[string]string{
						"app":     fmt.Sprintf("app-%d", podIndex%10),
						"version": "v1.0",
					},
				},
				Spec: corev1.PodSpec{
					Containers: generateMockContainers(config.ContainersPerPod),
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			pods = append(pods, pod)
		}
	}

	return pods
}

// generateMockContainers creates mock container specs with resource limits
func generateMockContainers(count int) []corev1.Container {
	containers := make([]corev1.Container, count)

	for i := 0; i < count; i++ {
		containers[i] = corev1.Container{
			Name:  fmt.Sprintf("container-%d", i),
			Image: "nginx:1.21",
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
					corev1.ResourceCPU:    resource.MustParse("500m"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
					corev1.ResourceCPU:    resource.MustParse("250m"),
				},
			},
		}
	}

	return containers
}

// GenerateMockMetrics creates mock metrics data for benchmarking
func GenerateMockMetrics(pods []corev1.Pod) []metrics.PodMetrics {
	podMetrics := make([]metrics.PodMetrics, 0, len(pods))

	for _, pod := range pods {
		pm := metrics.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
			Timestamp:  metav1.NewTime(time.Now()),
			Window:     metav1.Duration{Duration: 30 * time.Second},
			Containers: make([]metrics.ContainerMetrics, 0, len(pod.Spec.Containers)),
		}

		for _, container := range pod.Spec.Containers {
			cm := metrics.ContainerMetrics{
				Name: container.Name,
				Usage: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"), // 50% of limit
					corev1.ResourceCPU:    resource.MustParse("250m"),  // 50% of limit
				},
			}
			pm.Containers = append(pm.Containers, cm)
		}

		podMetrics = append(podMetrics, pm)
	}

	return podMetrics
}

// BenchmarkMemoryUsageSmall measures memory usage for small clusters
func BenchmarkMemoryUsageSmall(b *testing.B) {
	benchmarkMemoryUsage(b, SmallClusterConfig())
}

// BenchmarkMemoryUsageMedium measures memory usage for medium clusters
func BenchmarkMemoryUsageMedium(b *testing.B) {
	benchmarkMemoryUsage(b, MediumClusterConfig())
}

// BenchmarkMemoryUsageLarge measures memory usage for large clusters
func BenchmarkMemoryUsageLarge(b *testing.B) {
	benchmarkMemoryUsage(b, LargeClusterConfig())
}

// benchmarkMemoryUsage measures memory usage for different data sizes
func benchmarkMemoryUsage(b *testing.B, config BenchmarkConfig) {
	b.ReportAllocs()

	// Generate test data
	pods := GenerateMockPods(config)
	podMetrics := GenerateMockMetrics(pods)

	// Track memory before processing
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate processing
		processed := 0
		for _, pod := range pods {
			// Create pod spec info (simulates indexing)
			_ = metrics.NewPodSpecInfo(&pod)
			processed++
		}

		for _, pm := range podMetrics {
			// Simulate metrics processing
			_ = pm.Containers
			processed++
		}
	}

	b.StopTimer()

	// Track memory after processing
	var memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)

	// Report memory usage
	memoryUsedMB := (memStatsAfter.Alloc - memStatsBefore.Alloc) / 1024 / 1024
	b.ReportMetric(float64(memoryUsedMB), "MB")
	b.ReportMetric(float64(len(pods)), "pods")
	b.ReportMetric(float64(len(podMetrics)), "metrics")
}

// BenchmarkProcessingThroughputSmall measures processing throughput for small clusters
func BenchmarkProcessingThroughputSmall(b *testing.B) {
	benchmarkProcessingThroughput(b, SmallClusterConfig())
}

// BenchmarkProcessingThroughputMedium measures processing throughput for medium clusters
func BenchmarkProcessingThroughputMedium(b *testing.B) {
	benchmarkProcessingThroughput(b, MediumClusterConfig())
}

// BenchmarkProcessingThroughputLarge measures processing throughput for large clusters
func BenchmarkProcessingThroughputLarge(b *testing.B) {
	benchmarkProcessingThroughput(b, LargeClusterConfig())
}

// benchmarkProcessingThroughput measures processing throughput
func benchmarkProcessingThroughput(b *testing.B, config BenchmarkConfig) {
	pods := GenerateMockPods(config)
	podMetrics := GenerateMockMetrics(pods)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Build pod index
		podIndex := make(map[string]*metrics.PodSpecInfo, len(pods))
		for j := range pods {
			pod := &pods[j]
			key := pod.Namespace + "/" + pod.Name
			podIndex[key] = metrics.NewPodSpecInfo(pod)
		}

		// Process metrics
		results := make([]metrics.Row, 0, len(podMetrics))
		for _, pm := range podMetrics {
			key := pm.Namespace + "/" + pm.Name
			if podInfo, exists := podIndex[key]; exists {
				// Simulate pod-level computation
				if podInfo.HasMemoryLimit() {
					var totalUsageMi float64
					for _, container := range pm.Containers {
						if qty, ok := container.Usage[corev1.ResourceMemory]; ok {
							totalUsageMi += float64(qty.Value()) / (1024 * 1024)
						}
					}

					percentage := (totalUsageMi / podInfo.MemoryLimitMi) * 100
					row := metrics.Row{
						Namespace:  pm.Namespace,
						Name:       pm.Name,
						UsageMi:    totalUsageMi,
						LimitMi:    podInfo.MemoryLimitMi,
						Percentage: percentage,
					}
					results = append(results, row)
				}
			}
		}

		// Use results to prevent optimization
		_ = len(results)
	}

	// Report throughput metrics
	totalItems := len(pods) + len(podMetrics)
	b.ReportMetric(float64(totalItems), "items/op")
}

// BenchmarkPaginationSmall measures pagination performance for small clusters
func BenchmarkPaginationSmall(b *testing.B) {
	benchmarkPagination(b, SmallClusterConfig())
}

// BenchmarkPaginationMedium measures pagination performance for medium clusters
func BenchmarkPaginationMedium(b *testing.B) {
	benchmarkPagination(b, MediumClusterConfig())
}

// BenchmarkPaginationLarge measures pagination performance for large clusters
func BenchmarkPaginationLarge(b *testing.B) {
	benchmarkPagination(b, LargeClusterConfig())
}

// benchmarkPagination measures pagination performance
func benchmarkPagination(b *testing.B, config BenchmarkConfig) {
	pods := GenerateMockPods(config)
	pageSize := config.PageSize

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate paginated processing
		pageCount := 0
		for offset := 0; offset < len(pods); offset += int(pageSize) {
			end := offset + int(pageSize)
			if end > len(pods) {
				end = len(pods)
			}

			// Process page
			page := pods[offset:end]
			for j := range page {
				// Simulate processing each pod in the page
				_ = metrics.NewPodSpecInfo(&page[j])
			}
			pageCount++
		}

		b.ReportMetric(float64(pageCount), "pages")
	}
}

// BenchmarkPerformanceSmall runs comprehensive performance tests for small clusters
func BenchmarkPerformanceSmall(b *testing.B) {
	benchmarkPerformance(b, SmallClusterConfig())
}

// BenchmarkPerformanceMedium runs comprehensive performance tests for medium clusters
func BenchmarkPerformanceMedium(b *testing.B) {
	benchmarkPerformance(b, MediumClusterConfig())
}

// BenchmarkPerformanceLarge runs comprehensive performance tests for large clusters
func BenchmarkPerformanceLarge(b *testing.B) {
	benchmarkPerformance(b, LargeClusterConfig())
}

// benchmarkPerformance runs comprehensive performance tests
func benchmarkPerformance(b *testing.B, config BenchmarkConfig) {
	// Initialize metrics tracking
	perfMetrics := observability.NewMetrics()

	// Generate test data
	startTime := time.Now()
	pods := GenerateMockPods(config)
	podMetrics := GenerateMockMetrics(pods)
	dataGenTime := time.Since(startTime)

	// Test memory efficiency
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	initialMemMB := memStats.Alloc / 1024 / 1024

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate collection phase
		collectionStart := time.Now()
		perfMetrics.RecordProcessing(int64(len(pods)), int64(len(podMetrics)), 0)
		perfMetrics.SetCollectionDuration(time.Since(collectionStart))

		// Simulate analysis phase
		analysisStart := time.Now()

		// Build index
		podIndex := make(map[string]*metrics.PodSpecInfo, len(pods))
		for j := range pods {
			pod := &pods[j]
			key := pod.Namespace + "/" + pod.Name
			podIndex[key] = metrics.NewPodSpecInfo(pod)
		}

		// Process results
		resultCount := int64(0)
		for _, pm := range podMetrics {
			key := pm.Namespace + "/" + pm.Name
			if _, exists := podIndex[key]; exists {
				resultCount++
			}
		}

		perfMetrics.RecordProcessing(0, 0, resultCount)
		perfMetrics.SetAnalysisDuration(time.Since(analysisStart))

		// Update memory metrics
		perfMetrics.UpdateMemoryUsage()
	}

	b.StopTimer()

	// Finalize and report metrics
	perfMetrics.Finalize()
	summary := perfMetrics.GetSummary()

	runtime.ReadMemStats(&memStats)
	finalMemMB := memStats.Alloc / 1024 / 1024
	memoryGrowthMB := finalMemMB - initialMemMB

	// Report performance metrics
	b.ReportMetric(float64(summary.PodsProcessed), "pods_processed")
	b.ReportMetric(float64(summary.MetricsProcessed), "metrics_processed")
	b.ReportMetric(float64(summary.ResultsGenerated), "results_generated")
	b.ReportMetric(float64(summary.PeakMemoryUsageMB), "peak_memory_mb")
	b.ReportMetric(float64(memoryGrowthMB), "memory_growth_mb")
	b.ReportMetric(float64(summary.CollectionDuration.Milliseconds()), "collection_ms")
	b.ReportMetric(float64(summary.AnalysisDuration.Milliseconds()), "analysis_ms")
	b.ReportMetric(float64(dataGenTime.Milliseconds()), "data_gen_ms")

	b.Logf("Performance Summary: %d pods, %d metrics, peak memory: %dMB, total time: %v",
		len(pods), len(podMetrics), summary.PeakMemoryUsageMB, summary.TotalDuration)
}

// RunScalabilityTests runs tests across different cluster sizes
func RunScalabilityTests(b *testing.B) {
	configs := []struct {
		name   string
		config BenchmarkConfig
	}{
		{"Small", SmallClusterConfig()},
		{"Medium", MediumClusterConfig()},
		{"Large", LargeClusterConfig()},
	}

	for _, tc := range configs {
		b.Run(fmt.Sprintf("Memory_%s", tc.name), func(b *testing.B) {
			benchmarkMemoryUsage(b, tc.config)
		})

		b.Run(fmt.Sprintf("Throughput_%s", tc.name), func(b *testing.B) {
			benchmarkProcessingThroughput(b, tc.config)
		})

		b.Run(fmt.Sprintf("Pagination_%s", tc.name), func(b *testing.B) {
			benchmarkPagination(b, tc.config)
		})

		b.Run(fmt.Sprintf("Performance_%s", tc.name), func(b *testing.B) {
			benchmarkPerformance(b, tc.config)
		})
	}
}
