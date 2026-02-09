// utils.go
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

	// Deployment
	if manifests.Deployment != nil {
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
	}

	// ConfigMap
	if manifests.ConfigMap != nil {
		files[fmt.Sprintf("%s-configmap.yaml", taskDefName)] = manifests.ConfigMap
	}

	// Service
	if manifests.Service != nil {
		files[fmt.Sprintf("%s-service.yaml", taskDefName)] = manifests.Service
	}

	// Write files
	for filename, content := range files {
		data, err := yaml.Marshal(content)
		if err != nil {
			return err
		}

		filePath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

func awsInt64(i int64) *int64 { return &i }
func awsInt32(i int32) *int32 { return &i }
