package pkg

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCalculateNodePercentages(t *testing.T) {
	tests := []struct {
		name              string
		node              *corev1.Node
		cpuUsageMilli     int64
		memoryUsageBytes  int64
		showCapacity      bool
		expectedCPU       string
		expectedMemory    string
	}{
		{
			name: "allocatable percentages",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			cpuUsageMilli:    2000, // 2 cores
			memoryUsageBytes: 4 * 1024 * 1024 * 1024, // 4Gi
			showCapacity:      false,
			expectedCPU:       "50%",
			expectedMemory:    "50%",
		},
		{
			name: "capacity percentages",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			cpuUsageMilli:    2000, // 2 cores
			memoryUsageBytes: 4 * 1024 * 1024 * 1024, // 4Gi
			showCapacity:      true,
			expectedCPU:       "50%",
			expectedMemory:    "50%",
		},
		{
			name: "zero usage",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			cpuUsageMilli:    0,
			memoryUsageBytes: 0,
			showCapacity:      false,
			expectedCPU:       "0%",
			expectedMemory:    "0%",
		},
		{
			name: "missing allocatable",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
					Capacity:    corev1.ResourceList{},
				},
			},
			cpuUsageMilli:    1000,
			memoryUsageBytes: 1024 * 1024 * 1024,
			showCapacity:      false,
			expectedCPU:       "-",
			expectedMemory:    "-",
		},
		{
			name: "partial usage",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			cpuUsageMilli:    500, // 0.5 cores = 12.5%
			memoryUsageBytes: 1024 * 1024 * 1024, // 1Gi = 12.5%
			showCapacity:      false,
			expectedCPU:       "12%", // rounded
			expectedMemory:    "12%", // rounded
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuPercent, memoryPercent := CalculateNodePercentages(tt.node, tt.cpuUsageMilli, tt.memoryUsageBytes, tt.showCapacity)
			if cpuPercent != tt.expectedCPU {
				t.Errorf("CalculateNodePercentages() cpuPercent = %v, want %v", cpuPercent, tt.expectedCPU)
			}
			if memoryPercent != tt.expectedMemory {
				t.Errorf("CalculateNodePercentages() memoryPercent = %v, want %v", memoryPercent, tt.expectedMemory)
			}
		})
	}
}

func TestAggregatePodResourcesByNode(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		pods     []corev1.Pod
		expected map[string]*NodeAggregatedResources
	}{
		{
			name: "single pod with requests and limits",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
						Containers: []corev1.Container{
							{
								Name: "container1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]*NodeAggregatedResources{
				"node1": {
					NodeName:      "node1",
					CPURequest:    resource.MustParse("100m"),
					CPULimit:      resource.MustParse("200m"),
					MemoryRequest: resource.MustParse("128Mi"),
					MemoryLimit:   resource.MustParse("256Mi"),
				},
			},
		},
		{
			name: "multiple pods on same node",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
						Containers: []corev1.Container{
							{
								Name: "container1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
						Containers: []corev1.Container{
							{
								Name: "container2",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("400m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]*NodeAggregatedResources{
				"node1": {
					NodeName:      "node1",
					CPURequest:    resource.MustParse("300m"),
					CPULimit:      resource.MustParse("600m"),
					MemoryRequest: resource.MustParse("384Mi"),
					MemoryLimit:   resource.MustParse("768Mi"),
				},
			},
		},
		{
			name: "pods on different nodes",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
						Containers: []corev1.Container{
							{
								Name: "container1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("100m"),
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node2",
						Containers: []corev1.Container{
							{
								Name: "container2",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("200m"),
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]*NodeAggregatedResources{
				"node1": {
					NodeName:   "node1",
					CPURequest: resource.MustParse("100m"),
				},
				"node2": {
					NodeName:   "node2",
					CPURequest: resource.MustParse("200m"),
				},
			},
		},
		{
			name: "pod without node assignment",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "", // Not scheduled
						Containers: []corev1.Container{
							{
								Name: "container1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("100m"),
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]*NodeAggregatedResources{},
		},
		{
			name: "pod with init container",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
						InitContainers: []corev1.Container{
							{
								Name: "init1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name: "container1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]*NodeAggregatedResources{
				"node1": {
					NodeName:      "node1",
					CPURequest:    resource.MustParse("500m"), // Max of init and regular
					MemoryRequest: resource.MustParse("512Mi"), // Max of init and regular
				},
			},
		},
		{
			name:     "empty pod list",
			pods:     []corev1.Pod{},
			expected: map[string]*NodeAggregatedResources{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			for _, pod := range tt.pods {
				_, err := clientset.CoreV1().Pods(pod.Namespace).Create(ctx, &pod, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create pod: %v", err)
				}
			}

			result, err := AggregatePodResourcesByNode(ctx, clientset)
			if err != nil {
				t.Fatalf("AggregatePodResourcesByNode() error = %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("AggregatePodResourcesByNode() returned %d nodes, want %d", len(result), len(tt.expected))
			}

			for nodeName, expected := range tt.expected {
				actual, ok := result[nodeName]
				if !ok {
					t.Errorf("AggregatePodResourcesByNode() missing node %s", nodeName)
					continue
				}

				if actual.NodeName != expected.NodeName {
					t.Errorf("AggregatePodResourcesByNode() node %s name = %v, want %v", nodeName, actual.NodeName, expected.NodeName)
				}

				if !actual.CPURequest.Equal(expected.CPURequest) {
					t.Errorf("AggregatePodResourcesByNode() node %s CPURequest = %v, want %v", nodeName, actual.CPURequest, expected.CPURequest)
				}

				if !actual.CPULimit.Equal(expected.CPULimit) {
					t.Errorf("AggregatePodResourcesByNode() node %s CPULimit = %v, want %v", nodeName, actual.CPULimit, expected.CPULimit)
				}

				if !actual.MemoryRequest.Equal(expected.MemoryRequest) {
					t.Errorf("AggregatePodResourcesByNode() node %s MemoryRequest = %v, want %v", nodeName, actual.MemoryRequest, expected.MemoryRequest)
				}

				if !actual.MemoryLimit.Equal(expected.MemoryLimit) {
					t.Errorf("AggregatePodResourcesByNode() node %s MemoryLimit = %v, want %v", nodeName, actual.MemoryLimit, expected.MemoryLimit)
				}
			}
		})
	}
}

func TestGetNodeResources(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		nodes        []corev1.Node
		labelSelector string
		nodeNames    []string
		showCapacity bool
		expected     int
	}{
		{
			name: "get all nodes",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						},
					},
				},
			},
			expected: 2,
		},
		{
			name: "filter by node name",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						},
					},
				},
			},
			nodeNames: []string{"node1"},
			expected:  1,
		},
		{
			name: "filter by label selector",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "true",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
						Labels: map[string]string{
							"node-role.kubernetes.io/master": "true",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						},
					},
				},
			},
			labelSelector: "node-role.kubernetes.io/worker=true",
			expected:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			for _, node := range tt.nodes {
				_, err := clientset.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create node: %v", err)
				}
			}

			result, err := GetNodeResources(ctx, clientset, tt.labelSelector, tt.nodeNames, tt.showCapacity)
			if err != nil {
				t.Fatalf("GetNodeResources() error = %v", err)
			}

			if len(result) != tt.expected {
				t.Errorf("GetNodeResources() returned %d nodes, want %d", len(result), tt.expected)
			}
		})
	}
}

// Note: GetNodeMetrics test is skipped here as it requires complex mocking of metricsclientset.Interface
// It will be tested in integration tests instead

