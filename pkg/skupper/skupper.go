package skupper

import (
	"archive/tar"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"compress/gzip"
	"fmt"
	"io"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	skupperCLIPath string
	downloadOnce   sync.Once
	downloadError  error
)

func Install(c kube.Clusters) {
	logger.Info("Installing Skupper")

	// Deploy Site Controller
	CreateSiteController(c.Origin)
	CreateSiteController(c.Target)
}

func CreateSiteConnection(c kube.Clusters, siteNamespace string) {
	logger.Info("Creating Site Connection on Namespace: " + siteNamespace)

	// Create Site
	CreateSite(c.Origin, c.Origin.Name+"-"+siteNamespace, siteNamespace)
	CreateSite(c.Target, c.Target.Name+"-"+siteNamespace, siteNamespace)

	// Link target to origin
	CreateConnectionToken(c.Origin, "clustershift-token-"+c.Origin.Name+"-"+siteNamespace, siteNamespace)
	ExtractConnectionToken(c.Origin, c.Target, "clustershift-token-"+c.Origin.Name+"-"+siteNamespace, siteNamespace)

	// Link origin to target
	CreateConnectionToken(c.Target, "clustershift-token-"+c.Target.Name+"-"+siteNamespace, siteNamespace)
	ExtractConnectionToken(c.Target, c.Origin, "clustershift-token-"+c.Target.Name+"-"+siteNamespace, siteNamespace)
}

func CreateSiteController(c kube.Cluster) {
	logger.Info("Deploying Site Controller")

	c.CreateNewNamespace("skupper-site-controller")
	err := c.CreateResourcesFromURL(constants.SkupperSiteControllerURL, "skupper-site-controller")
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			logger.Info("Skupper site controller resources already exist, continuing...")
			return
		} else {
			exit.OnErrorWithMessage(err, "Failed to create resources from URL")
		}
	}

	err = kube.WaitForPodsReadyByLabel(c, "application=skupper-site-controller", "skupper-site-controller", 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for Site Controller pods to be ready")
}

func CreateSite(c kube.Cluster, name, namespace string) {
	logger.Info("Creating Site")

	data := map[string]string{
		"name": name,
	}
	c.CreateConfigmap("skupper-site", namespace, data)

	err := kube.WaitForPodsReadyByLabel(c, "application=skupper-router", namespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for Skupper pods to be ready")

	err = kube.WaitForPodsReadyByLabel(c, "app.kubernetes.io/name=skupper-service-controller", namespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for Skupper pods to be ready")
}

func CreateConnectionToken(c kube.Cluster, name, namespace string) {
	logger.Info("Creating Connection Token")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"skupper.io/type": "connection-token-request",
			},
			Annotations: map[string]string{
				"skupper.io/cost": "2",
			},
		},
	}

	err := c.CreateResource(kube.Secret, namespace, secret)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			logger.Info("Secret already existing...")
			return
		} else {
			exit.OnErrorWithMessage(err, "Failed to create secret")
		}
	}

	// Wait for the controller to populate the secret with data
	logger.Info("Waiting for token to be populated with data")
	timeout := 120 * time.Second
	pollInterval := 5 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		secretInterface, err := c.FetchResource(kube.Secret, name, namespace)
		if err == nil {
			secret, ok := secretInterface.(*v1.Secret)
			if ok && len(secret.Data) > 0 {
				logger.Info("Token successfully populated with data")
				return
			}
		}
		time.Sleep(pollInterval)
	}

	exit.OnErrorWithMessage(fmt.Errorf("timeout waiting for secret data"), "Secret data was not populated within timeout period")
}

func ExtractConnectionToken(from kube.Cluster, to kube.Cluster, name, namespace string) {
	logger.Info("Extracting Connection Token")
	secretInterface, err := from.FetchResource(kube.Secret, name, namespace)
	exit.OnErrorWithMessage(err, "Failed to fetch secret")
	cleanedSecretInterface := kube.CleanResourceForCreation(secretInterface)
	secret := cleanedSecretInterface.(*v1.Secret)
	err = to.CreateResource(kube.Secret, namespace, secret)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			logger.Info("Secret already existing...")
			return
		} else {
			exit.OnErrorWithMessage(err, "Failed to create secret")
		}
	}
}

// DownloadSkupperCLI downloads the Skupper CLI binary for the current platform
func DownloadSkupperCLI() (string, error) {
	downloadOnce.Do(func() {
		logger.Info("Downloading Skupper CLI")

		// Determine OS and architecture
		osName := runtime.GOOS
		arch := runtime.GOARCH

		// Map Go architecture names to Skupper release names
		archMap := map[string]string{
			"amd64": "amd64",
			"arm64": "arm64",
		}

		skupperArch, ok := archMap[arch]
		if !ok {
			downloadError = fmt.Errorf("unsupported architecture: %s", arch)
			return
		}

		// Skupper CLI download URL (using latest version)
		version := "1.8.3" // You can change this to the desired version
		var downloadURL string
		var fileName string

		switch osName {
		case "linux":
			fileName = fmt.Sprintf("skupper-cli-%s-linux-%s.tgz", version, skupperArch)
			downloadURL = fmt.Sprintf("https://github.com/skupperproject/skupper/releases/download/%s/skupper-cli-%s-linux-%s.tgz", version, version, skupperArch)
		case "darwin":
			fileName = fmt.Sprintf("skupper-cli-%s-mac-%s.tgz", version, skupperArch)
			downloadURL = fmt.Sprintf("https://github.com/skupperproject/skupper/releases/download/%s/skupper-cli-%s-mac-%s.tgz", version, version, skupperArch)
		case "windows":
			fileName = fmt.Sprintf("skupper-cli-%s-windows-%s.zip", version, skupperArch)
			downloadURL = fmt.Sprintf("https://github.com/skupperproject/skupper/releases/download/%s/skupper-cli-%s-windows-%s.zip", version, version, skupperArch)
		default:
			downloadError = fmt.Errorf("unsupported operating system: %s", osName)
			return
		}

		//https://github.com/skupperproject/skupper/releases/download/1.8.3/skupper-cli-1.8.3-mac-arm64.tgz
		//https://github.com/skupperproject/skupper/releases/1.8.3/download/skupper-cli-1.8.3-mac-arm64.tgz

		// Create temporary directory
		tmpDir := os.TempDir()
		downloadPath := filepath.Join(tmpDir, fileName)

		// Check if CLI already exists
		extractedPath := filepath.Join(tmpDir, "skupper")
		if osName == "windows" {
			extractedPath = filepath.Join(tmpDir, "skupper.exe")
		}

		if _, err := os.Stat(extractedPath); err == nil {
			logger.Info("Skupper CLI already exists, using cached version")
			skupperCLIPath = extractedPath
			return
		}

		// Download the file
		logger.Info(fmt.Sprintf("Downloading from: %s", downloadURL))
		resp, err := http.Get(downloadURL)
		if err != nil {
			downloadError = fmt.Errorf("failed to download Skupper CLI: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			downloadError = fmt.Errorf("failed to download Skupper CLI: HTTP %d", resp.StatusCode)
			return
		}

		// Create the file
		out, err := os.Create(downloadPath)
		if err != nil {
			downloadError = fmt.Errorf("failed to create download file: %v", err)
			return
		}
		defer out.Close()

		// Copy the response body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			downloadError = fmt.Errorf("failed to save download file: %v", err)
			return
		}

		// Extract the archive
		if strings.HasSuffix(fileName, ".tgz") || strings.HasSuffix(fileName, ".tar.gz") {
			err = extractTarGz(downloadPath, tmpDir)
		} else {
			downloadError = fmt.Errorf("unsupported archive format for file: %s", fileName)
			return
		}

		if err != nil {
			downloadError = fmt.Errorf("failed to extract archive: %v", err)
			return
		}

		// Make executable on Unix systems
		if osName != "windows" {
			err = os.Chmod(extractedPath, 0755)
			if err != nil {
				downloadError = fmt.Errorf("failed to make CLI executable: %v", err)
				return
			}
		}

		logger.Info(fmt.Sprintf("Skupper CLI downloaded and extracted to: %s", extractedPath))
		skupperCLIPath = extractedPath
	})

	return skupperCLIPath, downloadError
}

// extractTarGz extracts a tar.gz file to the specified destination
func extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip directories and only extract the skupper binary
		if header.Typeflag == tar.TypeReg && (header.Name == "skupper" || strings.HasSuffix(header.Name, "/skupper")) {
			target := filepath.Join(dest, "skupper")

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			_, err = io.Copy(f, tr)
			f.Close()
			if err != nil {
				return err
			}
			break
		}
	}

	return nil
}

// InitializeSkupperCLI initializes and downloads the Skupper CLI once at the beginning
// This should be called early in your application to ensure the CLI is ready for use
func InitializeSkupperCLI() error {
	logger.Info("Initializing Skupper CLI")
	_, err := DownloadSkupperCLI()
	return err
}

// GetSkupperCLIPath returns the path to the Skupper CLI binary
// Returns empty string if CLI hasn't been downloaded yet
func GetSkupperCLIPath() string {
	return skupperCLIPath
}

// IsSkupperCLIReady checks if the Skupper CLI is downloaded and ready to use
func IsSkupperCLIReady() bool {
	return skupperCLIPath != "" && downloadError == nil
}

// DebugKubectlEnvironment runs kubectl get nodes to verify environment variables are working
func DebugKubectlEnvironment(kubeconfigPath string) error {
	logger.Info("DEBUG: Testing kubectl with environment variables")

	// Convert relative path to absolute path
	var absoluteKubeconfigPath string
	if kubeconfigPath != "" {
		absPath, err := filepath.Abs(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for kubeconfig: %v", err)
		}
		absoluteKubeconfigPath = absPath

		// Verify the kubeconfig file exists
		if _, err := os.Stat(absoluteKubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file does not exist: %s", absoluteKubeconfigPath)
		}
	}

	// Set environment
	env := os.Environ()
	if absoluteKubeconfigPath != "" {
		env = append(env, fmt.Sprintf("KUBECONFIG=%s", absoluteKubeconfigPath))
		logger.Info(fmt.Sprintf("DEBUG: Setting KUBECONFIG environment variable to: %s", absoluteKubeconfigPath))
	}

	// Log environment variables for debugging
	logger.Info("DEBUG: Environment variables being passed to kubectl:")
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "KUBECONFIG=") || strings.HasPrefix(envVar, "PATH=") {
			logger.Info(fmt.Sprintf("  %s", envVar))
		}
	}

	// Execute kubectl get nodes
	cmd := exec.Command("kubectl", "get", "nodes")
	cmd.Env = env

	logger.Info("DEBUG: Executing: kubectl get nodes")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Info(fmt.Sprintf("DEBUG: kubectl command failed with error: %v", err))
		logger.Info(fmt.Sprintf("DEBUG: kubectl output: %s", string(output)))
		return fmt.Errorf("kubectl get nodes failed: %v\nOutput: %s", err, string(output))
	}

	logger.Info(fmt.Sprintf("DEBUG: kubectl get nodes successful. Output:\n%s", string(output)))
	return nil
}

// ExposeStatefulset exposes a StatefulSet using the Skupper CLI
func ExposeStatefulset(statefulsetName, namespace, kubeconfigPath string) error {
	logger.Info(fmt.Sprintf("Exposing StatefulSet %s in namespace %s", statefulsetName, namespace))

	/*
		// Debug: Test kubectl with the same environment setup
		if err := DebugKubectlEnvironment(kubeconfigPath); err != nil {
			logger.Info(fmt.Sprintf("DEBUG: kubectl environment test failed: %v", err))
			// Continue anyway, but this indicates potential environment issues
		}

		// Verify environment setup first
		if err := VerifySkupperCLIEnvironment(kubeconfigPath); err != nil {
			logger.Info(fmt.Sprintf("Environment verification warning: %v", err))
			// Continue anyway, but log the warning
		}
	*/

	// Download Skupper CLI if not already available
	cliPath, err := DownloadSkupperCLI()
	if err != nil {
		return fmt.Errorf("failed to download Skupper CLI: %v", err)
	}

	// Prepare the command
	args := []string{
		"expose", "statefulset", statefulsetName,
		"-n", namespace,
		"--headless",
	}

	// Convert relative path to absolute path and set kubeconfig environment variable
	env := os.Environ()
	var absoluteKubeconfigPath string
	if kubeconfigPath != "" {
		absPath, err := filepath.Abs(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for kubeconfig: %v", err)
		}
		absoluteKubeconfigPath = absPath

		// Verify the kubeconfig file exists
		if _, err := os.Stat(absoluteKubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file does not exist: %s", absoluteKubeconfigPath)
		}

		env = append(env, fmt.Sprintf("KUBECONFIG=%s", absoluteKubeconfigPath))
		logger.Info(fmt.Sprintf("Setting KUBECONFIG environment variable to: %s", absoluteKubeconfigPath))
	}

	// Log environment variables for debugging (optional - can be removed in production)
	logger.Info("Environment variables being passed to Skupper CLI:")
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "KUBECONFIG=") || strings.HasPrefix(envVar, "PATH=") {
			logger.Info(fmt.Sprintf("  %s", envVar))
		}
	}

	// Execute the command
	cmd := exec.Command(cliPath, args...)
	cmd.Env = env

	// Set working directory to ensure consistent execution context
	cmd.Dir = filepath.Dir(cliPath)

	// Verify the command will use the expected environment
	logger.Info(fmt.Sprintf("Command working directory: %s", cmd.Dir))
	logger.Info(fmt.Sprintf("Executing: %s %s", cliPath, strings.Join(args, " ")))

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Enhanced error reporting
		logger.Info(fmt.Sprintf("Command failed with error: %v", err))
		logger.Info(fmt.Sprintf("Command output: %s", string(output)))

		// Check for specific environment-related errors
		if strings.Contains(string(output), "Unable to load config") ||
			strings.Contains(string(output), "couldn't get current server API version") {
			return fmt.Errorf("kubeconfig environment issue - failed to expose StatefulSet: %v\nOutput: %s", err, string(output))
		}

		return fmt.Errorf("failed to expose StatefulSet: %v\nOutput: %s", err, string(output))
	}

	logger.Info(fmt.Sprintf("StatefulSet exposed successfully. Output: %s", string(output)))
	return nil
}

// UnexposeStatefulset unexposes a StatefulSet using the Skupper CLI
func UnexposeStatefulset(statefulsetName, namespace, kubeconfigPath string) error {
	logger.Info(fmt.Sprintf("Unexposing StatefulSet %s in namespace %s", statefulsetName, namespace))

	// Download Skupper CLI if not already available
	cliPath, err := DownloadSkupperCLI()
	if err != nil {
		return fmt.Errorf("failed to download Skupper CLI: %v", err)
	}

	// Prepare the command
	args := []string{
		"unexpose", "statefulset", statefulsetName,
		"-n", namespace,
	}

	// Convert relative path to absolute path and set kubeconfig environment variable
	env := os.Environ()
	var absoluteKubeconfigPath string
	if kubeconfigPath != "" {
		absPath, err := filepath.Abs(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for kubeconfig: %v", err)
		}
		absoluteKubeconfigPath = absPath

		// Verify the kubeconfig file exists
		if _, err := os.Stat(absoluteKubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file does not exist: %s", absoluteKubeconfigPath)
		}

		env = append(env, fmt.Sprintf("KUBECONFIG=%s", absoluteKubeconfigPath))
		logger.Info(fmt.Sprintf("Setting KUBECONFIG environment variable to: %s", absoluteKubeconfigPath))
	}

	// Execute the command
	cmd := exec.Command(cliPath, args...)
	cmd.Env = env
	cmd.Dir = filepath.Dir(cliPath)

	logger.Info(fmt.Sprintf("Executing: %s %s", cliPath, strings.Join(args, " ")))

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Info(fmt.Sprintf("Command failed with error: %v", err))
		logger.Info(fmt.Sprintf("Command output: %s", string(output)))

		// Check for specific environment-related errors
		if strings.Contains(string(output), "Unable to load config") ||
			strings.Contains(string(output), "couldn't get current server API version") {
			return fmt.Errorf("kubeconfig environment issue - failed to unexpose StatefulSet: %v\nOutput: %s", err, string(output))
		}

		return fmt.Errorf("failed to unexpose StatefulSet: %v\nOutput: %s", err, string(output))
	}

	logger.Info(fmt.Sprintf("StatefulSet unexposed successfully. Output: %s", string(output)))
	return nil
}

// VerifySkupperCLIEnvironment verifies that the Skupper CLI can access the specified kubeconfig
func VerifySkupperCLIEnvironment(kubeconfigPath string) error {
	logger.Info("Verifying Skupper CLI environment setup")

	// Download Skupper CLI if not already available
	cliPath, err := DownloadSkupperCLI()
	if err != nil {
		return fmt.Errorf("failed to download Skupper CLI: %v", err)
	}

	// Convert relative path to absolute path
	var absoluteKubeconfigPath string
	if kubeconfigPath != "" {
		absPath, err := filepath.Abs(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for kubeconfig: %v", err)
		}
		absoluteKubeconfigPath = absPath
	}

	// Test command to verify environment
	args := []string{"version"}

	// Set environment
	env := os.Environ()
	if absoluteKubeconfigPath != "" {
		env = append(env, fmt.Sprintf("KUBECONFIG=%s", absoluteKubeconfigPath))
	}

	// Execute test command
	cmd := exec.Command(cliPath, args...)
	cmd.Env = env
	cmd.Dir = filepath.Dir(cliPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to verify Skupper CLI environment: %v\nOutput: %s", err, string(output))
	}

	logger.Info(fmt.Sprintf("Skupper CLI version check successful: %s", string(output)))

	// If kubeconfig is specified, also test cluster connectivity
	if absoluteKubeconfigPath != "" {
		args = []string{"status"}
		cmd = exec.Command(cliPath, args...)
		cmd.Env = env
		cmd.Dir = filepath.Dir(cliPath)

		output, err = cmd.CombinedOutput()
		// Note: status might fail if skupper isn't installed yet, but we just want to verify
		// that the CLI can attempt to connect to the cluster (not get permission denied, etc.)
		logger.Info(fmt.Sprintf("Skupper status check output: %s", string(output)))

		if err != nil {
			// Check if it's a connectivity issue vs environment issue
			if strings.Contains(string(output), "Unable to get") || strings.Contains(string(output), "connection refused") {
				return fmt.Errorf("kubeconfig environment not properly applied - cluster connectivity failed: %v", err)
			}
			// Other errors might be expected (e.g., skupper not installed yet)
			logger.Info(fmt.Sprintf("Status check returned error (may be expected): %v", err))
		}
	}

	return nil
}
