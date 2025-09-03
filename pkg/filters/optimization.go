// Package filters provides efficient filtering for Kubernetes resources
package filters

import (
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

// OptimizedListOptions creates optimized list options for large-scale queries
// This reduces the amount of data transferred and processed by filtering at the API server level
func OptimizedListOptions(labelSelector string, pageSize int64, continueToken string) metav1.ListOptions {
	// Field selector to exclude terminated pods and reduce data volume
	fieldSelector := fields.Set{
		"status.phase": string(corev1.PodRunning),
	}.AsSelector().String()

	// Also include pending pods as they might have resource limits
	fieldSelector += "," + fields.Set{
		"status.phase": string(corev1.PodPending),
	}.AsSelector().String()

	return metav1.ListOptions{
		LabelSelector:   labelSelector,
		FieldSelector:   fieldSelector,
		Limit:           pageSize,
		Continue:        continueToken,
		ResourceVersion: "0", // Allow serving from cache for better performance
	}
}

// PodFilter provides efficient pod filtering logic
type PodFilter struct {
	excludeNamespaces *regexp.Regexp
	excludeLabels     *regexp.Regexp
	includePhases     map[corev1.PodPhase]bool
	minResourceLimits bool
}

// NewPodFilter creates a new pod filter with the specified criteria
func NewPodFilter(excludeNamespaces *regexp.Regexp, excludeLabels *regexp.Regexp, minResourceLimits bool) *PodFilter {
	return &PodFilter{
		excludeNamespaces: excludeNamespaces,
		excludeLabels:     excludeLabels,
		minResourceLimits: minResourceLimits,
		includePhases: map[corev1.PodPhase]bool{
			corev1.PodRunning: true,
			corev1.PodPending: true,
		},
	}
}

// ShouldIncludePod determines if a pod should be included in analysis
func (f *PodFilter) ShouldIncludePod(pod *corev1.Pod) bool {
	// Check namespace exclusion
	if f.excludeNamespaces != nil && f.excludeNamespaces.MatchString(pod.Namespace) {
		return false
	}

	// Check label exclusion
	if f.excludeLabels != nil {
		labelString := formatLabels(pod.Labels)
		if f.excludeLabels.MatchString(labelString) {
			return false
		}
	}

	// Check pod phase
	if !f.includePhases[pod.Status.Phase] {
		return false
	}

	// Check if pod has resource limits (if required)
	if f.minResourceLimits && !f.hasResourceLimits(pod) {
		return false
	}

	// Exclude system pods that typically don't have limits
	if f.isSystemPod(pod) {
		return false
	}

	return true
}

// hasResourceLimits checks if pod has any resource limits configured
func (f *PodFilter) hasResourceLimits(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits != nil {
			if _, hasMemory := container.Resources.Limits[corev1.ResourceMemory]; hasMemory {
				return true
			}
			if _, hasCPU := container.Resources.Limits[corev1.ResourceCPU]; hasCPU {
				return true
			}
		}
	}
	return false
}

// isSystemPod identifies system pods that typically don't have resource limits
func (f *PodFilter) isSystemPod(pod *corev1.Pod) bool {
	// System namespaces
	systemNamespaces := map[string]bool{
		"kube-system":        true,
		"kube-public":        true,
		"kube-node-lease":    true,
		"local-path-storage": true,
	}

	if systemNamespaces[pod.Namespace] {
		return true
	}

	// Check for system pod patterns
	systemPrefixes := []string{
		"kube-",
		"etcd-",
		"coredns-",
		"metrics-server-",
		"local-path-provisioner-",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(pod.Name, prefix) {
			return true
		}
	}

	return false
}

// MetricsFilter provides efficient metrics filtering
type MetricsFilter struct {
	podFilter *PodFilter
}

// NewMetricsFilter creates a new metrics filter
func NewMetricsFilter(podFilter *PodFilter) *MetricsFilter {
	return &MetricsFilter{
		podFilter: podFilter,
	}
}

// ShouldIncludeMetrics determines if pod metrics should be included
func (f *MetricsFilter) ShouldIncludeMetrics(namespace, name string) bool {
	// Create a minimal pod object for filtering
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning, // Assume running since we have metrics
		},
	}

	return f.podFilter.ShouldIncludePod(pod)
}

// NamespaceOptimizer provides namespace-aware optimization strategies
type NamespaceOptimizer struct {
	knownLargeNamespaces    map[string]bool
	smallNamespaceThreshold int
}

// NewNamespaceOptimizer creates a namespace optimizer
func NewNamespaceOptimizer() *NamespaceOptimizer {
	return &NamespaceOptimizer{
		knownLargeNamespaces: map[string]bool{
			"default":     true,
			"production":  true,
			"staging":     true,
			"development": true,
		},
		smallNamespaceThreshold: 50, // Consider namespaces with <50 pods as small
	}
}

// ShouldUsePagination determines if pagination should be used for a namespace
func (o *NamespaceOptimizer) ShouldUsePagination(namespace string, estimatedPodCount int) bool {
	// Always paginate for known large namespaces
	if o.knownLargeNamespaces[namespace] {
		return true
	}

	// Paginate if estimated pod count exceeds threshold
	return estimatedPodCount > o.smallNamespaceThreshold
}

// GetOptimalPageSize returns the optimal page size for a namespace
func (o *NamespaceOptimizer) GetOptimalPageSize(_ string, estimatedPodCount int) int64 {
	if estimatedPodCount < 100 {
		return 50 // Small pages for small namespaces
	}
	if estimatedPodCount < 1000 {
		return 200 // Medium pages for medium namespaces
	}
	return 500 // Large pages for large namespaces
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
