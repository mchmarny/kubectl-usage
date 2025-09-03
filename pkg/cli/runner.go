package cli

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/mchmarny/kusage/pkg/analyzer"
	"github.com/mchmarny/kusage/pkg/collector"
	"github.com/mchmarny/kusage/pkg/k8s"
	"github.com/mchmarny/kusage/pkg/observability"
	"github.com/mchmarny/kusage/pkg/output"
)

func Run() error {
	parser := NewParser()
	opts, err := parser.Parse(os.Args)
	if err != nil {
		return err
	}

	if opts == nil {
		return nil
	}

	// Initialize metrics if enabled
	var metrics *observability.Metrics
	if opts.EnableMetrics {
		metrics = observability.NewMetrics()
		defer func() {
			// For metrics output, we want to ensure it's always visible
			// So we'll use Warn level instead of Info level
			summary := metrics.GetSummary()
			slog.Warn("performance metrics summary",
				"api_calls_total", summary.APICallsTotal,
				"api_calls_successful", summary.APICallsSuccessful,
				"api_calls_failed", summary.APICallsFailed,
				"avg_api_call_duration_ms", summary.AvgAPICallDuration.Milliseconds(),
				"pods_processed", summary.PodsProcessed,
				"metrics_processed", summary.MetricsProcessed,
				"results_generated", summary.ResultsGenerated,
				"peak_memory_mb", summary.PeakMemoryUsageMB,
				"current_memory_mb", summary.CurrentMemoryMB,
				"collection_duration_ms", summary.CollectionDuration.Milliseconds(),
				"analysis_duration_ms", summary.AnalysisDuration.Milliseconds(),
				"total_duration_ms", summary.TotalDuration.Milliseconds(),
				"error_count", summary.ErrorCount)
		}()
	}

	clientManager, err := k8s.NewClientManager()
	if err != nil {
		if metrics != nil {
			metrics.RecordError(err, "kubernetes client initialization")
		}
		return err
	}

	// app components using dependency injection
	dataCollector := collector.New(clientManager.CoreClient(), clientManager.MetricsClient())
	dataAnalyzer := analyzer.New()
	outputFormatter := output.New()
	defer outputFormatter.Close()

	// Create context with timeout for all Kubernetes operations
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Record collection start time
	if metrics != nil {
		metrics.UpdateMemoryUsage()
	}

	// Collect data from Kubernetes APIs
	collectionStart := time.Now()
	rows, err := dataCollector.Collect(ctx, *opts)
	if err != nil {
		if metrics != nil {
			metrics.RecordError(err, "data collection")
		}
		return err
	}

	// Record collection completion
	if metrics != nil {
		metrics.SetCollectionDuration(time.Since(collectionStart))
		metrics.UpdateMemoryUsage()
	}

	// Analyze and sort the collected data
	analysisStart := time.Now()
	dataAnalyzer.Sort(rows, *opts)

	// Apply post-processing filters
	rows = dataAnalyzer.Filter(rows, *opts)

	// Record analysis completion
	if metrics != nil {
		metrics.SetAnalysisDuration(time.Since(analysisStart))
		metrics.ResultsGenerated = int64(len(rows))
	}

	// Format and output the results
	err = outputFormatter.PrintTable(rows, *opts)
	if err != nil && metrics != nil {
		metrics.RecordError(err, "output formatting")
	}

	return err
}
