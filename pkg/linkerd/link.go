// This code is copied from "https://github.com/linkerd/linkerd2" repository
// and has been modified for our specific use case.

package linkerd

import (
	"bytes"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"io"
	v1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"path"
	"regexp"

	"strconv"
	"time"

	"github.com/linkerd/linkerd2/pkg/charts"
	"github.com/linkerd/linkerd2/pkg/k8s"
	chartloader "helm.sh/helm/v3/pkg/chart/loader"
	valuespkg "helm.sh/helm/v3/pkg/cli/values"
)

func LinkCluster(fromCluster kube.Cluster, toCluster kube.Cluster, fromClusterName string) {
	opts, err := newLinkOptionsWithDefault()
	opts.clusterName = fromClusterName
	ip, err := fromCluster.FetchKubernetesAPIEndpoint()
	exit.OnErrorWithMessage(err, "Error fetching Kubernetes API endpoint")
	opts.apiServerAddress = "https://" + ip

	configMapInterface, err := fromCluster.FetchResource(kube.ConfigMap, "linkerd-config", "linkerd")
	exit.OnErrorWithMessage(err, "You need Linkerd to be installed on a cluster in order to get its credentials")
	configMap := configMapInterface.(*v1.ConfigMap)
	configMapValues := getConfigValues(configMap)

	kubeconfig := createKubeconfig(fromCluster, opts)
	createSecrets(toCluster, configMapValues, opts, kubeconfig)
	createLink(fromCluster, toCluster, opts)

	values, err := buildServiceMirrorValues(opts)
	exit.OnErrorWithMessage(err, "Error building service mirror values")

	// Create values overrides
	serviceInterface, err := fromCluster.FetchResource(kube.Service, "linkerd-gateway", "linkerd-multicluster")
	if err != nil {
		exit.OnErrorWithMessage(err, "Error fetching linkerd-gateway service")
	}
	linkerdGateway := serviceInterface.(*v1.Service)

	gatewayIP := linkerdGateway.Status.LoadBalancer.Ingress[0].IP
	valuesOptions, tempFileName, err := getValuesOverrides(fromClusterName, gatewayIP)
	defer os.Remove(tempFileName) // Remove the file after it is no longer needed
	exit.OnErrorWithMessage(err, "Error getting valueOptions")

	valuesOverrides, err := valuesOptions.MergeValues(nil)
	exit.OnErrorWithMessage(err, "Error getting values overrides")

	serviceMirrorOut, err := renderServiceMirror(values, valuesOverrides, opts.namespace, opts.output)
	exit.OnErrorWithMessage(err, "Error rendering service mirror")

	err = toCluster.CreateResourcesFromYaml(serviceMirrorOut, opts.namespace)
	exit.OnErrorWithMessage(err, "Error creating resources")
}

func renderServiceMirror(values *Values, valuesOverrides map[string]interface{}, namespace string, format string) ([]byte, error) {
	files := []*chartloader.BufferedFile{
		{Name: chartutil.ChartfileName},
		{Name: "templates/service-mirror.yaml"},
		{Name: "templates/psp.yaml"},
		{Name: "templates/gateway-mirror.yaml"},
	}

	var partialFiles []*chartloader.BufferedFile
	for _, template := range charts.L5dPartials {
		partialFiles = append(partialFiles,
			&chartloader.BufferedFile{Name: template},
		)
	}

	var templates http.FileSystem = http.Dir("pkg/linkerd/charts")
	var partialTemplates http.FileSystem = http.Dir("pkg/linkerd/partialCharts")

	// Load all multicluster link chart files into buffer
	if err := charts.FilesReader(templates, "linkerd-multicluster-link"+"/", files); err != nil {
		return nil, err
	}

	// Load all partial chart files into buffer
	if err := charts.FilesReader(partialTemplates, "", partialFiles); err != nil {
		return nil, err
	}

	// Create a Chart obj from the files
	chart, err := chartloader.LoadFiles(append(files, partialFiles...))
	if err != nil {
		return nil, err
	}

	// Render raw values and create chart config
	rawValues, err := yaml.Marshal(values)
	if err != nil {
		return nil, err
	}

	// Store final Values generated from values.yaml and CLI flags
	err = yaml.Unmarshal(rawValues, &chart.Values)
	if err != nil {
		return nil, err
	}

	// Merge the values with the overrides
	vals, err := chartutil.CoalesceValues(chart, valuesOverrides)
	if err != nil {
		return nil, err
	}

	// Debug output to inspect final values
	fmt.Printf("Final merged values: %v\n", vals)

	fullValues := map[string]interface{}{
		"Values": vals,
		"Release": map[string]interface{}{
			"Namespace": namespace,
			"Service":   "CLI",
		},
	}

	// Attach the final values into the `Values` field for rendering to work
	renderedTemplates, err := engine.Render(chart, fullValues)
	if err != nil {
		return nil, err
	}

	// Merge templates and inject
	var yamlBytes bytes.Buffer
	for _, tmpl := range chart.Templates {
		t := path.Join(chart.Metadata.Name, tmpl.Name)
		if _, err := yamlBytes.WriteString(renderedTemplates[t]); err != nil {
			return nil, err
		}
	}

	var out bytes.Buffer
	err = RenderYAMLAs(&yamlBytes, &out, format)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func buildServiceMirrorValues(opts *linkOptions) (*Values, error) {
	alphaNumDashDot := regexp.MustCompile(`^[\.a-zA-Z0-9-]+$`)

	if !alphaNumDashDot.MatchString(opts.controlPlaneVersion) {
		return nil, fmt.Errorf("%s is not a valid version", opts.controlPlaneVersion)
	}

	if opts.namespace == "" {
		return nil, errors.New("you need to specify a namespace")
	}

	if opts.selector != "" && opts.selector != fmt.Sprintf("%s=%s", k8s.DefaultExportedServiceSelector, "true") {
		if !opts.enableGateway {
			return nil, fmt.Errorf("--selector and --gateway=false are mutually exclusive")
		}
	}

	if opts.gatewayAddresses != "" && !opts.enableGateway {
		return nil, fmt.Errorf("--gateway-addresses and --gateway=false are mutually exclusive")
	}

	if opts.gatewayPort != 0 && !opts.enableGateway {
		return nil, fmt.Errorf("--gateway-port and --gateway=false are mutually exclusive")
	}

	defaults := NewLinkValues()
	defaults.TargetClusterName = opts.clusterName

	return defaults, nil
}

func extractGatewayPort(gateway *v1.Service) (uint32, error) {
	for _, port := range gateway.Spec.Ports {
		if port.Name == k8s.GatewayPortName {
			if gateway.Spec.Type == "NodePort" {
				return uint32(port.NodePort), nil
			}
			return uint32(port.Port), nil
		}
	}
	return 0, fmt.Errorf("gateway service %s has no gateway port named %s", gateway.Name, k8s.GatewayPortName)
}

func extractSAToken(secrets []v1.Secret, saName string) (string, error) {
	for _, secret := range secrets {
		boundSA := secret.Annotations["kubernetes.io/service-account.name"]
		if saName == boundSA {
			token, ok := secret.Data["token"]
			if !ok {
				return "", fmt.Errorf("could not find the token data in service account secret %s", secret.Name)
			}

			return string(token), nil
		}
	}

	return "", fmt.Errorf("could not find service account token secret for %s", saName)
}

// ExtractProbeSpec parses the ProbSpec from a gateway service's annotations.
// For now we're not including the failureThreshold and timeout fields which
// are new since edge-24.9.3, to avoid errors when attempting to apply them in
// clusters with an older Link CRD.
func extractProbeSpec(gateway *v1.Service) (ProbeSpec, error) {
	path := gateway.Annotations[k8s.GatewayProbePath]
	if path == "" {
		return ProbeSpec{}, errors.New("probe path is empty")
	}

	port, err := extractPort(gateway.Spec, k8s.ProbePortName)
	if err != nil {
		return ProbeSpec{}, err
	}

	// the `mirror.linkerd.io/probe-period` annotation is initialized with a
	// default value of "3", but we require a duration-formatted string. So we
	// perform the conversion, if required.
	period := gateway.Annotations[k8s.GatewayProbePeriod]
	if secs, err := strconv.ParseInt(period, 10, 64); err == nil {
		dur := time.Duration(secs) * time.Second
		period = dur.String()
	} else if _, err := time.ParseDuration(period); err != nil {
		return ProbeSpec{}, fmt.Errorf("could not parse probe period: %w", err)
	}

	return ProbeSpec{
		Path:   path,
		Port:   fmt.Sprintf("%d", port),
		Period: period,
	}, nil
}

func extractPort(spec v1.ServiceSpec, portName string) (uint32, error) {
	for _, p := range spec.Ports {
		if p.Name == portName {
			if spec.Type == "NodePort" {
				return uint32(p.NodePort), nil
			}
			return uint32(p.Port), nil
		}
	}
	return 0, fmt.Errorf("could not find port with name %s", portName)
}

func getConfigValues(configMap *v1.ConfigMap) LinkerdConfig {

	rawValues := configMap.Data["values"]

	// Convert into latest values, where global field is removed.
	rawValuesBytes, err := removeGlobalFieldIfPresent([]byte(rawValues))
	exit.OnErrorWithMessage(err, "Error removing global field from values")

	var config LinkerdConfig

	// Unmarshal the YAML data into the Config struct
	err = yaml.Unmarshal(rawValuesBytes, &config)
	exit.OnErrorWithMessage(err, "Error unmarshalling values")

	return config
}

func removeGlobalFieldIfPresent(bytes []byte) ([]byte, error) {
	// Check if Globals is present and remove that node if it has
	var valuesMap map[string]interface{}
	err := yaml.Unmarshal(bytes, &valuesMap)
	if err != nil {
		return nil, err
	}

	if globalValues, ok := valuesMap["global"]; ok {
		// attach those values
		// Check if its a map
		if val, ok := globalValues.(map[string]interface{}); ok {
			for k, v := range val {
				valuesMap[k] = v
			}
		}
		// Remove global now
		delete(valuesMap, "global")
	}

	bytes, err = yaml.Marshal(valuesMap)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func RenderYAMLAs(buf *bytes.Buffer, writer io.Writer, format string) error {

	_, err := writer.Write(buf.Bytes())
	return err
}

func getValuesOverrides(fromClusterName, gatewayIP string) (valuespkg.Options, string, error) {
	// Create YAML string with your values
	values := fmt.Sprintf(`
targetClusterName: %s
targetClusterDomain: cluster.local
gatewayAddress: %s:4143
gatewayIdentity: gateway.linkerd.cluster.local
probeSpec:
  path: /probe
  port: 4191
  period: 60s`, fromClusterName, gatewayIP)

	// Write YAML to a temporary file
	tempFile, err := os.CreateTemp("", "values-*.yaml")
	if err != nil {
		return valuespkg.Options{}, "", err
	}

	if _, err := tempFile.Write([]byte(values)); err != nil {
		return valuespkg.Options{}, "", err
	}
	if err := tempFile.Close(); err != nil {
		return valuespkg.Options{}, "", err
	}

	// Convert file path into values.Options
	options := valuespkg.Options{
		ValueFiles:   []string{tempFile.Name()},
		StringValues: []string{},
		Values:       []string{},
		FileValues:   []string{},
	}

	return options, tempFile.Name(), nil
}
