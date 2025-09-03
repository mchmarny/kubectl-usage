package analyzer

import (
	"testing"

	"github.com/mchmarny/kubectl-usage/pkg/config"
	"github.com/mchmarny/kubectl-usage/pkg/metrics"
)

func TestAnalyzer_Sort(t *testing.T) {
	tests := []struct {
		name     string
		rows     []metrics.Row
		opts     config.Options
		expected []string // Expected order of row names
	}{
		{
			name: "sort by percentage descending",
			rows: []metrics.Row{
				{Name: "pod-a", Percentage: 50.0},
				{Name: "pod-b", Percentage: 90.0},
				{Name: "pod-c", Percentage: 30.0},
			},
			opts: config.Options{
				Sort:     config.SortByPercentage,
				Resource: config.ResourceMemory,
			},
			expected: []string{"pod-b", "pod-a", "pod-c"},
		},
		{
			name: "sort by memory usage descending",
			rows: []metrics.Row{
				{Name: "pod-a", UsageMi: 100.0},
				{Name: "pod-b", UsageMi: 200.0},
				{Name: "pod-c", UsageMi: 50.0},
			},
			opts: config.Options{
				Sort:     config.SortByUsage,
				Resource: config.ResourceMemory,
			},
			expected: []string{"pod-b", "pod-a", "pod-c"},
		},
		{
			name: "stable sort with secondary criteria",
			rows: []metrics.Row{
				{Namespace: "ns-b", Name: "pod-a", Percentage: 50.0},
				{Namespace: "ns-a", Name: "pod-b", Percentage: 50.0},
				{Namespace: "ns-a", Name: "pod-a", Percentage: 50.0},
			},
			opts: config.Options{
				Sort:     config.SortByPercentage,
				Resource: config.ResourceMemory,
			},
			// Should sort by namespace then name when percentages are equal
			expected: []string{"pod-a", "pod-b", "pod-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := New()

			// Make a copy to avoid modifying the test data
			rowsCopy := make([]metrics.Row, len(tt.rows))
			copy(rowsCopy, tt.rows)

			analyzer.Sort(rowsCopy, tt.opts)

			// Verify the order
			if len(rowsCopy) != len(tt.expected) {
				t.Fatalf("expected %d rows, got %d", len(tt.expected), len(rowsCopy))
			}

			for i, expectedName := range tt.expected {
				if rowsCopy[i].Name != expectedName {
					t.Errorf("position %d: expected %s, got %s", i, expectedName, rowsCopy[i].Name)
				}
			}
		})
	}
}

func TestAnalyzer_Filter(t *testing.T) {
	tests := []struct {
		name     string
		rows     []metrics.Row
		opts     config.Options
		expected int
	}{
		{
			name: "filter with TopN limit",
			rows: []metrics.Row{
				{Name: "pod-1"},
				{Name: "pod-2"},
				{Name: "pod-3"},
				{Name: "pod-4"},
				{Name: "pod-5"},
			},
			opts:     config.Options{TopN: 3},
			expected: 3,
		},
		{
			name: "no filter when TopN is 0",
			rows: []metrics.Row{
				{Name: "pod-1"},
				{Name: "pod-2"},
			},
			opts:     config.Options{TopN: 0},
			expected: 2,
		},
		{
			name: "no filter when TopN exceeds row count",
			rows: []metrics.Row{
				{Name: "pod-1"},
				{Name: "pod-2"},
			},
			opts:     config.Options{TopN: 10},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := New()
			result := analyzer.Filter(tt.rows, tt.opts)

			if len(result) != tt.expected {
				t.Errorf("expected %d rows, got %d", tt.expected, len(result))
			}
		})
	}
}

// BenchmarkSort measures the performance of the sorting algorithm
func BenchmarkSort(b *testing.B) {
	// Create a large dataset for benchmarking
	rows := make([]metrics.Row, 1000)
	for i := range rows {
		rows[i] = metrics.Row{
			Namespace:  "default",
			Name:       "pod-" + string(rune(i)),
			Percentage: float64(i % 100),
			UsageMi:    float64(i),
			LimitMi:    100.0,
		}
	}

	opts := config.Options{
		Sort:     config.SortByPercentage,
		Resource: config.ResourceMemory,
	}

	analyzer := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make a copy for each iteration to ensure consistent state
		rowsCopy := make([]metrics.Row, len(rows))
		copy(rowsCopy, rows)

		analyzer.Sort(rowsCopy, opts)
	}
}
