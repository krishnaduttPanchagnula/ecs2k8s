package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

func extractClusterName(arn string) string {
	if arn == "" {
		log.Printf("Warning: Cluster ARN is empty")
		return ""
	}

	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if name == "" {
			log.Printf("Warning: Extracted empty cluster name from ARN: %s", arn)
			return ""
		}
		return name
	}
	return ""
}

func extractTaskDefName(arn string) string {
	if arn == "" {
		log.Printf("Warning: Task definition ARN is empty")
		return ""
	}

	// Try to match standard ARN format: arn:aws:ecs:region:account:task-definition/name:version
	re := regexp.MustCompile(`task-definition/([^:/]+)`)
	matches := re.FindStringSubmatch(arn)
	if len(matches) > 1 && matches[1] != "" {
		return matches[1]
	}

	// Fallback: extract from end and remove version suffix
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		namePart := parts[len(parts)-1]
		// Remove version suffix (e.g., ":1")
		name := strings.Split(namePart, ":")[0]
		if name != "" {
			return name
		}
	}

	log.Printf("Warning: Could not extract task definition name from ARN: %s", arn)
	return ""
}

func isValidFilename(name string) bool {
	if name == "" {
		return false
	}

	// Check for invalid filename characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return false
		}
	}

	return true
}

// serializePodSpec converts a PodSpec to a map suitable for YAML marshaling
func serializePodSpec(podSpec *corev1.PodSpec) map[string]interface{} {
	result := map[string]interface{}{}

	if podSpec == nil {
		return result
	}

	if len(podSpec.Containers) > 0 {
		var containersList []map[string]interface{}
		for _, container := range podSpec.Containers {
			containerMap := map[string]interface{}{
				"name":  container.Name,
				"image": container.Image,
			}

			// Add ports if present
			if len(container.Ports) > 0 {
				var portsList []map[string]interface{}
				for _, port := range container.Ports {
					portMap := map[string]interface{}{
						"containerPort": port.ContainerPort,
					}
					if port.Protocol != "" {
						portMap["protocol"] = string(port.Protocol)
					}
					if port.Name != "" {
						portMap["name"] = port.Name
					}
					portsList = append(portsList, portMap)
				}
				containerMap["ports"] = portsList
			}

			// Add environment variables if present
			if len(container.Env) > 0 {
				var envList []map[string]interface{}
				for _, env := range container.Env {
					envMap := map[string]interface{}{
						"name": env.Name,
					}
					if env.Value != "" {
						envMap["value"] = env.Value
					}
					envList = append(envList, envMap)
				}
				containerMap["env"] = envList
			}

			// Add resources with proper string formatting
			if len(container.Resources.Limits) > 0 || len(container.Resources.Requests) > 0 {
				resourcesMap := map[string]interface{}{}

				// Add limits
				if len(container.Resources.Limits) > 0 {
					limitsMap := make(map[string]string)
					for k, v := range container.Resources.Limits {
						limitsMap[string(k)] = v.String()
					}
					resourcesMap["limits"] = limitsMap
				}

				// Add requests
				if len(container.Resources.Requests) > 0 {
					requestsMap := make(map[string]string)
					for k, v := range container.Resources.Requests {
						requestsMap[string(k)] = v.String()
					}
					resourcesMap["requests"] = requestsMap
				}

				containerMap["resources"] = resourcesMap
			}

			containersList = append(containersList, containerMap)
		}
		result["containers"] = containersList
	}

	// Add init containers if present
	if len(podSpec.InitContainers) > 0 {
		var initContainersList []map[string]interface{}
		for _, container := range podSpec.InitContainers {
			containerMap := map[string]interface{}{
				"name":  container.Name,
				"image": container.Image,
			}
			initContainersList = append(initContainersList, containerMap)
		}
		result["initContainers"] = initContainersList
	}

	// Add restart policy if specified
	if podSpec.RestartPolicy != "" {
		result["restartPolicy"] = string(podSpec.RestartPolicy)
	}

	// Add service account name if specified
	if podSpec.ServiceAccountName != "" {
		result["serviceAccountName"] = podSpec.ServiceAccountName
	}

	return result
}

// serializeServiceAccount converts a ServiceAccount to a map suitable for YAML marshaling
func serializeServiceAccount(sa *corev1.ServiceAccount) map[string]interface{} {
	result := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
	}

	if sa == nil {
		return result
	}

	// Add metadata
	metadata := map[string]interface{}{
		"name": sa.Name,
	}

	if sa.Namespace != "" {
		metadata["namespace"] = sa.Namespace
	}

	// Add annotations (including IRSA annotations)
	if len(sa.Annotations) > 0 {
		metadata["annotations"] = sa.Annotations
	}

	// Add labels if present
	if len(sa.Labels) > 0 {
		metadata["labels"] = sa.Labels
	}

	result["metadata"] = metadata

	// Add ImagePullSecrets if present
	if len(sa.ImagePullSecrets) > 0 {
		var imagePullSecrets []map[string]string
		for _, ips := range sa.ImagePullSecrets {
			imagePullSecrets = append(imagePullSecrets, map[string]string{
				"name": ips.Name,
			})
		}
		result["imagePullSecrets"] = imagePullSecrets
	}

	// Add AutomountServiceAccountToken if explicitly set
	if sa.AutomountServiceAccountToken != nil {
		result["automountServiceAccountToken"] = *sa.AutomountServiceAccountToken
	}

	return result
}

// serializeConfigMap converts a ConfigMap to a clean map for YAML marshaling
func serializeConfigMap(cm *corev1.ConfigMap) map[string]interface{} {
	result := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": cm.Name,
		},
	}
	if cm.Namespace != "" {
		result["metadata"].(map[string]interface{})["namespace"] = cm.Namespace
	}
	if len(cm.Labels) > 0 {
		result["metadata"].(map[string]interface{})["labels"] = cm.Labels
	}
	if len(cm.Data) > 0 {
		result["data"] = cm.Data
	}
	return result
}

// serializeSecret converts a Secret to a clean map for YAML marshaling
func serializeSecret(secret *corev1.Secret) map[string]interface{} {
	result := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name": secret.Name,
		},
		"type": string(secret.Type),
	}
	if secret.Namespace != "" {
		result["metadata"].(map[string]interface{})["namespace"] = secret.Namespace
	}
	if len(secret.Labels) > 0 {
		result["metadata"].(map[string]interface{})["labels"] = secret.Labels
	}
	if len(secret.StringData) > 0 {
		result["stringData"] = secret.StringData
	}
	if len(secret.Data) > 0 {
		result["data"] = secret.Data
	}
	return result
}

// serializeService converts a Service to a clean map for YAML marshaling
func serializeService(svc *corev1.Service) map[string]interface{} {
	result := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name": svc.Name,
		},
	}
	if svc.Namespace != "" {
		result["metadata"].(map[string]interface{})["namespace"] = svc.Namespace
	}
	if len(svc.Labels) > 0 {
		result["metadata"].(map[string]interface{})["labels"] = svc.Labels
	}

	spec := map[string]interface{}{
		"type":     string(svc.Spec.Type),
		"selector": svc.Spec.Selector,
	}

	if len(svc.Spec.Ports) > 0 {
		var ports []map[string]interface{}
		for _, p := range svc.Spec.Ports {
			portMap := map[string]interface{}{
				"port":       p.Port,
				"targetPort": p.TargetPort.IntValue(),
				"protocol":   string(p.Protocol),
			}
			if p.Name != "" {
				portMap["name"] = p.Name
			}
			ports = append(ports, portMap)
		}
		spec["ports"] = ports
	}

	result["spec"] = spec
	return result
}

func writeManifests(outputDir, taskDefName string, manifests K8sManifests) error {
	if outputDir == "" {
		return fmt.Errorf("output directory path cannot be empty")
	}

	info, err := os.Stat(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory %s does not exist: %w", outputDir, err)
		}
		return fmt.Errorf("failed to stat output directory %s: %w", outputDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("output path %s is not a directory", outputDir)
	}

	if taskDefName == "" {
		return fmt.Errorf("task definition name cannot be empty")
	}

	if !isValidFilename(taskDefName) {
		return fmt.Errorf("invalid task definition name for filename: %s (contains invalid characters)", taskDefName)
	}

	files := map[string]interface{}{}

	// Deployment
	if manifests.Deployment != nil {
		deployment := map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      taskDefName,
				"namespace": "default",
				"labels": map[string]string{
					"app": taskDefName,
				},
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]string{
						"app": taskDefName,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"app": taskDefName,
						},
					},
					"spec": serializePodSpec(manifests.Deployment),
				},
			},
		}
		files[fmt.Sprintf("%s-deployment.yaml", taskDefName)] = deployment
	}

	// ConfigMaps
	for i, cm := range manifests.ConfigMaps {
		if cm == nil {
			continue
		}
		cmMap := serializeConfigMap(cm)
		if len(manifests.ConfigMaps) == 1 {
			files[fmt.Sprintf("%s-configmap.yaml", taskDefName)] = cmMap
		} else {
			files[fmt.Sprintf("%s-configmap-%d.yaml", taskDefName, i)] = cmMap
		}
	}

	// Services
	for _, svc := range manifests.Services {
		if svc == nil {
			continue
		}
		svcMap := serializeService(svc)
		if len(manifests.Services) == 1 {
			files[fmt.Sprintf("%s-service.yaml", taskDefName)] = svcMap
		} else {
			files[fmt.Sprintf("%s-service-%s.yaml", taskDefName, svc.Name)] = svcMap
		}
	}

	// Secrets
	for i, secret := range manifests.Secrets {
		if secret == nil {
			continue
		}
		secretMap := serializeSecret(secret)
		if len(manifests.Secrets) == 1 {
			files[fmt.Sprintf("%s-secret.yaml", taskDefName)] = secretMap
		} else {
			files[fmt.Sprintf("%s-secret-%d.yaml", taskDefName, i)] = secretMap
		}
	}

	// ServiceAccount
	if manifests.ServiceAccount != nil {
		saManifest := serializeServiceAccount(manifests.ServiceAccount)
		files[fmt.Sprintf("%s-serviceaccount.yaml", taskDefName)] = saManifest
	}

	// Write files
	for filename, content := range files {
		if !isValidFilename(filename) {
			return fmt.Errorf("constructed filename %s contains invalid characters", filename)
		}

		data, err := yaml.Marshal(content)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML for %s: %w", filename, err)
		}

		filePath := filepath.Join(outputDir, filename)

		// Prevent directory traversal
		absFilePath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}

		absOutputDir, err := filepath.Abs(outputDir)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for output dir: %w", err)
		}

		if !strings.HasPrefix(absFilePath, absOutputDir) {
			return fmt.Errorf("file path %s is outside output directory", filePath)
		}

		if err := os.WriteFile(filePath, data, 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filePath, err)
		}

		log.Printf("Wrote: %s", filePath)
	}

	return nil
}
