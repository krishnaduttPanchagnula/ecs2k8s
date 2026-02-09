package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// HelmChart represents a Helm chart structure
type HelmChart struct {
	Name      string
	ChartPath string
}

// HelmValues represents the values.yaml structure for the Helm chart
type HelmValues struct {
	Namespace string `yaml:"namespace"`
	Replicas  int    `yaml:"replicas"`
	Image     map[string]interface{} `yaml:"image"`
	Resources map[string]interface{} `yaml:"resources"`
	Service   map[string]interface{} `yaml:"service"`
	Containers []map[string]interface{} `yaml:"containers"`
	Environment map[string]string `yaml:"environment"`
}

// ChartYAML represents Chart.yaml for Helm
type ChartYAML struct {
	APIVersion  string `yaml:"apiVersion"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion"`
	Maintainers []map[string]string `yaml:"maintainers,omitempty"`
	Keywords    []string `yaml:"keywords,omitempty"`
}

// createHelmChart creates a Helm chart from the task definition
func createHelmChart(clusterName string, taskDefInfos []*TaskDefInfo, outputDir string) error {
	if !strings.Contains(outputDir, clusterName) {
		outputDir = filepath.Join(outputDir, clusterName)
	}

	helmChartPath := filepath.Join(outputDir, "helm", clusterName)

	// Create directory structure
	directories := []string{
		helmChartPath,
		filepath.Join(helmChartPath, "templates"),
		filepath.Join(helmChartPath, "templates", "deployment"),
		filepath.Join(helmChartPath, "templates", "service"),
		filepath.Join(helmChartPath, "templates", "configmap"),
		filepath.Join(helmChartPath, "templates", "secret"),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create helm chart directory %s: %w", dir, err)
		}
		log.Printf("Created Helm directory: %s", dir)
	}

	// Create Chart.yaml
	if err := createChartYAML(helmChartPath, clusterName); err != nil {
		return fmt.Errorf("failed to create Chart.yaml: %w", err)
	}

	// Create values.yaml for each task definition
	for _, taskDefInfo := range taskDefInfos {
		if err := createValuesYAML(helmChartPath, taskDefInfo); err != nil {
			return fmt.Errorf("failed to create values.yaml for %s: %w", taskDefInfo.Name, err)
		}
	}

	// Create Helm template files
	if err := createHelmTemplates(helmChartPath, taskDefInfos); err != nil {
		return fmt.Errorf("failed to create helm templates: %w", err)
	}

	log.Printf("✓ Created Helm chart at: %s", helmChartPath)
	return nil
}

// createChartYAML creates the Chart.yaml file
func createChartYAML(chartPath, clusterName string) error {
	chart := ChartYAML{
		APIVersion:  "v2",
		Name:        clusterName,
		Description: fmt.Sprintf("Helm chart for ECS cluster %s converted from AWS ECS to Kubernetes", clusterName),
		Type:        "application",
		Version:     "1.0.0",
		AppVersion:  "1.0.0",
		Maintainers: []map[string]string{
			{
				"name":  "ecs2k8s",
				"email": "auto-generated@ecs2k8s.local",
			},
		},
		Keywords: []string{"ecs", "kubernetes", "helm", "conversion"},
	}

	data, err := yaml.Marshal(chart)
	if err != nil {
		return fmt.Errorf("failed to marshal Chart.yaml: %w", err)
	}

	chartFile := filepath.Join(chartPath, "Chart.yaml")
	if err := os.WriteFile(chartFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write Chart.yaml: %w", err)
	}

	log.Printf("Created Chart.yaml at: %s", chartFile)
	return nil
}

// createValuesYAML creates the values.yaml file for a task definition
func createValuesYAML(chartPath string, taskDefInfo *TaskDefInfo) error {
	values := map[string]interface{}{
		"namespace": "default",
		"replicas":  1,
	}

	// Build container configurations
	var containers []map[string]interface{}

	for _, container := range taskDefInfo.Containers {
		containerConfig := map[string]interface{}{
			"name":  container.Name,
			"image": container.Image,
			"resources": map[string]interface{}{
				"limits": map[string]interface{}{
					"cpu":    container.CPU,
					"memory": container.Memory,
				},
				"requests": map[string]interface{}{
					"cpu":    container.CPU,
					"memory": container.Memory,
				},
			},
		}

		if len(container.Ports) > 0 {
			containerConfig["ports"] = container.Ports
		}

		if len(container.EnvVars) > 0 {
			envList := []map[string]string{}
			for key, value := range container.EnvVars {
				envList = append(envList, map[string]string{
					"name":  key,
					"value": value,
				})
			}
			containerConfig["env"] = envList
		}

		containers = append(containers, containerConfig)
	}

	values["containers"] = containers

	// Add service configuration if there are services
	if len(taskDefInfo.Manifests.Services) > 0 {
		svc := taskDefInfo.Manifests.Services[0]
		serviceConfig := map[string]interface{}{
			"name": svc.Name,
			"type": string(svc.Spec.Type),
		}

		if len(svc.Spec.Ports) > 0 {
			serviceConfig["port"] = svc.Spec.Ports[0].Port
		}

		values["service"] = serviceConfig
	}

	// Serialize to YAML with comments
	data, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to marshal values.yaml: %w", err)
	}

	// Add header comments
	header := `# Helm Chart Values for ` + taskDefInfo.Name + `
# Generated by ecs2k8s
#
# To customize the deployment, modify the values below:
# - namespace: Kubernetes namespace for deployment
# - replicas: Number of pod replicas
# - containers: Container configurations with image, resources, and environment variables
# - service: Service configuration for exposing the application
#
# Example usage:
#   helm install ` + taskDefInfo.Name + ` ./` + taskDefInfo.Name + ` -f values.yaml
#   helm upgrade ` + taskDefInfo.Name + ` ./` + taskDefInfo.Name + ` -f values.yaml

`

	fullContent := header + string(data)

	valuesFile := filepath.Join(chartPath, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte(fullContent), 0o644); err != nil {
		return fmt.Errorf("failed to write values.yaml: %w", err)
	}

	log.Printf("Created values.yaml at: %s", valuesFile)
	return nil
}

// CreateHelmChart is a wrapper for createHelmChart with reordered parameters
func CreateHelmChart(clusterName string, taskDefInfos []*TaskDefInfo, outputDir string) error {
	return createHelmChart(clusterName, taskDefInfos, outputDir)
}

// createHelmTemplates creates the Helm template files
func createHelmTemplates(chartPath string, taskDefInfos []*TaskDefInfo) error {
	// Create deployment template
	deploymentTemplate := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "` + filepath.Base(chartPath) + `.fullname" . }}
  labels:
    {{- include "` + filepath.Base(chartPath) + `.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels:
      {{- include "` + filepath.Base(chartPath) + `.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "` + filepath.Base(chartPath) + `.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      {{- range .Values.containers }}
      - name: {{ .name }}
        image: {{ .image }}
        imagePullPolicy: IfNotPresent
        {{- if .ports }}
        ports:
        {{- range .ports }}
        - containerPort: {{ . }}
          protocol: TCP
        {{- end }}
        {{- end }}
        {{- if .env }}
        env:
        {{- range $key, $value := .env }}
        - name: {{ $key }}
          value: "{{ $value }}"
        {{- end }}
        {{- end }}
        {{- if .resources }}
        resources:
          {{- if .resources.limits }}
          limits:
            cpu: {{ .resources.limits.cpu }}
            memory: {{ .resources.limits.memory }}
          {{- end }}
          {{- if .resources.requests }}
          requests:
            cpu: {{ .resources.requests.cpu }}
            memory: {{ .resources.requests.memory }}
          {{- end }}
        {{- end }}
      {{- end }}
`

	deploymentFile := filepath.Join(chartPath, "templates", "deployment", "deployment.yaml")
	if err := os.WriteFile(deploymentFile, []byte(deploymentTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write deployment template: %w", err)
	}

	log.Printf("Created deployment template at: %s", deploymentFile)

	// Create service template
	serviceTemplate := `{{- if .Values.service }}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "` + filepath.Base(chartPath) + `.fullname" . }}
  labels:
    {{- include "` + filepath.Base(chartPath) + `.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type | default "ClusterIP" }}
  ports:
  {{- range .Values.containers }}
    {{- if .ports }}
    {{- range .ports }}
    - port: {{ . }}
      targetPort: {{ . }}
      protocol: TCP
    {{- end }}
    {{- end }}
  {{- end }}
  selector:
    {{- include "` + filepath.Base(chartPath) + `.selectorLabels" . | nindent 4 }}
{{- end }}
`

	serviceFile := filepath.Join(chartPath, "templates", "service", "service.yaml")
	if err := os.WriteFile(serviceFile, []byte(serviceTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write service template: %w", err)
	}

	log.Printf("Created service template at: %s", serviceFile)

	// Create ConfigMap template
	configmapTemplate := `{{- if .Values.environment }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "` + filepath.Base(chartPath) + `.fullname" . }}-config
  labels:
    {{- include "` + filepath.Base(chartPath) + `.labels" . | nindent 4 }}
data:
  {{- range $key, $value := .Values.environment }}
  {{ $key }}: "{{ $value }}"
  {{- end }}
{{- end }}
`

	configmapFile := filepath.Join(chartPath, "templates", "configmap", "configmap.yaml")
	if err := os.WriteFile(configmapFile, []byte(configmapTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write configmap template: %w", err)
	}

	log.Printf("Created configmap template at: %s", configmapFile)

	// Create helpers template
	helpersTemplate := `{{/*
Expand the name of the chart.
*/}}
{{- define "` + filepath.Base(chartPath) + `.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "` + filepath.Base(chartPath) + `.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "` + filepath.Base(chartPath) + `.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "` + filepath.Base(chartPath) + `.labels" -}}
helm.sh/chart: {{ include "` + filepath.Base(chartPath) + `.chart" . }}
{{ include "` + filepath.Base(chartPath) + `.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "` + filepath.Base(chartPath) + `.selectorLabels" -}}
app.kubernetes.io/name: {{ include "` + filepath.Base(chartPath) + `.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
`

	helpersFile := filepath.Join(chartPath, "templates", "_helpers.tpl")
	if err := os.WriteFile(helpersFile, []byte(helpersTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write helpers template: %w", err)
	}

	log.Printf("Created helpers template at: %s", helpersFile)

	return nil
}

// createDefaultHelmValues creates a default values.yaml with all available options
func createDefaultHelmValues(chartPath string) error {
	defaultValues := map[string]interface{}{
		"namespace":   "default",
		"replicas":    1,
		"environment": make(map[string]string),
		"service": map[string]interface{}{
			"type": "ClusterIP",
		},
	}

	data, err := yaml.Marshal(defaultValues)
	if err != nil {
		return fmt.Errorf("failed to marshal default values: %w", err)
	}

	header := `# Default values for the Helm chart
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

# Number of replicas
replicas: 1

# Kubernetes namespace
namespace: default

# Service configuration
service:
  # Service type: ClusterIP, NodePort, LoadBalancer
  type: ClusterIP

# Environment variables (non-sensitive)
environment: {}

# Image pull policy
imagePullPolicy: IfNotPresent

# Pod annotations
podAnnotations: {}

# Node selector
nodeSelector: {}

# Tolerations
tolerations: []

# Affinity
affinity: {}
`

	fullContent := header + "\n" + string(data)

	valuesFile := filepath.Join(chartPath, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte(fullContent), 0o644); err != nil {
		return fmt.Errorf("failed to write default values.yaml: %w", err)
	}

	log.Printf("Created default values.yaml at: %s", valuesFile)
	return nil
}
