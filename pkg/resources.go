package pkg

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/resource"
)

// PodResources represents resource requests and limits for a pod
type PodResources struct {
	Name            string
	Namespace       string
	CPURequest      string
	CPULimit        string
	MemoryRequest   resource.Quantity
	MemoryLimit     resource.Quantity
	MemoryRequestStr string // Keep formatted string for backward compatibility
	MemoryLimitStr  string
}

// GetPodResources fetches pod resources (requests and limits) from pod specifications
func GetPodResources(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	labelSelector, fieldSelector string,
	podNames []string,
) ([]PodResources, error) {
	var podList *corev1.PodList
	var err error

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}

	if namespace == "" {
		podList, err = clientset.CoreV1().Pods("").List(ctx, listOptions)
	} else {
		podList, err = clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch pods: %w", err)
	}

	// Filter by pod names if specified
	if len(podNames) > 0 {
		podNameMap := make(map[string]bool)
		for _, name := range podNames {
			podNameMap[name] = true
		}
		filtered := make([]corev1.Pod, 0)
		for _, pod := range podList.Items {
			if podNameMap[pod.Name] {
				filtered = append(filtered, pod)
			}
		}
		podList.Items = filtered
	}

	resources := make([]PodResources, 0, len(podList.Items))
	for _, pod := range podList.Items {
		var totalCPURequest, totalCPULimit resource.Quantity
		var totalMemoryRequest, totalMemoryLimit resource.Quantity

		// Aggregate resources from all containers in the pod
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					totalCPURequest.Add(cpu)
				}
				if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					totalMemoryRequest.Add(memory)
				}
			}
			if container.Resources.Limits != nil {
				if cpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
					totalCPULimit.Add(cpu)
				}
				if memory, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
					totalMemoryLimit.Add(memory)
				}
			}
		}

		// Also check init containers (they can affect scheduling)
		for _, container := range pod.Spec.InitContainers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					// Init containers use max(request, initContainer request)
					if cpu.Cmp(totalCPURequest) > 0 {
						totalCPURequest = cpu
					}
				}
				if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					if memory.Cmp(totalMemoryRequest) > 0 {
						totalMemoryRequest = memory
					}
				}
			}
		}

		resources = append(resources, PodResources{
			Name:             pod.Name,
			Namespace:        pod.Namespace,
			CPURequest:       FormatResourceQuantity(totalCPURequest, true),
			CPULimit:         FormatResourceQuantity(totalCPULimit, true),
			MemoryRequest:    totalMemoryRequest,
			MemoryLimit:      totalMemoryLimit,
			MemoryRequestStr: FormatResourceQuantity(totalMemoryRequest, false),
			MemoryLimitStr:   FormatResourceQuantity(totalMemoryLimit, false),
		})
	}

	return resources, nil
}

// FormatResourceQuantity formats a resource.Quantity to a human-readable string
func FormatResourceQuantity(q resource.Quantity, isCPU bool) string {
	if q.IsZero() {
		return "-"
	}

	if isCPU {
		// For CPU, always show in millicores (normalized format)
		millicores := q.MilliValue()
		if millicores == 0 {
			return "0m"
		}
		return fmt.Sprintf("%dm", millicores)
	}

	// For memory, use the quantity's String() method which formats nicely
	return q.String()
}

// FormatMemoryInUnit formats a memory resource.Quantity to a specific unit (Mi, Gi, etc.)
func FormatMemoryInUnit(q resource.Quantity, targetUnit string) string {
	if q.IsZero() {
		return "-"
	}

	bytes := q.Value()
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch targetUnit {
	case "Gi":
		return fmt.Sprintf("%.2fGi", float64(bytes)/float64(GB))
	case "Mi":
		return fmt.Sprintf("%.2fMi", float64(bytes)/float64(MB))
	case "Ki":
		return fmt.Sprintf("%.2fKi", float64(bytes)/float64(KB))
	default:
		// Default to Mi if unit not recognized
		return fmt.Sprintf("%.2fMi", float64(bytes)/float64(MB))
	}
}

// ExtractMemoryUnit extracts the unit from a memory string (e.g., "128Mi" -> "Mi")
func ExtractMemoryUnit(memoryStr string) string {
	if memoryStr == "" || memoryStr == "-" || memoryStr == "<unknown>" {
		return "Mi" // Default unit
	}

	// Check for common units
	if strings.HasSuffix(memoryStr, "Gi") {
		return "Gi"
	}
	if strings.HasSuffix(memoryStr, "Mi") {
		return "Mi"
	}
	if strings.HasSuffix(memoryStr, "Ki") {
		return "Ki"
	}
	if strings.HasSuffix(memoryStr, "G") {
		return "Gi"
	}
	if strings.HasSuffix(memoryStr, "M") {
		return "Mi"
	}
	if strings.HasSuffix(memoryStr, "K") {
		return "Ki"
	}

	// Default to Mi if no unit found
	return "Mi"
}

