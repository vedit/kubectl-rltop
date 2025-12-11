package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/veditoid/kubectl-rl-top/cmd"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kubectl-rltop",
		Short: "Display resource usage with requests and limits for pods and nodes",
		Long: `kubectl-rltop is a kubectl plugin that displays resource usage (CPU and memory)
along with resource requests and limits for pods and nodes.

It works like 'kubectl top pods' and 'kubectl top nodes' but also shows the resource
requests and limits defined in pod specifications.

Usage:
  kubectl rltop pod [flags]    # Display pod resource usage with requests/limits
  kubectl rltop node [flags]   # Display node resource usage with aggregated requests/limits
  kubectl rltop pods [flags]   # Alias for pod
  kubectl rltop nodes [flags]  # Alias for node`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kubectl-rltop version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  date: %s\n", date)
			fmt.Printf("  go version: %s\n", runtime.Version())
			fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	// Add the pod subcommand (with aliases: pods, po)
	rootCmd.AddCommand(cmd.NewPodCommand())
	// Add the node subcommand (with aliases: nodes, no)
	rootCmd.AddCommand(cmd.NewNodeCommand())
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

