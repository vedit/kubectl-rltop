//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestPodCommand_Basic(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace)
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected columns
	expectedColumns := []string{"NAME", "CPU(cores)", "CPU REQUEST", "CPU LIMIT", "MEMORY(bytes)", "MEMORY REQUEST", "MEMORY LIMIT"}
	for _, col := range expectedColumns {
		if !strings.Contains(output, col) {
			t.Errorf("Output missing column: %s\nOutput: %s", col, output)
		}
	}

	// Verify test pods are in output
	if !strings.Contains(output, "test-pod-1") {
		t.Errorf("Output missing test-pod-1\nOutput: %s", output)
	}
}

func TestPodCommand_NamespaceFilter(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace)
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should only show pods from test namespace
	if strings.Contains(output, "kube-system") {
		t.Errorf("Output should not contain kube-system pods\nOutput: %s", output)
	}
}

func TestPodCommand_LabelSelector(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace, "-l", "env=test")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should only show pods with env=test label
	if !strings.Contains(output, "test-pod-1") {
		t.Errorf("Output should contain test-pod-1\nOutput: %s", output)
	}
	if strings.Contains(output, "test-pod-2") {
		t.Errorf("Output should not contain test-pod-2 (env=prod)\nOutput: %s", output)
	}
}

func TestPodCommand_PodName(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace, "test-pod-1")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should only show the specified pod
	if !strings.Contains(output, "test-pod-1") {
		t.Errorf("Output should contain test-pod-1\nOutput: %s", output)
	}
	if strings.Contains(output, "test-pod-2") {
		t.Errorf("Output should not contain test-pod-2\nOutput: %s", output)
	}
}

func TestPodCommand_SortByCPU(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace, "--sort-by=cpu")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output is sorted (first pod should have higher CPU)
	lines := strings.Split(output, "\n")
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines in output\nOutput: %s", output)
	}
}

func TestPodCommand_NoHeaders(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace, "--no-headers")
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should not contain header row
	if strings.Contains(output, "NAME") {
		t.Errorf("Output should not contain headers\nOutput: %s", output)
	}

	// Should still contain pod names
	if !strings.Contains(output, "test-pod") {
		t.Errorf("Output should contain pod names\nOutput: %s", output)
	}
}

func TestPodCommand_ResourceRequestsLimits(t *testing.T) {
	output, err := runCommand(t, "pod", "-n", testNamespace)
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify resource requests and limits are displayed
	// test-pod-1 has 100m CPU request, 200m CPU limit
	if !strings.Contains(output, "100m") {
		t.Errorf("Output should contain CPU request (100m)\nOutput: %s", output)
	}
	if !strings.Contains(output, "200m") {
		t.Errorf("Output should contain CPU limit (200m)\nOutput: %s", output)
	}

	// test-pod-no-limits should show "-" for limits
	if strings.Contains(output, "test-pod-no-limits") {
		// Check that limits column shows "-" for this pod
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "test-pod-no-limits") {
				// Should have "-" in the limit columns
				if !strings.Contains(line, "-") {
					t.Errorf("Pod without limits should show '-' in limit columns\nLine: %s", line)
				}
			}
		}
	}
}

