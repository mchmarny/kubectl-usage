// Package metrics provides data structures and utilities for Kubernetes resource metrics.
// This package defines the domain model for resource usage analysis, following
// domain-driven design principles common in distributed systems architecture.
package metrics

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodMetrics represents a simplified view of pod metrics for internal use.
// This type abstracts the upstream metrics API types and provides a stable
// internal representation that can evolve independently of the Kubernetes API.
type PodMetrics struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Timestamp         metav1.Time        `json:"timestamp"`
	Window            metav1.Duration    `json:"window"`
	Containers        []ContainerMetrics `json:"containers"`
}

// ContainerMetrics represents container-level resource usage.
// This type encapsulates the resource usage data for a single container,
// providing a clean abstraction over the underlying metrics API.
type ContainerMetrics struct {
	Name  string              `json:"name"`
	Usage corev1.ResourceList `json:"usage"`
}

// Row represents a single result row in the resource usage analysis.
// This type follows the data transfer object (DTO) pattern and contains
// all computed values needed for display and sorting.
type Row struct {
	// Namespace is the Kubernetes namespace of the resource
	Namespace string
	// Name is the resource name (pod name or "pod:container" for container mode)
	Name string
	// UsageMi is the memory usage in mebibytes (Mi)
	UsageMi float64
	// LimitMi is the memory limit in mebibytes (Mi)
	LimitMi float64
	// UsageMc is the CPU usage in millicores (mCPU)
	UsageMc int64
	// LimitMc is the CPU limit in millicores (mCPU)
	LimitMc int64
	// Percentage is the usage/limit ratio as a percentage
	Percentage float64
}

// PodSpecInfo contains computed resource limits and other metadata for a pod.
// This type serves as an optimized lookup structure that pre-computes resource
// limits to avoid repeated calculations during metrics processing.
type PodSpecInfo struct {
	// Pod is a reference to the original pod specification
	Pod *corev1.Pod
	// MemoryLimitMi is the total memory limit across all containers (Mi)
	MemoryLimitMi float64
	// CPULimitMc is the total CPU limit across all containers (millicores)
	CPULimitMc int64
	// ContainerMemoryLimits maps container names to their memory limits (Mi)
	ContainerMemoryLimits map[string]float64
	// ContainerCPULimits maps container names to their CPU limits (millicores)
	ContainerCPULimits map[string]int64
}

// NewPodSpecInfo creates a new PodSpecInfo from a pod specification.
// This constructor pre-computes all resource limits for efficient lookup
// during metrics processing, following the optimization patterns common
// in high-performance distributed systems.
func NewPodSpecInfo(pod *corev1.Pod) *PodSpecInfo {
	info := &PodSpecInfo{
		Pod:                   pod,
		ContainerMemoryLimits: make(map[string]float64, len(pod.Spec.Containers)),
		ContainerCPULimits:    make(map[string]int64, len(pod.Spec.Containers)),
	}

	// Pre-compute resource limits for all containers
	for _, container := range pod.Spec.Containers {
		// Memory limits
		if limit, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			memoryMi := float64(limit.Value()) / (1024 * 1024) // Convert bytes to Mi
			info.MemoryLimitMi += memoryMi
			info.ContainerMemoryLimits[container.Name] = memoryMi
		}

		// CPU limits
		if limit, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuMc := limit.MilliValue() // Already in millicores
			info.CPULimitMc += cpuMc
			info.ContainerCPULimits[container.Name] = cpuMc
		}
	}

	return info
}

// HasMemoryLimit returns true if the pod has memory limits configured.
func (p *PodSpecInfo) HasMemoryLimit() bool {
	return p.MemoryLimitMi > 0
}

// HasCPULimit returns true if the pod has CPU limits configured.
func (p *PodSpecInfo) HasCPULimit() bool {
	return p.CPULimitMc > 0
}

// ContainerHasMemoryLimit returns true if the specified container has a memory limit.
func (p *PodSpecInfo) ContainerHasMemoryLimit(containerName string) bool {
	limit, exists := p.ContainerMemoryLimits[containerName]
	return exists && limit > 0
}

// ContainerHasCPULimit returns true if the specified container has a CPU limit.
func (p *PodSpecInfo) ContainerHasCPULimit(containerName string) bool {
	limit, exists := p.ContainerCPULimits[containerName]
	return exists && limit > 0
}
