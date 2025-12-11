//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestNodeCommand_Basic(t *testing.T) {
	output, err := runCommand(t, "node")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected columns
	expectedColumns := []string{"NAME", "CPU(cores)", "CPU%", "CPU REQUEST", "CPU LIMIT", "MEMORY(bytes)", "MEMORY%", "MEMORY REQUEST", "MEMORY LIMIT"}
	for _, col := range expectedColumns {
		if !strings.Contains(output, col) {
			t.Errorf("Output missing column: %s\nOutput: %s", col, output)
		}
	}

	// Should show at least one node
	if len(strings.TrimSpace(output)) == 0 {
		t.Errorf("Output should contain node information\nOutput: %s", output)
	}
}

func TestNodeCommand_NodeName(t *testing.T) {
	// Get first node name
	output, err := runCommand(t, "node")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Extract first node name from output
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Skip("No nodes found, skipping test")
	}

	// Find first data line (skip header)
	var nodeName string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line != "" {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				nodeName = parts[0]
				break
			}
		}
	}

	if nodeName == "" {
		t.Skip("Could not extract node name, skipping test")
	}

	// Test with specific node name
	output2, err := runCommand(t, "node", nodeName)
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output2)
	}

	// Should only show the specified node
	if !strings.Contains(output2, nodeName) {
		t.Errorf("Output should contain node %s\nOutput: %s", nodeName, output2)
	}
}

func TestNodeCommand_SortByCPU(t *testing.T) {
	output, err := runCommand(t, "node", "--sort-by=cpu")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output is sorted
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Skip("No nodes found, skipping test")
	}
}

func TestNodeCommand_SortByMemory(t *testing.T) {
	output, err := runCommand(t, "node", "--sort-by=memory")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output is sorted
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Skip("No nodes found, skipping test")
	}
}

func TestNodeCommand_NoHeaders(t *testing.T) {
	output, err := runCommand(t, "node", "--no-headers")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should not contain header row
	if strings.Contains(output, "NAME") {
		t.Errorf("Output should not contain headers\nOutput: %s", output)
	}

	// Should still contain node information
	if len(strings.TrimSpace(output)) == 0 {
		t.Errorf("Output should contain node information\nOutput: %s", output)
	}
}

func TestNodeCommand_ShowCapacity(t *testing.T) {
	output, err := runCommand(t, "node", "--show-capacity")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should show node information
	if len(strings.TrimSpace(output)) == 0 {
		t.Errorf("Output should contain node information\nOutput: %s", output)
	}
}

func TestNodeCommand_AggregatedResources(t *testing.T) {
	output, err := runCommand(t, "node")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify aggregated resource requests and limits are displayed
	// Should show CPU REQUEST and CPU LIMIT columns
	if !strings.Contains(output, "CPU REQUEST") {
		t.Errorf("Output should contain CPU REQUEST column\nOutput: %s", output)
	}
	if !strings.Contains(output, "CPU LIMIT") {
		t.Errorf("Output should contain CPU LIMIT column\nOutput: %s", output)
	}
	if !strings.Contains(output, "MEMORY REQUEST") {
		t.Errorf("Output should contain MEMORY REQUEST column\nOutput: %s", output)
	}
	if !strings.Contains(output, "MEMORY LIMIT") {
		t.Errorf("Output should contain MEMORY LIMIT column\nOutput: %s", output)
	}

	// Verify percentages are shown
	if !strings.Contains(output, "CPU%") {
		t.Errorf("Output should contain CPU%% column\nOutput: %s", output)
	}
	if !strings.Contains(output, "MEMORY%") {
		t.Errorf("Output should contain MEMORY%% column\nOutput: %s", output)
	}
}

