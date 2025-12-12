package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/veditoid/kubectl-rltop/pkg"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// CombinedNodeData represents combined node metrics and aggregated pod resources
type CombinedNodeData struct {
	Name          string
	CPUUsage      string
	CPUPercent    string
	CPURequest    string
	CPULimit      string
	MemoryUsage   string
	MemoryPercent string
	MemoryRequest string
	MemoryLimit   string
}

// RunNode executes the node command
func RunNode(
	ctx context.Context,
	clientset kubernetes.Interface,
	metricsClient metricsclientset.Interface,
	labelSelector string,
	nodeNames []string,
	showCapacity bool,
	sortBy string,
	noHeaders bool,
) error {
	// Check if Metrics API is available
	if err := pkg.CheckMetricsAPIAvailable(ctx, clientset); err != nil {
		return fmt.Errorf("metrics API not available: %w\nPlease ensure metrics-server is installed in your cluster", err)
	}

	// Fetch node metrics, node resources, and aggregated pod resources in parallel
	nodeMetricsChan := make(chan []pkg.NodeMetrics, 1)
	nodeResourcesChan := make(chan map[string]*pkg.NodeAggregatedResources, 1)
	nodesChan := make(chan map[string]*corev1.Node, 1)
	errChan := make(chan error, 3)

	go func() {
		metrics, err := pkg.GetNodeMetrics(ctx, metricsClient, labelSelector, nodeNames)
		if err != nil {
			errChan <- err
			return
		}
		nodeMetricsChan <- metrics
	}()

	go func() {
		resources, err := pkg.AggregatePodResourcesByNode(ctx, clientset)
		if err != nil {
			errChan <- err
			return
		}
		nodeResourcesChan <- resources
	}()

	go func() {
		nodes, err := pkg.GetNodeResources(ctx, clientset, labelSelector, nodeNames, showCapacity)
		if err != nil {
			errChan <- err
			return
		}
		nodesChan <- nodes
	}()

	var nodeMetrics []pkg.NodeMetrics
	var nodeResources map[string]*pkg.NodeAggregatedResources
	var nodes map[string]*corev1.Node

	// Wait for all three to complete
	for i := 0; i < 3; i++ {
		select {
		case err := <-errChan:
			return err
		case nodeMetrics = <-nodeMetricsChan:
		case nodeResources = <-nodeResourcesChan:
		case nodes = <-nodesChan:
		}
	}

	// Combine metrics and resources
	combined := combineNodeMetricsAndResources(nodeMetrics, nodeResources, nodes, showCapacity)

	if len(combined) == 0 {
		fmt.Fprintf(os.Stderr, "No nodes found\n")
		return nil
	}

	// Sort based on sortBy parameter
	if sortBy != "" {
		sortNodeData(combined, sortBy)
	} else {
		// Default: sort by node name
		sort.Slice(combined, func(i, j int) bool {
			return combined[i].Name < combined[j].Name
		})
	}

	// Print table
	printNodeTable(combined, noHeaders)

	return nil
}

// combineNodeMetricsAndResources merges node metrics with aggregated pod resources
func combineNodeMetricsAndResources(
	metrics []pkg.NodeMetrics,
	resources map[string]*pkg.NodeAggregatedResources,
	nodes map[string]*corev1.Node,
	showCapacity bool,
) []CombinedNodeData {
	combined := make([]CombinedNodeData, 0, len(metrics))

	for _, m := range metrics {
		node := nodes[m.Name]
		aggResources := resources[m.Name]

		// Calculate percentages if we have node info
		cpuPercent := "-"
		memoryPercent := "-"
		if node != nil {
			// Parse CPU usage to get millicores
			cpuUsageMilli := parseCPUValueForNode(m.CPU)
			memoryUsageBytes := parseMemoryValueForNode(m.Memory)
			cpuPercent, memoryPercent = pkg.CalculateNodePercentages(node, int64(cpuUsageMilli), memoryUsageBytes, showCapacity)
		}

		// Format aggregated resources
		var cpuRequest, cpuLimit, memRequest, memLimit string
		if aggResources != nil {
			if !aggResources.CPURequest.IsZero() {
				cpuRequest = pkg.FormatResourceQuantity(aggResources.CPURequest, true)
			} else {
				cpuRequest = "-"
			}
			if !aggResources.CPULimit.IsZero() {
				cpuLimit = pkg.FormatResourceQuantity(aggResources.CPULimit, true)
			} else {
				cpuLimit = "-"
			}

			// Normalize memory to match usage unit
			memoryUnit := pkg.ExtractMemoryUnit(m.Memory)
			if !aggResources.MemoryRequest.IsZero() {
				memRequest = pkg.FormatMemoryInUnit(aggResources.MemoryRequest, memoryUnit)
			} else {
				memRequest = "-"
			}
			if !aggResources.MemoryLimit.IsZero() {
				memLimit = pkg.FormatMemoryInUnit(aggResources.MemoryLimit, memoryUnit)
			} else {
				memLimit = "-"
			}
		} else {
			cpuRequest = "-"
			cpuLimit = "-"
			memRequest = "-"
			memLimit = "-"
		}

		combined = append(combined, CombinedNodeData{
			Name:          m.Name,
			CPUUsage:      m.CPU,
			CPUPercent:    cpuPercent,
			CPURequest:    cpuRequest,
			CPULimit:      cpuLimit,
			MemoryUsage:   m.Memory,
			MemoryPercent: memoryPercent,
			MemoryRequest: memRequest,
			MemoryLimit:   memLimit,
		})
	}

	return combined
}

// printNodeTable prints the combined node data in a formatted table
func printNodeTable(data []CombinedNodeData, noHeaders bool) {
	// Calculate column widths
	nameWidth := 50
	cpuWidth := 12
	percentWidth := 7
	memWidth := 15

	for _, d := range data {
		if len(d.Name) > nameWidth {
			nameWidth = len(d.Name)
		}
	}

	// Print header unless --no-headers is set
	if !noHeaders {
		header := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s",
			nameWidth, "NAME",
			cpuWidth, "CPU(cores)",
			percentWidth, "CPU%",
			cpuWidth, "CPU REQUEST",
			cpuWidth, "CPU LIMIT",
			memWidth, "MEMORY(bytes)",
			percentWidth, "MEMORY%",
			memWidth, "MEMORY REQUEST",
			memWidth, "MEMORY LIMIT",
		)
		fmt.Println(header)
	}

	// Print rows
	for _, d := range data {
		row := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s",
			nameWidth, d.Name,
			cpuWidth, d.CPUUsage,
			percentWidth, d.CPUPercent,
			cpuWidth, d.CPURequest,
			cpuWidth, d.CPULimit,
			memWidth, d.MemoryUsage,
			percentWidth, d.MemoryPercent,
			memWidth, d.MemoryRequest,
			memWidth, d.MemoryLimit,
		)
		fmt.Println(row)
	}
}

const unknownValue = "<unknown>"

// parseCPUValueForNode parses CPU string to float64 for node calculations (handles "m" suffix)
func parseCPUValueForNode(cpuStr string) float64 {
	if cpuStr == "" || cpuStr == "-" || cpuStr == unknownValue {
		return 0
	}
	// Remove "m" suffix and convert to float
	cpuStr = strings.TrimSuffix(cpuStr, "m")
	value, _ := strconv.ParseFloat(cpuStr, 64)
	return value
}

// parseMemoryValueForNode parses memory string to bytes for node calculations
func parseMemoryValueForNode(memStr string) int64 {
	if memStr == "" || memStr == "-" || memStr == unknownValue {
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

// sortNodeData sorts the combined node data based on the sortBy field
func sortNodeData(data []CombinedNodeData, sortBy string) {
	switch sortBy {
	case "cpu":
		sort.Slice(data, func(i, j int) bool {
			return parseCPUValueForNode(data[i].CPUUsage) > parseCPUValueForNode(data[j].CPUUsage)
		})
	case "memory":
		sort.Slice(data, func(i, j int) bool {
			return parseMemoryValueForNode(data[i].MemoryUsage) > parseMemoryValueForNode(data[j].MemoryUsage)
		})
	default:
		// Default: sort by name
		sort.Slice(data, func(i, j int) bool {
			return data[i].Name < data[j].Name
		})
	}
}

// NewNodeCommand creates a new node command
func NewNodeCommand() *cobra.Command {
	var labelSelector string
	var showCapacity bool
	var sortBy string
	var noHeaders bool
	var useProtocolBuffers bool

	cmd := &cobra.Command{
		Use:     "node [NAME | -l label]",
		Aliases: []string{"nodes", "no"},
		Short:   "Display resource usage (CPU, memory) and aggregated requests/limits for nodes",
		Long: `Display resource usage (CPU, memory) and aggregated requests/limits for nodes.
Similar to 'kubectl top node' but also shows aggregated total CPU and memory requests and limits
from all pods running on each node.

You can use 'node', 'nodes', or 'no' as the command name, just like kubectl.

Examples:
  # Show metrics for all nodes
  kubectl rltop node
  
  # Show metrics for a given node
  kubectl rltop node NODE_NAME
  
  # Show metrics for nodes defined by label
  kubectl rltop node -l node-role.kubernetes.io/worker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract node names from args
			var nodeNames []string
			if len(args) > 0 {
				nodeNames = args
			}

			// Note: --use-protocol-buffers is not yet implemented but we accept the flag for compatibility
			_ = useProtocolBuffers

			// Use RESTClientGetter pattern - same as kubectl plugins use
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}

			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				loadingRules,
				configOverrides,
			)

			// Get REST config
			config, err := clientConfig.ClientConfig()
			if err != nil {
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
				return fmt.Errorf("failed to create metrics client: %w", err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			return RunNode(ctx, clientset, metricsClient, labelSelector, nodeNames, showCapacity, sortBy, noHeaders)
		},
	}

	// Add all flags matching kubectl top node
	cmd.Flags().StringVarP(&labelSelector, "selector", "l", "",
		"Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVar(&showCapacity, "show-capacity", false,
		"Print node resources based on Capacity instead of Allocatable(default) of the nodes.")
	cmd.Flags().StringVar(&sortBy, "sort-by", "",
		"If non-empty, sort nodes list using specified field. The field can be either 'cpu' or 'memory'.")
	cmd.Flags().BoolVar(&noHeaders, "no-headers", false,
		"If present, print output without headers.")
	cmd.Flags().BoolVar(&useProtocolBuffers, "use-protocol-buffers", true,
		"Enables using protocol-buffers to access Metrics API.")

	return cmd
}
