//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfigPath string
	clientset      kubernetes.Interface
	testNamespace  = "rltop-test"
)

func TestMain(m *testing.M) {
	// Setup
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown
	teardown()

	os.Exit(code)
}

func setup() error {
	// Check if k3s is already running
	if isK3sRunning() {
		fmt.Println("k3s is already running, using existing cluster")
		kubeconfigPath = "/etc/rancher/k3s/k3s.yaml"
	} else {
		// Install and start k3s
		fmt.Println("Installing k3s...")
		if err := installK3s(); err != nil {
			return fmt.Errorf("failed to install k3s: %w", err)
		}
		kubeconfigPath = "/etc/rancher/k3s/k3s.yaml"
	}

	// Wait for cluster to be ready
	fmt.Println("Waiting for cluster to be ready...")
	if err := waitForClusterReady(2 * time.Minute); err != nil {
		return fmt.Errorf("cluster not ready: %w", err)
	}

	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Wait for metrics-server
	fmt.Println("Waiting for metrics-server...")
	if err := waitForMetricsServer(2 * time.Minute); err != nil {
		return fmt.Errorf("metrics-server not ready: %w", err)
	}

	// Create test namespace
	ctx := context.Background()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create test namespace: %w", err)
	}

	// Create test pods with known resource requests/limits
	if err := createTestPods(ctx); err != nil {
		return fmt.Errorf("failed to create test pods: %w", err)
	}

	// Wait a bit for metrics to be available
	time.Sleep(10 * time.Second)

	return nil
}

func teardown() {
	ctx := context.Background()
	if clientset != nil {
		// Clean up test namespace
		clientset.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
	}
}

func isK3sRunning() bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", "k3s")
	return cmd.Run() == nil
}

func installK3s() error {
	// Check if k3s is already installed
	if _, err := exec.LookPath("k3s"); err == nil {
		// k3s is installed, try to start it
		cmd := exec.Command("sudo", "systemctl", "start", "k3s")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start k3s: %w", err)
		}
		return nil
	}

	// Install k3s
	cmd := exec.Command("sh", "-c", "curl -sfL https://get.k3s.io | sh -")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install k3s: %w", err)
	}

	// Wait a bit for k3s to start
	time.Sleep(5 * time.Second)

	return nil
}

func waitForClusterReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for cluster to be ready")
		default:
			cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "--no-headers")
			if err := cmd.Run(); err == nil {
				return nil
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func waitForMetricsServer(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for metrics-server")
		default:
			cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "wait", "--for=condition=ready",
				"pod", "-l", "k8s-app=metrics-server", "-n", "kube-system", "--timeout=30s")
			if err := cmd.Run(); err == nil {
				return nil
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func createTestPods(ctx context.Context) error {
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-1",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test",
					"env": "test",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "container1",
						Image: "nginx:alpine",
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
				Name:      "test-pod-2",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test",
					"env": "prod",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "container1",
						Image: "nginx:alpine",
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-no-limits",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "container1",
						Image: "nginx:alpine",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							// No limits
						},
					},
				},
			},
		},
	}

	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(testNamespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create pod %s: %w", pod.Name, err)
		}
	}

	return nil
}

func runCommand(t *testing.T, args ...string) (string, error) {
	binaryPath := filepath.Join("..", "..", "kubectl-rltop")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try to build it
		cmd := exec.Command("go", "build", "-o", binaryPath, "../..")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to build binary: %v", err)
		}
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.CombinedOutput()
	return string(output), err
}

