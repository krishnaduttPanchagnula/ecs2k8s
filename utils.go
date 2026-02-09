package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

func extractClusterName(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return arn
}

func extractTaskDefName(arn string) string {
	re := regexp.MustCompile(`task-definition/([^:/]+)`)
	matches := re.FindStringSubmatch(arn)
	if len(matches) > 1 {
		return matches[1]
	}
	return strings.TrimPrefix(arn, "arn:aws:ecs:")
}

func writeManifests(outputDir, taskDefName string, manifests K8sManifests) error {
	files := map[string]interface{}{}

	fmt.Printf("[DEBUG] writeManifests called for task: %s\n", taskDefName)

	// Deployment
	if manifests.Deployment != nil {
		fmt.Printf("[DEBUG] Adding deployment manifest\n")
		deployment := map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      taskDefName,
				"namespace": "default",
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
					"spec": manifests.Deployment,
				},
			},
		}
		files[fmt.Sprintf("%s-deployment.yaml", taskDefName)] = deployment
	} else {
		fmt.Printf("[DEBUG] Deployment is nil!\n")
	}

	// ConfigMap
	if manifests.ConfigMap != nil {
		fmt.Printf("[DEBUG] Adding configmap manifest\n")
		files[fmt.Sprintf("%s-configmap.yaml", taskDefName)] = manifests.ConfigMap
	}

	// Service
	if manifests.Service != nil {
		fmt.Printf("[DEBUG] Adding service manifest\n")
		files[fmt.Sprintf("%s-service.yaml", taskDefName)] = manifests.Service
	}

	fmt.Printf("[DEBUG] Total files to write: %d\n", len(files))

	// Write files
	for filename, content := range files {
		fmt.Printf("[DEBUG] Writing file: %s\n", filename)
		data, err := yaml.Marshal(content)
		if err != nil {
			return err
		}

		filePath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(filePath, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("[DEBUG] Successfully wrote: %s\n", filePath)
	}

	return nil
}
