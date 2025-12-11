package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/veditoid/kubectl-rl-top/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// CombinedPodData represents combined metrics and resources for a pod
type CombinedPodData struct {
	Name          string
	CPUUsage      string
	CPURequest    string
	CPULimit      string
	MemoryUsage   string
	MemoryRequest string
	MemoryLimit   string
}

// RunPod executes the pod command
func RunPod(
	ctx context.Context,
	clientset kubernetes.Interface,
	metricsClient metricsclientset.Interface,
	namespace, labelSelector, fieldSelector string,
	podNames []string,
	sortBy string,
	noHeaders bool,
) error {
	// Check if Metrics API is available
	if err := pkg.CheckMetricsAPIAvailable(ctx, clientset); err != nil {
		return fmt.Errorf("metrics API not available: %w\nPlease ensure metrics-server is installed in your cluster", err)
	}

	// Fetch metrics and resources in parallel
	metricsChan := make(chan []pkg.PodMetrics, 1)
	resourcesChan := make(chan []pkg.PodResources, 1)
	errChan := make(chan error, 2)

	go func() {
		metrics, err := pkg.GetPodMetrics(ctx, metricsClient, namespace, labelSelector, fieldSelector, podNames)
		if err != nil {
			errChan <- err
			return
		}
		metricsChan <- metrics
	}()

	go func() {
		resources, err := pkg.GetPodResources(ctx, clientset, namespace, labelSelector, fieldSelector, podNames)
		if err != nil {
			errChan <- err
			return
		}
		resourcesChan <- resources
	}()

	var metrics []pkg.PodMetrics
	var resources []pkg.PodResources

	// Wait for both to complete
	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			return err
		case metrics = <-metricsChan:
		case resources = <-resourcesChan:
		}
	}

	// Combine metrics and resources
	combined := combineMetricsAndResources(metrics, resources)

	if len(combined) == 0 {
		fmt.Fprintf(os.Stderr, "No pods found\n")
		return nil
	}

	// Sort based on sortBy parameter
	if sortBy != "" {
		sortCombinedData(combined, sortBy)
	} else {
		// Default: sort by pod name
		sort.Slice(combined, func(i, j int) bool {
			return combined[i].Name < combined[j].Name
		})
	}

	// Print table
	printTable(combined, noHeaders)

	return nil
}

// combineMetricsAndResources merges metrics and resources data
func combineMetricsAndResources(metrics []pkg.PodMetrics, resources []pkg.PodResources) []CombinedPodData {
	// Create maps for quick lookup
	metricsMap := make(map[string]pkg.PodMetrics)
	for _, m := range metrics {
		key := fmt.Sprintf("%s/%s", m.Namespace, m.Name)
		metricsMap[key] = m
	}

	resourcesMap := make(map[string]pkg.PodResources)
	for _, r := range resources {
		key := fmt.Sprintf("%s/%s", r.Namespace, r.Name)
		resourcesMap[key] = r
	}

	// Combine data
	combined := make([]CombinedPodData, 0)
	seen := make(map[string]bool)

	// Add entries from metrics (pods with metrics)
	for key, m := range metricsMap {
		if seen[key] {
			continue
		}
		seen[key] = true

		r, hasResources := resourcesMap[key]
		var cpuRequest, cpuLimit, memRequest, memLimit string
		if hasResources {
			cpuRequest = r.CPURequest
			cpuLimit = r.CPULimit
			// Normalize memory requests/limits to match the usage unit
			memoryUnit := pkg.ExtractMemoryUnit(m.Memory)
			if !r.MemoryRequest.IsZero() {
				memRequest = pkg.FormatMemoryInUnit(r.MemoryRequest, memoryUnit)
			} else {
				memRequest = "-"
			}
			if !r.MemoryLimit.IsZero() {
				memLimit = pkg.FormatMemoryInUnit(r.MemoryLimit, memoryUnit)
			} else {
				memLimit = "-"
			}
		} else {
			cpuRequest = "-"
			cpuLimit = "-"
			memRequest = "-"
			memLimit = "-"
		}
		combined = append(combined, CombinedPodData{
			Name:          m.Name,
			CPUUsage:      m.CPU,
			CPURequest:    cpuRequest,
			CPULimit:      cpuLimit,
			MemoryUsage:   m.Memory,
			MemoryRequest: memRequest,
			MemoryLimit:   memLimit,
		})
	}

	// Add entries from resources that don't have metrics (pods without metrics)
	for key, r := range resourcesMap {
		if seen[key] {
			continue
		}
		seen[key] = true

		// For pods without metrics, use default unit (Mi) for memory
		var memRequest, memLimit string
		if !r.MemoryRequest.IsZero() {
			memRequest = pkg.FormatMemoryInUnit(r.MemoryRequest, "Mi")
		} else {
			memRequest = "-"
		}
		if !r.MemoryLimit.IsZero() {
			memLimit = pkg.FormatMemoryInUnit(r.MemoryLimit, "Mi")
		} else {
			memLimit = "-"
		}

		combined = append(combined, CombinedPodData{
			Name:          r.Name,
			CPUUsage:      "<unknown>",
			CPURequest:    r.CPURequest,
			CPULimit:      r.CPULimit,
			MemoryUsage:   "<unknown>",
			MemoryRequest: memRequest,
			MemoryLimit:   memLimit,
		})
	}

	return combined
}

// printTable prints the combined pod data in a formatted table
func printTable(data []CombinedPodData, noHeaders bool) {
	// Calculate column widths
	nameWidth := 40
	cpuWidth := 12
	memWidth := 15

	for _, d := range data {
		if len(d.Name) > nameWidth {
			nameWidth = len(d.Name)
		}
	}

	// Print header unless --no-headers is set
	if !noHeaders {
		header := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s",
			nameWidth, "NAME",
			cpuWidth, "CPU(cores)",
			cpuWidth, "CPU REQUEST",
			cpuWidth, "CPU LIMIT",
			memWidth, "MEMORY(bytes)",
			memWidth, "MEMORY REQUEST",
			memWidth, "MEMORY LIMIT",
		)
		fmt.Println(header)
	}

	// Print rows
	for _, d := range data {
		row := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s",
			nameWidth, d.Name,
			cpuWidth, d.CPUUsage,
			cpuWidth, d.CPURequest,
			cpuWidth, d.CPULimit,
			memWidth, d.MemoryUsage,
			memWidth, d.MemoryRequest,
			memWidth, d.MemoryLimit,
		)
		fmt.Println(row)
	}
}

// sortCombinedData sorts the combined pod data based on the sortBy field
func sortCombinedData(data []CombinedPodData, sortBy string) {
	switch sortBy {
	case "cpu":
		sort.Slice(data, func(i, j int) bool {
			// Parse CPU values for comparison (handle "m" suffix for millicores)
			return parseCPUValue(data[i].CPUUsage) > parseCPUValue(data[j].CPUUsage)
		})
	case "memory":
		sort.Slice(data, func(i, j int) bool {
			// Parse memory values for comparison
			return parseMemoryValue(data[i].MemoryUsage) > parseMemoryValue(data[j].MemoryUsage)
		})
	default:
		// Default: sort by name
		sort.Slice(data, func(i, j int) bool {
			return data[i].Name < data[j].Name
		})
	}
}

// parseCPUValue parses CPU string to float64 for sorting (handles "m" suffix)
func parseCPUValue(cpuStr string) float64 {
	if cpuStr == "" || cpuStr == "-" || cpuStr == "<unknown>" {
		return 0
	}
	// Remove "m" suffix and convert to float
	cpuStr = strings.TrimSuffix(cpuStr, "m")
	var value float64
	_, _ = fmt.Sscanf(cpuStr, "%f", &value)
	return value
}

// parseMemoryValue parses memory string to bytes for sorting
func parseMemoryValue(memStr string) int64 {
	if memStr == "" || memStr == "-" || memStr == "<unknown>" {
		return 0
	}
	// Simple parsing - convert common units to bytes
	if strings.HasSuffix(memStr, "Gi") {
		var value float64
		_, _ = fmt.Sscanf(memStr, "%fGi", &value)
		return int64(value * 1024 * 1024 * 1024)
	}
	if strings.HasSuffix(memStr, "Mi") {
		var value float64
		_, _ = fmt.Sscanf(memStr, "%fMi", &value)
		return int64(value * 1024 * 1024)
	}
	if strings.HasSuffix(memStr, "Ki") {
		var value float64
		_, _ = fmt.Sscanf(memStr, "%fKi", &value)
		return int64(value * 1024)
	}
	return 0
}

// NewPodCommand creates a new pod command
func NewPodCommand() *cobra.Command {
	var namespace string
	var allNamespaces bool
	var labelSelector string
	var fieldSelector string
	var sortBy string
	var noHeaders bool
	var containers bool
	var useProtocolBuffers bool

	cmd := &cobra.Command{
		Use:     "pod [NAME | -l label]",
		Aliases: []string{"pods", "po"},
		Short:   "Display resource usage (CPU, memory) and requests/limits for pods",
		Long: `Display resource usage (CPU, memory) and requests/limits for pods.
Similar to 'kubectl top pods' but also shows resource requests and limits.

You can use 'pod', 'pods', or 'po' as the command name, just like kubectl.

Examples:
  # Show metrics for all pods in the default namespace
  kubectl rltop pod
  
  # Show metrics for all pods in the given namespace
  kubectl rltop pod --namespace=NAMESPACE
  
  # Show metrics for a given pod
  kubectl rltop pod POD_NAME
  
  # Show metrics for the pods defined by label name=myLabel
  kubectl rltop pod -l name=myLabel`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle -A/--all-namespaces flag
			if allNamespaces {
				namespace = ""
			}

			// Extract pod names from args
			var podNames []string
			if len(args) > 0 {
				podNames = args
			}

			// Note: --containers and --use-protocol-buffers are not yet implemented
			// but we accept the flags for compatibility
			_ = containers
			_ = useProtocolBuffers
			// Use RESTClientGetter pattern - same as kubectl plugins use
			// This properly handles kubeconfig loading with exec plugins
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}

			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				loadingRules,
				configOverrides,
			)

			// Get REST config
			config, err := clientConfig.ClientConfig()
			if err != nil {
				// Provide helpful error message for common exec plugin issues
				errMsg := err.Error()
				if strings.Contains(errMsg, "exec plugin") && strings.Contains(errMsg, "apiVersion") {
					return fmt.Errorf("failed to load kubeconfig: %w. "+
						"Your kubeconfig uses an exec plugin with an outdated API version. "+
						"To fix this, update your kubeconfig by running: "+
						"kubectl config view --raw > ~/.kube/config.new && "+
						"mv ~/.kube/config.new ~/.kube/config. "+
						"Or regenerate your kubeconfig using your cloud provider's CLI tool", err)
				}
				return fmt.Errorf("failed to load kubeconfig: %w", err)
			}

			// Create clients
			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "exec plugin") && strings.Contains(errMsg, "apiVersion") {
					return fmt.Errorf("failed to create kubernetes client: %w. "+
						"Your kubeconfig uses an exec plugin with an outdated API version (v1alpha1). "+
						"This version of kubectl-rltop requires exec plugins to use v1beta1 or v1. "+
						"To fix this, update your kubeconfig: "+
						"1. Run: kubectl config view --raw > ~/.kube/config.new "+
						"2. Check the file and update any exec plugin apiVersion from v1alpha1 to v1beta1 "+
						"3. Replace: mv ~/.kube/config.new ~/.kube/config. "+
						"Or regenerate your kubeconfig using your cloud provider's CLI tool", err)
				}
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			metricsClient, err := metricsclientset.NewForConfig(config)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "exec plugin") && strings.Contains(errMsg, "apiVersion") {
					return fmt.Errorf("failed to create metrics client: %w. "+
						"Your kubeconfig uses an exec plugin with an outdated API version. "+
						"See the error above for instructions on how to fix this", err)
				}
				return fmt.Errorf("failed to create metrics client: %w", err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			return RunPod(
				ctx, clientset, metricsClient,
				namespace, labelSelector, fieldSelector,
				podNames, sortBy, noHeaders,
			)
		},
	}

	// Add all flags matching kubectl top pods
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to query (default: all namespaces)")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false,
		"If present, list the requested object(s) across all namespaces. "+
			"Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&labelSelector, "selector", "l", "",
		"Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().StringVar(&fieldSelector, "field-selector", "",
		"Selector (field query) to filter on, supports '=', '==', and '!='. "+
			"(e.g. --field-selector key1=value1,key2=value2). "+
			"The server only supports a limited number of field queries per type.")
	cmd.Flags().StringVar(&sortBy, "sort-by", "",
		"If non-empty, sort pods list using specified field. The field can be either 'cpu' or 'memory'.")
	cmd.Flags().BoolVar(&noHeaders, "no-headers", false,
		"If present, print output without headers.")
	cmd.Flags().BoolVar(&containers, "containers", false,
		"If present, print usage of containers within a pod.")
	cmd.Flags().BoolVar(&useProtocolBuffers, "use-protocol-buffers", true,
		"Enables using protocol-buffers to access Metrics API.")

	return cmd
}
