package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
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

func writeManifests(outputDir, taskDefName string, manifests K8sManifests) error {
	// Validate directory path
	if outputDir == "" {
		return fmt.Errorf("output directory path cannot be empty")
	}

	// Check directory exists and is a directory
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

	// Check directory is writable by attempting to write a test file
	testFile := filepath.Join(outputDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		return fmt.Errorf("output directory %s is not writable: %w", outputDir, err)
	}
	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		log.Printf("Warning: Failed to remove test file %s: %v", testFile, err)
	}

	// Validate task definition name for filename safety
	if taskDefName == "" {
		return fmt.Errorf("task definition name cannot be empty")
	}

	if !isValidFilename(taskDefName) {
		return fmt.Errorf("invalid task definition name for filename: %s (contains invalid characters)", taskDefName)
	}

	files := map[string]interface{}{}

	log.Printf("[DEBUG] writeManifests called for task: %s", taskDefName)

	// Deployment
	if manifests.Deployment != nil {
		log.Printf("[DEBUG] Adding deployment manifest")
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
		log.Printf("[DEBUG] Deployment is nil!")
	}

	// ConfigMap
	if manifests.ConfigMap != nil {
		log.Printf("[DEBUG] Adding configmap manifest")
		files[fmt.Sprintf("%s-configmap.yaml", taskDefName)] = manifests.ConfigMap
	}

	// Service
	if manifests.Service != nil {
		log.Printf("[DEBUG] Adding service manifest")
		files[fmt.Sprintf("%s-service.yaml", taskDefName)] = manifests.Service
	}

	log.Printf("[DEBUG] Total files to write: %d", len(files))

	// Write files
	for filename, content := range files {
		log.Printf("[DEBUG] Writing file: %s", filename)

		// Additional validation of constructed filename
		if !isValidFilename(filename) {
			return fmt.Errorf("constructed filename %s contains invalid characters", filename)
		}

		data, err := yaml.Marshal(content)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML for %s: %w", filename, err)
		}

		filePath := filepath.Join(outputDir, filename)

		// Prevent directory traversal attacks
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

		log.Printf("[DEBUG] Successfully wrote: %s", filePath)
	}

	return nil
}
