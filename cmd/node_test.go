package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/veditoid/kubectl-rl-top/pkg"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)


func TestCombineNodeMetricsAndResources(t *testing.T) {
	tests := []struct {
		name         string
		metrics      []pkg.NodeMetrics
		resources    map[string]*pkg.NodeAggregatedResources
		nodes        map[string]*corev1.Node
		showCapacity bool
		expected     int
	}{
		{
			name: "combine metrics and resources",
			metrics: []pkg.NodeMetrics{
				{
					Name:   "node1",
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			resources: map[string]*pkg.NodeAggregatedResources{
				"node1": {
					NodeName:      "node1",
					CPURequest:    resource.MustParse("200m"),
					CPULimit:      resource.MustParse("400m"),
					MemoryRequest: resource.MustParse("256Mi"),
					MemoryLimit:   resource.MustParse("512Mi"),
				},
			},
			nodes: map[string]*corev1.Node{
				"node1": {
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
					},
				},
			},
			showCapacity: false,
			expected:    1,
		},
		{
			name: "node without resources",
			metrics: []pkg.NodeMetrics{
				{
					Name:   "node1",
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			resources: map[string]*pkg.NodeAggregatedResources{},
			nodes: map[string]*corev1.Node{
				"node1": {
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
					},
				},
			},
			showCapacity: false,
			expected:    1,
		},
		{
			name: "node without node info",
			metrics: []pkg.NodeMetrics{
				{
					Name:   "node1",
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			resources: map[string]*pkg.NodeAggregatedResources{
				"node1": {
					NodeName:   "node1",
					CPURequest: resource.MustParse("200m"),
				},
			},
			nodes:        map[string]*corev1.Node{},
			showCapacity: false,
			expected:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineNodeMetricsAndResources(tt.metrics, tt.resources, tt.nodes, tt.showCapacity)
			if len(result) != tt.expected {
				t.Errorf("combineNodeMetricsAndResources() returned %d nodes, want %d", len(result), tt.expected)
			}

			if len(result) > 0 {
				if result[0].Name == "" {
					t.Errorf("combineNodeMetricsAndResources() node name is empty")
				}
				if result[0].CPUUsage == "" {
					t.Errorf("combineNodeMetricsAndResources() CPU usage is empty")
				}
				if result[0].MemoryUsage == "" {
					t.Errorf("combineNodeMetricsAndResources() memory usage is empty")
				}
			}
		})
	}
}

func TestSortNodeData(t *testing.T) {
	data := []CombinedNodeData{
		{Name: "node3", CPUUsage: "300m", MemoryUsage: "3Gi"},
		{Name: "node1", CPUUsage: "100m", MemoryUsage: "1Gi"},
		{Name: "node2", CPUUsage: "200m", MemoryUsage: "2Gi"},
	}

	tests := []struct {
		name     string
		sortBy   string
		expected string // First node name after sorting
	}{
		{
			name:     "sort by CPU descending",
			sortBy:   "cpu",
			expected: "node3",
		},
		{
			name:     "sort by memory descending",
			sortBy:   "memory",
			expected: "node3",
		},
		{
			name:     "sort by name (default)",
			sortBy:   "",
			expected: "node1",
		},
		{
			name:     "sort by invalid field",
			sortBy:   "invalid",
			expected: "node1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testData := make([]CombinedNodeData, len(data))
			copy(testData, data)
			sortNodeData(testData, tt.sortBy)

			if len(testData) == 0 {
				t.Errorf("sortNodeData() returned empty slice")
				return
			}

			if testData[0].Name != tt.expected {
				t.Errorf("sortNodeData() first node = %v, want %v", testData[0].Name, tt.expected)
			}
		})
	}
}

func TestPrintNodeTable(t *testing.T) {
	data := []CombinedNodeData{
		{
			Name:          "node1",
			CPUUsage:      "100m",
			CPUPercent:    "2%",
			CPURequest:    "200m",
			CPULimit:      "400m",
			MemoryUsage:   "128Mi",
			MemoryPercent: "1%",
			MemoryRequest: "256Mi",
			MemoryLimit:   "512Mi",
		},
	}

	tests := []struct {
		name      string
		noHeaders bool
		checkFunc func(output string) bool
	}{
		{
			name:      "with headers",
			noHeaders: false,
			checkFunc: func(output string) bool {
				return strings.Contains(output, "NAME") && strings.Contains(output, "CPU(cores)")
			},
		},
		{
			name:      "without headers",
			noHeaders: true,
			checkFunc: func(output string) bool {
				return !strings.Contains(output, "NAME") && strings.Contains(output, "node1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printNodeTable(data, tt.noHeaders)

			w.Close()
			os.Stdout = oldStdout

			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			if !tt.checkFunc(output) {
				t.Errorf("printNodeTable() output doesn't match expected format. Output: %s", output)
			}
		})
	}
}

// Note: TestRunNode is skipped here as it requires complex mocking of metricsclientset.Interface
// and CheckMetricsAPIAvailable. It will be tested in integration tests instead.

