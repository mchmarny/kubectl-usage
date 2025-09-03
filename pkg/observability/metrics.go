// Package observability provides comprehensive monitoring and metrics for large-scale operations
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"sync"
	"time"
)

// Metrics tracks performance and resource usage metrics
type Metrics struct {
	// API call metrics
	APICallsTotal      int64
	APICallsSuccessful int64
	APICallsFailed     int64
	APICallDuration    time.Duration

	// Processing metrics
	PodsProcessed    int64
	MetricsProcessed int64
	ResultsGenerated int64

	// Memory metrics
	PeakMemoryUsageMB int64
	CurrentMemoryMB   int64

	// Timing metrics
	StartTime          time.Time
	CollectionDuration time.Duration
	AnalysisDuration   time.Duration
	TotalDuration      time.Duration

	// Error tracking
	Errors []string

	mutex sync.RWMutex
}

// NewMetrics creates a new metrics tracker
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}
}

// RecordAPICall records an API call attempt
func (m *Metrics) RecordAPICall(duration time.Duration, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.APICallsTotal++
	m.APICallDuration += duration

	if success {
		m.APICallsSuccessful++
	} else {
		m.APICallsFailed++
	}
}

// RecordProcessing records processing metrics
func (m *Metrics) RecordProcessing(pods, metrics, results int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.PodsProcessed += pods
	m.MetricsProcessed += metrics
	m.ResultsGenerated += results
}

// UpdateMemoryUsage updates memory usage metrics
func (m *Metrics) UpdateMemoryUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Safe conversion to avoid integer overflow
	// Convert to MB first, then to int64 to avoid overflow
	allocMB := memStats.Alloc / 1024 / 1024
	var currentMB int64
	if allocMB > math.MaxInt64 {
		currentMB = math.MaxInt64
	} else {
		currentMB = int64(allocMB) // #nosec G115 - safe after bounds check
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.CurrentMemoryMB = currentMB
	if currentMB > m.PeakMemoryUsageMB {
		m.PeakMemoryUsageMB = currentMB
	}
} // RecordError records an error with context
func (m *Metrics) RecordError(err error, context string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	errorMsg := fmt.Sprintf("%s: %v", context, err)
	m.Errors = append(m.Errors, errorMsg)
}

// SetCollectionDuration sets the collection phase duration
func (m *Metrics) SetCollectionDuration(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.CollectionDuration = duration
}

// SetAnalysisDuration sets the analysis phase duration
func (m *Metrics) SetAnalysisDuration(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.AnalysisDuration = duration
}

// Finalize calculates final metrics
func (m *Metrics) Finalize() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.TotalDuration = time.Since(m.StartTime)
}

// GetSummary returns a summary of metrics
func (m *Metrics) GetSummary() MetricsSummary {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var avgAPICallDuration time.Duration
	if m.APICallsTotal > 0 {
		avgAPICallDuration = m.APICallDuration / time.Duration(m.APICallsTotal)
	}

	return MetricsSummary{
		APICallsTotal:      m.APICallsTotal,
		APICallsSuccessful: m.APICallsSuccessful,
		APICallsFailed:     m.APICallsFailed,
		AvgAPICallDuration: avgAPICallDuration,
		PodsProcessed:      m.PodsProcessed,
		MetricsProcessed:   m.MetricsProcessed,
		ResultsGenerated:   m.ResultsGenerated,
		PeakMemoryUsageMB:  m.PeakMemoryUsageMB,
		CurrentMemoryMB:    m.CurrentMemoryMB,
		CollectionDuration: m.CollectionDuration,
		AnalysisDuration:   m.AnalysisDuration,
		TotalDuration:      m.TotalDuration,
		ErrorCount:         len(m.Errors),
		Errors:             make([]string, len(m.Errors)),
	}
}

// MetricsSummary provides a snapshot of metrics
type MetricsSummary struct {
	APICallsTotal      int64         `json:"api_calls_total"`
	APICallsSuccessful int64         `json:"api_calls_successful"`
	APICallsFailed     int64         `json:"api_calls_failed"`
	AvgAPICallDuration time.Duration `json:"avg_api_call_duration"`
	PodsProcessed      int64         `json:"pods_processed"`
	MetricsProcessed   int64         `json:"metrics_processed"`
	ResultsGenerated   int64         `json:"results_generated"`
	PeakMemoryUsageMB  int64         `json:"peak_memory_usage_mb"`
	CurrentMemoryMB    int64         `json:"current_memory_mb"`
	CollectionDuration time.Duration `json:"collection_duration"`
	AnalysisDuration   time.Duration `json:"analysis_duration"`
	TotalDuration      time.Duration `json:"total_duration"`
	ErrorCount         int           `json:"error_count"`
	Errors             []string      `json:"errors,omitempty"`
}

// LogSummary logs a comprehensive metrics summary
func (s MetricsSummary) LogSummary() {
	slog.Info("operation completed",
		"api_calls_total", s.APICallsTotal,
		"api_calls_successful", s.APICallsSuccessful,
		"api_calls_failed", s.APICallsFailed,
		"avg_api_call_duration_ms", s.AvgAPICallDuration.Milliseconds(),
		"pods_processed", s.PodsProcessed,
		"metrics_processed", s.MetricsProcessed,
		"results_generated", s.ResultsGenerated,
		"peak_memory_mb", s.PeakMemoryUsageMB,
		"current_memory_mb", s.CurrentMemoryMB,
		"collection_duration_ms", s.CollectionDuration.Milliseconds(),
		"analysis_duration_ms", s.AnalysisDuration.Milliseconds(),
		"total_duration_ms", s.TotalDuration.Milliseconds(),
		"error_count", s.ErrorCount)

	if s.ErrorCount > 0 {
		slog.Warn("errors encountered during operation",
			"error_count", s.ErrorCount,
			"errors", s.Errors)
	}
}

// PerformanceMonitor provides real-time performance monitoring
type PerformanceMonitor struct {
	metrics      *Metrics
	ticker       *time.Ticker
	stopChan     chan struct{}
	updateMemory bool
	mutex        sync.RWMutex
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(metrics *Metrics, updateInterval time.Duration) *PerformanceMonitor {
	return &PerformanceMonitor{
		metrics:      metrics,
		ticker:       time.NewTicker(updateInterval),
		stopChan:     make(chan struct{}),
		updateMemory: true,
	}
}

// Start begins monitoring performance metrics
func (pm *PerformanceMonitor) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-pm.ticker.C:
				if pm.updateMemory {
					pm.metrics.UpdateMemoryUsage()
				}
			case <-pm.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the performance monitor
func (pm *PerformanceMonitor) Stop() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.ticker != nil {
		pm.ticker.Stop()
	}

	select {
	case pm.stopChan <- struct{}{}:
	default:
	}
}

// ProgressTracker tracks progress for long-running operations
type ProgressTracker struct {
	totalItems     int64
	processedItems int64
	startTime      time.Time
	lastUpdate     time.Time
	mutex          sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(totalItems int64) *ProgressTracker {
	return &ProgressTracker{
		totalItems: totalItems,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
	}
}

// Update updates the progress with the number of items processed
func (pt *ProgressTracker) Update(processed int64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.processedItems += processed
	pt.lastUpdate = time.Now()
}

// GetProgress returns current progress information
func (pt *ProgressTracker) GetProgress() ProgressInfo {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	elapsed := time.Since(pt.startTime)
	var eta time.Duration
	var rate float64

	if pt.processedItems > 0 {
		rate = float64(pt.processedItems) / elapsed.Seconds()
		if rate > 0 {
			remaining := pt.totalItems - pt.processedItems
			eta = time.Duration(float64(remaining)/rate) * time.Second
		}
	}

	percentage := float64(pt.processedItems) / float64(pt.totalItems) * 100

	return ProgressInfo{
		Processed:  pt.processedItems,
		Total:      pt.totalItems,
		Percentage: percentage,
		Rate:       rate,
		Elapsed:    elapsed,
		ETA:        eta,
	}
}

// ProgressInfo contains progress information
type ProgressInfo struct {
	Processed  int64         `json:"processed"`
	Total      int64         `json:"total"`
	Percentage float64       `json:"percentage"`
	Rate       float64       `json:"rate_per_second"`
	Elapsed    time.Duration `json:"elapsed"`
	ETA        time.Duration `json:"eta"`
}

// LogProgress logs current progress
func (pi ProgressInfo) LogProgress() {
	slog.Info("progress update",
		"processed", pi.Processed,
		"total", pi.Total,
		"percentage", fmt.Sprintf("%.1f%%", pi.Percentage),
		"rate_per_second", fmt.Sprintf("%.1f", pi.Rate),
		"elapsed_ms", pi.Elapsed.Milliseconds(),
		"eta_ms", pi.ETA.Milliseconds())
}
