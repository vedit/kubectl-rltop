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
		Short: "Display pod resource usage with requests and limits",
		Long: `kubectl-rltop is a kubectl plugin that displays pod resource usage
(CPU and memory) along with resource requests and limits.

It works like 'kubectl top pods' but also shows the resource requests and limits
defined in the pod specifications.

Usage:
  kubectl rltop pod [flags]
  kubectl rltop pods [flags]
  kubectl rltop po [flags]`,
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
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

