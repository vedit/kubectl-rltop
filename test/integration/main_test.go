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
	clusterName    = "kubectl-rltop-test"
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
	// Check if kind cluster already exists
	if isKindClusterExists() {
		fmt.Printf("kind cluster '%s' already exists, using existing cluster\n", clusterName)
	} else {
		// Create kind cluster
		fmt.Printf("Creating kind cluster '%s'...\n", clusterName)
		if err := createKindCluster(); err != nil {
			return fmt.Errorf("failed to create kind cluster: %w", err)
		}
	}

	// Get kubeconfig path for kind cluster
	var err error
	kubeconfigPath, err = getKindKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to get kind kubeconfig: %w", err)
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
	// Note: We don't delete the kind cluster here to allow reuse
	// Users can delete it manually with: kind delete cluster --name kubectl-rltop-test
}

func isKindClusterExists() bool {
	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), clusterName)
}

func createKindCluster() error {
	// Check if kind is installed
	if _, err := exec.LookPath("kind"); err != nil {
		return fmt.Errorf("kind is not installed. Please install it: https://kind.sigs.k8s.io/docs/user/quick-start/#installation")
	}

	// Check if Docker is running
	if err := checkDockerRunning(); err != nil {
		return fmt.Errorf("docker is not running: %w. Please start Docker", err)
	}

	// Create kind cluster with metrics-server enabled
	// We use a config that enables metrics-server
	config := `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP`

	// Write config to temp file
	tmpfile, err := os.CreateTemp("", "kind-config-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("failed to close config file: %w", err)
	}

	// Create cluster
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--config", tmpfile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create kind cluster: %w", err)
	}

	// Get kubeconfig for the newly created cluster
	kubeconfig, err := getKindKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig after cluster creation: %w", err)
	}

	// Install metrics-server
	fmt.Println("Installing metrics-server...")
	cmd = exec.Command("kubectl", "apply", "-f", "https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install metrics-server: %w", err)
	}

	// Patch metrics-server to work in kind (disable TLS verification)
	cmd = exec.Command("kubectl", "patch", "deployment", "metrics-server", "-n", "kube-system",
		"--type", "json", "-p", `[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]`)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to patch metrics-server: %w", err)
	}

	return nil
}

func getKindKubeconfig() (string, error) {
	// Get kubeconfig from kind
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Write to temp file
	tmpfile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp kubeconfig file: %w", err)
	}

	if _, err := tmpfile.Write(output); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return "", fmt.Errorf("failed to close kubeconfig file: %w", err)
	}

	return tmpfile.Name(), nil
}

func checkDockerRunning() error {
	cmd := exec.Command("docker", "info")
	return cmd.Run()
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
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
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
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
			if err := cmd.Run(); err == nil {
				// Wait a bit more for metrics to be available
				time.Sleep(5 * time.Second)
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

