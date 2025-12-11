package pkg

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// PodMetrics represents CPU and memory usage for a pod
type PodMetrics struct {
	Name      string
	Namespace string
	CPU       string
	Memory    string
}

// GetPodMetrics fetches pod metrics from the Metrics API
func GetPodMetrics(
	ctx context.Context,
	metricsClient metricsclientset.Interface,
	namespace string,
	labelSelector, fieldSelector string,
	podNames []string,
) ([]PodMetrics, error) {
	var podMetricsList *metricsv1beta1.PodMetricsList
	var err error

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}

	if namespace == "" {
		podMetricsList, err = metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, listOptions)
	} else {
		podMetricsList, err = metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, listOptions)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch pod metrics: %w", err)
	}

	// Filter by pod names if specified
	if len(podNames) > 0 {
		podNameMap := make(map[string]bool)
		for _, name := range podNames {
			podNameMap[name] = true
		}
		filtered := make([]metricsv1beta1.PodMetrics, 0)
		for _, pm := range podMetricsList.Items {
			if podNameMap[pm.Name] {
				filtered = append(filtered, pm)
			}
		}
		podMetricsList.Items = filtered
	}

	metrics := make([]PodMetrics, 0, len(podMetricsList.Items))
	for _, pm := range podMetricsList.Items {
		var totalCPU, totalMemory int64
		for _, container := range pm.Containers {
			totalCPU += container.Usage.Cpu().MilliValue()
			totalMemory += container.Usage.Memory().Value()
		}

		metrics = append(metrics, PodMetrics{
			Name:      pm.Name,
			Namespace: pm.Namespace,
			CPU:       formatCPU(totalCPU),
			Memory:    formatMemory(totalMemory),
		})
	}

	return metrics, nil
}

// CheckMetricsAPIAvailable checks if the Metrics API is available
func CheckMetricsAPIAvailable(ctx context.Context, clientset kubernetes.Interface) error {
	discoveryClient := clientset.Discovery()
	apiGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return fmt.Errorf("failed to discover API groups: %w", err)
	}

	for _, group := range apiGroups.Groups {
		if group.Name == "metrics.k8s.io" {
			return nil
		}
	}

	return fmt.Errorf("metrics API (metrics.k8s.io) not available in the cluster")
}

// formatCPU formats CPU value in millicores to a human-readable string
func formatCPU(millicores int64) string {
	if millicores == 0 {
		return "0"
	}
	if millicores < 1000 {
		return fmt.Sprintf("%dm", millicores)
	}
	cores := float64(millicores) / 1000.0
	if cores == float64(int64(cores)) {
		return fmt.Sprintf("%.0f", cores)
	}
	return fmt.Sprintf("%.2f", cores)
}

// formatMemory formats memory value in bytes to a human-readable string
func formatMemory(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2fGi", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2fMi", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2fKi", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

