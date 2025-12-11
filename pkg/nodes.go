package pkg

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// NodeMetrics represents CPU and memory usage for a node
type NodeMetrics struct {
	Name      string
	CPU       string
	CPUPercent string
	Memory    string
	MemoryPercent string
}

// NodeAggregatedResources represents aggregated resource requests and limits for all pods on a node
type NodeAggregatedResources struct {
	NodeName       string
	CPURequest     resource.Quantity
	CPULimit       resource.Quantity
	MemoryRequest  resource.Quantity
	MemoryLimit    resource.Quantity
}

// GetNodeMetrics fetches node metrics from the Metrics API
func GetNodeMetrics(ctx context.Context, metricsClient metricsclientset.Interface, labelSelector string, nodeNames []string) ([]NodeMetrics, error) {
	nodeMetricsList, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch node metrics: %w", err)
	}

	// Filter by node names if specified
	if len(nodeNames) > 0 {
		nodeNameMap := make(map[string]bool)
		for _, name := range nodeNames {
			nodeNameMap[name] = true
		}
		filtered := make([]metricsv1beta1.NodeMetrics, 0)
		for _, nm := range nodeMetricsList.Items {
			if nodeNameMap[nm.Name] {
				filtered = append(filtered, nm)
			}
		}
		nodeMetricsList.Items = filtered
	}

	metrics := make([]NodeMetrics, 0, len(nodeMetricsList.Items))
	for _, nm := range nodeMetricsList.Items {
		cpuUsage := nm.Usage.Cpu().MilliValue()
		memoryUsage := nm.Usage.Memory().Value()

		metrics = append(metrics, NodeMetrics{
			Name:   nm.Name,
			CPU:    formatCPU(cpuUsage),
			Memory: formatMemory(memoryUsage),
			// CPUPercent and MemoryPercent will be calculated later when we have node capacity/allocatable
		})
	}

	return metrics, nil
}

// GetNodeResources fetches node capacity and allocatable resources
// Returns a map of node name to Node object
func GetNodeResources(ctx context.Context, clientset kubernetes.Interface, labelSelector string, nodeNames []string, showCapacity bool) (map[string]*corev1.Node, error) {
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}

	// Filter by node names if specified
	if len(nodeNames) > 0 {
		nodeNameMap := make(map[string]bool)
		for _, name := range nodeNames {
			nodeNameMap[name] = true
		}
		filtered := make([]corev1.Node, 0)
		for _, node := range nodeList.Items {
			if nodeNameMap[node.Name] {
				filtered = append(filtered, node)
			}
		}
		nodeList.Items = filtered
	}

	nodesMap := make(map[string]*corev1.Node)
	for i := range nodeList.Items {
		nodesMap[nodeList.Items[i].Name] = &nodeList.Items[i]
	}

	return nodesMap, nil
}

// AggregatePodResourcesByNode groups pods by node and aggregates their resource requests and limits
func AggregatePodResourcesByNode(ctx context.Context, clientset kubernetes.Interface) (map[string]*NodeAggregatedResources, error) {
	// Get all pods across all namespaces
	podList, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pods: %w", err)
	}

	// Map to store aggregated resources per node
	nodeResources := make(map[string]*NodeAggregatedResources)

	for _, pod := range podList.Items {
		// Skip pods that are not scheduled or don't have a node assigned
		if pod.Spec.NodeName == "" {
			continue
		}

		// Initialize node entry if it doesn't exist
		if nodeResources[pod.Spec.NodeName] == nil {
			nodeResources[pod.Spec.NodeName] = &NodeAggregatedResources{
				NodeName: pod.Spec.NodeName,
			}
		}

		// Aggregate resources from all containers in the pod
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					nodeResources[pod.Spec.NodeName].CPURequest.Add(cpu)
				}
				if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					nodeResources[pod.Spec.NodeName].MemoryRequest.Add(memory)
				}
			}
			if container.Resources.Limits != nil {
				if cpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
					nodeResources[pod.Spec.NodeName].CPULimit.Add(cpu)
				}
				if memory, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
					nodeResources[pod.Spec.NodeName].MemoryLimit.Add(memory)
				}
			}
		}

		// Also check init containers (they can affect scheduling)
		for _, container := range pod.Spec.InitContainers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					// Init containers use max(request, initContainer request)
					if cpu.Cmp(nodeResources[pod.Spec.NodeName].CPURequest) > 0 {
						nodeResources[pod.Spec.NodeName].CPURequest = cpu
					}
				}
				if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					if memory.Cmp(nodeResources[pod.Spec.NodeName].MemoryRequest) > 0 {
						nodeResources[pod.Spec.NodeName].MemoryRequest = memory
					}
				}
			}
		}
	}

	return nodeResources, nil
}

// CalculateNodePercentages calculates CPU and memory percentages based on allocatable or capacity
func CalculateNodePercentages(node *corev1.Node, cpuUsageMilli int64, memoryUsageBytes int64, showCapacity bool) (cpuPercent, memoryPercent string) {
	var cpuTotal, memoryTotal resource.Quantity

	if showCapacity {
		cpuTotal = node.Status.Capacity[corev1.ResourceCPU]
		memoryTotal = node.Status.Capacity[corev1.ResourceMemory]
	} else {
		cpuTotal = node.Status.Allocatable[corev1.ResourceCPU]
		memoryTotal = node.Status.Allocatable[corev1.ResourceMemory]
	}

	// Calculate CPU percentage
	if !cpuTotal.IsZero() {
		cpuTotalMilli := cpuTotal.MilliValue()
		if cpuTotalMilli > 0 {
			percent := float64(cpuUsageMilli) / float64(cpuTotalMilli) * 100
			cpuPercent = fmt.Sprintf("%.0f%%", percent)
		} else {
			cpuPercent = "0%"
		}
	} else {
		cpuPercent = "-"
	}

	// Calculate Memory percentage
	if !memoryTotal.IsZero() {
		memoryTotalBytes := memoryTotal.Value()
		if memoryTotalBytes > 0 {
			percent := float64(memoryUsageBytes) / float64(memoryTotalBytes) * 100
			memoryPercent = fmt.Sprintf("%.0f%%", percent)
		} else {
			memoryPercent = "0%"
		}
	} else {
		memoryPercent = "-"
	}

	return cpuPercent, memoryPercent
}

